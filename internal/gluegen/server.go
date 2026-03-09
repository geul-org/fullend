package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// generateServerStruct creates service/server.go with Server struct definition and Handler function.
func generateServerStruct(intDir string, models, funcs, components []string, modulePath string, doc *openapi3.T) error {
	serviceDir := filepath.Join(intDir, "service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return err
	}

	var b strings.Builder

	b.WriteString("package service\n\n")

	// Import model package if there are models.
	if len(models) > 0 {
		b.WriteString(fmt.Sprintf("import \"%s/internal/model\"\n\n", modulePath))
	}

	// Build fields.
	var fields []string
	for _, m := range models {
		fieldName := ucFirst(lcFirst(m) + "Model")
		fields = append(fields, fmt.Sprintf("\t%s model.%sModel", fieldName, m))
	}
	for _, c := range components {
		fieldName := ucFirst(c)
		fields = append(fields, fmt.Sprintf("\t%s %sService", fieldName, fieldName))
	}
	for _, f := range funcs {
		fieldName := ucFirst(f)
		fields = append(fields, fmt.Sprintf("\t%s func(args ...interface{}) (interface{}, error)", fieldName))
	}
	// Auth.
	fields = append(fields, "\tAuthz Authorizer")

	b.WriteString("// Server implements api.ServerInterface.\n")
	b.WriteString("type Server struct {\n")
	for _, f := range fields {
		b.WriteString(f + "\n")
	}
	b.WriteString("}\n\n")

	// Authorizer interface.
	b.WriteString("// Authorizer checks permissions.\n")
	b.WriteString("type Authorizer interface {\n")
	b.WriteString("\tCheck(user *CurrentUser, action, resource string, id interface{}) (bool, error)\n")
	b.WriteString("}\n\n")

	// Component interfaces.
	for _, c := range components {
		typeName := ucFirst(c) + "Service"
		b.WriteString(fmt.Sprintf("// %s provides %s functionality.\n", typeName, c))
		b.WriteString(fmt.Sprintf("type %s interface {\n", typeName))
		b.WriteString("\tExecute(args ...interface{}) error\n")
		b.WriteString("}\n\n")
	}

	// Handler function.
	b.WriteString("// Handler creates an http.Handler that routes requests to the Server.\n")
	b.WriteString("func Handler(s *Server) http.Handler {\n")
	b.WriteString("\tmux := http.NewServeMux()\n")

	if doc != nil {
		// Generate routes from OpenAPI paths.
		for pathStr, pathItem := range doc.Paths.Map() {
			for method, op := range pathItem.Operations() {
				if op.OperationID == "" {
					continue
				}
				// Convert OpenAPI path params {Param} to Go 1.22 mux {param} style.
				muxPath := convertPathParams(pathStr)
				pattern := fmt.Sprintf("%s %s", method, muxPath)
				handlerName := op.OperationID

				// Check if path has parameters.
				var pathParams []pathParamInfo
				if pathItem.Parameters != nil {
					for _, p := range pathItem.Parameters {
						if p.Value != nil && p.Value.In == "path" {
							pathParams = append(pathParams, pathParamInfo{
								Name:   p.Value.Name,
								GoName: snakeToGo(p.Value.Name),
								IsInt:  p.Value.Schema != nil && p.Value.Schema.Value != nil && p.Value.Schema.Value.Type != nil && ((*p.Value.Schema.Value.Type)[0] == "integer"),
							})
						}
					}
				}
				if op.Parameters != nil {
					for _, p := range op.Parameters {
						if p.Value != nil && p.Value.In == "path" {
							pathParams = append(pathParams, pathParamInfo{
								Name:   p.Value.Name,
								GoName: snakeToGo(p.Value.Name),
								IsInt:  p.Value.Schema != nil && p.Value.Schema.Value != nil && p.Value.Schema.Value.Type != nil && ((*p.Value.Schema.Value.Type)[0] == "integer"),
							})
						}
					}
				}

				if len(pathParams) == 0 {
					b.WriteString(fmt.Sprintf("\tmux.HandleFunc(\"%s\", s.%s)\n", pattern, handlerName))
				} else {
					// Generate inline handler that extracts path params.
					b.WriteString(fmt.Sprintf("\tmux.HandleFunc(\"%s\", func(w http.ResponseWriter, r *http.Request) {\n", pattern))
					for _, pp := range pathParams {
						lcName := lcFirst(pp.GoName)
						if pp.IsInt {
							b.WriteString(fmt.Sprintf("\t\t%sStr := r.PathValue(\"%s\")\n", lcName, pp.Name))
							b.WriteString(fmt.Sprintf("\t\t%s, err := strconv.ParseInt(%sStr, 10, 64)\n", lcName, lcName))
							b.WriteString("\t\tif err != nil {\n")
							b.WriteString("\t\t\thttp.Error(w, \"invalid path parameter\", http.StatusBadRequest)\n")
							b.WriteString("\t\t\treturn\n")
							b.WriteString("\t\t}\n")
						} else {
							b.WriteString(fmt.Sprintf("\t\t%s := r.PathValue(\"%s\")\n", lcName, pp.Name))
						}
					}
					// Call server method with extracted params.
					var args []string
					args = append(args, "w", "r")
					for _, pp := range pathParams {
						args = append(args, lcFirst(pp.GoName))
					}
					b.WriteString(fmt.Sprintf("\t\ts.%s(%s)\n", handlerName, strings.Join(args, ", ")))
					b.WriteString("\t})\n")
				}
			}
		}
	}

	b.WriteString("\treturn mux\n")
	b.WriteString("}\n")

	// Add imports at the top based on content.
	content := b.String()
	imports := []string{fmt.Sprintf("\"%s/internal/model\"", modulePath)}
	if strings.Contains(content, "http.") {
		imports = append(imports, "\"net/http\"")
	}
	if strings.Contains(content, "strconv.") {
		imports = append(imports, "\"strconv\"")
	}

	var header strings.Builder
	header.WriteString("package service\n\n")
	header.WriteString("import (\n")
	for _, imp := range imports {
		header.WriteString("\t" + imp + "\n")
	}
	header.WriteString(")\n\n")

	// Replace the original "package service\n\nimport ..." section.
	// Remove the original package + import block and prepend the new one.
	body := content
	if idx := strings.Index(body, "// Server implements"); idx > 0 {
		body = body[idx:]
	}
	final := header.String() + body

	path := filepath.Join(serviceDir, "server.go")
	return os.WriteFile(path, []byte(final), 0644)
}

type pathParamInfo struct {
	Name   string // original param name e.g. "CourseID"
	GoName string // PascalCase e.g. "CourseID"
	IsInt  bool
}

// convertPathParams converts OpenAPI path params like {CourseID} to Go 1.22 mux style {CourseID}.
// Go 1.22 mux uses the same brace syntax, so this is mostly a pass-through.
func convertPathParams(path string) string {
	return path
}

func ucFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
