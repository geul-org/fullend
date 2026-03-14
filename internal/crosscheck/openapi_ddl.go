package crosscheck

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ettle/strcase"
	"github.com/jinzhu/inflection"
	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
)

// CheckOpenAPIDDL validates x-sort, x-filter, x-include against DDL tables,
// and checks for ghost properties (OpenAPI schema properties not in DDL).
func CheckOpenAPIDDL(doc *openapi3.T, st *ssacvalidator.SymbolTable, funcs []ssacparser.ServiceFunc) []CrossError {
	var errs []CrossError

	if doc.Paths == nil {
		return errs
	}

	// Build funcName → first @model's table name for x-include FK lookup.
	funcPrimaryTable := buildFuncPrimaryTable(funcs)

	for path, pi := range doc.Paths.Map() {
		for method, op := range pi.Operations() {
			if op == nil {
				continue
			}
			ctx := fmt.Sprintf("%s %s (%s)", method, path, op.OperationID)
			primaryTable := funcPrimaryTable[op.OperationID]

			errs = append(errs, checkXSort(op, st, ctx)...)
			errs = append(errs, checkXFilter(op, st, ctx)...)
			errs = append(errs, checkXInclude(op, st, ctx, primaryTable)...)
			errs = append(errs, checkCursorSort(op, st, ctx)...)
		}
	}

	// Ghost property check: OpenAPI schema properties → DDL columns.
	errs = append(errs, checkGhostProperties(doc, st)...)

	return errs
}

// buildFuncPrimaryTable maps function names to their primary DDL table.
// Primary table is derived from the first @model annotation (e.g. "Reservation.FindByID" → "reservations").
func buildFuncPrimaryTable(funcs []ssacparser.ServiceFunc) map[string]string {
	m := make(map[string]string)
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			if seq.Model != "" {
				parts := strings.SplitN(seq.Model, ".", 2)
				if len(parts) >= 1 {
					m[fn.Name] = modelToTable(parts[0])
				}
				break
			}
		}
	}
	return m
}

func checkXSort(op *openapi3.Operation, st *ssacvalidator.SymbolTable, ctx string) []CrossError {
	var errs []CrossError

	raw, ok := op.Extensions["x-sort"]
	if !ok {
		return errs
	}

	var sortExt struct {
		Allowed []string `json:"allowed"`
	}
	if err := unmarshalExt(raw, &sortExt); err != nil {
		return errs
	}

	for _, col := range sortExt.Allowed {
		snake := pascalToSnake(col)
		if !columnExistsInAnyTable(snake, st) {
			table := inferTableFromCtx(op, st)
			errs = append(errs, CrossError{
				Rule:       "x-sort ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("x-sort column %q (→ %s) not found in any DDL table", col, snake),
				Suggestion: fmt.Sprintf("DDL에 추가: ALTER TABLE %s ADD COLUMN %s -- TODO: 타입 지정;", table, snake),
			})
		} else if !columnHasUsableIndex(snake, st) {
			table := findTableWithColumn(snake, st)
			errs = append(errs, CrossError{
				Rule:       "x-sort ↔ DDL index",
				Context:    ctx,
				Message:    fmt.Sprintf("x-sort column %q (→ %s) has no index (performance)", col, snake),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("DDL에 추가: CREATE INDEX idx_%s_%s ON %s(%s);", table, snake, table, snake),
			})
		}
	}

	return errs
}

func checkXFilter(op *openapi3.Operation, st *ssacvalidator.SymbolTable, ctx string) []CrossError {
	var errs []CrossError

	raw, ok := op.Extensions["x-filter"]
	if !ok {
		return errs
	}

	var filterExt struct {
		Allowed []string `json:"allowed"`
	}
	if err := unmarshalExt(raw, &filterExt); err != nil {
		return errs
	}

	for _, col := range filterExt.Allowed {
		snake := pascalToSnake(col)
		if !columnExistsInAnyTable(snake, st) {
			table := inferTableFromCtx(op, st)
			errs = append(errs, CrossError{
				Rule:       "x-filter ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("x-filter column %q (→ %s) not found in any DDL table", col, snake),
				Suggestion: fmt.Sprintf("DDL에 추가: ALTER TABLE %s ADD COLUMN %s -- TODO: 타입 지정;", table, snake),
			})
		}
	}

	return errs
}

