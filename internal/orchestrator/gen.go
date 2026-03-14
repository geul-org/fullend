package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geul-org/fullend/internal/contract"
	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/gluegen"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/projectconfig"
	"github.com/geul-org/fullend/internal/reporter"
	ssacgenerator "github.com/geul-org/fullend/internal/ssac/generator"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
	stmlgenerator "github.com/geul-org/fullend/internal/stml/generator"
	stmlparser "github.com/geul-org/fullend/internal/stml/parser"
)

// Gen runs validate first, then generates code from all detected SSOTs.
// Returns the validate report (with gen steps appended) and whether gen succeeded.
func Gen(specsDir, artifactsDir string, skipKinds map[SSOTKind]bool, reset ...bool) (*reporter.Report, bool) {
	r := len(reset) > 0 && reset[0]
	return GenWith(DefaultProfile(), specsDir, artifactsDir, skipKinds, r)
}

// GenWith runs code generation with the specified TargetProfile.
// When reset is false (default), preserved function bodies are restored after generation.
func GenWith(profile *TargetProfile, specsDir, artifactsDir string, skipKinds map[SSOTKind]bool, reset ...bool) (*reporter.Report, bool) {
	isReset := len(reset) > 0 && reset[0]
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

	skip := skipKinds
	if skip == nil {
		skip = make(map[SSOTKind]bool)
	}

	// Parse all SSOTs once.
	parsed := ParseAll(specsDir, detected, skip)

	// 1. Validate first (reuse parsed data).
	report := ValidateWith(specsDir, detected, parsed, skip)
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

	// Pre-gen: scan preserved functions (unless reset).
	var preserveSnap *contract.PreserveSnapshot
	backendDir := filepath.Join(artifactsDir, "backend")
	if !isReset {
		preserveSnap = contract.ScanPreserveSnapshot(backendDir)
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
	if _, ok := has[KindSSaC]; ok {
		report.Steps = append(report.Steps, genSSaC(profile, specsDir, artifactsDir, parsed)...)
	}

	// 6. STML Generate → frontend/src/pages/
	var stmlDeps map[string]string
	var stmlPages []string
	var stmlPageOps map[string]string
	if _, ok := has[KindSTML]; ok {
		var step reporter.StepResult
		step, stmlDeps, stmlPages, stmlPageOps = genSTML(profile, specsDir, artifactsDir, parsed.STMLPages)
		report.Steps = append(report.Steps, step)
	}

	// 7. Glue code generation (Server struct + main.go + frontend setup)
	report.Steps = append(report.Steps, genGlue(specsDir, artifactsDir, has, parsed, stmlDeps, stmlPages, stmlPageOps))

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
	if _, ok := has[KindStates]; ok {
		report.Steps = append(report.Steps, genStateMachines(specsDir, artifactsDir, parsed))
	}

	// 10. OPA Authorizer code generation.
	if _, ok := has[KindPolicy]; ok {
		report.Steps = append(report.Steps, genAuthz(artifactsDir, parsed))
	}

	// 11. Scenario: user writes .hurl directly, no generation needed.

	// 12. Func copy (custom func specs → artifacts).
	if d, ok := has[KindFunc]; ok {
		modulePath := determineModulePath(specsDir, artifactsDir, parsed.Config)
		report.Steps = append(report.Steps, genFunc(d.Path, specsDir, artifactsDir, modulePath))
	}

	// Post-gen: restore preserved function bodies.
	if preserveSnap != nil {
		hasPreserves := len(preserveSnap.FilePreserves) > 0 || len(preserveSnap.FuncPreserves) > 0
		if hasPreserves {
			warnings := contract.RestorePreserved(preserveSnap)
			preserveCount := len(preserveSnap.FilePreserves)
			for _, funcs := range preserveSnap.FuncPreserves {
				preserveCount += len(funcs)
			}
			step := reporter.StepResult{
				Name:    "preserve",
				Status:  reporter.Pass,
				Summary: fmt.Sprintf("%d preserved", preserveCount),
			}
			for _, w := range warnings {
				step.Errors = append(step.Errors,
					fmt.Sprintf("[WARN] contract changed: %s:%s (old=%s new=%s)",
						w.File, w.Function, w.OldContract, w.NewContract))
			}
			if len(warnings) > 0 {
				step.Suggestions = append(step.Suggestions, ".new 파일에서 새 계약 코드를 확인하세요")
			}
			report.Steps = append(report.Steps, step)
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

func genSSaC(profile *TargetProfile, specsDir, artifactsDir string, parsed *ParsedSSOTs) []reporter.StepResult {
	var steps []reporter.StepResult

	funcs := parsed.ServiceFuncs
	if funcs == nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-gen",
			Status: reporter.Fail,
			Errors: []string{"SSaC parse failed"},
		})
		return steps
	}

	if parsed.SymbolTable == nil {
		steps = append(steps, reporter.StepResult{
			Name:   "ssac-gen",
			Status: reporter.Fail,
			Errors: []string{"SSaC symbol table not available"},
		})
		return steps
	}

	// Clone SymbolTable for gen path — injectFuncErrStatus mutates Models.
	genST := parsed.SymbolTable.Clone()

	// Inject @error annotations from func specs into the cloned symbol table.
	injectFuncErrStatusFromParsed(genST, parsed)

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

	if gt, ok := profile.Backend.(*ssacgenerator.GoTarget); ok {
		gt.FuncSpecs = append(parsed.PkgFuncSpecs, parsed.FuncSpecs...)
	}

	step := reporter.StepResult{Name: "ssac-gen"}
	if err := ssacgenerator.GenerateWith(profile.Backend, funcs, serviceOutDir, genST); err != nil {
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
	if err := profile.Backend.GenerateModelInterfaces(funcs, genST, modelOutDir); err != nil {
		modelStep.Status = reporter.Fail
		modelStep.Errors = append(modelStep.Errors, fmt.Sprintf("SSaC model interface error: %v", err))
	} else {
		modelStep.Status = reporter.Pass
		modelStep.Summary = "model interfaces generated"
	}
	steps = append(steps, modelStep)

	return steps
}

// injectFuncErrStatusFromParsed uses pre-parsed func specs to inject @error annotations.
func injectFuncErrStatusFromParsed(st *ssacvalidator.SymbolTable, parsed *ParsedSSOTs) {
	var allSpecs []funcspec.FuncSpec
	allSpecs = append(allSpecs, parsed.PkgFuncSpecs...)
	allSpecs = append(allSpecs, parsed.FuncSpecs...)

	for _, fs := range allSpecs {
		if fs.ErrStatus == 0 || fs.Package == "" {
			continue
		}
		// Register as "pkg._func" model key (matches SSaC convention).
		modelKey := fs.Package + "._func"
		ms, exists := st.Models[modelKey]
		if !exists {
			ms = ssacvalidator.ModelSymbol{Methods: make(map[string]ssacvalidator.MethodInfo)}
		}
		funcName := strings.ToUpper(fs.Name[:1]) + fs.Name[1:]
		mi := ms.Methods[funcName]
		mi.ErrStatus = fs.ErrStatus
		ms.Methods[funcName] = mi
		st.Models[modelKey] = ms
	}
}

func genSTML(profile *TargetProfile, specsDir, artifactsDir string, pages []stmlparser.PageSpec) (reporter.StepResult, map[string]string, []string, map[string]string) {
	step := reporter.StepResult{Name: "stml-gen"}

	if pages == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "STML parse failed")
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

func genGlue(specsDir, artifactsDir string, has map[SSOTKind]DetectedSSOT, parsed *ParsedSSOTs, stmlDeps map[string]string, stmlPages []string, stmlPageOps map[string]string) reporter.StepResult {
	step := reporter.StepResult{Name: "glue-gen"}

	// Determine module path from fullend.yaml, fallback to directory-based.
	modulePath := determineModulePath(specsDir, artifactsDir, parsed.Config)

	// Extract claims, queue, and authz config from pre-parsed config.
	var claims map[string]string
	var queueBackend string
	var authzPackage string
	if parsed.Config != nil {
		if parsed.Config.Backend.Auth != nil {
			claims = parsed.Config.Backend.Auth.Claims
		}
		if parsed.Config.Queue != nil {
			queueBackend = parsed.Config.Queue.Backend
		}
		if parsed.Config.Authz != nil {
			authzPackage = parsed.Config.Authz.Package
		}
	}

	input := &gluegen.GlueInput{
		ArtifactsDir: artifactsDir,
		SpecsDir:     specsDir,
		ModulePath:   modulePath,
		STMLDeps:     stmlDeps,
		STMLPages:    stmlPages,
		STMLPageOps:  stmlPageOps,
		Claims:       claims,
		QueueBackend: queueBackend,
		AuthzPackage: authzPackage,
	}

	// Use pre-parsed data.
	input.OpenAPIDoc = parsed.OpenAPIDoc
	input.ServiceFuncs = parsed.ServiceFuncs
	input.SymbolTable = parsed.SymbolTable
	input.StateDiagrams = parsed.States
	input.Policies = parsed.Policies

	if err := gluegen.Generate(input); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("glue-gen error: %v", err))
		return step
	}

	step.Status = reporter.Pass
	step.Summary = "server + main.go + frontend setup generated"
	return step
}

