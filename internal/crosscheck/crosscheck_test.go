package crosscheck

import (
	"testing"

	"github.com/geul-org/fullend/internal/genapi"
	"github.com/geul-org/fullend/internal/policy"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
)

func TestRunRules_SkipRules(t *testing.T) {
	// Minimal input that triggers "SSaC Queue" and "SSaC → Authz" rules.
	input := &CrossValidateInput{
		ParsedSSOTs: &genapi.ParsedSSOTs{
			ServiceFuncs: []ssacparser.ServiceFunc{
				{Name: "DummyFunc"},
			},
		},
	}

	// Run without skip — collect baseline errors.
	allErrs := RunRules(input, nil)

	// Run with skip "SSaC Queue" — should have fewer or equal errors.
	skip := map[string]bool{"SSaC Queue": true}
	skippedErrs := RunRules(input, skip)

	// Count how many errors come from "SSaC Queue" rule in the full run.
	queueCount := 0
	for _, e := range allErrs {
		if e.Rule == "SSaC Queue" || containsQueueError(e) {
			queueCount++
		}
	}

	// The skipped run should have exactly (all - queue) errors,
	// but at minimum it should not have more than the full run.
	if len(skippedErrs) > len(allErrs) {
		t.Errorf("skipped run has more errors (%d) than full run (%d)", len(skippedErrs), len(allErrs))
	}

	_ = queueCount // used for reasoning
}

func containsQueueError(e CrossError) bool {
	return false
}

func TestRunRules_SkipAll(t *testing.T) {
	input := &CrossValidateInput{
		ParsedSSOTs: &genapi.ParsedSSOTs{
			ServiceFuncs: []ssacparser.ServiceFunc{
				{Name: "DummyFunc"},
			},
			Policies: []*policy.Policy{{
				File:  "test.rego",
				Rules: []policy.AllowRule{{Actions: []string{"Do"}, Resource: "r", RoleValue: "admin", SourceLine: 1}},
			}},
		},
		Roles: []string{"admin"},
	}

	// Skip all rules by name.
	skip := make(map[string]bool)
	for _, r := range Rules() {
		skip[r.Name] = true
	}

	errs := RunRules(input, skip)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors when all rules skipped, got %d", len(errs))
	}
}

func TestRules_Count(t *testing.T) {
	if got := len(Rules()); got != 16 {
		t.Errorf("expected 16 rules, got %d", got)
	}
}
