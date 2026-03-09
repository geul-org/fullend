package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/gluegen"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/projectconfig"
	"github.com/geul-org/fullend/internal/reporter"
	"github.com/geul-org/fullend/internal/scenario"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacgenerator "github.com/geul-org/ssac/generator"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
	stmlgenerator "github.com/geul-org/stml/generator"
	stmlparser "github.com/geul-org/stml/parser"
)

// Gen runs validate first, then generates code from all detected SSOTs.
// Returns the validate report (with gen steps appended) and whether gen succeeded.
func Gen(specsDir, artifactsDir string, skipKinds ...map[SSOTKind]bool) (*reporter.Report, bool) {
	skip := make(map[SSOTKind]bool)
	if len(skipKinds) > 0 && skipKinds[0] != nil {
		skip = skipKinds[0]
	}
	return GenWith(DefaultProfile(), specsDir, artifactsDir, skip)
}

// GenWith runs code generation with the specified TargetProfile.
func GenWith(profile *TargetProfile, specsDir, artifactsDir string, skipKinds map[SSOTKind]bool) (*reporter.Report, bool) {
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
	report := Validate(specsDir, detected, skipKinds)
	if report.HasFailure() {
		return report, false
	}

	has := make(map[SSOTKind]DetectedSSOT)
	for _, d := range detected {
		has[d.Kind] = d
	}

	// Terraform: optional — warn and skip if not installed.
	terraformAvailable := true
	if _, ok := has[KindTerraform]; ok {
		if _, err := exec.LookPath("terraform"); err != nil {
			terraformAvailable = false
		}
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

	// 2. sqlc generate (exec) — auto-generate sqlc.yaml if needed.
	if _, ok := has[KindDDL]; ok {
		report.Steps = append(report.Steps, genSqlc(specsDir, artifactsDir))
	}

	// 3. oapi-codegen (exec) → backend/internal/api/
	if _, ok := has[KindOpenAPI]; ok {
		report.Steps = append(report.Steps, genOpenAPI(specsDir, artifactsDir))
	}

	// 4. SSaC Generate → backend/internal/service/
	// 5. SSaC GenerateModelInterfaces → backend/internal/model/
	if d, ok := has[KindSSaC]; ok {
		report.Steps = append(report.Steps, genSSaC(profile, specsDir, d.Path, artifactsDir)...)
	}

	// 6. STML Generate → frontend/src/pages/
	var stmlDeps map[string]string
	var stmlPages []string
	var stmlPageOps map[string]string
	if d, ok := has[KindSTML]; ok {
		var step reporter.StepResult
		step, stmlDeps, stmlPages, stmlPageOps = genSTML(profile, specsDir, d.Path, artifactsDir)
		report.Steps = append(report.Steps, step)
	}

	// 7. Glue code generation (Server struct + main.go + frontend setup)
	report.Steps = append(report.Steps, genGlue(specsDir, artifactsDir, has, stmlDeps, stmlPages, stmlPageOps))

	// 8. Hurl smoke test generation (part of glue-gen, report separately)
	{
		testsDir := filepath.Join(artifactsDir, "tests")
		if _, err := os.Stat(filepath.Join(testsDir, "smoke.hurl")); err == nil {
			report.Steps = append(report.Steps, reporter.StepResult{
				Name:    "hurl-gen",
				Status:  reporter.Pass,
				Summary: "smoke.hurl generated",
			})
		}
	}

	// 9. State machine code generation.
	if d, ok := has[KindStates]; ok {
		report.Steps = append(report.Steps, genStateMachines(d.Path, specsDir, artifactsDir))
	}

	// 10. OPA Authorizer code generation.
	if d, ok := has[KindPolicy]; ok {
		report.Steps = append(report.Steps, genAuthz(d.Path, specsDir, artifactsDir))
	}

	// 11. Scenario Hurl generation.
	if d, ok := has[KindScenario]; ok {
		report.Steps = append(report.Steps, genScenarioHurl(d.Path, specsDir, artifactsDir))
	}

	// 12. Func copy (custom func specs → artifacts).
	if d, ok := has[KindFunc]; ok {
		report.Steps = append(report.Steps, genFunc(d.Path, specsDir, artifactsDir))
	}

	// 13. terraform fmt (외부 도구, 선택)
	if _, ok := has[KindTerraform]; ok {
		if terraformAvailable {
			report.Steps = append(report.Steps, genTerraform(specsDir))
		} else {
			report.Steps = append(report.Steps, reporter.StepResult{
				Name:    "terraform",
				Status:  reporter.Skip,
				Summary: "terraform 미설치, 스킵",
				Errors:  []string{"[WARN] terraform이 설치되어 있지 않습니다 — HCL 포맷팅을 건너뜁니다"},
			})
		}
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

func genSqlc(specsDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "sqlc"}

	// Auto-generate sqlc.yaml if not present.
	configPath, err := generateSqlcConfig(specsDir, artifactsDir)
	if err != nil {
		step.Status = reporter.Skip
		step.Summary = err.Error()
		return step
	}

	res := RunExec("sqlc", "generate", "-f", configPath)
	if res.Skipped {
		step.Status = reporter.Skip
		step.Summary = "sqlc 미설치, 스킵"
		step.Errors = append(step.Errors, "[WARN] sqlc가 설치되어 있지 않습니다 — go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest")
		return step
	}
	if res.Err != nil {
		step.Status = reporter.Skip
		step.Summary = "sqlc generate 실패, 스킵"
		step.Errors = append(step.Errors, fmt.Sprintf("[WARN] %v", res.Err))
		if res.Stderr != "" {
			step.Errors = append(step.Errors, res.Stderr)
		}
		return step
	}
	step.Status = reporter.Pass
	step.Summary = "DB models generated"
	return step
}

func genOpenAPI(specsDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "oapi-gen"}
	apiPath := filepath.Join(specsDir, "api", "openapi.yaml")

	outDir := filepath.Join(artifactsDir, "backend", "internal", "api")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("cannot create dir: %v", err))
		return step
	}

	// Generate types.
	typesOut := filepath.Join(outDir, "types.gen.go")
	res := RunExec("oapi-codegen", "-package", "api", "-generate", "types", "-o", typesOut, apiPath)
	if res.Skipped {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "oapi-codegen이 설치되어 있지 않습니다 — go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest")
		return step
	}
	if res.Err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("oapi-codegen types error: %v", res.Err))
		if res.Stderr != "" {
			step.Errors = append(step.Errors, res.Stderr)
		}
		return step
	}

	// Generate server (net/http std).
	serverOut := filepath.Join(outDir, "server.gen.go")
	res = RunExec("oapi-codegen", "-package", "api", "-generate", "std-http-server", "-o", serverOut, apiPath)
	if res.Err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("oapi-codegen server error: %v", res.Err))
		if res.Stderr != "" {
			step.Errors = append(step.Errors, res.Stderr)
		}
		return step
	}

	step.Status = reporter.Pass
	step.Summary = "types + server generated"
	return step
}

