package crosscheck

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/scenario"
	"github.com/geul-org/fullend/internal/statemachine"
)

// CheckScenarios validates Scenario ↔ OpenAPI and Scenario ↔ States.
func CheckScenarios(features []*scenario.Feature, doc *openapi3.T, diagrams []*statemachine.StateDiagram) []CrossError {
	var errs []CrossError

	if doc == nil || len(features) == 0 {
		return nil
	}

	// Build operationId → (method, path) lookup.
	opMap := buildOpMap(doc)

	for _, f := range features {
		allScenarios := f.Scenarios
		if f.Background != nil {
			allScenarios = append([]scenario.Scenario{*f.Background}, allScenarios...)
		}

		for _, sc := range allScenarios {
			for _, step := range sc.Steps {
				if !step.IsAction {
					continue
				}

				// Rule 1: operationId must exist in OpenAPI.
				info, ok := opMap[step.OperationID]
				if !ok {
					errs = append(errs, CrossError{
						Rule:       "Scenario ↔ OpenAPI",
						Context:    fmt.Sprintf("%s: %s %s", f.File, step.Method, step.OperationID),
						Message:    fmt.Sprintf("operationId %q not found in OpenAPI", step.OperationID),
						Level:      "ERROR",
						Suggestion: fmt.Sprintf("Add operationId %q to OpenAPI spec", step.OperationID),
					})
					continue
				}

				// Rule 2: HTTP method must match.
				if !strings.EqualFold(info.method, step.Method) {
					errs = append(errs, CrossError{
						Rule:       "Scenario ↔ OpenAPI",
						Context:    fmt.Sprintf("%s: %s %s", f.File, step.Method, step.OperationID),
						Message:    fmt.Sprintf("method mismatch: scenario uses %s but OpenAPI defines %s", step.Method, info.method),
						Level:      "ERROR",
						Suggestion: fmt.Sprintf("Change method to %s in scenario or update OpenAPI", info.method),
					})
				}

				// Rule 3: JSON fields must exist in request schema.
				if step.JSON != "" {
					errs = append(errs, checkJSONFields(f.File, step, info.op)...)
				}

				// Rule 5: security check — token capture should precede auth endpoints.
				// (This is a WARNING-level heuristic checked at feature level, not here.)
			}
		}
	}

	// Scenario ↔ States: state transition order.
	if len(diagrams) > 0 {
		errs = append(errs, checkScenarioStates(features, diagrams, opMap)...)
	}

	return errs
}

type opInfo struct {
	method string
	path   string
	op     *openapi3.Operation
}

func buildOpMap(doc *openapi3.T) map[string]opInfo {
	m := make(map[string]opInfo)
	if doc.Paths == nil {
		return m
	}
	for path, pi := range doc.Paths.Map() {
		for method, op := range pi.Operations() {
			if op != nil && op.OperationID != "" {
				m[op.OperationID] = opInfo{method: method, path: path, op: op}
			}
		}
	}
	return m
}

// checkJSONFields validates that JSON body field names exist in request schema.
func checkJSONFields(file string, step scenario.Step, op *openapi3.Operation) []CrossError {
	var errs []CrossError
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}

	ct := op.RequestBody.Value.Content.Get("application/json")
	if ct == nil || ct.Schema == nil || ct.Schema.Value == nil {
		return nil
	}
	schema := ct.Schema.Value

	// Extract field names from JSON (simplified: look for quoted keys).
	fields := extractJSONKeys(step.JSON)
	for _, field := range fields {
		if _, ok := schema.Properties[field]; !ok {
			// Check if it's a path parameter (not in body schema).
			if isPathParam(op, field) {
				continue
			}
			errs = append(errs, CrossError{
				Rule:       "Scenario ↔ OpenAPI",
				Context:    fmt.Sprintf("%s: %s %s", file, step.Method, step.OperationID),
				Message:    fmt.Sprintf("field %q not found in request schema for %s", field, step.OperationID),
				Level:      "ERROR",
				Suggestion: fmt.Sprintf("Add field %q to the request schema or remove from scenario", field),
			})
		}
	}
	return errs
}

