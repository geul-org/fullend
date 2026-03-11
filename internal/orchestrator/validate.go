package orchestrator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/crosscheck"
	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/projectconfig"
	"github.com/geul-org/fullend/internal/reporter"
	"github.com/geul-org/fullend/internal/scenario"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
	stmlparser "github.com/geul-org/stml/parser"
	stmlvalidator "github.com/geul-org/stml/validator"
)

// allKinds defines the display order of SSOT kinds for validation.
var allKinds = []SSOTKind{KindConfig, KindOpenAPI, KindDDL, KindSSaC, KindModel, KindSTML, KindStates, KindPolicy, KindScenario, KindFunc, KindTerraform}

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
	var parsedFeatures []*scenario.Feature
	var projectFuncSpecs []funcspec.FuncSpec
	var projConfig *projectconfig.ProjectConfig
	var modelDir string

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
			} else if kind == KindFunc {
				// Func is optional — no func/ dir is not an error.
				// SSaC @func references with missing implementations are caught by crosscheck.
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Skip,
					Summary: "no func/ directory",
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
		case KindConfig:
			step, cfg := validateConfig(d.Path)
			report.Steps = append(report.Steps, step)
			projConfig = cfg
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
		case KindScenario:
			step, features := validateScenario(d.Path)
			report.Steps = append(report.Steps, step)
			parsedFeatures = features
		case KindFunc:
			step, specs := validateFunc(d.Path)
			report.Steps = append(report.Steps, step)
			projectFuncSpecs = specs
		case KindModel:
			report.Steps = append(report.Steps, validateModel(d.Path))
			modelDir = d.Path
		case KindTerraform:
			report.Steps = append(report.Steps, validateTerraform(d.Path))
		}
	}

	// Cross-validation step.
	report.Steps = append(report.Steps, runCrossValidate(root, openAPIDoc, symTable, serviceFuncs, stateDiagrams, parsedPolicies, parsedFeatures, projectFuncSpecs, modelDir, projConfig))

	return report
}

