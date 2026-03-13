package crosscheck

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/scenario"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/ssac/parser"
)

// CheckScenarios validates Scenario ↔ OpenAPI, Scenario ↔ States, and internal consistency.
func CheckScenarios(
	features []*scenario.Feature,
	doc *openapi3.T,
	diagrams []*statemachine.StateDiagram,
	policies []*policy.Policy,
	funcs []ssacparser.ServiceFunc,
) []CrossError {
	var errs []CrossError

	if doc == nil || len(features) == 0 {
		return nil
	}

	// Build operationId → (method, path) lookup.
	opMap := buildOpMap(doc)

	// Build operationId → allowed roles lookup (via SSaC auth action → OPA role).
	opRoles := buildOpRoleMap(funcs, policies)

	for _, f := range features {
		allScenarios := f.Scenarios
		if f.Background != nil {
			allScenarios = append([]scenario.Scenario{*f.Background}, allScenarios...)
		}

		for _, sc := range allScenarios {
			// Collect all steps (background + scenario).
			var steps []scenario.Step
			if f.Background != nil {
				steps = append(steps, f.Background.Steps...)
			}
			steps = append(steps, sc.Steps...)

			for i, step := range steps {
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

				// Rule 6: status code must be defined in OpenAPI responses.
				errs = append(errs, checkStatusCode(f.File, steps, i, info.op, step.OperationID)...)
			}

			// Rule 4: capture reference validity.
			errs = append(errs, checkCaptureRefs(f.File, sc.Name, steps)...)

			// Rule 5: token role matching.
			if len(opRoles) > 0 {
				errs = append(errs, checkTokenRoles(f.File, sc.Name, steps, opRoles)...)
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

// --- Rule 4: Capture reference validity ---

// reVarDotted matches dotted variable references like "varName.field.sub".
// Captures the full dotted expression.
var reVarDotted = regexp.MustCompile(`\b([a-zA-Z_]\w*(?:\.[a-zA-Z_]\w*)+)`)

// checkCaptureRefs verifies that variables referenced in JSON bodies have been captured earlier.
func checkCaptureRefs(file, scenarioName string, steps []scenario.Step) []CrossError {
	var errs []CrossError
	captured := make(map[string]bool)

	for _, step := range steps {
		if step.IsAction {
			// Check JSON references against captured variables.
			if step.JSON != "" {
				refs := extractVarRefs(step.JSON)
				for _, ref := range refs {
					if !captured[ref] {
						errs = append(errs, CrossError{
							Rule:       "Scenario (capture)",
							Context:    fmt.Sprintf("%s: %s → %s", file, scenarioName, step.OperationID),
							Message:    fmt.Sprintf("variable %q referenced in JSON but not captured in a previous step", ref),
							Level:      "ERROR",
							Suggestion: fmt.Sprintf("Add -> %s capture to a prior step, or fix the variable name", ref),
						})
					}
				}
			}
			// Record capture.
			if step.Capture != "" {
				captured[step.Capture] = true
			}
		}
	}
	return errs
}

// extractVarRefs extracts the root variable name from dotted references in JSON values.
// e.g. {"id": gigResult.gig.id, "bid": 900} → ["gigResult"]
func extractVarRefs(json string) []string {
	seen := make(map[string]bool)
	json = strings.TrimSpace(json)
	json = strings.TrimPrefix(json, "{")
	json = strings.TrimSuffix(json, "}")

	parts := strings.Split(json, ",")
	for _, part := range parts {
		colonIdx := strings.Index(part, ":")
		if colonIdx < 0 {
			continue
		}
		val := strings.TrimSpace(part[colonIdx+1:])
		// Skip quoted string values.
		if strings.HasPrefix(val, `"`) {
			continue
		}
		// Find dotted references and extract root name.
		matches := reVarDotted.FindAllStringSubmatch(val, -1)
		for _, m := range matches {
			root := strings.SplitN(m[1], ".", 2)[0]
			if root == "true" || root == "false" || root == "null" {
				continue
			}
			seen[root] = true
		}
	}

	var refs []string
	for name := range seen {
		refs = append(refs, name)
	}
	return refs
}

// --- Rule 5: Token role matching ---

// buildOpRoleMap builds operationId → allowed roles by chaining:
// SSaC operationId → auth action → OPA policy role.
func buildOpRoleMap(funcs []ssacparser.ServiceFunc, policies []*policy.Policy) map[string][]string {
	if len(funcs) == 0 || len(policies) == 0 {
		return nil
	}

	// OPA action → roles.
	actionRoles := make(map[string][]string)
	for _, p := range policies {
		for _, rule := range p.Rules {
			if rule.UsesRole && rule.RoleValue != "" {
				for _, action := range rule.Actions {
					actionRoles[action] = append(actionRoles[action], rule.RoleValue)
				}
			}
		}
	}

	// SSaC operationId → auth actions → roles.
	result := make(map[string][]string)
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Type == ssacparser.SeqAuth && seq.Action != "" {
				if roles, ok := actionRoles[seq.Action]; ok {
					result[fn.Name] = append(result[fn.Name], roles...)
				}
			}
		}
	}
	return result
}

// checkTokenRoles verifies that the current token's role matches the operation's allowed roles.
func checkTokenRoles(file, scenarioName string, steps []scenario.Step, opRoles map[string][]string) []CrossError {
	var errs []CrossError

	// Track: email → role (from Register steps), capture name → role (from Login steps).
	emailRoles := make(map[string]string)  // email → role
	tokenRoles := make(map[string]string)  // capture name → role
	var currentToken string                // last captured token name

	for i, step := range steps {
		if !step.IsAction {
			continue
		}

		switch step.OperationID {
		case "Register":
			// Extract role and email from JSON.
			role := extractJSONValue(step.JSON, "role")
			email := extractJSONValue(step.JSON, "email")
			if role != "" && email != "" {
				emailRoles[email] = role
			}

		case "Login":
			// If this Login captures a token, look up role by email.
			if step.Capture != "" {
				email := extractJSONValue(step.JSON, "email")
				if role, ok := emailRoles[email]; ok {
					tokenRoles[step.Capture] = role
				}
				currentToken = step.Capture
			}

		default:
			// Check if operation requires specific roles.
			allowedRoles, ok := opRoles[step.OperationID]
			if !ok || currentToken == "" {
				continue
			}

			// Skip if this is an intentional rejection test.
			if expectsClientError(steps, i) {
				continue
			}

			// Use explicit token from step if specified, otherwise currentToken.
			activeToken := currentToken
			if step.Token != "" {
				activeToken = step.Token
			}
			tokenRole := tokenRoles[activeToken]
			if tokenRole == "" {
				continue
			}

			roleAllowed := false
			for _, r := range allowedRoles {
				if r == tokenRole {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				errs = append(errs, CrossError{
					Rule:    "Scenario ↔ Policy",
					Context: fmt.Sprintf("%s: %s → %s (token=%s, role=%s)", file, scenarioName, step.OperationID, activeToken, tokenRole),
					Message: fmt.Sprintf("token %q has role %q but %s requires one of %v",
						activeToken, tokenRole, step.OperationID, allowedRoles),
					Level:      "WARNING",
					Suggestion: fmt.Sprintf("Use a token with role %v or add role %q to policy for %s", allowedRoles, tokenRole, step.OperationID),
				})
			}
		}
	}
	return errs
}

// extractJSONValue extracts a simple string value for a key from JSON-like text.
func extractJSONValue(json, key string) string {
	json = strings.TrimSpace(json)
	json = strings.TrimPrefix(json, "{")
	json = strings.TrimSuffix(json, "}")

	parts := strings.Split(json, ",")
	for _, part := range parts {
		colonIdx := strings.Index(part, ":")
		if colonIdx <= 0 {
			continue
		}
		k := strings.TrimSpace(part[:colonIdx])
		k = strings.Trim(k, `"`)
		if k == key {
			v := strings.TrimSpace(part[colonIdx+1:])
			v = strings.Trim(v, `"`)
			return v
		}
	}
	return ""
}

// --- Rule 6: Status code validity ---

// checkStatusCode verifies that assertion status codes are defined in OpenAPI responses.
func checkStatusCode(file string, steps []scenario.Step, actionIdx int, op *openapi3.Operation, opID string) []CrossError {
	var errs []CrossError
	if op == nil || op.Responses == nil {
		return nil
	}

	// Find status assertions following this action step.
	for j := actionIdx + 1; j < len(steps); j++ {
		s := steps[j]
		if s.IsAction {
			break
		}
		if s.Assertion.Kind != "status" {
			continue
		}

		code := s.Assertion.Value
		if code == "" {
			continue
		}

		// Check if this status code is defined in OpenAPI responses.
		found := false
		for respCode := range op.Responses.Map() {
			if respCode == code {
				found = true
				break
			}
		}

		if !found {
			errs = append(errs, CrossError{
				Rule:       "Scenario ↔ OpenAPI",
				Context:    fmt.Sprintf("%s: %s status %s", file, opID, code),
				Message:    fmt.Sprintf("status code %s not defined in OpenAPI responses for %s", code, opID),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("Add %q response to OpenAPI for %s, or verify the expected status", code, opID),
			})
		}
	}
	return errs
}
