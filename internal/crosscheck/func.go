package crosscheck

import (
	"fmt"
	"strings"

	"github.com/ettle/strcase"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/funcspec"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
)

// CheckFuncs validates SSaC @func references against parsed func specs.
// Checks: existence, stub, param count, positional type match, result/response match, source variable definition.
func CheckFuncs(
	serviceFuncs []ssacparser.ServiceFunc,
	fullendPkgSpecs, projectFuncSpecs []funcspec.FuncSpec,
	symbolTable *ssacvalidator.SymbolTable,
	openAPIDoc *openapi3.T,
) []CrossError {
	var errs []CrossError

	// Build lookup: "package.funcName" → FuncSpec.
	// Project custom overrides fullend default.
	specMap := make(map[string]*funcspec.FuncSpec)
	for i := range fullendPkgSpecs {
		key := fullendPkgSpecs[i].Package + "." + fullendPkgSpecs[i].Name
		specMap[key] = &fullendPkgSpecs[i]
	}
	for i := range projectFuncSpecs {
		key := projectFuncSpecs[i].Package + "." + projectFuncSpecs[i].Name
		specMap[key] = &projectFuncSpecs[i]
	}

	for _, sf := range serviceFuncs {
		// Track defined variables per function for rule 4.
		definedVars := make(map[string]string) // var name → result type

		for seqIdx, seq := range sf.Sequences {
			// Track @result variables from all sequence types.
			if seq.Result != nil {
				definedVars[seq.Result.Var] = seq.Result.Type
			}

			if seq.Type != "call" || seq.Model == "" {
				continue
			}

			// v2: seq.Model = "auth.VerifyPassword" or "billing.CalculateRefund"
			callParts := strings.SplitN(seq.Model, ".", 2)
			pkg := ""
			funcName := seq.Model
			if len(callParts) == 2 {
				pkg = callParts[0]
				funcName = callParts[1]
			}
			// Func spec uses camelCase name (e.g. "verifyPassword"), v2 Model has PascalCase.
			camelName := strcase.ToGoCamel(funcName)
			key := pkg + "." + camelName
			if pkg == "" {
				key = camelName
			}
			ctx := fmt.Sprintf("%s seq[%d] @call %s", sf.Name, seqIdx, key)

			spec, found := specMap[key]
			if !found {
				skeleton := generateSkeleton(pkg, camelName, seq)
				errs = append(errs, CrossError{
					Rule:       "Func ↔ SSaC",
					Context:    ctx,
					Message:    fmt.Sprintf("@call %s — 구현 없음", key),
					Level:      "ERROR",
					Suggestion: skeleton,
				})
				continue
			}

			// Check HasBody.
			if !spec.HasBody {
				errs = append(errs, CrossError{
					Rule:    "Func ↔ SSaC",
					Context: ctx,
					Message: "본체 미구현 (TODO)",
					Level:   "ERROR",
				})
			}

			// Check Func purity: I/O imports are forbidden in @call func.
			if ioImports := checkForbiddenImports(spec.Imports); len(ioImports) > 0 {
				for _, imp := range ioImports {
					errs = append(errs, CrossError{
						Rule:    "Func ↔ SSaC",
						Context: ctx,
						Message: fmt.Sprintf("@call func에서 I/O 패키지 %q import 금지. @call func은 순수 계산/판단 로직만 허용됩니다. DB, 네트워크, 파일 등 I/O가 필요하면 @model을 활용하세요.", imp),
						Level:   "ERROR",
					})
				}
			}

			// Rule 1: Input field count = Request field count.
			// @call uses seq.Inputs (map[string]string) since 수정지시서007.
			inputCount := len(seq.Inputs)
			reqFieldCount := len(spec.RequestFields)
			if inputCount != reqFieldCount {
				errs = append(errs, CrossError{
					Rule:    "Func ↔ SSaC",
					Context: ctx,
					Message: fmt.Sprintf("@call Inputs %d개, Request 필드 %d개 (불일치)", inputCount, reqFieldCount),
					Level:   "ERROR",
				})
			}

			// Rule 2: Input key names + types must match Request field names + types.
			if inputCount > 0 {
				reqFieldMap := make(map[string]string) // name → type
				for _, rf := range spec.RequestFields {
					reqFieldMap[rf.Name] = rf.Type
				}
				for inputKey, inputValue := range seq.Inputs {
					reqType, exists := reqFieldMap[inputKey]
					if !exists {
						errs = append(errs, CrossError{
							Rule:    "Func ↔ SSaC",
							Context: ctx,
							Message: fmt.Sprintf("@call Input 필드 %q가 %sRequest에 없음", inputKey, strcase.ToGoPascal(funcName)),
							Level:   "ERROR",
						})
						continue
					}
					// Type validation.
					allSpecs := append(fullendPkgSpecs, projectFuncSpecs...)
				valueType := resolveInputValueType(inputValue, definedVars, symbolTable, openAPIDoc, sf.Name, allSpecs)
					if valueType != "" && !typesCompatible(valueType, reqType) {
						errs = append(errs, CrossError{
							Rule:    "Func ↔ SSaC",
							Context: ctx,
							Message: fmt.Sprintf("@call Input %s 타입 불일치: %s(source) ≠ %s(Request)", inputKey, valueType, reqType),
							Level:   "ERROR",
						})
					}
				}
			}

			// Rule 3: Result ↔ Response match.
			if seq.Result != nil && len(spec.ResponseFields) == 0 {
				errs = append(errs, CrossError{
					Rule:    "Func ↔ SSaC",
					Context: ctx,
					Message: "@result 있지만 Response 필드 없음",
					Level:   "ERROR",
				})
			} else if seq.Result == nil && len(spec.ResponseFields) > 0 {
				errs = append(errs, CrossError{
					Rule:    "Func ↔ SSaC",
					Context: ctx,
					Message: "@result 없지만 Response 필드 존재 (반환값 무시)",
					Level:   "WARNING",
				})
			}

			// Rule 4: Source variable defined in prior sequences.
			for _, value := range seq.Inputs {
				parts := strings.SplitN(value, ".", 2)
				source := parts[0]
				if source == "request" || source == "currentUser" {
					continue
				}
				// Check if it's a literal (quoted string, numeric, boolean, nil).
				if strings.HasPrefix(value, "\"") {
					continue
				}
				if ssacparser.IsLiteral(value) {
					continue
				}
				if _, ok := definedVars[source]; !ok {
					errs = append(errs, CrossError{
						Rule:    "Func ↔ SSaC",
						Context: ctx,
						Message: fmt.Sprintf("arg source %q 미정의", source),
						Level:   "WARNING",
					})
				}
			}
		}
	}

	return errs
}