func genSSaC(profile *TargetProfile, specsDir, serviceDir, artifactsDir string) []reporter.StepResult {
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

	// Generate service functions → backend/internal/service/
	serviceOutDir := filepath.Join(artifactsDir, "backend", "internal", "service")
	if err := os.MkdirAll(serviceOutDir, 0755); err != nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-gen",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("cannot create dir: %v", err)},
		})
		return steps
	}

	step := reporter.StepResult{Name: "ssac-gen"}
	if err := ssacgenerator.GenerateWith(profile.Backend, funcs, serviceOutDir, st); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("SSaC generate error: %v", err))
	} else {
		step.Status = reporter.Pass
		step.Summary = fmt.Sprintf("%d service files generated", len(funcs))
	}
	steps = append(steps, step)

	// Generate model interfaces → backend/internal/model/
	// SSaC writes to outDir/model/, so pass backend/internal/ as outDir.
	modelOutDir := filepath.Join(artifactsDir, "backend", "internal")
	if err := os.MkdirAll(modelOutDir, 0755); err != nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-model",
			Status: reporter.Fail,
			Errors: []string{fmt.Sprintf("cannot create dir: %v", err)},
		})
		return steps
	}

	modelStep := reporter.StepResult{Name: "ssac-model"}
	if err := profile.Backend.GenerateModelInterfaces(funcs, st, modelOutDir); err != nil {
		modelStep.Status = reporter.Fail
		modelStep.Errors = append(modelStep.Errors, fmt.Sprintf("SSaC model interface error: %v", err))
	} else {
		modelStep.Status = reporter.Pass
		modelStep.Summary = "model interfaces generated"
	}
	steps = append(steps, modelStep)

	return steps
}

