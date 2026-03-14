package orchestrator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/contract"
	"github.com/geul-org/fullend/internal/crosscheck"
	"github.com/geul-org/fullend/internal/genapi"
	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/projectconfig"
	"github.com/geul-org/fullend/internal/reporter"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
	stmlparser "github.com/geul-org/fullend/internal/stml/parser"
	stmlvalidator "github.com/geul-org/fullend/internal/stml/validator"
)

// allKinds defines the display order of SSOT kinds for validation.
var allKinds = []SSOTKind{KindConfig, KindOpenAPI, KindDDL, KindSSaC, KindModel, KindSTML, KindStates, KindPolicy, KindScenario, KindFunc}

// Validate runs individual SSOT validations on the detected sources,
// then runs cross-validation if OpenAPI + DDL + SSaC are all present.
// skipKinds specifies SSOT kinds to explicitly skip (via --skip flag).
func Validate(root string, detected []DetectedSSOT, skipKinds ...map[SSOTKind]bool) *reporter.Report {
	skip := make(map[SSOTKind]bool)
	if len(skipKinds) > 0 && skipKinds[0] != nil {
		skip = skipKinds[0]
	}

	// Parse all SSOTs once.
	parsed := ParseAll(root, detected, skip)

	return ValidateWith(root, detected, parsed, skip)
}

// ValidateWith runs validation using pre-parsed SSOTs.
func ValidateWith(root string, detected []DetectedSSOT, parsed *genapi.ParsedSSOTs, skip map[SSOTKind]bool) *reporter.Report {
	report := &reporter.Report{}

	has := make(map[SSOTKind]DetectedSSOT)
	for _, d := range detected {
		has[d.Kind] = d
	}

	done := make(map[SSOTKind]bool)

	// Emit steps in fixed order.
	for _, kind := range allKinds {
		if done[kind] {
			continue
		}

		// --skip takes precedence even if detected.
		if skip[kind] {
			report.Steps = append(report.Steps, reporter.StepResult{
				Name:    string(kind),
				Status:  reporter.Skip,
				Summary: "skipped (--skip)",
			})
			continue
		}

		d, ok := has[kind]
		if !ok {
			if kind == KindFunc {
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Skip,
					Summary: "no func/ directory",
				})
			} else if kind == KindStates {
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Skip,
					Summary: "no states/ directory",
				})
			} else if kind == KindPolicy {
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Skip,
					Summary: "no policy/ directory",
				})
			} else if kind == KindScenario {
				report.Steps = append(report.Steps, reporter.StepResult{
					Name:    string(kind),
					Status:  reporter.Pass,
					Summary: "no scenario tests",
					Errors:  []string{"[WARN] tests/scenario-*.hurl 파일이 없습니다 — 시나리오 테스트를 작성하세요 (--skip scenario로 억제 가능)"},
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
			report.Steps = append(report.Steps, validateConfig(d.Path, parsed.Config))
		case KindOpenAPI:
			report.Steps = append(report.Steps, validateOpenAPI(d.Path, parsed.OpenAPIDoc))
		case KindDDL:
			report.Steps = append(report.Steps, validateDDL(root, parsed.SymbolTable))
			// Run SSaC right after DDL to reuse symbol table.
			if _, ok := has[KindSSaC]; ok {
				report.Steps = append(report.Steps, validateSSaC(root, parsed.ServiceFuncs, parsed.SymbolTable))
				done[KindSSaC] = true
			}
		case KindSSaC:
			report.Steps = append(report.Steps, validateSSaC(root, parsed.ServiceFuncs, parsed.SymbolTable))
		case KindSTML:
			report.Steps = append(report.Steps, validateSTML(root, parsed.STMLPages))
		case KindStates:
			report.Steps = append(report.Steps, validateStates(parsed.StateDiagrams, parsed.StatesErr))
		case KindPolicy:
			report.Steps = append(report.Steps, validatePolicy(parsed.Policies))
		case KindScenario:
			step, files := validateScenarioHurl(d.Path, root)
			report.Steps = append(report.Steps, step)
			parsed.HurlFiles = files
		case KindFunc:
			report.Steps = append(report.Steps, validateFunc(parsed.ProjectFuncSpecs))
		case KindModel:
			report.Steps = append(report.Steps, validateModel(d.Path))
		}
	}

	// Cross-validation step.
	report.Steps = append(report.Steps, runCrossValidate(root, parsed))

	// Contract validation step (if artifacts exist).
	report.Steps = append(report.Steps, runContractValidate(root))

	return report
}

