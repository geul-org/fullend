package gluegen

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ssacparser "github.com/geul-org/ssac/parser"
)

// ddlColumn represents a column parsed from a CREATE TABLE statement.
type ddlColumn struct {
	Name   string // e.g. "instructor_id"
	GoName string // e.g. "InstructorID"
	GoType string // e.g. "int64"
}

// ddlTable represents a parsed CREATE TABLE definition.
type ddlTable struct {
	TableName string      // e.g. "courses"
	ModelName string      // e.g. "Course"
	Columns   []ddlColumn // ordered columns
}

// sqlcQuery represents a parsed sqlc query annotation.
type sqlcQuery struct {
	Name        string   // e.g. "FindByID"
	Cardinality string   // "one", "many", "exec"
	SQL         string   // the raw SQL string
	ParamCount  int      // number of $N placeholders
	Columns     []string // INSERT/UPDATE column names (for param mapping)
}

// ifaceMethod represents a parsed interface method from models_gen.go.
type ifaceMethod struct {
	Name       string
	ParamSig   string // e.g. "courseID int64, opts QueryOpts"
	ReturnSig  string // e.g. "(*Course, error)"
	Params     []ifaceParam
}

// ifaceParam is a single parameter parsed from an interface method.
type ifaceParam struct {
	Name string
	Type string
}

// generateModelImpls generates model implementation files that use database/sql directly.
func generateModelImpls(intDir string, models []string, modulePath, specsDir string, serviceFuncs []ssacparser.ServiceFunc) error {
	if len(models) == 0 {
		return nil
	}

	modelDir := filepath.Join(intDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return err
	}

	// Parse DDL files to get table/column info.
	tables := parseDDLFiles(specsDir)

	// Parse query SQL files to get embedded SQL and metadata.
	queriesByModel := parseQueryFiles(specsDir)

	// Parse models_gen.go to get exact interface signatures.
	ifaceMethods := parseModelsGen(modelDir)

	// Collect per-model methods from service functions (for seq type info).
	seqTypeByModel := collectSeqTypes(serviceFuncs)

	// Generate types.go from DDL.
	if err := generateTypesFile(modelDir, models, tables); err != nil {
		return fmt.Errorf("types.go: %w", err)
	}

	// Generate per-model implementation files.
	for _, m := range models {
		methods := ifaceMethods[m]
		table := tables[m]
		queries := queriesByModel[m]
		seqTypes := seqTypeByModel[m]
		if err := generateModelFile(modelDir, m, methods, table, queries, seqTypes); err != nil {
			return fmt.Errorf("%s.go: %w", strings.ToLower(m), err)
		}
	}

	return nil
}

// parseModelsGen reads models_gen.go and extracts interface method signatures.
// Returns map[ModelName][]ifaceMethod.
func parseModelsGen(modelDir string) map[string][]ifaceMethod {
	result := make(map[string][]ifaceMethod)

	path := filepath.Join(modelDir, "models_gen.go")
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}

	// Parse "type XxxModel interface {" blocks.
	ifaceRe := regexp.MustCompile(`type\s+(\w+)Model\s+interface\s*\{`)
	// Parse method lines: "MethodName(params) (returns)"
	methodRe := regexp.MustCompile(`^\s+(\w+)\(([^)]*)\)\s*(.+)$`)
	// Parse individual params: "name type"
	paramRe := regexp.MustCompile(`(\w+)\s+([\w.*\[\]]+)`)

	lines := strings.Split(string(data), "\n")
	var currentModel string

	for _, line := range lines {
		if m := ifaceRe.FindStringSubmatch(line); m != nil {
			currentModel = m[1]
			continue
		}
		if currentModel != "" && strings.TrimSpace(line) == "}" {
			currentModel = ""
			continue
		}
		if currentModel != "" {
			if m := methodRe.FindStringSubmatch(line); m != nil {
				method := ifaceMethod{
					Name:      m[1],
					ParamSig:  m[2],
					ReturnSig: m[3],
				}
				// Parse individual params.
				for _, pm := range paramRe.FindAllStringSubmatch(m[2], -1) {
					method.Params = append(method.Params, ifaceParam{Name: pm[1], Type: pm[2]})
				}
				result[currentModel] = append(result[currentModel], method)
			}
		}
	}

	return result
}