func checkXInclude(op *openapi3.Operation, st *ssacvalidator.SymbolTable, ctx string, primaryTable string) []CrossError {
	var errs []CrossError

	raw, ok := op.Extensions["x-include"]
	if !ok {
		return errs
	}

	var includeExt struct {
		Allowed []string `json:"allowed"`
	}
	if err := unmarshalExt(raw, &includeExt); err != nil {
		return errs
	}

	for _, spec := range includeExt.Allowed {
		// Parse "column:table.column" format (e.g. "instructor_id:users.id").
		colonIdx := strings.Index(spec, ":")
		if colonIdx <= 0 {
			errs = append(errs, CrossError{
				Rule:       "x-include ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("x-include %q: invalid format, expected 'column:table.column'", spec),
				Suggestion: "예시: instructor_id:users.id",
			})
			continue
		}
		localCol := spec[:colonIdx]
		targetRef := spec[colonIdx+1:]
		dotIdx := strings.Index(targetRef, ".")
		if dotIdx <= 0 {
			errs = append(errs, CrossError{
				Rule:       "x-include ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("x-include %q: invalid format, expected 'column:table.column'", spec),
				Suggestion: "예시: instructor_id:users.id",
			})
			continue
		}
		targetTable := targetRef[:dotIdx]

		// Validate target table exists.
		if _, ok := st.DDLTables[targetTable]; !ok {
			errs = append(errs, CrossError{
				Rule:       "x-include ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("x-include %q: target table %q not found in DDL", spec, targetTable),
				Suggestion: fmt.Sprintf("DDL에 추가: CREATE TABLE %s (...);", targetTable),
			})
			continue
		}

		// Validate FK column exists in primary table and references target.
		if primaryTable != "" {
			if !hasFKColumn(primaryTable, localCol, targetTable, st) {
				errs = append(errs, CrossError{
					Rule:       "x-include ↔ DDL FK",
					Context:    ctx,
					Message:    fmt.Sprintf("x-include %q: column %s.%s does not reference %s", spec, primaryTable, localCol, targetTable),
					Level:      "WARNING",
					Suggestion: fmt.Sprintf("DDL에 추가: ALTER TABLE %s ADD COLUMN %s BIGINT REFERENCES %s(id);", primaryTable, localCol, targetTable),
				})
			}
		}
	}

	return errs
}

// checkGhostProperties detects OpenAPI schema properties that have no corresponding DDL column.
// Exceptions: x-include FK join fields, @dto models (no DDL table).
func checkGhostProperties(doc *openapi3.T, st *ssacvalidator.SymbolTable) []CrossError {
	var errs []CrossError

	if doc.Components == nil || doc.Components.Schemas == nil {
		return errs
	}

	// Collect x-include local field names (FK join fields are legitimate extensions).
	xIncludeFields := collectXIncludeLocalFields(doc)

	for schemaName, schemaRef := range doc.Components.Schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}
		schema := schemaRef.Value

		// Map schema name to DDL table.
		tableName := modelToTable(schemaName)
		table, ok := st.DDLTables[tableName]
		if !ok {
			// No DDL table for this schema — likely @dto or non-entity. Skip.
			continue
		}

		for propName := range schema.Properties {
			// Skip x-include FK join fields.
			if xIncludeFields[propName] {
				continue
			}
			if _, colExists := table.Columns[propName]; !colExists {
				errs = append(errs, CrossError{
					Rule:       "OpenAPI ↔ DDL",
					Context:    fmt.Sprintf("schema %s.%s", schemaName, propName),
					Message:    fmt.Sprintf("OpenAPI property %q — DDL %s에 해당 컬럼 없음 (유령 property)", propName, tableName),
					Level:      "ERROR",
					Suggestion: fmt.Sprintf("DDL에 추가하거나 OpenAPI에서 제거: %s.%s", tableName, propName),
				})
			}
		}
	}

	return errs
}

