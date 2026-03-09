package crosscheck

import (
	"fmt"
	"strings"

	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CheckPolicy validates policy against SSaC, DDL, and States.
func CheckPolicy(policies []*policy.Policy, funcs []ssacparser.ServiceFunc, st *ssacvalidator.SymbolTable, diagrams []*statemachine.StateDiagram) []CrossError {
	var errs []CrossError

	// Merge all policies into one combined view.
	allPairs := make(map[[2]string]bool)
	ownerResources := make(map[string]bool)
	var allOwnerships []policy.OwnershipMapping

	for _, p := range policies {
		for _, pair := range p.ActionResourcePairs() {
			allPairs[pair] = true
		}
		for _, res := range p.ResourcesUsingOwner() {
			ownerResources[res] = true
		}
		allOwnerships = append(allOwnerships, p.Ownerships...)
	}

	// Build SSaC authorize (action, resource) pairs.
	ssacPairs := make(map[[2]string]bool)
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Type == "authorize" {
				ssacPairs[[2]string{seq.Action, seq.Resource}] = true
			}
		}
	}

	// --- Policy ↔ SSaC ---

	// 1. SSaC authorize pair → Rego allow rule exists.
	for pair := range ssacPairs {
		if !allPairs[pair] {
			errs = append(errs, CrossError{
				Rule:       "Policy ↔ SSaC",
				Context:    fmt.Sprintf("action=%s resource=%s", pair[0], pair[1]),
				Message:    fmt.Sprintf("SSaC authorize (%s, %s) has no matching Rego allow rule", pair[0], pair[1]),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("Add allow rule for action=%q resource=%q in policy/*.rego", pair[0], pair[1]),
			})
		}
	}

	// 2. Rego allow rule → SSaC authorize pair exists.
	for pair := range allPairs {
		if !ssacPairs[pair] {
			errs = append(errs, CrossError{
				Rule:       "Policy ↔ SSaC",
				Context:    fmt.Sprintf("action=%s resource=%s", pair[0], pair[1]),
				Message:    fmt.Sprintf("Rego allow rule (%s, %s) has no matching SSaC authorize sequence", pair[0], pair[1]),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("Add @sequence authorize with @action %s @resource %s to SSaC", pair[0], pair[1]),
			})
		}
	}

	// 3. Rego uses input.resource_owner → @ownership annotation exists.
	ownershipMap := make(map[string]bool)
	for _, om := range allOwnerships {
		ownershipMap[om.Resource] = true
	}
	for res := range ownerResources {
		if !ownershipMap[res] {
			errs = append(errs, CrossError{
				Rule:       "Policy ↔ SSaC",
				Context:    fmt.Sprintf("resource=%s", res),
				Message:    fmt.Sprintf("Rego references input.resource_owner for %q but no @ownership annotation found", res),
				Level:      "ERROR",
				Suggestion: fmt.Sprintf("Add # @ownership %s: <table>.<column> to policy/*.rego", res),
			})
		}
	}

	// --- Policy ↔ DDL ---
	if st != nil {
		for _, om := range allOwnerships {
			// Check table.column exists.
			tbl, ok := st.DDLTables[om.Table]
			if !ok {
				errs = append(errs, CrossError{
					Rule:       "Policy ↔ DDL",
					Context:    fmt.Sprintf("@ownership %s", om.Resource),
					Message:    fmt.Sprintf("ownership table %q does not exist in DDL", om.Table),
					Level:      "ERROR",
					Suggestion: fmt.Sprintf("Create table %s in DDL or fix @ownership annotation", om.Table),
				})
				continue
			}
			if _, colOk := tbl.Columns[om.Column]; !colOk {
				errs = append(errs, CrossError{
					Rule:       "Policy ↔ DDL",
					Context:    fmt.Sprintf("@ownership %s", om.Resource),
					Message:    fmt.Sprintf("ownership column %s.%s does not exist in DDL", om.Table, om.Column),
					Level:      "ERROR",
					Suggestion: fmt.Sprintf("Add column %s to table %s in DDL", om.Column, om.Table),
				})
			}

			// Check via join table if present.
			if om.JoinTable != "" {
				joinTbl, ok := st.DDLTables[om.JoinTable]
				if !ok {
					errs = append(errs, CrossError{
						Rule:       "Policy ↔ DDL",
						Context:    fmt.Sprintf("@ownership %s via", om.Resource),
						Message:    fmt.Sprintf("join table %q does not exist in DDL", om.JoinTable),
						Level:      "ERROR",
						Suggestion: fmt.Sprintf("Create table %s in DDL or fix @ownership via annotation", om.JoinTable),
					})
				} else if _, colOk := joinTbl.Columns[om.JoinFK]; !colOk {
					errs = append(errs, CrossError{
						Rule:       "Policy ↔ DDL",
						Context:    fmt.Sprintf("@ownership %s via", om.Resource),
						Message:    fmt.Sprintf("join column %s.%s does not exist in DDL", om.JoinTable, om.JoinFK),
						Level:      "ERROR",
						Suggestion: fmt.Sprintf("Add column %s to table %s in DDL", om.JoinFK, om.JoinTable),
					})
				}
			}
		}
	}

	// --- Policy ↔ States ---
	if len(diagrams) > 0 && funcs != nil {
		// Build map: transition event → SSaC authorize action (if exists).
		eventToAction := make(map[string]string)
		for _, fn := range funcs {
			for _, seq := range fn.Sequences {
				if seq.Type == "authorize" {
					eventToAction[fn.Name] = seq.Action
				}
			}
		}

		for _, d := range diagrams {
			for _, event := range d.Events() {
				action, hasAuthorize := eventToAction[event]
				if !hasAuthorize {
					continue // States ↔ SSaC crosscheck handles this
				}
				// Determine resource from diagram ID.
				resource := strings.ToLower(d.ID)
				pair := [2]string{action, resource}
				if !allPairs[pair] {
					errs = append(errs, CrossError{
						Rule:       "Policy ↔ States",
						Context:    fmt.Sprintf("%s.%s", d.ID, event),
						Message:    fmt.Sprintf("state transition %q has authorize (action=%s) but no matching Rego allow rule", event, action),
						Level:      "WARNING",
						Suggestion: fmt.Sprintf("Add allow rule for action=%q resource=%q in policy/*.rego", action, resource),
					})
				}
			}
		}
	}

	return errs
}