func genSTML(profile *TargetProfile, specsDir, frontendDir, artifactsDir string) (reporter.StepResult, map[string]string, []string, map[string]string) {
	step := reporter.StepResult{Name: "stml-gen"}

	pages, err := stmlparser.ParseDir(frontendDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("STML parse error: %v", err))
		return step, nil, nil, nil
	}

	// Output to frontend/src/pages/
	outDir := filepath.Join(artifactsDir, "frontend", "src", "pages")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("cannot create dir: %v", err))
		return step, nil, nil, nil
	}

	result, err := stmlgenerator.GenerateWith(profile.Frontend, pages, specsDir, outDir, stmlgenerator.GenerateOptions{
		APIImportPath: "../api",
		UseClient:     false,
	})
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("STML generate error: %v", err))
		return step, nil, nil, nil
	}

	// Collect generated page names and primary operationIDs for glue-gen.
	var pageNames []string
	pageOps := make(map[string]string)
	for _, p := range pages {
		pageNames = append(pageNames, p.Name)
		// Determine primary operationID from first fetch or first action.
		// PageSpec.Name already includes "-page" suffix (e.g. "login-page").
		if len(p.Fetches) > 0 {
			pageOps[p.Name] = p.Fetches[0].OperationID
		} else if len(p.Actions) > 0 {
			pageOps[p.Name] = p.Actions[0].OperationID
		}
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d pages generated", result.Pages)
	return step, result.Dependencies, pageNames, pageOps
}

func genGlue(specsDir, artifactsDir string, has map[SSOTKind]DetectedSSOT, stmlDeps map[string]string, stmlPages []string, stmlPageOps map[string]string) reporter.StepResult {
	step := reporter.StepResult{Name: "glue-gen"}

	// Determine module path from fullend.yaml, fallback to directory-based.
	modulePath := determineModulePath(specsDir, artifactsDir)

	input := &gluegen.GlueInput{
		ArtifactsDir: artifactsDir,
		SpecsDir:     specsDir,
		ModulePath:   modulePath,
		STMLDeps:     stmlDeps,
		STMLPages:    stmlPages,
		STMLPageOps:  stmlPageOps,
	}

	// Load OpenAPI doc.
	if _, ok := has[KindOpenAPI]; ok {
		apiPath := filepath.Join(specsDir, "api", "openapi.yaml")
		doc, err := openapi3.NewLoader().LoadFromFile(apiPath)
		if err == nil {
			input.OpenAPIDoc = doc
		}
	}

	// Load service funcs.
	if d, ok := has[KindSSaC]; ok {
		funcs, err := ssacparser.ParseDir(d.Path)
		if err == nil {
			input.ServiceFuncs = funcs
		}
	}

	// Load symbol table.
	st, err := ssacvalidator.LoadSymbolTable(specsDir)
	if err == nil {
		input.SymbolTable = st
	}

	if err := gluegen.Generate(input); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("glue-gen error: %v", err))
		return step
	}

	step.Status = reporter.Pass
	step.Summary = "server + main.go + frontend setup generated"
	return step
}

