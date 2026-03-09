package orchestrator

import (
	"fmt"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/artifacts/internal/crosscheck"
	"github.com/geul-org/fullend/artifacts/internal/policy"
	"github.com/geul-org/fullend/artifacts/internal/reporter"
	"github.com/geul-org/fullend/artifacts/internal/statemachine"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
	stmlparser "github.com/geul-org/stml/parser"
	stmlvalidator "github.com/geul-org/stml/validator"
)

// allKinds defines the display order of SSOT kinds for validation.
var allKinds = []SSOTKind{KindOpenAPI, KindDDL, KindSSaC, KindModel, KindSTML, KindStates, KindPolicy, KindTerraform}

// Validate runs individual SSOT validations on the detected sources,
// then runs cross-validation if OpenAPI + DDL + SSaC are all present.
// skipKinds specifies SSOT kinds to explicitly skip (via --skip flag).
func Validate(root string, detected []DetectedSSOT, skipKinds ...map[SSOTKind]bool) *reporter.Report {
	report := &reporter.Report{}

	skip := make(map[SSOTKind]bool)
	if len(skipKinds) > 0 && skipKinds[0] != nil {
		skip = skipKinds[0]
	}

	has := make(map[SSOTKind]DetectedSSOT)
	for _, d := range detected {
		has[d.Kind] = d
	}

	// Intermediate results for cross-validation.
	var openAPIDoc *openapi3.T
	var symTable *ssacvalidator.SymbolTable
	var serviceFuncs []ssacparser.ServiceFunc
	var stateDiagrams []*statemachine.StateDiagram
	var parsedPolicies []*policy.Policy

	done := make(map[SSOTKind]bool)

	// Emit steps in fixed order.
	for _, kind := range allKinds {
		if done[kind] {
			continue
		}

		d, ok := has[kind]
		if !ok {
			if skip[kind] {
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Skip,
					Summary: "skipped (--skip)",
				})
			} else {
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Fail,
					Summary: "required but not found",
				})
			}
			continue
		}

		switch kind {
		case KindOpenAPI:
			step, doc := validateOpenAPI(d.Path)
			report.Steps = append(report.Steps, step)
			openAPIDoc = doc
		case KindDDL:
			step, st := validateDDL(root)
			report.Steps = append(report.Steps, step)
			symTable = st
			// Run SSaC right after DDL to reuse symbol table.
			if ssacD, ok := has[KindSSaC]; ok {
				step, funcs := validateSSaC(root, ssacD.Path, st)
				report.Steps = append(report.Steps, step)
				serviceFuncs = funcs
				done[KindSSaC] = true
			}
		case KindSSaC:
			step, funcs := validateSSaC(root, d.Path, nil)
			report.Steps = append(report.Steps, step)
			serviceFuncs = funcs
		case KindSTML:
			report.Steps = append(report.Steps, validateSTML(root, d.Path))
		case KindStates:
			step, diagrams := validateStates(d.Path)
			report.Steps = append(report.Steps, step)
			stateDiagrams = diagrams
		case KindPolicy:
			step, policies := validatePolicy(d.Path)
			report.Steps = append(report.Steps, step)
			parsedPolicies = policies
		case KindModel:
			report.Steps = append(report.Steps, validateModel(d.Path))
		case KindTerraform:
			report.Steps = append(report.Steps, validateTerraform(d.Path))
		}
	}

	// Cross-validation step.
	report.Steps = append(report.Steps, runCrossValidate(openAPIDoc, symTable, serviceFuncs, stateDiagrams, parsedPolicies))

	return report
}

func runCrossValidate(doc *openapi3.T, st *ssacvalidator.SymbolTable, funcs []ssacparser.ServiceFunc, diagrams []*statemachine.StateDiagram, policies []*policy.Policy) reporter.StepResult {
	step := reporter.StepResult{Name: "Cross"}

	// Require OpenAPI + DDL + SSaC for cross-validation.
	if doc == nil || st == nil || funcs == nil {
		step.Status = reporter.Skip
		step.Summary = "skipped (incomplete SSOT)"
		return step
	}

	input := &crosscheck.CrossValidateInput{
		OpenAPIDoc:    doc,
		SymbolTable:   st,
		ServiceFuncs:  funcs,
		StateDiagrams: diagrams,
		Policies:      policies,
	}

	cerrs := crosscheck.Run(input)

	hasError := false
	for _, ce := range cerrs {
		prefix := ce.Rule
		if ce.Level == "WARNING" {
			prefix = "[WARN] " + prefix
		} else {
			hasError = true
		}
		step.Errors = append(step.Errors, fmt.Sprintf("%s: %s — %s", prefix, ce.Context, ce.Message))
		step.Suggestions = append(step.Suggestions, ce.Suggestion)
	}

	if hasError {
		step.Status = reporter.Fail
	} else {
		step.Status = reporter.Pass
	}

	errCount := 0
	warnCount := 0
	for _, ce := range cerrs {
		if ce.Level == "WARNING" {
			warnCount++
		} else {
			errCount++
		}
	}
	if errCount > 0 {
		step.Summary = fmt.Sprintf("%d errors, %d warnings", errCount, warnCount)
	} else if warnCount > 0 {
		step.Summary = fmt.Sprintf("%d warnings", warnCount)
	} else {
		step.Summary = "0 mismatches"
	}
	return step
}

