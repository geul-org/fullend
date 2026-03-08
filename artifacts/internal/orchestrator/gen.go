package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/geul-org/fullend/artifacts/internal/reporter"
	ssacgenerator "github.com/geul-org/ssac/generator"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
	stmlgenerator "github.com/geul-org/stml/generator"
	stmlparser "github.com/geul-org/stml/parser"
)

// Gen runs validate first, then generates code from all detected SSOTs.
// Returns the validate report (with gen steps appended) and whether gen succeeded.
func Gen(specsDir, artifactsDir string) (*reporter.Report, bool) {
	detected, err := DetectSSOTs(specsDir)
	if err != nil {
		report := &reporter.Report{}
		report.Steps = append(report.Steps, reporter.StepResult{
			Name:   "detect",
			Status: reporter.Fail,
			Errors: []string{err.Error()},
		})
		return report, false
	}

	// 1. Validate first.
	report := Validate(specsDir, detected)
	if report.HasFailure() {
		return report, false
	}

	has := make(map[SSOTKind]DetectedSSOT)
	for _, d := range detected {
		has[d.Kind] = d
	}

	// Add separator between validate and gen steps.
	report.Steps = append(report.Steps, reporter.StepResult{
		Name:    "---",
		Status:  reporter.Pass,
		Summary: "codegen",
	})

	// Ensure artifacts directory exists.
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		report.Steps = append(report.Steps, reporter.StepResult{
			Name:   "gen",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("cannot create artifacts dir: %v", err)},
		})
		return report, false
	}

	// 2. sqlc generate
	report.Steps = append(report.Steps, genSqlc(specsDir))

	// 3. SSaC Generate (service functions)
	// 4. SSaC GenerateModelInterfaces
	if d, ok := has[KindSSaC]; ok {
		report.Steps = append(report.Steps, genSSaC(specsDir, d.Path, artifactsDir)...)
	}

	// 5. STML Generate (React TSX)
	if d, ok := has[KindSTML]; ok {
		report.Steps = append(report.Steps, genSTML(specsDir, d.Path, artifactsDir))
	}

	// 6. terraform fmt
	if _, ok := has[KindTerraform]; ok {
		report.Steps = append(report.Steps, genTerraform(specsDir))
	}

	genOk := true
	for _, s := range report.Steps {
		if s.Status == reporter.Fail {
			genOk = false
			break
		}
	}

	return report, genOk
}

func genSqlc(specsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "sqlc"}
	res := RunExec("sqlc", "generate", "-f", filepath.Join(specsDir, "sqlc.yaml"))
	if res.Skipped {
		step.Status = reporter.Skip
		step.Summary = "sqlc not installed, skipped"
		return step
	}
	if res.Err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, res.Err.Error())
		if res.Stderr != "" {
			step.Errors = append(step.Errors, res.Stderr)
		}
		return step
	}
	step.Status = reporter.Pass
	step.Summary = "DB models generated"
	return step
}

func genSSaC(specsDir, serviceDir, artifactsDir string) []reporter.StepResult {
	var steps []reporter.StepResult

	funcs, err := ssacparser.ParseDir(serviceDir)
	if err != nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-gen",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("SSaC parse error: %v", err)},
		})
		return steps
	}

	st, err := ssacvalidator.LoadSymbolTable(specsDir)
	if err != nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-gen",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("SSaC symbol table error: %v", err)},
		})
		return steps
	}

	// Generate service functions.
	serviceOutDir := filepath.Join(artifactsDir, "backend", "service")
	if err := os.MkdirAll(serviceOutDir, 0755); err != nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-gen",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("cannot create dir: %v", err)},
		})
		return steps
	}

	step := reporter.StepResult{Name: "ssac-gen"}
	if err := ssacgenerator.Generate(funcs, serviceOutDir, st); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("SSaC generate error: %v", err))
	} else {
		step.Status = reporter.Pass
		step.Summary = fmt.Sprintf("%d service files generated", len(funcs))
	}
	steps = append(steps, step)

	// Generate model interfaces.
	modelOutDir := filepath.Join(artifactsDir, "backend", "model")
	if err := os.MkdirAll(modelOutDir, 0755); err != nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-model",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("cannot create dir: %v", err)},
		})
		return steps
	}

	modelStep := reporter.StepResult{Name: "ssac-model"}
	if err := ssacgenerator.GenerateModelInterfaces(funcs, st, modelOutDir); err != nil {
		modelStep.Status = reporter.Fail
		modelStep.Errors = append(modelStep.Errors, fmt.Sprintf("SSaC model interface error: %v", err))
	} else {
		modelStep.Status = reporter.Pass
		modelStep.Summary = "model interfaces generated"
	}
	steps = append(steps, modelStep)

	return steps
}

func genSTML(specsDir, frontendDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "stml-gen"}

	pages, err := stmlparser.ParseDir(frontendDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("STML parse error: %v", err))
		return step
	}

	outDir := filepath.Join(artifactsDir, "frontend")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("cannot create dir: %v", err))
		return step
	}

	if err := stmlgenerator.Generate(pages, specsDir, outDir); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("STML generate error: %v", err))
		return step
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d pages generated", len(pages))
	return step
}

func genTerraform(specsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "terraform"}
	tfDir := filepath.Join(specsDir, "terraform")
	res := RunExec("terraform", "fmt", tfDir)
	if res.Skipped {
		step.Status = reporter.Skip
		step.Summary = "terraform not installed, skipped"
		return step
	}
	if res.Err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, res.Err.Error())
		if res.Stderr != "" {
			step.Errors = append(step.Errors, res.Stderr)
		}
		return step
	}
	step.Status = reporter.Pass
	step.Summary = "HCL formatted"
	return step
}
