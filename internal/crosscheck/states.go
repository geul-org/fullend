package crosscheck

import (
	"fmt"
	"strings"

	"github.com/ettle/strcase"
	"github.com/jinzhu/inflection"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
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
			if seq.Type != "state" {
				continue
			}
			diagramID := seq.DiagramID
			if _, ok := diagramByID[diagramID]; !ok {
				errs = append(errs, CrossError{
					Rule:       "States ↔ SSaC",
					Context:    fn.Name,
					Message:    fmt.Sprintf("@state references diagram %q which does not exist", diagramID),
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
			if seq.Type == "state" {
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
					Message:    fmt.Sprintf("function %q has a state transition in %s but no @state sequence", event, d.ID),
					Level:      "WARNING",
					Suggestion: fmt.Sprintf("Add @state %s sequence to %s", d.ID, event),
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

	// 5. @state Inputs field → DDL column exists.
	if st != nil {
		for _, fn := range funcs {
			for _, seq := range fn.Sequences {
				if seq.Type != "state" {
					continue
				}
				diagramID := seq.DiagramID
				d, ok := diagramByID[diagramID]
				if !ok {
					continue // already reported above
				}

				// Extract status field from Inputs map.
				// v2: @state reservation {status: reservation.Status} "cancel" "msg"
				// Inputs = {"status": "reservation.Status"}
				if len(seq.Inputs) == 0 {
					continue
				}
				statusField := ""
				for _, v := range seq.Inputs {
					parts := strings.SplitN(v, ".", 2)
					if len(parts) == 2 {
						statusField = parts[1]
					}
					break // use first input
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
	return inflection.Plural(id)
}

// pascalToSnakeState converts PascalCase to snake_case.
func pascalToSnakeState(s string) string {
	return strcase.ToSnake(s)
}