func runContractValidate(specsDir string) reporter.StepResult {
	step := reporter.StepResult{Name: "Contract"}

	// Infer artifacts dir: ../artifacts/<basename(specsDir)>
	base := filepath.Base(specsDir)
	artifactsDir := filepath.Join(filepath.Dir(specsDir), "artifacts", base)
	backendDir := filepath.Join(artifactsDir, "backend")

	if _, err := os.Stat(backendDir); os.IsNotExist(err) {
		step.Status = reporter.Skip
		step.Summary = "no artifacts"
		return step
	}

	funcs, err := contract.ScanDir(artifactsDir)
	if err != nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, err.Error())
		return step
	}

	if len(funcs) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no directives"
		return step
	}

	funcs = contract.Verify(specsDir, funcs)
	gen, preserve, broken, orphan := contract.Summary(funcs)

	parts := []string{}
	if gen > 0 {
		parts = append(parts, fmt.Sprintf("%d gen", gen))
	}
	if preserve > 0 {
		parts = append(parts, fmt.Sprintf("%d preserve", preserve))
	}
	if broken > 0 {
		parts = append(parts, fmt.Sprintf("%d broken", broken))
	}
	if orphan > 0 {
		parts = append(parts, fmt.Sprintf("%d orphan", orphan))
	}
	step.Summary = strings.Join(parts, ", ")

	if broken > 0 || orphan > 0 {
		step.Status = reporter.Fail
		for _, f := range funcs {
			if f.Status == "broken" || f.Status == "orphan" {
				step.Errors = append(step.Errors, fmt.Sprintf("%s: %s %s — %s", f.Status, f.File, f.Function, f.Detail))
			}
		}
	} else {
		step.Status = reporter.Pass
	}

	return step
}

func runCrossValidate(root string, parsed *genapi.ParsedSSOTs) reporter.StepResult {
	step := reporter.StepResult{Name: "Cross"}

	// Require OpenAPI + DDL + SSaC for cross-validation.
	if parsed.OpenAPIDoc == nil || parsed.SymbolTable == nil || parsed.ServiceFuncs == nil {
		step.Status = reporter.Skip
		step.Summary = "skipped (incomplete SSOT)"
		return step
	}

	// Load @dto types from model files.
	dtoTypes := loadDTOTypes(parsed.ModelDir)

	var middleware []string
	var claims map[string]string
	var roles []string
	if parsed.Config != nil {
		middleware = parsed.Config.Backend.Middleware
		if parsed.Config.Backend.Auth != nil {
			claims = parsed.Config.Backend.Auth.Claims
			roles = parsed.Config.Backend.Auth.Roles
		}
	}

	// Parse @archived tags from DDL files.
	archived, _ := crosscheck.ParseArchived(filepath.Join(root, "db"))

	// Parse @sensitive / @nosensitive tags from DDL files.
	sensitiveCols, noSensitiveCols, _ := crosscheck.ParseSensitive(filepath.Join(root, "db"))

	var queueBackend string
	if parsed.Config != nil && parsed.Config.Queue != nil {
		queueBackend = parsed.Config.Queue.Backend
	}

	var authzPackage string
	if parsed.Config != nil && parsed.Config.Authz != nil {
		authzPackage = parsed.Config.Authz.Package
	}

	input := &crosscheck.CrossValidateInput{
		ParsedSSOTs:     parsed,
		DTOTypes:        dtoTypes,
		Middleware:      middleware,
		Archived:        archived,
		Claims:          claims,
		QueueBackend:    queueBackend,
		AuthzPackage:    authzPackage,
		SensitiveCols:   sensitiveCols,
		NoSensitiveCols: noSensitiveCols,
		Roles:           roles,
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

func validateOpenAPI(path string, doc *openapi3.T) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindOpenAPI)}
	if doc == nil {
		// Parse failed in ParseAll; try again for error message.
		var err error
		doc, err = openapi3.NewLoader().LoadFromFile(path)
		if err != nil {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("OpenAPI load error: %v", err))
			return step
		}
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
	return step
}

