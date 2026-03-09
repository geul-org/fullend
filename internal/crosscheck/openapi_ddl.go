package crosscheck

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CheckOpenAPIDDL validates x-sort, x-filter, x-include against DDL tables.
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
		}
	}

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
	candidates := []string{
		strings.ToLower(resource) + "s",
		strings.ToLower(resource),
		pascalToSnake(resource) + "s",
		pascalToSnake(resource),
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

// pascalToSnake converts PascalCase to snake_case.
// e.g. "RoomID" → "room_id", "StartAt" → "start_at", "CreatedAt" → "created_at"
func pascalToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			prev := rune(s[i-1])
			if unicode.IsLower(prev) {
				result = append(result, '_')
			} else if unicode.IsUpper(prev) && i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}