// resolveInputValueType resolves the Go type of an Input value string.
// Patterns: "request.Field" → OpenAPI, "var.Field" → DDL/FuncResponse, "\"literal\"" → string,
// bare variable → definedVars type, numeric/bool/nil literal → inferred type, "currentUser.*" → skip.
func resolveInputValueType(value string, definedVars map[string]string, st *ssacvalidator.SymbolTable, doc *openapi3.T, funcName string, funcSpecs []funcspec.FuncSpec) string {
	// Literal string.
	if strings.HasPrefix(value, "\"") {
		return "string"
	}

	// Numeric/boolean/nil literal.
	if ssacparser.IsLiteral(value) {
		return inferLiteralType(value)
	}

	// Bare variable (no dot) → return type from definedVars.
	parts := strings.SplitN(value, ".", 2)
	if len(parts) < 2 {
		typeName, ok := definedVars[value]
		if !ok {
			return ""
		}
		return typeName
	}
	source, field := parts[0], parts[1]

	// request.Field → OpenAPI.
	if source == "request" {
		return resolveOpenAPIFieldType(doc, funcName, field)
	}

	// currentUser → skip (claims type not tracked here).
	if source == "currentUser" {
		return ""
	}

	// variable.Field → DDL or FuncResponse via definedVars.
	typeName, ok := definedVars[source]
	if !ok {
		return ""
	}

	// DDL model field: modelToTable로 변환 후 조회.
	tableName := modelToTable(typeName)
	if goType := resolveDDLColumnType(st, tableName, field); goType != "" {
		return goType
	}

	// Func Response field fallback.
	return resolveFuncResponseFieldType(funcSpecs, typeName, field)
}

// inferLiteralType infers the Go type of a literal value.
func inferLiteralType(s string) string {
	if s == "true" || s == "false" {
		return "bool"
	}
	if s == "nil" {
		return ""
	}
	if strings.Contains(s, ".") {
		return "float64"
	}
	return "int"
}

// resolveFuncResponseFieldType looks up a field type from func spec Response fields.
// Works when the SSaC result type follows the "<FuncName>Response" convention.
func resolveFuncResponseFieldType(specs []funcspec.FuncSpec, respTypeName, field string) string {
	for _, spec := range specs {
		if spec.Name+"Response" == respTypeName {
			for _, f := range spec.ResponseFields {
				if f.Name == field {
					return f.Type
				}
			}
		}
	}
	return ""
}

// resolveDDLColumnType looks up a column's Go type from the SymbolTable.
// DDLTables: map[string]DDLTable, DDLTable.Columns: map[string]string (column name → Go type).
func resolveDDLColumnType(st *ssacvalidator.SymbolTable, tableName, columnName string) string {
	if st == nil || st.DDLTables == nil {
		return ""
	}
	table, ok := st.DDLTables[tableName]
	if !ok {
		return ""
	}
	// 1) Exact match.
	if goType, ok := table.Columns[columnName]; ok {
		return goType
	}
	// 2) PascalCase→snake_case: "OrgID" → "org_id".
	snakeCol := toSnakeCase(columnName)
	if goType, ok := table.Columns[snakeCol]; ok {
		return goType
	}
	// 3) Case-insensitive fallback.
	for colName, goType := range table.Columns {
		if strings.EqualFold(colName, columnName) {
			return goType
		}
	}
	return ""
}