func determineModulePath(specsDir, artifactsDir string, cfg *projectconfig.ProjectConfig) string {
	// 1. Try pre-parsed config first.
	if cfg != nil && cfg.Backend.Module != "" {
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

func genStateMachines(specsDir, artifactsDir string, parsed *ParsedSSOTs) reporter.StepResult {
	step := reporter.StepResult{Name: "state-gen"}

	diagrams := parsed.States
	if diagrams == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "States parse failed")
		return step
	}
	if len(diagrams) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no state diagrams"
		return step
	}

	modulePath := determineModulePath(specsDir, artifactsDir, parsed.Config)
	if err := gluegen.GenerateStateMachines(diagrams, artifactsDir, modulePath); err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("state-gen error: %v", err))
		return step
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d state machines generated", len(diagrams))
	return step
}

func genAuthz(artifactsDir string, parsed *ParsedSSOTs) reporter.StepResult {
	step := reporter.StepResult{Name: "authz-gen"}

	policies := parsed.Policies
	if policies == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "Policy parse failed")
		return step
	}
	if len(policies) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no policy files"
		return step
	}

	if err := gluegen.GenerateAuthzPackage(policies, artifactsDir); err != nil {
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

func genFunc(funcDir, specsDir, artifactsDir, modulePath string) reporter.StepResult {
	step := reporter.StepResult{Name: "func-gen"}

	entries, err := os.ReadDir(funcDir)
	if err != nil {
		step.Status = reporter.Skip
		step.Summary = "no func/ directory"
		return step
	}

	// Scan SSaC files to find import paths for each func package.
	funcImportPaths, err := scanFuncImports(specsDir, modulePath)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("failed to scan SSaC imports: %v", err))
		return step
	}

	copied := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg := entry.Name()
		srcDir := filepath.Join(funcDir, pkg)

		// Determine destination from SSaC import path.
		importPath, ok := funcImportPaths[pkg]
		if !ok {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("func/%s: SSaC에서 import하는 곳이 없습니다", pkg))
			return step
		}

		// Extract relative path within module.
		relPath := strings.TrimPrefix(importPath, modulePath+"/")
		if relPath == importPath {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("func/%s: import 경로 %q가 모듈 %q에 속하지 않습니다", pkg, importPath, modulePath))
			return step
		}

		// Validate: must be under internal/ or pkg/.
		if !strings.HasPrefix(relPath, "internal/") && !strings.HasPrefix(relPath, "pkg/") {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("func/%s: import 경로 %q는 internal/ 또는 pkg/ 하위여야 합니다 (현재: %s)", pkg, importPath, relPath))
			return step
		}

		dstDir := filepath.Join(artifactsDir, "backend", relPath)

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

// scanFuncImports scans SSaC files for import statements that reference func packages.
// Returns a map of package name → full import path.
func scanFuncImports(specsDir, modulePath string) (map[string]string, error) {
	result := make(map[string]string)

	ssacFiles, _ := filepath.Glob(filepath.Join(specsDir, "service", "**", "*.ssac"))
	if len(ssacFiles) == 0 {
		ssacFiles, _ = filepath.Glob(filepath.Join(specsDir, "service", "*.ssac"))
	}

	for _, f := range ssacFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "import ") {
				continue
			}
			// Extract quoted path.
			q1 := strings.Index(line, "\"")
			q2 := strings.LastIndex(line, "\"")
			if q1 < 0 || q2 <= q1 {
				continue
			}
			importPath := line[q1+1 : q2]

			// Skip fullend built-in packages.
			if strings.HasPrefix(importPath, "github.com/geul-org/fullend/") {
				continue
			}

			// Only consider imports within the project module.
			if !strings.HasPrefix(importPath, modulePath+"/") {
				continue
			}

			// Extract package name (last segment).
			pkg := filepath.Base(importPath)
			result[pkg] = importPath
		}
	}

	return result, nil
}