func runCrossValidate(root string, doc *openapi3.T, st *ssacvalidator.SymbolTable, funcs []ssacparser.ServiceFunc, diagrams []*statemachine.StateDiagram, policies []*policy.Policy, features []*scenario.Feature, projectFuncSpecs []funcspec.FuncSpec, modelDir string, projConfig *projectconfig.ProjectConfig) reporter.StepResult {
	step := reporter.StepResult{Name: "Cross"}

	// Require OpenAPI + DDL + SSaC for cross-validation.
	if doc == nil || st == nil || funcs == nil {
		step.Status = reporter.Skip
		step.Summary = "skipped (incomplete SSOT)"
		return step
	}

	// Try to load fullend pkg/ specs from the module root.
	var fullendPkgSpecs []funcspec.FuncSpec
	if pkgRoot := findFullendPkgRoot(); pkgRoot != "" {
		if specs, err := funcspec.ParseDir(pkgRoot); err == nil {
			fullendPkgSpecs = specs
		}
	}

	// Load @dto types from model files.
	dtoTypes := loadDTOTypes(modelDir)

	var middleware []string
	var claims map[string]string
	if projConfig != nil {
		middleware = projConfig.Backend.Middleware
		if projConfig.Backend.Auth != nil {
			claims = projConfig.Backend.Auth.Claims
		}
	}

	// Parse @archived tags from DDL files.
	archived, _ := crosscheck.ParseArchived(filepath.Join(root, "db"))

	var queueBackend string
	if projConfig != nil && projConfig.Queue != nil {
		queueBackend = projConfig.Queue.Backend
	}

	var authzPackage string
	if projConfig != nil && projConfig.Authz != nil {
		authzPackage = projConfig.Authz.Package
	}

	input := &crosscheck.CrossValidateInput{
		OpenAPIDoc:       doc,
		SymbolTable:      st,
		ServiceFuncs:     funcs,
		StateDiagrams:    diagrams,
		Policies:         policies,
		Features:         features,
		ProjectFuncSpecs: projectFuncSpecs,
		FullendPkgSpecs:  fullendPkgSpecs,
		DTOTypes:         dtoTypes,
		Middleware:       middleware,
		Archived:         archived,
		Claims:           claims,
		QueueBackend:     queueBackend,
		AuthzPackage:     authzPackage,
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

	// Check path param name conflicts.
	if conflicts := checkPathParamConflicts(doc); len(conflicts) > 0 {
		for _, c := range conflicts {
			step.Errors = append(step.Errors, c)
		}
	}

	if len(step.Errors) > 0 {
		step.Status = reporter.Fail
	} else {
		step.Status = reporter.Pass
	}
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

	// Check sqlc query name duplicates across files.
	if dupes := checkSqlcQueryDuplicates(root); len(dupes) > 0 {
		for _, d := range dupes {
			step.Errors = append(step.Errors, d)
		}
	}

	// Check nullable columns (NOT NULL required on all columns).
	if nullables := checkDDLNullableColumns(root); len(nullables) > 0 {
		for _, n := range nullables {
			step.Errors = append(step.Errors, n)
		}
	}

	if len(step.Errors) > 0 {
		step.Status = reporter.Fail
	} else {
		step.Status = reporter.Pass
	}
	step.Summary = fmt.Sprintf("%d tables, %d columns", tables, cols)
	return step, st
}

// checkSqlcQueryDuplicates scans db/queries/*.sql for duplicate -- name: entries.
func checkSqlcQueryDuplicates(root string) []string {
	queriesDir := filepath.Join(root, "db", "queries")
	entries, err := os.ReadDir(queriesDir)
	if err != nil {
		return nil
	}

	nameRe := regexp.MustCompile(`^--\s*name:\s*(\w+)\s+:(\w+)`)
	// nameToFiles maps query name -> list of filenames where it appears.
	nameToFiles := make(map[string][]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		f, err := os.Open(filepath.Join(queriesDir, entry.Name()))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if m := nameRe.FindStringSubmatch(scanner.Text()); m != nil {
				nameToFiles[m[1]] = append(nameToFiles[m[1]], entry.Name())
			}
		}
		f.Close()
	}

	var errs []string
	for name, files := range nameToFiles {
		if len(files) > 1 {
			errs = append(errs, fmt.Sprintf(
				"db/queries: %q 이름이 중복됩니다 (%s) — sqlc는 전역 네임스페이스이므로 ModelPrefix를 붙이세요 (예: User%s, Gig%s)",
				name, strings.Join(files, ", "), name, name))
		}
	}
	return errs
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
		hasError := false
		for _, ve := range verrs {
			prefix := ""
			if ve.Level == "WARNING" {
				prefix = "[WARN] "
			} else {
				hasError = true
			}
			step.Errors = append(step.Errors, fmt.Sprintf("%s%s:%s seq[%d] %s — %s",
				prefix, ve.FileName, ve.FuncName, ve.SeqIndex, ve.Tag, ve.Message))
		}
		if hasError {
			step.Status = reporter.Fail
		} else {
			step.Status = reporter.Pass
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

func validateScenario(scenarioDir string) (reporter.StepResult, []*scenario.Feature) {
	step := reporter.StepResult{Name: string(KindScenario)}
	features, err := scenario.ParseDir(scenarioDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("Scenario parse error: %v", err))
		return step, nil
	}
	if len(features) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no feature files found"
		return step, nil
	}

	totalScenarios := 0
	for _, f := range features {
		totalScenarios += len(f.Scenarios)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d features, %d scenarios", len(features), totalScenarios)
	return step, features
}

func validateFunc(funcDir string) (reporter.StepResult, []funcspec.FuncSpec) {
	step := reporter.StepResult{Name: string(KindFunc)}
	specs, err := funcspec.ParseDir(funcDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, fmt.Sprintf("Func parse error: %v", err))
		return step, nil
	}
	if len(specs) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no func spec files found"
		return step, nil
	}

	// Count stubs.
	stubs := 0
	for _, s := range specs {
		if !s.HasBody {
			stubs++
		}
	}

	step.Status = reporter.Pass
	if stubs > 0 {
		step.Summary = fmt.Sprintf("%d funcs (%d TODO)", len(specs), stubs)
	} else {
		step.Summary = fmt.Sprintf("%d funcs", len(specs))
	}
	return step, specs
}