// resolveOpenAPIFieldType looks up a field's Go type from the OpenAPI request schema.
func resolveOpenAPIFieldType(doc *openapi3.T, operationID, fieldName string) string {
	if doc == nil || doc.Paths == nil {
		return ""
	}
	for _, pathItem := range doc.Paths.Map() {
		for _, op := range []*openapi3.Operation{
			pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Delete, pathItem.Patch,
		} {
			if op == nil || op.OperationID != operationID {
				continue
			}
			if op.RequestBody == nil || op.RequestBody.Value == nil {
				return ""
			}
			for _, mt := range op.RequestBody.Value.Content {
				if mt.Schema == nil || mt.Schema.Value == nil {
					continue
				}
				propRef, ok := mt.Schema.Value.Properties[fieldName]
				if !ok {
					// Try camelCase lookup.
					propRef, ok = mt.Schema.Value.Properties[strcase.ToGoCamel(fieldName)]
					if !ok {
						continue
					}
				}
				if propRef.Value == nil {
					continue
				}
				return openAPITypeToGo(propRef.Value)
			}
		}
	}
	return ""
}

// openAPITypeToGo converts an OpenAPI schema type to a Go type.
func openAPITypeToGo(schema *openapi3.Schema) string {
	switch schema.Type.Slice()[0] {
	case "string":
		if schema.Format == "date-time" {
			return "time.Time"
		}
		return "string"
	case "integer":
		if schema.Format == "int32" {
			return "int32"
		}
		return "int64"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			return "[]" + openAPITypeToGo(schema.Items.Value)
		}
		return "[]interface{}"
	default:
		return "interface{}"
	}
}

// typesCompatible checks if two Go type strings are compatible.
// Go does NOT allow implicit conversion between int/int32/int64,
// so only exact matches are considered compatible.
func typesCompatible(a, b string) bool {
	return a == b
}

// generateSkeleton creates a skeleton code hint for a missing func.
func generateSkeleton(pkg, funcName string, seq ssacparser.Sequence) string {
	uc := strcase.ToGoPascal(funcName)
	if pkg == "" {
		pkg = "custom"
	}

	var requestFields []string
	for key := range seq.Inputs {
		requestFields = append(requestFields, fmt.Sprintf("\t%s string", key))
	}

	var responseFields []string
	if seq.Result != nil {
		typeName := "string"
		if seq.Result.Type != "" {
			typeName = seq.Result.Type
		}
		responseFields = append(responseFields, fmt.Sprintf("\t%s %s", strcase.ToGoPascal(seq.Result.Var), typeName))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("다음 파일을 작성하세요: func/%s/%s.go\n\n", pkg, toSnakeCase(funcName)))
	b.WriteString(fmt.Sprintf("package %s\n\n", pkg))
	b.WriteString(fmt.Sprintf("// @func %s\n", funcName))
	b.WriteString("// @description <이 함수가 무엇을 하는지 한 줄로 설명>\n\n")
	b.WriteString(fmt.Sprintf("type %sRequest struct {\n", uc))
	for _, f := range requestFields {
		b.WriteString(f + "\n")
	}
	b.WriteString("}\n\n")
	b.WriteString(fmt.Sprintf("type %sResponse struct {\n", uc))
	for _, f := range responseFields {
		b.WriteString(f + "\n")
	}
	b.WriteString("}\n\n")
	b.WriteString(fmt.Sprintf("func %s(req %sRequest) (%sResponse, error) {\n", uc, uc, uc))
	b.WriteString("\t// TODO: implement\n")
	b.WriteString(fmt.Sprintf("\treturn %sResponse{}, nil\n", uc))
	b.WriteString("}\n")

	return b.String()
}

// forbiddenImportPrefixes are I/O packages that @call func must not import.
var forbiddenImportPrefixes = []string{
	// DB
	"database/sql",
	"github.com/lib/pq",
	"github.com/jackc/pgx",
	// Network
	"net/http",
	"net/rpc",
	"google.golang.org/grpc",
	// File I/O
	"io",
	"io/ioutil",
	"bufio",
}

// checkForbiddenImports returns any forbidden I/O imports found in the list.
func checkForbiddenImports(imports []string) []string {
	var found []string
	for _, imp := range imports {
		for _, forbidden := range forbiddenImportPrefixes {
			if imp == forbidden || strings.HasPrefix(imp, forbidden+"/") {
				found = append(found, imp)
				break
			}
		}
	}
	return found
}

// toSnakeCase converts camelCase/PascalCase to snake_case.
func toSnakeCase(s string) string {
	return strcase.ToSnake(s)
}
