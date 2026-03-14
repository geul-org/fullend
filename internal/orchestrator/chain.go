package orchestrator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jinzhu/inflection"

	"github.com/geul-org/fullend/internal/contract"
	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
)

// ChainLink represents one SSOT or artifact node in a feature chain.
type ChainLink struct {
	Kind      string // "OpenAPI", "SSaC", "DDL", "Rego", "StateDiag", "FuncSpec", "Hurl", "STML", "Handler", "Model", "Authz", "Types"
	File      string // relative path from specs-dir or artifacts-dir
	Line      int    // 1-based line number, 0 if unknown
	Summary   string // brief description of the match
	Ownership string // "", "gen", "preserve" (empty for SSOT nodes)
}

// Chain traces all SSOT nodes connected to the given operationId.
func Chain(specsDir string, operationID string) ([]ChainLink, error) {
	abs, err := filepath.Abs(specsDir)
	if err != nil {
		return nil, err
	}

	detected, err := DetectSSOTs(abs)
	if err != nil {
		return nil, err
	}

	has := make(map[SSOTKind]DetectedSSOT)
	for _, d := range detected {
		has[d.Kind] = d
	}

	// Parse all available SSOTs once.
	parsed := ParseAll(abs, detected, nil)

	allFuncSpecs := append(parsed.ProjectFuncSpecs, parsed.FullendPkgSpecs...)

	// Trace the chain.
	var links []ChainLink

	// 1. OpenAPI
	if parsed.OpenAPIDoc != nil {
		link := traceOpenAPI(parsed.OpenAPIDoc, operationID, abs)
		if link != nil {
			links = append(links, *link)
		} else {
			return nil, fmt.Errorf("operationId %q not found in OpenAPI", operationID)
		}
	}

	// Find the matching SSaC function.
	var matchedFunc *ssacparser.ServiceFunc
	for i := range parsed.ServiceFuncs {
		if parsed.ServiceFuncs[i].Name == operationID {
			matchedFunc = &parsed.ServiceFuncs[i]
			break
		}
	}

	// 2. SSaC
	if matchedFunc != nil {
		links = append(links, traceSSaC(matchedFunc, abs))
	}

	// 3. DDL — trace tables referenced by SSaC sequences
	if matchedFunc != nil && parsed.SymbolTable != nil {
		ddlLinks := traceDDL(matchedFunc, parsed.SymbolTable, abs)
		links = append(links, ddlLinks...)
	}

	// 4. Rego — trace policies referenced by @auth sequences
	if matchedFunc != nil && parsed.Policies != nil {
		regoLinks := tracePolicy(matchedFunc, parsed.Policies, abs)
		links = append(links, regoLinks...)
	}

	// 5. StateDiagram — trace diagrams referenced by @state sequences
	if matchedFunc != nil && parsed.StateDiagrams != nil {
		stateLinks := traceStates(matchedFunc, parsed.StateDiagrams, abs)
		links = append(links, stateLinks...)
	}

	// 6. FuncSpec — trace funcs referenced by @call sequences
	if matchedFunc != nil && len(allFuncSpecs) > 0 {
		funcLinks := traceFuncSpecs(matchedFunc, allFuncSpecs, abs)
		links = append(links, funcLinks...)
	}

	// 7. Hurl scenario — trace .hurl files referencing this endpoint
	if d, ok := has[KindScenario]; ok {
		hurlLinks := traceHurlScenarios(operationID, parsed.OpenAPIDoc, d.Path, abs)
		links = append(links, hurlLinks...)
	}

	// 8. STML — trace frontend files referencing this endpoint
	if parsed.OpenAPIDoc != nil {
		if d, ok := has[KindSTML]; ok {
			stmlLinks := traceSTML(parsed.OpenAPIDoc, operationID, d.Path, abs)
			links = append(links, stmlLinks...)
		}
	}

	// 9. Artifacts — trace generated code referencing this operationId
	if matchedFunc != nil {
		artifactsDir := inferArtifactsDir(abs)
		if artifactsDir != "" {
			artifactLinks := traceArtifacts(artifactsDir, operationID, matchedFunc)
			links = append(links, artifactLinks...)
		}
	}

	return links, nil
}