// extractJSONKeys extracts field names from a JSON-like string.
// Handles both "Key": and unquoted Key: patterns.
func extractJSONKeys(json string) []string {
	var keys []string
	// Simple regex-free approach: split by comma, find keys before ':'
	json = strings.TrimSpace(json)
	json = strings.TrimPrefix(json, "{")
	json = strings.TrimSuffix(json, "}")

	parts := strings.Split(json, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		colonIdx := strings.Index(part, ":")
		if colonIdx <= 0 {
			continue
		}
		key := strings.TrimSpace(part[:colonIdx])
		key = strings.Trim(key, `"`)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func isPathParam(op *openapi3.Operation, name string) bool {
	for _, p := range op.Parameters {
		if p.Value != nil && p.Value.In == "path" && p.Value.Name == name {
			return true
		}
	}
	return false
}

// checkScenarioStates checks that scenario steps follow state transition order.
func checkScenarioStates(features []*scenario.Feature, diagrams []*statemachine.StateDiagram, opMap map[string]opInfo) []CrossError {
	var errs []CrossError

	// Build event → diagrams lookup (1:N — one event can appear in multiple diagrams).
	eventDiagrams := make(map[string][]*statemachine.StateDiagram)
	for _, d := range diagrams {
		for _, ev := range d.Events() {
			eventDiagrams[ev] = append(eventDiagrams[ev], d)
		}
	}

	for _, f := range features {
		for _, sc := range f.Scenarios {
			// Collect all action operationIDs in order.
			var steps []scenario.Step
			if f.Background != nil {
				steps = append(steps, f.Background.Steps...)
			}
			steps = append(steps, sc.Steps...)

			// Track current state per diagram.
			currentState := make(map[string]string)
			for _, d := range diagrams {
				currentState[d.ID] = d.InitialState
			}

			for i, step := range steps {
				if !step.IsAction {
					continue
				}

				// Skip state transition check if the next assertion expects a 4xx status
				// (intentional rejection test, e.g. @invariant scenarios).
				if expectsClientError(steps, i) {
					continue
				}

				ds, ok := eventDiagrams[step.OperationID]
				if !ok {
					continue
				}

				for _, d := range ds {
					state := currentState[d.ID]
					validFroms := d.ValidFromStates(step.OperationID)
					isValid := false
					for _, vs := range validFroms {
						if vs == state {
							isValid = true
							break
						}
					}

					if !isValid && state != "" {
						errs = append(errs, CrossError{
							Rule:    "Scenario ↔ States",
							Context: fmt.Sprintf("%s: %s → %s (current state: %s)", f.File, sc.Name, step.OperationID, state),
							Message: fmt.Sprintf("event %q is not valid from state %q in %s diagram", step.OperationID, state, d.ID),
							Level:   "WARNING",
							Suggestion: fmt.Sprintf("Ensure scenario follows %s state transitions: valid from states %v",
								d.ID, validFroms),
						})
					}

					// Advance state.
					for _, tr := range d.Transitions {
						if tr.Event == step.OperationID && tr.From == state {
							currentState[d.ID] = tr.To
							break
						}
					}
				}
			}
		}
	}

	return errs
}

// expectsClientError checks if the steps following index i contain a status == 4xx
// assertion before the next action step. This indicates an intentional rejection test.
func expectsClientError(steps []scenario.Step, i int) bool {
	for j := i + 1; j < len(steps); j++ {
		s := steps[j]
		if s.IsAction {
			return false // next action reached without 4xx assertion
		}
		if s.Assertion.Kind == "status" && len(s.Assertion.Value) == 3 && s.Assertion.Value[0] == '4' {
			return true
		}
	}
	return false
}
