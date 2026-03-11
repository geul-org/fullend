package crosscheck

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CheckDDLCoverage validates that DDL tables and columns are referenced by SSaC/OpenAPI.
func CheckDDLCoverage(
	st *ssacvalidator.SymbolTable,
	funcs []ssacparser.ServiceFunc,
	doc *openapi3.T,
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

	// Build set of OpenAPI schema properties per model name.
	schemaProps := buildSchemaProps(doc)

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
				Suggestion: fmt.Sprintf("더 이상 사용하지 않는 테이블이면 DDL에 -- @archived를 추가하세요"),
			})
		}
	}

	// Rule 2: DDL column → OpenAPI schema property.
	for tableName, table := range st.DDLTables {
		if archived.Tables[tableName] {
			continue
		}
		if !referencedTables[tableName] {
			continue // already warned at table level
		}

		modelName := tableToModel(tableName)
		props, ok := schemaProps[modelName]
		if !ok {
			continue // no OpenAPI schema for this model
		}

		archivedCols := archived.Columns[tableName]

		for _, colName := range table.ColumnOrder {
			if archivedCols != nil && archivedCols[colName] {
				continue
			}
			pascalCol := snakeToPascal(colName)
			if !props[pascalCol] {
				errs = append(errs, CrossError{
					Rule:       "DDL → OpenAPI",
					Context:    fmt.Sprintf("%s.%s", tableName, colName),
					Message:    fmt.Sprintf("DDL 컬럼 %q가 OpenAPI %s 스키마에 없습니다", colName, modelName),
					Level:      "WARNING",
					Suggestion: "더 이상 사용하지 않는 컬럼이면 DDL에 -- @archived를 추가하세요",
				})
			}
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

// buildSchemaProps collects OpenAPI schema properties per schema name.
func buildSchemaProps(doc *openapi3.T) map[string]map[string]bool {
	props := make(map[string]map[string]bool)
	if doc == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return props
	}
	for name, ref := range doc.Components.Schemas {
		if ref.Value == nil {
			continue
		}
		m := make(map[string]bool)
		for propName := range ref.Value.Properties {
			m[propName] = true
		}
		props[name] = m
	}
	return props
}

// tableToModel converts a DDL table name to a model name.
// "courses" → "Course", "enrollments" → "Enrollment"
func tableToModel(table string) string {
	// Singularize.
	name := table
	if strings.HasSuffix(name, "ies") {
		name = name[:len(name)-3] + "y"
	} else if strings.HasSuffix(name, "sses") || strings.HasSuffix(name, "xes") {
		name = name[:len(name)-2]
	} else if strings.HasSuffix(name, "s") {
		name = name[:len(name)-1]
	}
	// PascalCase.
	return snakeToPascal(name)
}

// commonAcronyms are Go-style uppercase acronyms.
var commonAcronyms = map[string]string{
	"id": "ID", "url": "URL", "uri": "URI", "api": "API",
	"ip": "IP", "http": "HTTP", "https": "HTTPS",
	"json": "JSON", "xml": "XML", "sql": "SQL",
	"html": "HTML", "css": "CSS", "jwt": "JWT",
	"uuid": "UUID", "ssh": "SSH", "tls": "TLS",
}

// snakeToPascal converts snake_case to PascalCase with Go acronym handling.
// "instructor_id" → "InstructorID", "video_url" → "VideoURL"
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if acronym, ok := commonAcronyms[strings.ToLower(p)]; ok {
			result.WriteString(acronym)
		} else {
			result.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return result.String()
}