// generateTypesFile creates model/types.go with struct definitions from DDL columns.
func generateTypesFile(modelDir string, models []string, tables map[string]*ddlTable) error {
	var b strings.Builder
	b.WriteString("package model\n\n")

	// Determine if we need time import.
	needsTime := false
	for _, m := range models {
		t := tables[m]
		if t == nil {
			continue
		}
		for _, col := range t.Columns {
			if col.GoType == "time.Time" {
				needsTime = true
				break
			}
		}
		if needsTime {
			break
		}
	}

	if needsTime {
		b.WriteString("import \"time\"\n\n")
	}

	for i, m := range models {
		t := tables[m]
		if t == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("type %s struct {\n", m))
		for _, col := range t.Columns {
			b.WriteString(fmt.Sprintf("\t%-12s %s `json:\"%s\"`\n", col.GoName, col.GoType, col.Name))
		}
		b.WriteString("}\n")
		if i < len(models)-1 {
			b.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(modelDir, "types.go"), []byte(b.String()), 0644)
}

// generateModelFile creates model/{model}.go with the implementation struct using *sql.DB.
func generateModelFile(modelDir string, modelName string, methods []ifaceMethod, table *ddlTable, queries map[string]sqlcQuery, seqTypes map[string]string) error {
	var b strings.Builder
	lowerName := strings.ToLower(modelName)
	implName := lowerName + "ModelImpl"

	b.WriteString("package model\n\n")

	b.WriteString("import (\n")
	b.WriteString("\t\"context\"\n")
	b.WriteString("\t\"database/sql\"\n")
	b.WriteString(")\n\n")

	// Struct definition.
	b.WriteString(fmt.Sprintf("type %s struct {\n", implName))
	b.WriteString("\tdb *sql.DB\n")
	b.WriteString("}\n\n")

	// Constructor.
	b.WriteString(fmt.Sprintf("func New%sModel(db *sql.DB) %sModel {\n", modelName, modelName))
	b.WriteString(fmt.Sprintf("\treturn &%s{db: db}\n", implName))
	b.WriteString("}\n")

	// Scan helper.
	if table != nil {
		b.WriteString("\n")
		generateScanFunc(&b, modelName, table)
	}

	// Generate methods.
	for _, method := range methods {
		b.WriteString("\n")
		query := queries[method.Name]
		seqType := seqTypes[method.Name]
		generateMethodFromIface(&b, implName, modelName, method, &query, seqType, table)
	}

	fileName := lowerName + ".go"
	return os.WriteFile(filepath.Join(modelDir, fileName), []byte(b.String()), 0644)
}

// generateScanFunc generates a scan helper function for a model.
func generateScanFunc(b *strings.Builder, modelName string, table *ddlTable) {
	lowerName := strings.ToLower(modelName[:1]) + modelName[1:]
	varName := string(lowerName[0])

	b.WriteString(fmt.Sprintf("func scan%s(s interface{ Scan(...interface{}) error }) (*%s, error) {\n", modelName, modelName))
	b.WriteString(fmt.Sprintf("\tvar %s %s\n", varName, modelName))

	scanFields := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		scanFields[i] = fmt.Sprintf("&%s.%s", varName, col.GoName)
	}

	b.WriteString(fmt.Sprintf("\terr := s.Scan(%s)\n", strings.Join(scanFields, ", ")))
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn nil, err\n")
	b.WriteString("\t}\n")
	b.WriteString(fmt.Sprintf("\treturn &%s, nil\n", varName))
	b.WriteString("}\n")
}

