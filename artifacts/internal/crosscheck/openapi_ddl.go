package crosscheck

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"

	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CheckOpenAPIDDL validates x-sort, x-filter, x-include against DDL tables.
func CheckOpenAPIDDL(doc *openapi3.T, st *ssacvalidator.SymbolTable) []CrossError {
	var errs []CrossError

	if doc.Paths == nil {
		return errs
	}

	for path, pi := range doc.Paths.Map() {
		for method, op := range pi.Operations() {
			if op == nil {
				continue
			}
			ctx := fmt.Sprintf("%s %s (%s)", method, path, op.OperationID)

			errs = append(errs, checkXSort(op, st, ctx)...)
			errs = append(errs, checkXFilter(op, st, ctx)...)
			errs = append(errs, checkXInclude(op, st, ctx)...)
		}
	}

	return errs
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
			errs = append(errs, CrossError{
				Rule:    "x-sort ↔ DDL",
				Context: ctx,
				Message: fmt.Sprintf("x-sort column %q (→ %s) not found in any DDL table", col, snake),
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
			errs = append(errs, CrossError{
				Rule:    "x-filter ↔ DDL",
				Context: ctx,
				Message: fmt.Sprintf("x-filter column %q (→ %s) not found in any DDL table", col, snake),
			})
		}
	}

	return errs
}

func checkXInclude(op *openapi3.Operation, st *ssacvalidator.SymbolTable, ctx string) []CrossError {
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

	for _, resource := range includeExt.Allowed {
		tableName := strings.ToLower(resource) + "s"
		if _, exists := st.DDLTables[tableName]; !exists {
			// Try without adding 's'
			if _, exists := st.DDLTables[strings.ToLower(resource)]; !exists {
				errs = append(errs, CrossError{
					Rule:    "x-include ↔ DDL",
					Context: ctx,
					Message: fmt.Sprintf("x-include resource %q has no matching DDL table", resource),
				})
			}
		}
	}

	return errs
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
