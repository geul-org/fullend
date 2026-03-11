package gluegen

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	ssacparser "github.com/geul-org/ssac/parser"
)

// ddlColumn represents a column parsed from a CREATE TABLE statement.
type ddlColumn struct {
	Name    string // e.g. "instructor_id"
	GoName  string // e.g. "InstructorID"
	GoType  string // e.g. "int64"
	FKTable string // e.g. "users" — REFERENCES target table (empty if no FK)
	NotNull bool   // true if column has NOT NULL or is PRIMARY KEY
}

// includeMapping represents a resolved x-include → DDL FK mapping (forward FK only).
type includeMapping struct {
	IncludeName string // "instructor" — derived from FK column (strip _id)
	FieldName   string // "Instructor"
	FieldType   string // "*User"
	FKColumn    string // "instructor_id"
	TargetTable string // "users"
	TargetModel string // "User"
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
func generateModelImpls(intDir string, models []string, modulePath, specsDir string, serviceFuncs []ssacparser.ServiceFunc, modelIncludeSpecs map[string][]string) error {
	if len(models) == 0 {
		return nil
	}

	modelDir := filepath.Join(intDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return err
	}

	// Parse DDL files to get table/column info.
	tables := parseDDLFiles(specsDir)

	// Resolve per-model includes against DDL FK.
	includesByModel := make(map[string][]includeMapping)
	for modelName, specs := range modelIncludeSpecs {
		mappings, err := resolveIncludes(modelName, specs, tables)
		if err != nil {
			return fmt.Errorf("resolve includes for %s: %w", modelName, err)
		}
		if len(mappings) > 0 {
			includesByModel[modelName] = mappings
		}
	}

	// Parse query SQL files to get embedded SQL and metadata.
	queriesByModel := parseQueryFiles(specsDir)

	// Parse models_gen.go to get exact interface signatures.
	ifaceMethods := parseModelsGen(modelDir)

	// Collect per-model methods from service functions (for seq type info).
	seqTypeByModel := collectSeqTypes(serviceFuncs)

	// Generate types.go from DDL.
	if err := generateTypesFile(modelDir, models, tables, includesByModel); err != nil {
		return fmt.Errorf("types.go: %w", err)
	}

	// Generate queryopts.go (parseQueryOpts + SQL builders).
	if err := generateQueryOpts(modelDir); err != nil {
		return fmt.Errorf("queryopts.go: %w", err)
	}

	// Generate per-model implementation files.
	for _, m := range models {
		methods := ifaceMethods[m]
		table := tables[m]
		queries := queriesByModel[m]
		seqTypes := seqTypeByModel[m]
		if err := generateModelFile(modelDir, m, methods, table, queries, seqTypes, includesByModel[m]); err != nil {
			return fmt.Errorf("%s.go: %w", strings.ToLower(m), err)
		}
	}

	// Generate include helpers if any model has includes.
	if len(includesByModel) > 0 {
		if err := generateIncludeHelpersFile(modelDir); err != nil {
			return fmt.Errorf("include_helpers.go: %w", err)
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
func generateTypesFile(modelDir string, models []string, tables map[string]*ddlTable, includesByModel map[string][]includeMapping) error {
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
		if includes, ok := includesByModel[m]; ok && len(includes) > 0 {
			b.WriteString("\n\t// Include fields\n")
			for _, inc := range includes {
				jsonTag := lcFirst(inc.FieldName)
				b.WriteString(fmt.Sprintf("\t%-12s %s `json:\"%s,omitempty\"`\n", inc.FieldName, inc.FieldType, jsonTag))
			}
		}
		b.WriteString("}\n")
		if i < len(models)-1 {
			b.WriteString("\n")
		}
	}

	return os.WriteFile(filepath.Join(modelDir, "types.go"), []byte(b.String()), 0644)
}

// generateModelFile creates model/{model}.go with the implementation struct using *sql.DB.
func generateModelFile(modelDir string, modelName string, methods []ifaceMethod, table *ddlTable, queries map[string]sqlcQuery, seqTypes map[string]string, includes []includeMapping) error {
	var b strings.Builder
	lowerName := strings.ToLower(modelName)
	implName := lowerName + "ModelImpl"

	b.WriteString("package model\n\n")

	// Check if any method returns pagination.Page or pagination.Cursor.
	needsPagination := false
	for _, method := range methods {
		if strings.Contains(method.ReturnSig, "pagination.Page[") || strings.Contains(method.ReturnSig, "pagination.Cursor[") {
			needsPagination = true
			break
		}
	}

	b.WriteString("import (\n")
	b.WriteString("\t\"context\"\n")
	b.WriteString("\t\"database/sql\"\n")
	if needsPagination {
		b.WriteString("\n\t\"github.com/geul-org/fullend/pkg/pagination\"\n")
	}
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
		generateMethodFromIface(&b, implName, modelName, method, &query, seqType, table, includes)
	}

	// Generate include helper methods.
	for _, inc := range includes {
		b.WriteString("\n")
		generateIncludeHelper(&b, implName, modelName, inc)
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
func generateMethodFromIface(b *strings.Builder, implName, modelName string, m ifaceMethod, query *sqlcQuery, seqType string, table *ddlTable, includes []includeMapping) {
	sqlStr := "-- TODO: " + m.Name
	if query != nil && query.SQL != "" {
		sqlStr = query.SQL
	}

	// Build call args from interface params (excluding QueryOpts params).
	// For INSERT/UPDATE, reorder args to match SQL column order from sqlcQuery.Columns.
	var callArgNames []string
	if query != nil && len(query.Columns) > 0 && len(m.Params) > 0 {
		// Build param lookup: goName (lowercase first) → param name.
		paramByCol := make(map[string]string) // sql_column → param name
		for _, p := range m.Params {
			if p.Type == "QueryOpts" {
				continue
			}
			// Convert param name (camelCase) to snake_case for matching.
			snakeName := goToSnake(p.Name)
			paramByCol[snakeName] = p.Name
		}
		// Reorder: follow SQL column order, then append unmatched WHERE params.
		matched := make(map[string]bool)
		for _, col := range query.Columns {
			if paramName, ok := paramByCol[col]; ok {
				callArgNames = append(callArgNames, paramName)
				matched[paramName] = true
			}
		}
		// Append remaining params not matched by columns (e.g. WHERE id = $N).
		for _, p := range m.Params {
			if p.Type == "QueryOpts" {
				continue
			}
			if !matched[p.Name] {
				callArgNames = append(callArgNames, p.Name)
			}
		}
	} else {
		for _, p := range m.Params {
			if p.Type == "QueryOpts" {
				continue
			}
			callArgNames = append(callArgNames, p.Name)
		}
	}
	callArgs := ""
	if len(callArgNames) > 0 {
		callArgs = ",\n\t\t" + strings.Join(callArgNames, ", ")
	}

	// Determine method pattern from return signature and seq type.
	// A List method with QueryOpts uses dynamic SQL with pagination/sort/filter.
	// A List method without QueryOpts is a simple query returning a slice.
	hasQueryOpts := false
	for _, p := range m.Params {
		if p.Type == "QueryOpts" {
			hasQueryOpts = true
			break
		}
	}
	isList := isListMethod(m.Name) && hasQueryOpts
	isPageReturn := strings.Contains(m.ReturnSig, "pagination.Page[")
	isFind := strings.HasPrefix(m.Name, "Find")

	// Check if return type is a slice (e.g. "[]Lesson" in "([]Lesson, error)").
	isSliceReturn := strings.Contains(m.ReturnSig, "[]")

	switch {
	case isList:
		// List method with dynamic SQL returning *pagination.Page[T] or ([]T, int, error).
		baseWhere := ""
		baseArgCount := 0
		if query != nil && query.SQL != "" {
			baseWhere, baseArgCount = extractBaseWhere(query.SQL)
		}

		tableName := ""
		if table != nil {
			tableName = table.TableName
		}

		b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))

		// Build base args from non-opts params.
		if len(callArgNames) > 0 {
			b.WriteString(fmt.Sprintf("\tbaseArgs := []interface{}{%s}\n", strings.Join(callArgNames, ", ")))
		}

		// Count query.
		b.WriteString(fmt.Sprintf("\tcountSQL, countArgs := BuildCountQuery(%q, %q, %d, opts)\n", tableName, baseWhere, baseArgCount))
		if len(callArgNames) > 0 {
			b.WriteString("\tcountArgs = append(baseArgs, countArgs...)\n")
		}
		b.WriteString("\tvar total int64\n")
		b.WriteString("\tif err := m.db.QueryRowContext(context.Background(), countSQL, countArgs...).Scan(&total); err != nil {\n")
		b.WriteString("\t\treturn nil, err\n")
		b.WriteString("\t}\n\n")

		// Select query.
		b.WriteString(fmt.Sprintf("\tselectSQL, selectArgs := BuildSelectQuery(%q, %q, %d, opts)\n", tableName, baseWhere, baseArgCount))
		if len(callArgNames) > 0 {
			b.WriteString("\tselectArgs = append(baseArgs, selectArgs...)\n")
		}
		b.WriteString("\trows, err := m.db.QueryContext(context.Background(), selectSQL, selectArgs...)\n")
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\treturn nil, err\n")
		b.WriteString("\t}\n")
		b.WriteString("\tdefer rows.Close()\n")
		b.WriteString(fmt.Sprintf("\titems := make([]%s, 0)\n", modelName))
		b.WriteString("\tfor rows.Next() {\n")
		b.WriteString(fmt.Sprintf("\t\tv, err := scan%s(rows)\n", modelName))
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn nil, err\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\titems = append(items, *v)\n")
		b.WriteString("\t}\n")
		// Include loading — always applied (x-include is codegen metadata, not runtime option).
		for _, inc := range includes {
			helperName := "include" + strings.ToUpper(inc.IncludeName[:1]) + inc.IncludeName[1:]
			b.WriteString(fmt.Sprintf("\tif err := m.%s(items); err != nil {\n", helperName))
			b.WriteString("\t\treturn nil, err\n")
			b.WriteString("\t}\n")
		}
		if isPageReturn {
			b.WriteString(fmt.Sprintf("\treturn &pagination.Page[%s]{Items: items, Total: total}, nil\n", modelName))
		} else {
			b.WriteString("\treturn items, total, nil\n")
		}
		b.WriteString("}\n")

	case isSliceReturn:
		// Multi-row query without pagination: ([]Type, error)
		b.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", implName, m.Name, m.ParamSig, m.ReturnSig))
		b.WriteString(fmt.Sprintf("\trows, err := m.db.QueryContext(context.Background(),\n\t\t%q%s)\n", sqlStr, callArgs))
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\treturn nil, err\n")
		b.WriteString("\t}\n")
		b.WriteString("\tdefer rows.Close()\n")
		b.WriteString(fmt.Sprintf("\titems := make([]%s, 0)\n", modelName))
		b.WriteString("\tfor rows.Next() {\n")
		b.WriteString(fmt.Sprintf("\t\tv, err := scan%s(rows)\n", modelName))
		b.WriteString("\t\tif err != nil {\n")
		b.WriteString("\t\t\treturn nil, err\n")
		b.WriteString("\t\t}\n")
		b.WriteString("\t\titems = append(items, *v)\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn items, nil\n")
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
			// Skip @call — package-level funcs are not models.
			if seq.Type == "call" {
				continue
			}
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
	fkRe := regexp.MustCompile(`REFERENCES\s+(\w+)\s*\(`)

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

			fkTable := ""
			if fkMatch := fkRe.FindStringSubmatch(line); fkMatch != nil {
				fkTable = fkMatch[1]
			}

			upperLine := strings.ToUpper(line)
			notNull := strings.Contains(upperLine, "NOT NULL") || strings.Contains(upperLine, "PRIMARY KEY")

			table.Columns = append(table.Columns, ddlColumn{
				Name:    colName,
				GoName:  snakeToGo(colName),
				GoType:  sqlTypeToGo(sqlType),
				FKTable: fkTable,
				NotNull: notNull,
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

	// For UPDATE: extract all column = $N patterns, ordered by $N position.
	// This maps interface params to the correct $N placeholder positions.
	if len(q.Columns) == 0 && strings.HasPrefix(strings.TrimSpace(strings.ToUpper(sql)), "UPDATE") {
		updateColNRe := regexp.MustCompile(`(\w+)\s*=\s*\$(\d+)`)
		colMatches := updateColNRe.FindAllStringSubmatch(sql, -1)
		if len(colMatches) > 0 {
			type colPos struct {
				col string
				pos int
			}
			var positions []colPos
			for _, m := range colMatches {
				pos, _ := strconv.Atoi(m[2])
				positions = append(positions, colPos{col: m[1], pos: pos})
			}
			sort.Slice(positions, func(i, j int) bool { return positions[i].pos < positions[j].pos })
			for _, cp := range positions {
				q.Columns = append(q.Columns, cp.col)
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

// goToSnake converts a Go camelCase/PascalCase name to snake_case.
// Handles consecutive uppercase (e.g. clientID → client_id, freelancerID → freelancer_id).
func goToSnake(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := runes[i-1]
				// Insert underscore before uppercase if previous is lowercase,
				// or if previous is uppercase and next is lowercase (e.g. "ID" at end is kept together).
				if prev >= 'a' && prev <= 'z' {
					b.WriteByte('_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' {
					b.WriteByte('_')
				}
			}
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
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

// fkColumnToFieldName converts a FK column name to a Go struct field name.
// "instructor_id" → "Instructor", "course_id" → "Course"
func fkColumnToFieldName(colName string) string {
	name := colName
	if strings.HasSuffix(name, "_id") {
		name = name[:len(name)-3]
	}
	return snakeToGo(name)
}

// resolveIncludes resolves x-include specs against DDL FK relationships.
// Format: "column:table.column" (e.g. "instructor_id:users.id"). Forward FK only.
func resolveIncludes(modelName string, includeSpecs []string, tables map[string]*ddlTable) ([]includeMapping, error) {
	currentTable := tables[modelName]
	if currentTable == nil {
		return nil, nil
	}

	var mappings []includeMapping

	for _, spec := range includeSpecs {
		// Parse "instructor_id:users.id"
		colonIdx := strings.Index(spec, ":")
		if colonIdx <= 0 {
			return nil, fmt.Errorf("invalid x-include format %q: expected 'column:table.column'", spec)
		}
		localColumn := spec[:colonIdx]
		targetRef := spec[colonIdx+1:]

		dotIdx := strings.Index(targetRef, ".")
		if dotIdx <= 0 {
			return nil, fmt.Errorf("invalid x-include format %q: expected 'column:table.column'", spec)
		}
		targetTable := targetRef[:dotIdx]

		// Validate: localColumn exists in current table with FK to targetTable.
		var fkCol *ddlColumn
		for i, col := range currentTable.Columns {
			if col.Name == localColumn {
				if col.FKTable != targetTable {
					return nil, fmt.Errorf("x-include %q: column %s.%s does not reference %s (references %q)",
						spec, currentTable.TableName, localColumn, targetTable, col.FKTable)
				}
				fkCol = &currentTable.Columns[i]
				break
			}
		}
		if fkCol == nil {
			return nil, fmt.Errorf("x-include %q: column %s not found in table %s",
				spec, localColumn, currentTable.TableName)
		}

		includeName := strings.TrimSuffix(localColumn, "_id")
		fieldName := fkColumnToFieldName(localColumn)
		targetModelName := singularize(targetTable)

		mappings = append(mappings, includeMapping{
			IncludeName: includeName,
			FieldName:   fieldName,
			FieldType:   "*" + targetModelName,
			FKColumn:    localColumn,
			TargetTable: targetTable,
			TargetModel: targetModelName,
		})
	}

	return mappings, nil
}

// generateIncludeHelper generates a forward FK include helper method for a model.
func generateIncludeHelper(b *strings.Builder, implName, modelName string, inc includeMapping) {
	helperName := "include" + strings.ToUpper(inc.IncludeName[:1]) + inc.IncludeName[1:]
	fkGoName := snakeToGo(inc.FKColumn)

	b.WriteString(fmt.Sprintf("func (m *%s) %s(items []%s) error {\n", implName, helperName, modelName))
	b.WriteString("\tids := make(map[int64]bool)\n")
	b.WriteString("\tfor _, item := range items {\n")
	b.WriteString(fmt.Sprintf("\t\tids[item.%s] = true\n", fkGoName))
	b.WriteString("\t}\n")
	b.WriteString("\tif len(ids) == 0 {\n")
	b.WriteString("\t\treturn nil\n")
	b.WriteString("\t}\n")
	b.WriteString("\tkeys := collectInt64s(ids)\n")
	b.WriteString("\tplaceholders := buildPlaceholders(len(keys))\n")
	b.WriteString("\targs := int64sToArgs(keys)\n")
	b.WriteString(fmt.Sprintf("\trows, err := m.db.QueryContext(context.Background(),\n\t\t\"SELECT * FROM %s WHERE id IN (\"+placeholders+\")\", args...)\n", inc.TargetTable))
	b.WriteString("\tif err != nil {\n")
	b.WriteString("\t\treturn err\n")
	b.WriteString("\t}\n")
	b.WriteString("\tdefer rows.Close()\n")
	b.WriteString(fmt.Sprintf("\tlookup := make(map[int64]*%s)\n", inc.TargetModel))
	b.WriteString("\tfor rows.Next() {\n")
	b.WriteString(fmt.Sprintf("\t\tv, err := scan%s(rows)\n", inc.TargetModel))
	b.WriteString("\t\tif err != nil {\n")
	b.WriteString("\t\t\treturn err\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tlookup[v.ID] = v\n")
	b.WriteString("\t}\n")
	b.WriteString("\tfor i := range items {\n")
	b.WriteString(fmt.Sprintf("\t\titems[i].%s = lookup[items[i].%s]\n", inc.FieldName, fkGoName))
	b.WriteString("\t}\n")
	b.WriteString("\treturn nil\n")
	b.WriteString("}\n")
}

// generateIncludeHelpersFile creates model/include_helpers.go with shared utility functions.
func generateIncludeHelpersFile(modelDir string) error {
	var b strings.Builder
	b.WriteString("package model\n\n")
	b.WriteString("import (\n")
	b.WriteString("\t\"fmt\"\n")
	b.WriteString("\t\"strings\"\n")
	b.WriteString(")\n\n")

	b.WriteString("func collectInt64s(ids map[int64]bool) []int64 {\n")
	b.WriteString("\tkeys := make([]int64, 0, len(ids))\n")
	b.WriteString("\tfor k := range ids {\n")
	b.WriteString("\t\tkeys = append(keys, k)\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn keys\n")
	b.WriteString("}\n\n")

	b.WriteString("func buildPlaceholders(n int) string {\n")
	b.WriteString("\tps := make([]string, n)\n")
	b.WriteString("\tfor i := range ps {\n")
	b.WriteString("\t\tps[i] = fmt.Sprintf(\"$%d\", i+1)\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn strings.Join(ps, \", \")\n")
	b.WriteString("}\n\n")

	b.WriteString("func int64sToArgs(keys []int64) []interface{} {\n")
	b.WriteString("\targs := make([]interface{}, len(keys))\n")
	b.WriteString("\tfor i, k := range keys {\n")
	b.WriteString("\t\targs[i] = k\n")
	b.WriteString("\t}\n")
	b.WriteString("\treturn args\n")
	b.WriteString("}\n")

	return os.WriteFile(filepath.Join(modelDir, "include_helpers.go"), []byte(b.String()), 0644)
}
