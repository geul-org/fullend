package crosscheck

import (
	"fmt"
	"strings"

	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// primitiveTypes are Go types that never map to DDL tables.
var primitiveTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
	"string": true, "bool": true, "byte": true, "rune": true,
	"error": true, "any": true,
}

// CheckSSaCDDL validates SSaC @result types and @param types against DDL.
func CheckSSaCDDL(funcs []ssacparser.ServiceFunc, st *ssacvalidator.SymbolTable, dtoTypes map[string]bool) []CrossError {
	var errs []CrossError

	for _, fn := range funcs {
		ctx := fmt.Sprintf("%s:%s", fn.FileName, fn.Name)

		for i, seq := range fn.Sequences {
			// Rule 4: @result Type ↔ DDL table
			if seq.Result != nil && seq.Result.Type != "" {
				errs = append(errs, checkResultType(seq, st, ctx, i, dtoTypes)...)
			}

			// Rule 5: @param type ↔ DDL column type (when @model is present)
			if seq.Model != "" {
				errs = append(errs, checkParamTypes(seq, st, ctx, i)...)
			}
		}
	}

	return errs
}

// normalizeTypeName strips slice prefix and pointer prefix from a type name.
// e.g. "[]Reservation" → "Reservation", "*User" → "User"
func normalizeTypeName(t string) string {
	t = strings.TrimPrefix(t, "[]")
	t = strings.TrimPrefix(t, "*")
	return t
}

func checkResultType(seq ssacparser.Sequence, st *ssacvalidator.SymbolTable, ctx string, seqIdx int, dtoTypes map[string]bool) []CrossError {
	var errs []CrossError

	typeName := normalizeTypeName(seq.Result.Type)

	// Skip primitive Go types.
	if primitiveTypes[typeName] {
		return errs
	}

	// Skip @dto types (no DDL table).
	if dtoTypes != nil && dtoTypes[typeName] {
		return errs
	}

	tableName := modelToTable(typeName)

	if _, ok := st.DDLTables[tableName]; !ok {
		// Not all types map to DDL tables (e.g. Token, Refund are DTOs).
		// Emit as WARNING.
		errs = append(errs, CrossError{
			Rule:       "SSaC @result ↔ DDL",
			Context:    ctx,
			Message:    fmt.Sprintf("seq[%d] @result type %q has no matching DDL table %q", seqIdx, seq.Result.Type, tableName),
			Level:      "WARNING",
			Suggestion: fmt.Sprintf("DDL에 추가: CREATE TABLE %s (...); 또는 model에 // @dto 선언", tableName),
		})
	}

	return errs
}

func checkParamTypes(seq ssacparser.Sequence, st *ssacvalidator.SymbolTable, ctx string, seqIdx int) []CrossError {
	var errs []CrossError

	// Extract table name from @model (e.g. "User.FindByEmail" → "users")
	parts := strings.SplitN(seq.Model, ".", 2)
	if len(parts) < 2 {
		return errs
	}
	modelName := parts[0]
	tableName := modelToTable(modelName)

	table, ok := st.DDLTables[tableName]
	if !ok {
		return errs // Table not found; already caught by other rules
	}

	for _, p := range seq.Params {
		if p.Source != "request" {
			continue // Only check request params that map to columns
		}

		// Use explicit column mapping if provided (e.g. @param PaymentMethod request -> method).
		colName := p.Column
		if colName == "" {
			colName = pascalToSnake(p.Name)
		}

		// Handle {Model}ID → id pattern.
		// e.g. @model Room.FindByID with @param RoomID → check "id" column.
		if p.Column == "" && strings.EqualFold(p.Name, modelName+"ID") {
			colName = "id"
		}

		if _, ok := table.Columns[colName]; !ok {
			errs = append(errs, CrossError{
				Rule:       "SSaC @param ↔ DDL",
				Context:    ctx,
				Message:    fmt.Sprintf("seq[%d] @param %s (→ %s) not found in table %s", seqIdx, p.Name, colName, tableName),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("DDL에 추가: ALTER TABLE %s ADD COLUMN %s -- TODO: 타입 지정;", tableName, colName),
			})
		}
	}

	return errs
}

// modelToTable converts a model name to a table name.
// e.g. "User" → "users", "Reservation" → "reservations", "Room" → "rooms"
func modelToTable(model string) string {
	snake := pascalToSnake(model)
	if strings.HasSuffix(snake, "s") {
		return snake
	}
	return snake + "s"
}