// findFullendPkgRoot locates the fullend pkg/ directory.
// Walks up from CWD looking for go.mod with module github.com/geul-org/fullend.
func findFullendPkgRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(goModPath); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(line) == "module github.com/geul-org/fullend" {
					pkgDir := filepath.Join(dir, "pkg")
					if fi, err := os.Stat(pkgDir); err == nil && fi.IsDir() {
						return pkgDir
					}
					return ""
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// loadDTOTypes scans model/*.go files for types preceded by a // @dto comment.
func loadDTOTypes(modelDir string) map[string]bool {
	dtoTypes := make(map[string]bool)
	if modelDir == "" {
		return dtoTypes
	}
	matches, _ := filepath.Glob(filepath.Join(modelDir, "*.go"))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		dtoNext := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "// @dto" || strings.HasPrefix(trimmed, "// @dto ") {
				dtoNext = true
				continue
			}
			if dtoNext && strings.HasPrefix(trimmed, "type ") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					dtoTypes[parts[1]] = true
				}
				dtoNext = false
			} else if dtoNext && trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				dtoNext = false
			}
		}
	}
	return dtoTypes
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

func validateConfig(path string) (reporter.StepResult, *projectconfig.ProjectConfig) {
	step := reporter.StepResult{Name: string(KindConfig)}
	cfg, err := projectconfig.Load(filepath.Dir(path))
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, err.Error())
		return step, nil
	}
	step.Status = reporter.Pass
	parts := []string{cfg.Metadata.Name}
	if cfg.Backend.Module != "" {
		parts = append(parts, cfg.Backend.Lang+"/"+cfg.Backend.Framework)
	}
	if cfg.Frontend.Name != "" {
		parts = append(parts, cfg.Frontend.Lang+"/"+cfg.Frontend.Framework)
	}
	step.Summary = strings.Join(parts, ", ")
	return step, cfg
}

