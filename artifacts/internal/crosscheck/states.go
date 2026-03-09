package crosscheck

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/artifacts/internal/statemachine"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CheckStates validates state diagrams against SSaC, DDL, and OpenAPI.
func CheckStates(diagrams []*statemachine.StateDiagram, funcs []ssacparser.ServiceFunc, st *ssacvalidator.SymbolTable, doc *openapi3.T) []CrossError {
	var errs []CrossError

	if len(diagrams) == 0 {
		return errs
	}

	// Build lookup maps.
	diagramByID := make(map[string]*statemachine.StateDiagram)
	for _, d := range diagrams {
		diagramByID[d.ID] = d
	}

	funcNames := make(map[string]bool)
	for _, fn := range funcs {
		funcNames[fn.Name] = true
	}

	opIDs := make(map[string]bool)
	if doc != nil && doc.Paths != nil {
		for _, pi := range doc.Paths.Map() {
			for _, op := range pi.Operations() {
				if op != nil && op.OperationID != "" {
					opIDs[op.OperationID] = true
				}
			}
		}
	}

	// 1. Transition events → SSaC function exists.
	for _, d := range diagrams {
		for _, event := range d.Events() {
			if !funcNames[event] {
				errs = append(errs, CrossError{
					Rule:       "States ↔ SSaC",
					Context:    fmt.Sprintf("%s.%s", d.ID, event),
					Message:    fmt.Sprintf("transition event %q has no matching SSaC function", event),
					Level:      "ERROR",
					Suggestion: fmt.Sprintf("Add SSaC function %s or remove transition from states/%s.md", event, d.ID),
				})
			}
		}
	}

	// 2. SSaC guard state → diagram exists.
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Type != "guard state" {
				continue
			}
			diagramID := seq.Target
			if _, ok := diagramByID[diagramID]; !ok {
				errs = append(errs, CrossError{
					Rule:       "States ↔ SSaC",
					Context:    fn.Name,
					Message:    fmt.Sprintf("guard state references diagram %q which does not exist", diagramID),
					Level:      "ERROR",
					Suggestion: fmt.Sprintf("Create states/%s.md with a Mermaid stateDiagram", diagramID),
				})
				continue
			}

			// Check that the function name is a valid event in the diagram.
			d := diagramByID[diagramID]
			validStates := d.ValidFromStates(fn.Name)
			if len(validStates) == 0 {
				errs = append(errs, CrossError{
					Rule:       "States ↔ SSaC",
					Context:    fn.Name,
					Message:    fmt.Sprintf("function %q is not a valid transition event in diagram %q", fn.Name, diagramID),
					Level:      "ERROR",
					Suggestion: fmt.Sprintf("Add transition to states/%s.md: someState --> targetState: %s", diagramID, fn.Name),
				})
			}
		}
	}

	// 3. Diagram with transitions for an operationId but no guard state → warning.
	guardStateFuncs := make(map[string]bool)
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Type == "guard state" {
				guardStateFuncs[fn.Name] = true
			}
		}
	}
	for _, d := range diagrams {
		for _, event := range d.Events() {
			if funcNames[event] && !guardStateFuncs[event] {
				errs = append(errs, CrossError{
					Rule:       "States ↔ SSaC",
					Context:    event,
					Message:    fmt.Sprintf("function %q has a state transition in %s but no guard state sequence", event, d.ID),
					Level:      "WARNING",
					Suggestion: fmt.Sprintf("Add @sequence guard state %s to %s", d.ID, event),
				})
			}
		}
	}

	// 4. Transition events → OpenAPI operationId exists.
	if doc != nil {
		for _, d := range diagrams {
			for _, event := range d.Events() {
				if !opIDs[event] {
					errs = append(errs, CrossError{
						Rule:       "States ↔ OpenAPI",
						Context:    fmt.Sprintf("%s.%s", d.ID, event),
						Message:    fmt.Sprintf("transition event %q has no matching OpenAPI operationId", event),
						Level:      "ERROR",
						Suggestion: fmt.Sprintf("Add operationId: %s to OpenAPI spec", event),
					})
				}
			}
		}
	}

	// 5. guard state @param StatusField → DDL column exists.
	if st != nil {
		for _, fn := range funcs {
			for _, seq := range fn.Sequences {
				if seq.Type != "guard state" {
					continue
				}
				diagramID := seq.Target
				d, ok := diagramByID[diagramID]
				if !ok {
					continue // already reported above
				}

				// Find the @param entity.Field.
				if len(seq.Params) == 0 {
					continue
				}
				param := seq.Params[0]
				// SSaC stores "course.Published" in param.Name.
				// Split by dot to get entity and field.
				paramParts := strings.SplitN(param.Name, ".", 2)
				statusField := ""
				if len(paramParts) == 2 {
					statusField = paramParts[1]
				}
				tableName := diagramIDToTable(diagramID)
				colName := pascalToSnakeState(statusField)

				found := false
				if tbl, ok := st.DDLTables[tableName]; ok {
					if _, colOk := tbl.Columns[colName]; colOk {
						found = true
					}
				}
				if !found {
					errs = append(errs, CrossError{
						Rule:       "States ↔ DDL",
						Context:    fn.Name,
						Message:    fmt.Sprintf("state field %q maps to column %s.%s which does not exist", statusField, tableName, colName),
						Level:      "ERROR",
						Suggestion: fmt.Sprintf("Add column %s to table %s in DDL", colName, tableName),
					})
				}

				// 6. Initial state ↔ DDL DEFAULT value (warning only).
				if d.InitialState != "" {
					// This is informational — exact DEFAULT matching is complex.
					// We just warn that users should verify.
					_ = d.InitialState // acknowledged, no automated check yet
				}
			}
		}
	}

	return errs
}

// diagramIDToTable converts a diagram ID to a DDL table name.
// "course" → "courses"
func diagramIDToTable(id string) string {
	// Simple pluralization.
	if len(id) == 0 {
		return id
	}
	last := id[len(id)-1]
	switch {
	case last == 'y':
		return id[:len(id)-1] + "ies"
	case last == 's' || last == 'x':
		return id + "es"
	default:
		return id + "s"
	}
}

// pascalToSnakeState converts PascalCase to snake_case.
func pascalToSnakeState(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				prev := s[i-1]
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					result = append(result, '_')
				}
			}
			result = append(result, byte(c-'A'+'a'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
