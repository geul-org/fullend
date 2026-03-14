package crosscheck

import (
	"fmt"
	"strings"

	"github.com/ettle/strcase"
	"github.com/jinzhu/inflection"

	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
)

// CheckDDLCoverage validates that DDL tables and columns are referenced by SSaC/OpenAPI.
func CheckDDLCoverage(
	st *ssacvalidator.SymbolTable,
	funcs []ssacparser.ServiceFunc,
	archived *ArchivedInfo,
) []CrossError {
	var errs []CrossError

	if st == nil || len(st.DDLTables) == 0 {
		return errs
	}

	if archived == nil {
		archived = &ArchivedInfo{
			Tables:  make(map[string]bool),
			Columns: make(map[string]map[string]bool),
		}
	}

	// Build set of tables referenced by SSaC (@model and @result).
	referencedTables := buildReferencedTables(funcs)

	// Rule 1: DDL table → SSaC reference.
	for tableName := range st.DDLTables {
		if archived.Tables[tableName] {
			continue
		}
		if !referencedTables[tableName] {
			errs = append(errs, CrossError{
				Rule:       "DDL → SSaC",
				Context:    tableName,
				Message:    fmt.Sprintf("DDL 테이블 %q가 SSaC에서 참조되지 않습니다", tableName),
				Level:      "ERROR",
				Suggestion: "더 이상 사용하지 않는 테이블이면 DDL에 -- @archived를 추가하세요",
			})
		}
	}

	return errs
}

// buildReferencedTables collects DDL table names referenced by SSaC @model and @result.
func buildReferencedTables(funcs []ssacparser.ServiceFunc) map[string]bool {
	tables := make(map[string]bool)
	for _, fn := range funcs {
		for _, seq := range fn.Sequences {
			// 패키지 접두사 모델은 DDL 체크 스킵.
			if seq.Package != "" {
				continue
			}
			// @model "Course.FindByID" → "courses"
			if seq.Model != "" {
				parts := strings.SplitN(seq.Model, ".", 2)
				if len(parts) >= 1 {
					tables[modelToTable(parts[0])] = true
				}
			}
			// @result type
			if seq.Result != nil && seq.Result.Type != "" {
				typeName := strings.TrimPrefix(seq.Result.Type, "[]")
				typeName = strings.TrimPrefix(typeName, "*")
				if typeName != "" && !primitiveTypes[typeName] {
					tables[modelToTable(typeName)] = true
				}
			}
		}
	}
	return tables
}

// tableToModel converts a DDL table name to a model name.
// "courses" → "Course", "enrollments" → "Enrollment"
func tableToModel(table string) string {
	return snakeToPascal(inflection.Singular(table))
}

// snakeToPascal converts snake_case to PascalCase with Go acronym handling.
func snakeToPascal(s string) string {
	return strcase.ToGoPascal(s)
}