func validateOpenAPI(path string) (reporter.StepResult, *openapi3.T) {
	step := reporter.StepResult{Name: string(KindOpenAPI)}
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("OpenAPI load error: %v", err))
		return step, nil
	}
	count := 0
	for _, pi := range doc.Paths.Map() {
		for range pi.Operations() {
			count++
		}
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d endpoints", count)
	return step, doc
}

func validateDDL(root string) (reporter.StepResult, *ssacvalidator.SymbolTable) {
	step := reporter.StepResult{Name: string(KindDDL)}
	st, err := ssacvalidator.LoadSymbolTable(root)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("DDL/SymbolTable load error: %v", err))
		return step, nil
	}
	tables := len(st.DDLTables)
	cols := 0
	for _, t := range st.DDLTables {
		cols += len(t.Columns)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d tables, %d columns", tables, cols)
	return step, st
}

func validateSSaC(root, serviceDir string, st *ssacvalidator.SymbolTable) (reporter.StepResult, []ssacparser.ServiceFunc) {
	step := reporter.StepResult{Name: string(KindSSaC)}
	funcs, err := ssacparser.ParseDir(serviceDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("SSaC parse error: %v", err))
		return step, nil
	}

	if st == nil {
		var stErr error
		st, stErr = ssacvalidator.LoadSymbolTable(root)
		if stErr != nil {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("SSaC symbol table load error: %v", stErr))
			return step, funcs
		}
	}

	verrs := ssacvalidator.ValidateWithSymbols(funcs, st)
	if len(verrs) > 0 {
		step.Status = reporter.Fail
		for _, ve := range verrs {
			step.Errors = append(step.Errors, fmt.Sprintf("%s:%s seq[%d] %s — %s",
				ve.FileName, ve.FuncName, ve.SeqIndex, ve.Tag, ve.Message))
		}
	} else {
		step.Status = reporter.Pass
	}
	step.Summary = fmt.Sprintf("%d service functions", len(funcs))
	return step, funcs
}

func validateStates(statesDir string) (reporter.StepResult, []*statemachine.StateDiagram) {
	step := reporter.StepResult{Name: string(KindStates)}
	diagrams, err := statemachine.ParseDir(statesDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("States parse error: %v", err))
		return step, nil
	}
	if len(diagrams) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no state diagrams found"
		return step, nil
	}

	totalTransitions := 0
	for _, d := range diagrams {
		totalTransitions += len(d.Transitions)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d diagrams, %d transitions", len(diagrams), totalTransitions)
	return step, diagrams
}

func validatePolicy(policyDir string) (reporter.StepResult, []*policy.Policy) {
	step := reporter.StepResult{Name: string(KindPolicy)}
	policies, err := policy.ParseDir(policyDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("Policy parse error: %v", err))
		return step, nil
	}
	if len(policies) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no policy files found"
		return step, nil
	}

	totalRules := 0
	totalOwnerships := 0
	for _, p := range policies {
		totalRules += len(p.Rules)
		totalOwnerships += len(p.Ownerships)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d files, %d rules, %d ownership mappings", len(policies), totalRules, totalOwnerships)
	return step, policies
}

func validateModel(modelDir string) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindModel)}
	matches, _ := filepath.Glob(filepath.Join(modelDir, "*.go"))
	if len(matches) == 0 {
		step.Status = reporter.Fail
		step.Summary = "no model files found"
		return step
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d files", len(matches))
	return step
}

func validateTerraform(tfDir string) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindTerraform)}
	matches, _ := filepath.Glob(filepath.Join(tfDir, "*.tf"))
	if len(matches) == 0 {
		step.Status = reporter.Fail
		step.Summary = "no terraform files found"
		return step
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d files", len(matches))
	return step
}

func validateSTML(root, frontendDir string) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindSTML)}
	pages, err := stmlparser.ParseDir(frontendDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("STML parse error: %v", err))
		return step
	}

	bindings := 0
	for _, p := range pages {
		bindings += len(p.Fetches) + len(p.Actions)
	}

	verrs := stmlvalidator.Validate(pages, root)
	if len(verrs) > 0 {
		step.Status = reporter.Fail
		for _, ve := range verrs {
			step.Errors = append(step.Errors, fmt.Sprintf("%s [%s] — %s",
				ve.File, ve.Attr, ve.Message))
		}
	} else {
		step.Status = reporter.Pass
	}
	step.Summary = fmt.Sprintf("%d pages, %d bindings", len(pages), bindings)
	return step
}
