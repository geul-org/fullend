package generator

import (
	"strings"

	"github.com/jinzhu/inflection"

	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/ssac/validator"
)

// varSource는 변수의 출처 정보를 나타낸다.
type varSource struct {
	Kind      string // "ddl" or "func"
	ModelName string // DDL: "Workflow", Func: "CheckCredits"
}

// FieldTypeResolver는 "변수.필드"의 Go 타입을 조회한다.
type FieldTypeResolver struct {
	vars map[string]varSource
	st   *validator.SymbolTable
	fs   []funcspec.FuncSpec
}

// ResolveFieldType은 dotted target의 필드 타입을 반환한다.
// "cr.Balance" → "int64", "wf.OrgID" → "int64", "wf" → "" (변수 자체는 default nil 유지)
func (r *FieldTypeResolver) ResolveFieldType(target string) string {
	parts := strings.SplitN(target, ".", 2)
	if len(parts) < 2 {
		return ""
	}
	varName, fieldName := parts[0], parts[1]
	src, ok := r.vars[varName]
	if !ok {
		return ""
	}
	switch src.Kind {
	case "ddl":
		tableName := inflection.Plural(toSnakeCase(src.ModelName))
		if table, ok := r.st.DDLTables[tableName]; ok {
			snakeField := toSnakeCase(fieldName)
			if goType, ok := table.Columns[snakeField]; ok {
				return goType
			}
		}
	case "func":
		for _, spec := range r.fs {
			if spec.Name == src.ModelName {
				for _, f := range spec.ResponseFields {
					if f.Name == fieldName {
						return f.Type
					}
				}
			}
		}
	}
	return ""
}
