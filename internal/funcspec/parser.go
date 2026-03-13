package funcspec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ettle/strcase"
)

// FuncSpec holds a parsed func spec file.
type FuncSpec struct {
	Package        string   // "auth"
	Name           string   // "hashPassword"
	Description    string   // @description value
	ErrStatus      int      // @error HTTP status code (0 = unspecified)
	RequestFields  []Field  // FuncNameRequest struct fields
	ResponseFields []Field  // FuncNameResponse struct fields
	HasBody        bool     // true if function body is not just "// TODO: implement"
	Imports        []string // import paths (e.g. "database/sql", "net/http")
}

// Field represents a struct field.
type Field struct {
	Name string
	Type string
}

// ParseDir parses all .go files under dir (recursively by package subdirectory).
// Returns a flat list of FuncSpecs.
func ParseDir(dir string) ([]FuncSpec, error) {
	var specs []FuncSpec
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return err
		}
		fs, err := ParseFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if fs != nil {
			// Derive package from parent dir name.
			rel, _ := filepath.Rel(dir, path)
			parts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
			if parts[0] != "." {
				fs.Package = parts[0]
			}
			specs = append(specs, *fs)
		}
		return nil
	})
	return specs, err
}

// ParseFile parses a single func spec .go file.
func ParseFile(path string) (*FuncSpec, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("Go parse error: %w", err)
	}

	spec := &FuncSpec{
		Package: f.Name.Name,
	}

	// Extract @func and @description from file-level comments.
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			line := strings.TrimPrefix(c.Text, "//")
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "@func ") {
				spec.Name = strings.TrimSpace(strings.TrimPrefix(line, "@func "))
			} else if strings.HasPrefix(line, "@error ") {
				if code, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "@error "))); err == nil {
					spec.ErrStatus = code
				}
			} else if strings.HasPrefix(line, "@description ") {
				spec.Description = strings.TrimSpace(strings.TrimPrefix(line, "@description "))
			}
		}
	}

	if spec.Name == "" {
		return nil, nil // Not a func spec file.
	}

	// Extract imports.
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		spec.Imports = append(spec.Imports, path)
	}

	// Extract Request/Response structs and function body.
	expectedRequest := ucFirst(spec.Name) + "Request"
	expectedResponse := ucFirst(spec.Name) + "Response"

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, s := range d.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := extractFields(st)
				if ts.Name.Name == expectedRequest {
					spec.RequestFields = fields
				} else if ts.Name.Name == expectedResponse {
					spec.ResponseFields = fields
				}
			}
		case *ast.FuncDecl:
			funcName := ucFirst(spec.Name)
			if d.Name.Name == funcName && d.Body != nil {
				spec.HasBody = !isStubBody(fset, d.Body)
			}
		}
	}

	return spec, nil
}

// extractFields extracts field names and types from a struct.
func extractFields(st *ast.StructType) []Field {
	var fields []Field
	for _, f := range st.Fields.List {
		typeName := exprToString(f.Type)
		for _, name := range f.Names {
			fields = append(fields, Field{Name: name.Name, Type: typeName})
		}
	}
	return fields
}

// exprToString converts an AST type expression to a string.
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	default:
		return "interface{}"
	}
}

// isStubBody checks if function body only contains "// TODO: implement" and a return.
func isStubBody(fset *token.FileSet, body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return true
	}
	if len(body.List) > 1 {
		return false
	}
	// Single statement: check if it's a return.
	_, isReturn := body.List[0].(*ast.ReturnStmt)
	return isReturn
}

// ucFirst converts to Go PascalCase (uppercases the first character with Go initialism handling).
func ucFirst(s string) string {
	return strcase.ToGoPascal(s)
}