// checkDDLNullableColumns scans DDL files for columns missing NOT NULL.
// PRIMARY KEY columns are implicitly NOT NULL and are excluded.
// Also checks FK + DEFAULT 0 columns for sentinel record (id=0) in referenced table.
func checkDDLNullableColumns(root string) []string {
	dbDir := filepath.Join(root, "db")
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return nil
	}

	createRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(\w+)`)
	colRe := regexp.MustCompile(`^(\w+)\s+\w+`)
	refRe := regexp.MustCompile(`(?i)REFERENCES\s+(\w+)`)

	// 1단계: 모든 DDL 파일 내용을 테이블별로 수집.
	tableContents := make(map[string]string) // tableName → 파일 전체 내용
	type fileInfo struct {
		tableName string
		content   string
	}
	var files []fileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dbDir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		tableMatch := createRe.FindStringSubmatch(content)
		if tableMatch == nil {
			continue
		}
		tableName := tableMatch[1]
		tableContents[tableName] = content
		files = append(files, fileInfo{tableName: tableName, content: content})
	}

	// 2단계: 컬럼별 NOT NULL 체크 + FK DEFAULT 0 센티널 체크.
	var errs []string
	for _, f := range files {
		for _, line := range strings.Split(f.content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(strings.ToUpper(trimmed), "CREATE") || strings.HasPrefix(trimmed, ")") {
				continue
			}
			upper := strings.ToUpper(trimmed)
			// Skip non-DDL statements (INSERT, ON, etc.).
			if strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "ON ") || strings.HasPrefix(upper, "VALUES") {
				continue
			}
			if strings.HasPrefix(upper, "PRIMARY KEY") || strings.HasPrefix(upper, "UNIQUE") || strings.HasPrefix(upper, "CHECK") || strings.HasPrefix(upper, "FOREIGN KEY") || strings.HasPrefix(upper, "CONSTRAINT") {
				continue
			}
			m := colRe.FindStringSubmatch(trimmed)
			if m == nil {
				continue
			}
			colName := m[1]
			if strings.Contains(upper, "PRIMARY KEY") || strings.Contains(upper, "NOT NULL") {
				// FK + DEFAULT 0 패턴: 참조 대상 테이블에 id=0 센티널 레코드 확인.
				if strings.Contains(upper, "DEFAULT 0") && strings.Contains(upper, "REFERENCES") {
					refMatch := refRe.FindStringSubmatch(trimmed)
					if refMatch != nil {
						refTable := refMatch[1]
						if refContent, ok := tableContents[refTable]; ok {
							if !hasSentinelInsert(refContent, refTable) {
								errs = append(errs, fmt.Sprintf("DDL: 테이블 %q 컬럼 %q — FK + DEFAULT 0이지만 참조 대상 %q에 id=0 센티널 레코드가 없습니다. INSERT INTO %s (id, ...) VALUES (0, ...) ON CONFLICT DO NOTHING; 을 추가하세요", f.tableName, colName, refTable, refTable))
							}
						}
					}
				}
				continue
			}
			errs = append(errs, fmt.Sprintf("DDL: 테이블 %q 컬럼 %q — NOT NULL이 없습니다. NOT NULL DEFAULT 값을 지정하세요", f.tableName, colName))
		}
	}
	return errs
}

// hasSentinelInsert checks if the DDL content contains an INSERT with id=0 for the given table.
func hasSentinelInsert(content, tableName string) bool {
	upper := strings.ToUpper(content)
	// INSERT INTO <table> ... VALUES (0, ...)
	insertRe := regexp.MustCompile(`(?i)INSERT\s+INTO\s+` + tableName + `\b`)
	if !insertRe.MatchString(content) {
		return false
	}
	// VALUES 절에서 첫 번째 값이 0인지 확인.
	valuesRe := regexp.MustCompile(`(?i)VALUES\s*\(\s*0\s*,`)
	idx := insertRe.FindStringIndex(upper)
	if idx == nil {
		return false
	}
	return valuesRe.MatchString(content[idx[0]:])
}

// checkPathParamConflicts detects path param name conflicts at the same segment position.
// e.g. /gigs/{ID} and /gigs/{GigID}/proposals conflict because segment[1] has both {ID} and {GigID}.
func checkPathParamConflicts(doc *openapi3.T) []string {
	if doc == nil || doc.Paths == nil {
		return nil
	}

	// Group: "prefix" → map[paramName][]fullPath
	// prefix is the path up to but not including the param segment, plus position index.
	// e.g. "/gigs/{ID}" → prefix="/gigs/", position=1
	type segKey struct {
		prefix   string
		position int
	}
	paramAt := make(map[segKey]map[string][]string) // segKey → paramName → []paths

	for path := range doc.Paths.Map() {
		segments := strings.Split(strings.Trim(path, "/"), "/")
		for i, seg := range segments {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				paramName := seg[1 : len(seg)-1]
				key := segKey{
					prefix:   strings.Join(segments[:i], "/"),
					position: i,
				}
				if paramAt[key] == nil {
					paramAt[key] = make(map[string][]string)
				}
				paramAt[key][paramName] = append(paramAt[key][paramName], path)
			}
		}
	}

	var errs []string
	for key, names := range paramAt {
		if len(names) <= 1 {
			continue
		}
		var nameList []string
		for n := range names {
			nameList = append(nameList, "{"+n+"}")
		}
		errs = append(errs, fmt.Sprintf(
			"path param 충돌: segment[%d] (prefix=/%s/)에 %s가 혼재 — 이름을 통일하세요",
			key.position, key.prefix, strings.Join(nameList, ", "),
		))
	}
	return errs
}