// collectXIncludeLocalFields collects local column names from x-include across all operations.
// e.g., x-include: [client_id:users.id] → "client_id" is a legitimate extension.
func collectXIncludeLocalFields(doc *openapi3.T) map[string]bool {
	result := make(map[string]bool)
	if doc.Paths == nil {
		return result
	}
	for _, pi := range doc.Paths.Map() {
		for _, op := range pi.Operations() {
			if op == nil {
				continue
			}
			raw, ok := op.Extensions["x-include"]
			if !ok {
				continue
			}
			var includeExt struct {
				Allowed []string `json:"allowed"`
			}
			if err := unmarshalExt(raw, &includeExt); err != nil {
				continue
			}
			for _, spec := range includeExt.Allowed {
				colonIdx := strings.Index(spec, ":")
				if colonIdx > 0 {
					localCol := spec[:colonIdx]
					result[localCol] = true
				}
			}
		}
	}
	return result
}

// inferTableFromCtx guesses the primary table name from the operation's path.
// e.g. "/courses/{CourseID}" → "courses", "/me/enrollments" → "enrollments"
func inferTableFromCtx(op *openapi3.Operation, st *ssacvalidator.SymbolTable) string {
	if op.OperationID != "" {
		// Try deriving from operationId: ListCourses → courses, GetCourse → courses
		name := op.OperationID
		for _, prefix := range []string{"List", "Get", "Create", "Update", "Delete"} {
			name = strings.TrimPrefix(name, prefix)
		}
		if name != "" {
			table := modelToTable(name)
			if _, ok := st.DDLTables[table]; ok {
				return table
			}
		}
	}
	return "???"
}

// findTableWithColumn returns the first table name containing the given column.
func findTableWithColumn(col string, st *ssacvalidator.SymbolTable) string {
	for tableName, table := range st.DDLTables {
		if _, ok := table.Columns[col]; ok {
			return tableName
		}
	}
	return "???"
}

// resolveTableName finds the DDL table for a resource name.
func resolveTableName(resource string, st *ssacvalidator.SymbolTable) string {
	snake := pascalToSnake(resource)
	candidates := []string{
		inflection.Plural(strings.ToLower(resource)),
		strings.ToLower(resource),
		inflection.Plural(snake),
		snake,
	}
	for _, c := range candidates {
		if _, ok := st.DDLTables[c]; ok {
			return c
		}
	}
	return ""
}

// hasFKTo checks if srcTable has a FK pointing to dstTable.
func hasFKTo(srcTable, dstTable string, st *ssacvalidator.SymbolTable) bool {
	table, ok := st.DDLTables[srcTable]
	if !ok {
		return false
	}
	for _, fk := range table.ForeignKeys {
		if fk.RefTable == dstTable {
			return true
		}
	}
	return false
}

// hasFKColumn checks if srcTable has a FK column named colName that references dstTable.
func hasFKColumn(srcTable, colName, dstTable string, st *ssacvalidator.SymbolTable) bool {
	table, ok := st.DDLTables[srcTable]
	if !ok {
		return false
	}
	for _, fk := range table.ForeignKeys {
		if fk.Column == colName && fk.RefTable == dstTable {
			return true
		}
	}
	return false
}

// columnHasUsableIndex checks if a column has a usable index (leading column or single-column index).
func columnHasUsableIndex(col string, st *ssacvalidator.SymbolTable) bool {
	for _, table := range st.DDLTables {
		if _, ok := table.Columns[col]; !ok {
			continue
		}
		for _, idx := range table.Indexes {
			if len(idx.Columns) > 0 && idx.Columns[0] == col {
				return true // Leading column in index
			}
			if len(idx.Columns) == 1 && idx.Columns[0] == col {
				return true // Single-column index
			}
		}
	}
	return false
}