// generateMethodFromIface writes a single method implementation based on the interface signature.
func generateMethodFromIface(b *strings.Builder, implName, modelName string, m ifaceMethod, query *sqlcQuery, seqType string, table *ddlTable) {
	sqlStr := "-- TODO: " + m.Name
	if query != nil && query.SQL != "" {
		sqlStr = query.SQL
	}

	// Build call args from interface params (excluding QueryOpts params).
	var callArgNames []string
	for _, p := range m.Params {
		if p.Type == "QueryOpts" {
			continue
		}
		callArgNames = append(callArgNames, p.Name)
	}
	callArgs := ""
	if len(callArgNames) > 0 {
		callArgs = ",\n\t\t" + strings.Join(callArgNames, ", ")
	}

	// Determine method pattern from return signature and seq type.
	isList := isListMethod(m.Name)
	isFind := strings.HasPrefix(m.Name, "Find")

	switch {
	case isList:
		// List method: ([]Type, int, error)
		b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
		b.WriteString(fmt.Sprintf("\trows, err := m.db.QueryContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\treturn nil, 0, err\n")
		b.WriteString("\t}\n")
		b.WriteString("\tdefer rows.Close()\n")
		b.WriteString(fmt.Sprintf("\tvar items []%s\n", modelName))
		b.WriteString("\tfor rows.Next() {\n")
		b.WriteString(fmt.Sprintf("\t\tv, err := scan%s(rows)\n", modelName))
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn nil, 0, err\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\titems = append(items, *v)\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn items, len(items), nil\n")
		b.WriteString("}\n")

	case isFind || seqType == "get":
		// Find method: (*Type, error)
		b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
		b.WriteString(fmt.Sprintf("\trow := m.db.QueryRowContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
		b.WriteString(fmt.Sprintf("\tv, err := scan%s(row)\n", modelName))
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\tif err == sql.ErrNoRows {\n")
		b.WriteString("\t\t\treturn nil, nil\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\treturn nil, err\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn v, nil\n")
		b.WriteString("}\n")

	case seqType == "post":
		// Create method: (*Type, error)
		b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
		b.WriteString(fmt.Sprintf("\trow := m.db.QueryRowContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
		b.WriteString(fmt.Sprintf("\treturn scan%s(row)\n", modelName))
		b.WriteString("}\n")

	case seqType == "put" || seqType == "delete":
		// Update/Delete: error
		b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
		b.WriteString(fmt.Sprintf("\t_, err := m.db.ExecContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
		b.WriteString("\treturn err\n")
		b.WriteString("}\n")

	default:
		// Custom/unknown: determine from query cardinality or default to exec.
		if query != nil && query.Cardinality == "one" {
			b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
			b.WriteString(fmt.Sprintf("\trow := m.db.QueryRowContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
			b.WriteString(fmt.Sprintf("\treturn scan%s(row)\n", modelName))
			b.WriteString("}\n")
		} else {
			b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
			b.WriteString(fmt.Sprintf("\t_, err := m.db.ExecContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
			b.WriteString("\treturn err\n")
			b.WriteString("}\n")
		}
	}
}

// isListMethod returns true if the method name indicates a list query.
func isListMethod(name string) bool {
	return strings.HasPrefix(name, "List")
}

// stripModelPrefix removes the model name prefix from a sqlc query name.
// e.g. "CourseFindByID" with modelName "Course" -> "FindByID".
// If no prefix matches, returns the original name (backward compat).
func stripModelPrefix(queryName, modelName string) string {
	if strings.HasPrefix(queryName, modelName) && len(queryName) > len(modelName) {
		return queryName[len(modelName):]
	}
	return queryName
}

// collectSeqTypes extracts per-model method → sequence type mapping from service functions.
// Returns map[ModelName]map[MethodName]seqType.
func collectSeqTypes(funcs []ssacparser.ServiceFunc) map[string]map[string]string {
	result := make(map[string]map[string]string)

	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Model == "" {
				continue
			}
			parts := strings.SplitN(seq.Model, ".", 2)
			if len(parts) != 2 {
				continue
			}
			modelName := parts[0]
			methodName := parts[1]

			if result[modelName] == nil {
				result[modelName] = make(map[string]string)
			}
			result[modelName][methodName] = seq.Type
		}
	}

	return result
}

// parseDDLFiles parses CREATE TABLE statements from specsDir/db/*.sql.
func parseDDLFiles(specsDir string) map[string]*ddlTable {
	tables := make(map[string]*ddlTable)

	dbDir := filepath.Join(specsDir, "db")
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return tables
	}

	createRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(\w+)\s*\(`)
	// Match column definitions: name TYPE(...) constraints
	// Stop at lines starting with constraints or indexes.
	colRe := regexp.MustCompile(`^\s+(\w+)\s+(BIGSERIAL|BIGINT|INT|INTEGER|VARCHAR\(\d+\)|TEXT|BOOLEAN|BOOL|TIMESTAMPTZ|TIMESTAMP)`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(dbDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := string(data)
		tableMatch := createRe.FindStringSubmatch(content)
		if tableMatch == nil {
			continue
		}

		tableName := tableMatch[1]
		modelName := singularize(tableName)

		table := &ddlTable{
			TableName: tableName,
			ModelName: modelName,
		}

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			colMatch := colRe.FindStringSubmatch(line)
			if colMatch == nil {
				continue
			}
			colName := colMatch[1]
			sqlType := strings.ToUpper(colMatch[2])

			table.Columns = append(table.Columns, ddlColumn{
				Name:   colName,
				GoName: snakeToGo(colName),
				GoType: sqlTypeToGo(sqlType),
			})
		}

		tables[modelName] = table
	}

	return tables
}

// parseQueryFiles parses query SQL files from specsDir/db/queries/*.sql.
// Returns map[ModelName]map[MethodName]sqlcQuery.
func parseQueryFiles(specsDir string) map[string]map[string]sqlcQuery {
	result := make(map[string]map[string]sqlcQuery)

	queriesDir := filepath.Join(specsDir, "db", "queries")
	entries, err := os.ReadDir(queriesDir)
	if err != nil {
		return result
	}

	nameRe := regexp.MustCompile(`^--\s*name:\s*(\w+)\s+:(\w+)`)
	paramRe := regexp.MustCompile(`\$(\d+)`)
	insertColRe := regexp.MustCompile(`(?i)INSERT\s+INTO\s+\w+\s*\(([^)]+)\)`)
	updateSetRe := regexp.MustCompile(`(?i)SET\s+(.+?)(?:\s+WHERE|\s*;|\s*$)`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Derive model name from filename (e.g. "course.sql" -> "Course").
		baseName := strings.TrimSuffix(entry.Name(), ".sql")
		modelName := singularize(baseName)

		if result[modelName] == nil {
			result[modelName] = make(map[string]sqlcQuery)
		}

		path := filepath.Join(queriesDir, entry.Name())
		f, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		var currentQuery *sqlcQuery
		var sqlBuf strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			if matches := nameRe.FindStringSubmatch(line); matches != nil {
				// Save previous query.
				if currentQuery != nil {
					finishQuery(currentQuery, sqlBuf.String(), paramRe, insertColRe, updateSetRe)
					result[modelName][currentQuery.Name] = *currentQuery
				}
				currentQuery = &sqlcQuery{
					Name:        stripModelPrefix(matches[1], modelName),
					Cardinality: matches[2],
				}
				sqlBuf.Reset()
			} else if currentQuery != nil {
				sqlBuf.WriteString(line)
				sqlBuf.WriteString("\n")
			}
		}
		// Save last query in file.
		if currentQuery != nil {
			finishQuery(currentQuery, sqlBuf.String(), paramRe, insertColRe, updateSetRe)
			result[modelName][currentQuery.Name] = *currentQuery
		}
		f.Close()
	}

	return result
}

// finishQuery extracts param count, column names, and cleans up the SQL body.
func finishQuery(q *sqlcQuery, sql string, paramRe, insertColRe, updateSetRe *regexp.Regexp) {
	// Store cleaned SQL (trim trailing whitespace).
	q.SQL = strings.TrimSpace(sql)

	// Count parameters.
	matches := paramRe.FindAllString(sql, -1)
	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m] = true
	}
	q.ParamCount = len(seen)

	// Extract column names for INSERT.
	if insMatch := insertColRe.FindStringSubmatch(sql); insMatch != nil {
		cols := strings.Split(insMatch[1], ",")
		for _, c := range cols {
			q.Columns = append(q.Columns, strings.TrimSpace(c))
		}
	}

	// Extract column names for UPDATE SET.
	if updMatch := updateSetRe.FindStringSubmatch(sql); updMatch != nil {
		parts := strings.Split(updMatch[1], ",")
		for _, p := range parts {
			eqIdx := strings.Index(p, "=")
			if eqIdx > 0 {
				q.Columns = append(q.Columns, strings.TrimSpace(p[:eqIdx]))
			}
		}
	}
}

// singularize converts a plural table name to a singular model name.
// Rules: 'ies'-> 'y', 'sses'-> 'ss', 'xes'-> 'x', default strip trailing 's'.
func singularize(name string) string {
	lower := strings.ToLower(name)
	var singular string
	switch {
	case strings.HasSuffix(lower, "sses"):
		singular = lower[:len(lower)-2] // sses -> ss
	case strings.HasSuffix(lower, "xes"):
		singular = lower[:len(lower)-2] // xes -> x
	case strings.HasSuffix(lower, "ies"):
		singular = lower[:len(lower)-3] + "y" // ies -> y
	case strings.HasSuffix(lower, "s"):
		singular = lower[:len(lower)-1] // s -> (remove)
	default:
		singular = lower
	}
	// Capitalize first letter.
	if len(singular) == 0 {
		return name
	}
	return strings.ToUpper(singular[:1]) + singular[1:]
}

// snakeToGo converts a snake_case column name to a Go PascalCase field name.
func snakeToGo(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		// Special case: "id" -> "ID"
		if strings.ToLower(p) == "id" {
			b.WriteString("ID")
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// sqlTypeToGo maps a SQL type to a Go type.
func sqlTypeToGo(sqlType string) string {
	// Normalize: strip parenthesized args.
	upper := strings.ToUpper(sqlType)
	if idx := strings.Index(upper, "("); idx > 0 {
		upper = upper[:idx]
	}

	switch upper {
	case "BIGSERIAL", "BIGINT":
		return "int64"
	case "INT", "INTEGER":
		return "int64"
	case "VARCHAR", "TEXT":
		return "string"
	case "BOOLEAN", "BOOL":
		return "bool"
	case "TIMESTAMPTZ", "TIMESTAMP":
		return "time.Time"
	default:
		return "string"
	}
}