// inferArtifactsDir tries to find the artifacts directory for a specs dir.
// Convention: specs/<project> → artifacts/<project>
func inferArtifactsDir(specsDir string) string {
	base := filepath.Base(specsDir)
	candidate := filepath.Join(filepath.Dir(specsDir), "..", "artifacts", base)
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(abs); err == nil {
		return abs
	}
	// Also try: specsDir/../artifacts/<base>
	candidate = filepath.Join(specsDir, "..", "artifacts", base)
	abs, err = filepath.Abs(candidate)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(abs); err == nil {
		return abs
	}
	return ""
}

// traceArtifacts finds generated code artifacts connected to the operationId.
func traceArtifacts(artifactsDir, operationID string, sf *ssacparser.ServiceFunc) []ChainLink {
	var links []ChainLink

	funcs, err := contract.ScanDir(artifactsDir)
	if err != nil {
		return links
	}

	// Build SSOT path for matching.
	ssotPath := "service/" + sf.FileName
	if sf.Domain != "" {
		ssotPath = "service/" + sf.Domain + "/" + sf.FileName
	}

	for _, f := range funcs {
		if f.Directive.SSOT != ssotPath {
			continue
		}
		kind := "Handler"
		if strings.Contains(f.File, "/model/") {
			kind = "Model"
		} else if strings.Contains(f.File, "/authz/") {
			kind = "Authz"
		} else if strings.Contains(f.File, "/states/") {
			kind = "States"
		}
		links = append(links, ChainLink{
			Kind:      kind,
			File:      f.File,
			Summary:   f.Function,
			Ownership: f.Status,
		})
	}

	// Also trace model methods for tables used by this operation.
	for _, seq := range sf.Sequences {
		if seq.Model == "" || seq.Type == "call" {
			continue
		}
		parts := strings.SplitN(seq.Model, ".", 2)
		if len(parts) != 2 {
			continue
		}
		modelName := parts[0]
		methodName := parts[1]

		for _, f := range funcs {
			if f.Function == methodName && strings.Contains(f.File, "/model/") {
				// Check if it's for this model (DDL table).
				tableName := inflection.Plural(strings.ToLower(modelName))
				if strings.Contains(f.Directive.SSOT, tableName) {
					links = append(links, ChainLink{
						Kind:      "Model",
						File:      f.File,
						Summary:   modelName + "." + methodName,
						Ownership: f.Status,
					})
				}
			}
		}
	}

	// Deduplicate.
	seen := make(map[string]bool)
	var unique []ChainLink
	for _, l := range links {
		key := l.Kind + "|" + l.File + "|" + l.Summary
		if !seen[key] {
			seen[key] = true
			unique = append(unique, l)
		}
	}

	return unique
}

// --- trace functions ---

func traceOpenAPI(doc *openapi3.T, opID string, specsDir string) *ChainLink {
	if doc.Paths == nil {
		return nil
	}
	for path, pi := range doc.Paths.Map() {
		for method, op := range pi.Operations() {
			if op.OperationID == opID {
				line := grepLine(filepath.Join(specsDir, "api", "openapi.yaml"), "operationId: "+opID)
				return &ChainLink{
					Kind:    "OpenAPI",
					File:    "api/openapi.yaml",
					Line:    line,
					Summary: strings.ToUpper(method) + " " + path,
				}
			}
		}
	}
	return nil
}

func traceSSaC(sf *ssacparser.ServiceFunc, specsDir string) ChainLink {
	// Build sequence summary.
	var seqTypes []string
	seen := map[string]bool{}
	for _, seq := range sf.Sequences {
		tag := "@" + seq.Type
		if !seen[tag] {
			seqTypes = append(seqTypes, tag)
			seen[tag] = true
		}
	}

	// Find the file.
	relPath := findSSaCFile(sf, specsDir)
	line := 0
	if relPath != "" {
		line = grepLine(filepath.Join(specsDir, relPath), "func "+sf.Name)
	}

	return ChainLink{
		Kind:    "SSaC",
		File:    relPath,
		Line:    line,
		Summary: strings.Join(seqTypes, " "),
	}
}