// unmarshalExt handles kin-openapi extension values which may be json.RawMessage.
func unmarshalExt(v any, dst any) error {
	switch val := v.(type) {
	case json.RawMessage:
		return json.Unmarshal(val, dst)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, dst)
	}
}

// columnExistsInAnyTable checks if a snake_case column exists in any DDL table.
func columnExistsInAnyTable(snake string, st *ssacvalidator.SymbolTable) bool {
	for _, table := range st.DDLTables {
		if _, ok := table.Columns[snake]; ok {
			return true
		}
	}
	return false
}

// checkCursorSort validates cursor pagination + x-sort constraints.
// Rules:
// 1. cursor + x-sort allowed 2개 이상 → ERROR (런타임 정렬 전환 불가)
// 2. cursor + x-sort default가 DDL UNIQUE 아님 → ERROR (중복값 시 cursor 깨짐)
func checkCursorSort(op *openapi3.Operation, st *ssacvalidator.SymbolTable, ctx string) []CrossError {
	var errs []CrossError

	// Check if this operation uses cursor pagination.
	pagRaw, ok := op.Extensions["x-pagination"]
	if !ok {
		return errs
	}
	var pagExt struct {
		Style string `json:"style"`
	}
	if err := unmarshalExt(pagRaw, &pagExt); err != nil || pagExt.Style != "cursor" {
		return errs
	}

	// No x-sort → default id DESC, always OK.
	sortRaw, ok := op.Extensions["x-sort"]
	if !ok {
		return errs
	}
	var sortExt struct {
		Allowed   []string `json:"allowed"`
		Default   string   `json:"default"`
		Direction string   `json:"direction"`
	}
	if err := unmarshalExt(sortRaw, &sortExt); err != nil {
		return errs
	}

	// Rule 1: allowed가 2개 이상이면 ERROR.
	if len(sortExt.Allowed) > 1 {
		errs = append(errs, CrossError{
			Rule:    "x-pagination ↔ x-sort",
			Context: ctx,
			Message: fmt.Sprintf("cursor 모드에서 x-sort allowed가 %d개 — 런타임 정렬 전환은 cursor를 깨뜨립니다", len(sortExt.Allowed)),
			Level:   "ERROR",
		})
		return errs
	}

	// Rule 2: default 컬럼이 DDL UNIQUE인지 확인.
	defaultCol := sortExt.Default
	if defaultCol == "" && len(sortExt.Allowed) == 1 {
		defaultCol = sortExt.Allowed[0]
	}
	if defaultCol != "" {
		tableName := inferTableFromCtx(op, st)
		if tableName != "???" && !isUniqueColumn(defaultCol, tableName, st) {
			errs = append(errs, CrossError{
				Rule:       "x-pagination ↔ x-sort ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("cursor 모드의 x-sort default %q — DDL %s에서 UNIQUE가 아닙니다. 중복값 시 cursor가 깨집니다", defaultCol, tableName),
				Level:      "ERROR",
				Suggestion: fmt.Sprintf("DDL에 UNIQUE 제약 추가: ALTER TABLE %s ADD CONSTRAINT uniq_%s_%s UNIQUE (%s);", tableName, tableName, defaultCol, defaultCol),
			})
		}
	}

	return errs
}

// isUniqueColumn checks if a column is PRIMARY KEY or has a UNIQUE constraint.
func isUniqueColumn(col, tableName string, st *ssacvalidator.SymbolTable) bool {
	table, ok := st.DDLTables[tableName]
	if !ok {
		return false
	}
	for _, pk := range table.PrimaryKey {
		if pk == col {
			return true
		}
	}
	for _, idx := range table.Indexes {
		if idx.IsUnique && len(idx.Columns) == 1 && idx.Columns[0] == col {
			return true
		}
	}
	return false
}

// pascalToSnake converts PascalCase to snake_case.
func pascalToSnake(s string) string {
	return strcase.ToSnake(s)
}