func determineModulePath(specsDir, artifactsDir string) string {
	// 1. Try fullend.yaml first.
	cfg, err := projectconfig.Load(specsDir)
	if err == nil && cfg.Backend.Module != "" {
		return cfg.Backend.Module
	}

	// 2. Fallback: check existing backend/go.mod.
	goModPath := filepath.Join(artifactsDir, "backend", "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module "))
			}
		}
	}

	// 3. Last resort: derive from directory name.
	base := filepath.Base(artifactsDir)
	return base + "/backend"
}

func genStateMachines(statesDir, specsDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "state-gen"}

	diagrams, err := statemachine.ParseDir(statesDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("States parse error: %v", err))
		return step
	}
	if len(diagrams) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no state diagrams"
		return step
	}

	modulePath := determineModulePath(specsDir, artifactsDir)
	if err := gluegen.GenerateStateMachines(diagrams, artifactsDir, modulePath); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("state-gen error: %v", err))
		return step
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d state machines generated", len(diagrams))
	return step
}

func genAuthz(policyDir, specsDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "authz-gen"}

	policies, err := policy.ParseDir(policyDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("Policy parse error: %v", err))
		return step
	}
	if len(policies) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no policy files"
		return step
	}

	modulePath := determineModulePath(specsDir, artifactsDir)
	if err := gluegen.GenerateAuthzPackage(policies, artifactsDir, modulePath); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("authz-gen error: %v", err))
		return step
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("OPA authorizer generated (%d rules)", countPolicyRules(policies))
	return step
}

func countPolicyRules(policies []*policy.Policy) int {
	total := 0
	for _, p := range policies {
		total += len(p.Rules)
	}
	return total
}

func genScenarioHurl(scenarioDir, specsDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "scenario-gen"}

	features, err := scenario.ParseDir(scenarioDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("Scenario parse error: %v", err))
		return step
	}
	if len(features) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no feature files"
		return step
	}

	// Load OpenAPI doc for path resolution.
	apiPath := filepath.Join(specsDir, "api", "openapi.yaml")
	doc, err := openapi3.NewLoader().LoadFromFile(apiPath)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("OpenAPI load error: %v", err))
		return step
	}

	if err := gluegen.GenerateScenarioHurl(features, doc, artifactsDir); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("scenario-gen error: %v", err))
		return step
	}

	totalScenarios := 0
	for _, f := range features {
		totalScenarios += len(f.Scenarios)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d feature files → %d hurl files", len(features), len(features))
	return step
}

func genFunc(funcDir, specsDir, artifactsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "func-gen"}

	// Copy custom func files from specs/<project>/func/<pkg>/ → artifacts/<project>/backend/internal/<pkg>/.
	entries, err := os.ReadDir(funcDir)
	if err != nil {
		step.Status = reporter.Skip
		step.Summary = "no func/ directory"
		return step
	}

	copied := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg := entry.Name()
		srcDir := filepath.Join(funcDir, pkg)
		dstDir := filepath.Join(artifactsDir, "backend", "internal", pkg)

		if err := os.MkdirAll(dstDir, 0755); err != nil {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("cannot create dir %s: %v", dstDir, err))
			return step
		}

		files, _ := filepath.Glob(filepath.Join(srcDir, "*.go"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				step.Status = reporter.Fail
				step.Errors = append(step.Errors, fmt.Sprintf("read %s: %v", f, err))
				return step
			}
			dst := filepath.Join(dstDir, filepath.Base(f))
			if err := os.WriteFile(dst, data, 0644); err != nil {
				step.Status = reporter.Fail
				step.Errors = append(step.Errors, fmt.Sprintf("write %s: %v", dst, err))
				return step
			}
			copied++
		}
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d func files copied", copied)
	return step
}

func genTerraform(specsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "terraform"}
	tfDir := filepath.Join(specsDir, "terraform")
	res := RunExec("terraform", "fmt", tfDir)
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