func validateDDL(root string, st *ssacvalidator.SymbolTable) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindDDL)}
	if st == nil {
		// Parse failed in ParseAll; try again for error message.
		var err error
		st, err = ssacvalidator.LoadSymbolTable(root)
		if err != nil {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("DDL/SymbolTable load error: %v", err))
			return step
		}
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
	return step
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

func validateSSaC(root string, funcs []ssacparser.ServiceFunc, st *ssacvalidator.SymbolTable) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindSSaC)}
	if funcs == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "SSaC parse failed")
		return step
	}

	if st == nil {
		var stErr error
		st, stErr = ssacvalidator.LoadSymbolTable(root)
		if stErr != nil {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, fmt.Sprintf("SSaC symbol table load error: %v", stErr))
			return step
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
	return step
}

func validateStates(diagrams []*statemachine.StateDiagram, parseErr error) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindStates)}
	if diagrams == nil {
		step.Status = reporter.Fail
		if parseErr != nil {
			step.Errors = append(step.Errors, parseErr.Error())
		} else {
			step.Errors = append(step.Errors, "States parse failed")
		}
		return step
	}
	if len(diagrams) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no state diagrams found"
		return step
	}

	totalTransitions := 0
	for _, d := range diagrams {
		totalTransitions += len(d.Transitions)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d diagrams, %d transitions", len(diagrams), totalTransitions)
	return step
}

func validatePolicy(policies []*policy.Policy) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindPolicy)}
	if policies == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "Policy parse failed")
		return step
	}
	if len(policies) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no policy files found"
		return step
	}

	totalRules := 0
	totalOwnerships := 0
	for _, p := range policies {
		totalRules += len(p.Rules)
		totalOwnerships += len(p.Ownerships)
	}
	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d files, %d rules, %d ownership mappings", len(policies), totalRules, totalOwnerships)
	return step
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

func validateScenarioHurl(testsDir string, specsRoot string) (reporter.StepResult, []string) {
	step := reporter.StepResult{Name: string(KindScenario)}

	// Check for deprecated .feature files anywhere under specs root.
	scenarioDir := filepath.Join(specsRoot, "scenario")
	if featureFiles, _ := filepath.Glob(filepath.Join(scenarioDir, "*.feature")); len(featureFiles) > 0 {
		step.Status = reporter.Fail
		for _, f := range featureFiles {
			rel, _ := filepath.Rel(specsRoot, f)
			step.Errors = append(step.Errors, fmt.Sprintf("%s: .feature is no longer supported. Delete this file.\n       Write scenario tests directly in Hurl format: tests/scenario-*.hurl\n       See: https://hurl.dev/docs/manual.html", rel))
		}
		return step, nil
	}

	// Collect scenario and invariant .hurl files.
	scenarioHurls, _ := filepath.Glob(filepath.Join(testsDir, "scenario-*.hurl"))
	invariantHurls, _ := filepath.Glob(filepath.Join(testsDir, "invariant-*.hurl"))
	allHurls := append(scenarioHurls, invariantHurls...)

	if len(allHurls) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no scenario .hurl files found"
		return step, nil
	}

	step.Status = reporter.Pass
	step.Summary = fmt.Sprintf("%d scenario hurl files", len(allHurls))
	return step, allHurls
}

func validateFunc(specs []funcspec.FuncSpec) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindFunc)}
	if specs == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "Func parse failed")
		return step
	}
	if len(specs) == 0 {
		step.Status = reporter.Skip
		step.Summary = "no func spec files found"
		return step
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
	return step
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

func validateSTML(root string, pages []stmlparser.PageSpec) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindSTML)}
	if pages == nil {
		step.Status = reporter.Fail
		step.Errors = append(step.Errors, "STML parse failed")
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

func validateConfig(path string, cfg *projectconfig.ProjectConfig) reporter.StepResult {
	step := reporter.StepResult{Name: string(KindConfig)}
	if cfg == nil {
		// Parse failed in ParseAll; try again for error message.
		var err error
		cfg, err = projectconfig.Load(filepath.Dir(path))
		if err != nil {
			step.Status = reporter.Fail
			step.Errors = append(step.Errors, err.Error())
			return step
		}
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
	return step
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