func traceDDL(sf *ssacparser.ServiceFunc, st *ssacvalidator.SymbolTable, specsDir string) []ChainLink {
	tables := map[string]bool{}
	for _, seq := range sf.Sequences {
		if seq.Model == "" || seq.Type == "call" || seq.Type == "response" {
			continue
		}
		parts := strings.SplitN(seq.Model, ".", 2)
		if len(parts) < 2 {
			continue
		}
		tableName := inflection.Plural(toSnakeCase(parts[0]))
		if _, ok := st.DDLTables[tableName]; ok {
			tables[tableName] = true
		}
	}

	var links []ChainLink
	sortedTables := sortedStringKeys(tables)
	for _, table := range sortedTables {
		// Find the DDL file.
		relPath, line := findDDLTable(table, specsDir)
		links = append(links, ChainLink{
			Kind:    "DDL",
			File:    relPath,
			Line:    line,
			Summary: "CREATE TABLE " + table,
		})
	}
	return links
}

func tracePolicy(sf *ssacparser.ServiceFunc, policies []*policy.Policy, specsDir string) []ChainLink {
	resources := map[string]bool{}
	actions := map[string]bool{}
	for _, seq := range sf.Sequences {
		if seq.Type != "auth" {
			continue
		}
		if seq.Resource != "" {
			resources[seq.Resource] = true
		}
		if seq.Action != "" {
			actions[seq.Action] = true
		}
	}

	if len(resources) == 0 {
		return nil
	}

	var links []ChainLink
	seen := map[string]bool{}
	for _, p := range policies {
		for _, rule := range p.Rules {
			if !resources[rule.Resource] {
				continue
			}
			relPath, _ := filepath.Rel(specsDir, p.File)
			if relPath == "" {
				relPath = p.File
			}
			if seen[relPath] {
				continue
			}
			seen[relPath] = true

			line := grepLine(p.File, rule.Resource)
			var actList []string
			for _, a := range rule.Actions {
				if actions[a] {
					actList = append(actList, a)
				}
			}
			summary := "resource: " + rule.Resource
			if len(actList) > 0 {
				summary += " [" + strings.Join(actList, ", ") + "]"
			}
			links = append(links, ChainLink{
				Kind:    "Rego",
				File:    relPath,
				Line:    line,
				Summary: summary,
			})
		}
	}
	return links
}

func traceStates(sf *ssacparser.ServiceFunc, diagrams []*statemachine.StateDiagram, specsDir string) []ChainLink {
	diagramIDs := map[string]bool{}
	transitions := map[string]string{} // diagramID → transition name
	for _, seq := range sf.Sequences {
		if seq.Type != "state" {
			continue
		}
		diagramIDs[seq.DiagramID] = true
		transitions[seq.DiagramID] = seq.Transition
	}

	if len(diagramIDs) == 0 {
		return nil
	}

	var links []ChainLink
	for _, d := range diagrams {
		if !diagramIDs[d.ID] {
			continue
		}
		relPath := "states/" + d.ID + ".md"
		trans := transitions[d.ID]
		// Find the transition line.
		line := 0
		if trans != "" {
			line = grepLine(filepath.Join(specsDir, relPath), trans)
		}
		summary := "diagram: " + d.ID
		if trans != "" {
			summary += " → " + trans
		}
		links = append(links, ChainLink{
			Kind:    "StateDiag",
			File:    relPath,
			Line:    line,
			Summary: summary,
		})
	}
	return links
}

func traceFuncSpecs(sf *ssacparser.ServiceFunc, specs []funcspec.FuncSpec, specsDir string) []ChainLink {
	callPkgFuncs := map[string]string{} // "pkg.Func" → pkg
	for _, seq := range sf.Sequences {
		if seq.Type != "call" || seq.Model == "" {
			continue
		}
		parts := strings.SplitN(seq.Model, ".", 2)
		if len(parts) == 2 {
			callPkgFuncs[seq.Model] = parts[0]
		}
	}

	if len(callPkgFuncs) == 0 {
		return nil
	}

	var links []ChainLink
	for callRef, pkg := range callPkgFuncs {
		parts := strings.SplitN(callRef, ".", 2)
		funcName := ""
		if len(parts) == 2 {
			funcName = parts[1]
		}
		for _, spec := range specs {
			if spec.Package == pkg && strings.EqualFold(spec.Name, funcName) {
				relPath := "func/" + spec.Package + "/" + toSnakeCase(spec.Name) + ".go"
				// Try to find actual file.
				if _, err := os.Stat(filepath.Join(specsDir, relPath)); err != nil {
					// Try glob.
					matches, _ := filepath.Glob(filepath.Join(specsDir, "func", spec.Package, "*.go"))
					for _, m := range matches {
						if grepLine(m, "@func") > 0 && grepLine(m, funcName) > 0 {
							rel, _ := filepath.Rel(specsDir, m)
							relPath = rel
							break
						}
					}
				}
				line := grepLine(filepath.Join(specsDir, relPath), funcName)
				links = append(links, ChainLink{
					Kind:    "FuncSpec",
					File:    relPath,
					Line:    line,
					Summary: "@func " + callRef,
				})
				break
			}
		}
	}
	return links
}

func traceHurlScenarios(opID string, doc *openapi3.T, testsDir string, specsDir string) []ChainLink {
	if doc == nil || doc.Paths == nil {
		return nil
	}

	// Find the endpoint path for this operationId.
	var endpointPath string
	for path, pi := range doc.Paths.Map() {
		for _, op := range pi.Operations() {
			if op.OperationID == opID {
				endpointPath = path
				break
			}
		}
		if endpointPath != "" {
			break
		}
	}
	if endpointPath == "" {
		return nil
	}

	// Search .hurl files for the endpoint path.
	var links []ChainLink
	hurlFiles, _ := filepath.Glob(filepath.Join(testsDir, "*.hurl"))
	for _, f := range hurlFiles {
		line := grepLine(f, endpointPath)
		if line > 0 {
			relPath, _ := filepath.Rel(specsDir, f)
			links = append(links, ChainLink{
				Kind:    "Hurl",
				File:    relPath,
				Line:    line,
				Summary: "scenario: " + filepath.Base(f),
			})
		}
	}
	return links
}

func traceSTML(doc *openapi3.T, opID string, stmlDir string, specsDir string) []ChainLink {
	// Find the endpoint path for this operationId.
	var endpointPath, endpointMethod string
	if doc.Paths != nil {
		for path, pi := range doc.Paths.Map() {
			for method, op := range pi.Operations() {
				if op.OperationID == opID {
					endpointPath = path
					endpointMethod = strings.ToUpper(method)
					break
				}
			}
			if endpointPath != "" {
				break
			}
		}
	}
	if endpointPath == "" {
		return nil
	}

	// Grep STML files for the endpoint path.
	var links []ChainLink
	matches, _ := filepath.Glob(filepath.Join(stmlDir, "*.html"))
	for _, m := range matches {
		line := grepLine(m, endpointPath)
		if line > 0 {
			relPath, _ := filepath.Rel(specsDir, m)
			links = append(links, ChainLink{
				Kind:    "STML",
				File:    relPath,
				Line:    line,
				Summary: endpointMethod + " " + endpointPath,
			})
		}
	}
	return links
}

// --- helpers ---

func findSSaCFile(sf *ssacparser.ServiceFunc, specsDir string) string {
	// Try domain structure first.
	if sf.Domain != "" {
		rel := filepath.Join("service", sf.Domain, sf.FileName)
		if _, err := os.Stat(filepath.Join(specsDir, rel)); err == nil {
			return rel
		}
	}
	// Try flat structure.
	rel := filepath.Join("service", sf.FileName)
	if _, err := os.Stat(filepath.Join(specsDir, rel)); err == nil {
		return rel
	}
	return "service/" + sf.FileName
}

func findDDLTable(tableName string, specsDir string) (string, int) {
	dbDir := filepath.Join(specsDir, "db")
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return "db/?.sql", 0
	}
	createRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+` + regexp.QuoteMeta(tableName))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		fullPath := filepath.Join(dbDir, entry.Name())
		f, err := os.Open(fullPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if createRe.MatchString(scanner.Text()) {
				f.Close()
				return "db/" + entry.Name(), lineNum
			}
		}
		f.Close()
	}
	return "db/?.sql", 0
}

// grepLine returns the first line number (1-based) containing substr, or 0 if not found.
func grepLine(filePath string, substr string) int {
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if strings.Contains(scanner.Text(), substr) {
			return lineNum
		}
	}
	return 0
}

func toSnakeCase(s string) string {
	var result []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(r+'a'-'A'))
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

func sortedStringKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
