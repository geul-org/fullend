package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/ssac/parser"
)

// transformServiceFilesWithDomains transforms service files in both flat and domain subdirectories.
func transformServiceFilesWithDomains(intDir string, serviceFuncs []ssacparser.ServiceFunc, models, funcs, components []string, modulePath string, xConfigs map[string]string) error {
	serviceDir := filepath.Join(intDir, "service")

	// Transform flat files (Domain="") directly in serviceDir.
	entries, err := os.ReadDir(serviceDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(serviceDir, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		transformed := transformSource(string(src), models, funcs, components, modulePath, xConfigs, false)
		if err := os.WriteFile(path, []byte(transformed), 0644); err != nil {
			return err
		}
	}

	// Transform domain subdirectory files.
	domains := uniqueDomains(serviceFuncs)
	for _, domain := range domains {
		domainDir := filepath.Join(serviceDir, domain)
		entries, err := os.ReadDir(domainDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			path := filepath.Join(domainDir, entry.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			transformed := transformSource(string(src), models, funcs, components, modulePath, xConfigs, true)
			if err := os.WriteFile(path, []byte(transformed), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateAuthStubWithDomains creates model/auth.go with shared auth types.
// JWT middleware is provided by github.com/geul-org/fullend/pkg/middleware.
func generateAuthStubWithDomains(intDir string, modulePath string) error {
	modelDir := filepath.Join(intDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return err
	}

	modelAuth := `package model

import "github.com/geul-org/fullend/pkg/middleware"

// CurrentUser is the authenticated user extracted by JWT middleware.
type CurrentUser = middleware.CurrentUser

// Authorizer checks permissions.
type Authorizer interface {
	Check(user *CurrentUser, action, resource string, id interface{}) (bool, error)
}
`
	return os.WriteFile(filepath.Join(modelDir, "auth.go"), []byte(modelAuth), 0644)
}

// generateServerStructWithDomains creates per-domain handler.go files and central server.go.
func generateServerStructWithDomains(intDir string, serviceFuncs []ssacparser.ServiceFunc, modulePath string, doc *openapi3.T) error {
	serviceDir := filepath.Join(intDir, "service")
	domains := uniqueDomains(serviceFuncs)

	// 1. Generate per-domain handler.go.
	for _, domain := range domains {
		if err := generateDomainHandler(serviceDir, domain, serviceFuncs, modulePath); err != nil {
			return fmt.Errorf("domain %s handler: %w", domain, err)
		}
	}

	// 2. Generate central server.go.
	return generateCentralServer(serviceDir, domains, serviceFuncs, modulePath, doc)
}

// generateDomainHandler creates service/{domain}/handler.go with the Handler struct.
func generateDomainHandler(serviceDir, domain string, serviceFuncs []ssacparser.ServiceFunc, modulePath string) error {
	domainDir := filepath.Join(serviceDir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return err
	}

	models := collectModelsForDomain(serviceFuncs, domain)
	funcs := collectFuncsForDomain(serviceFuncs, domain)
	components := collectComponentsForDomain(serviceFuncs, domain)
	needsAuth := domainNeedsAuth(serviceFuncs, domain)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("package %s\n\n", domain))
	b.WriteString(fmt.Sprintf("import \"%s/internal/model\"\n\n", modulePath))

	b.WriteString("// Handler handles requests for the " + domain + " domain.\n")
	b.WriteString("type Handler struct {\n")

	for _, m := range models {
		fieldName := ucFirst(lcFirst(m) + "Model")
		b.WriteString(fmt.Sprintf("\t%s model.%sModel\n", fieldName, m))
	}

	for _, c := range components {
		fieldName := ucFirst(c)
		b.WriteString(fmt.Sprintf("\t%s %sService\n", fieldName, fieldName))
	}

	for _, f := range funcs {
		fieldName := ucFirst(f)
		b.WriteString(fmt.Sprintf("\t%s func(args ...interface{}) (interface{}, error)\n", fieldName))
	}

	if needsAuth {
		b.WriteString("\tAuthz model.Authorizer\n")
	}

	b.WriteString("}\n")

	// Component interfaces.
	for _, c := range components {
		typeName := ucFirst(c) + "Service"
		b.WriteString(fmt.Sprintf("\n// %s provides %s functionality.\n", typeName, c))
		b.WriteString(fmt.Sprintf("type %s interface {\n", typeName))
		b.WriteString("\tExecute(args ...interface{}) error\n")
		b.WriteString("}\n")
	}

	path := filepath.Join(domainDir, "handler.go")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// opHasSecurity returns true if an OpenAPI operation has a non-empty security requirement.
func opHasSecurity(op *openapi3.Operation) bool {
	if op.Security == nil {
		return false
	}
	// security: [] means explicitly no auth.
	// security: [{bearerAuth: []}] means auth required.
	return len(*op.Security) > 0
}

// convertPathParamsGin converts OpenAPI path params {Name} to gin style :Name.
func convertPathParamsGin(path string) string {
	result := path
	for {
		start := strings.Index(result, "{")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end < 0 {
			break
		}
		paramName := result[start+1 : start+end]
		result = result[:start] + ":" + paramName + result[start+end+1:]
	}
	return result
}

// generateCentralServer creates service/server.go that composes domain handlers with gin router.
func generateCentralServer(serviceDir string, domains []string, serviceFuncs []ssacparser.ServiceFunc, modulePath string, doc *openapi3.T) error {
	// Build operationId → domain map.
	opDomains := make(map[string]string)
	for _, sf := range serviceFuncs {
		if sf.Domain != "" {
			opDomains[sf.Name] = sf.Domain
		}
	}

	// Collect flat (Domain="") resources.
	flatModels := collectModelsForDomain(serviceFuncs, "")
	flatFuncs := collectFuncsForDomain(serviceFuncs, "")
	flatComponents := collectComponentsForDomain(serviceFuncs, "")
	hasFlatFuncs := len(flatModels) > 0 || len(flatFuncs) > 0 || len(flatComponents) > 0

	var b strings.Builder
	b.WriteString("package service\n\n")

	// Server struct.
	b.WriteString("// Server composes domain handlers.\n")
	b.WriteString("type Server struct {\n")

	// Domain handler fields.
	for _, d := range domains {
		fieldName := ucFirst(d)
		b.WriteString(fmt.Sprintf("\t%s *%ssvc.Handler\n", fieldName, d))
	}

	// Flat model fields.
	for _, m := range flatModels {
		fieldName := ucFirst(lcFirst(m) + "Model")
		b.WriteString(fmt.Sprintf("\t%s model.%sModel\n", fieldName, m))
	}
	for _, c := range flatComponents {
		fieldName := ucFirst(c)
		b.WriteString(fmt.Sprintf("\t%s %sService\n", fieldName, fieldName))
	}
	for _, f := range flatFuncs {
		fieldName := ucFirst(f)
		b.WriteString(fmt.Sprintf("\t%s func(args ...interface{}) (interface{}, error)\n", fieldName))
	}

	if hasFlatFuncs {
		b.WriteString("\tAuthz model.Authorizer\n")
	}

	b.WriteString("}\n\n")

	// Flat component interfaces.
	for _, c := range flatComponents {
		typeName := ucFirst(c) + "Service"
		b.WriteString(fmt.Sprintf("// %s provides %s functionality.\n", typeName, c))
		b.WriteString(fmt.Sprintf("type %s interface {\n", typeName))
		b.WriteString("\tExecute(args ...interface{}) error\n")
		b.WriteString("}\n\n")
	}

	// Detect security schemes from OpenAPI.
	hasBearer := false
	if doc != nil && doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for _, ref := range doc.Components.SecuritySchemes {
			if ref.Value != nil && ref.Value.Type == "http" && ref.Value.Scheme == "bearer" {
				hasBearer = true
				break
			}
		}
	}

	// SetupRouter creates a gin.Engine with routes.
	b.WriteString("// SetupRouter creates a gin.Engine that routes requests to the Server.\n")
	b.WriteString("func SetupRouter(s *Server) *gin.Engine {\n")
	b.WriteString("\tr := gin.Default()\n\n")

	if hasBearer {
		b.WriteString("\t// Auth group — JWT middleware extracts currentUser into context.\n")
		b.WriteString("\tauth := r.Group(\"/\")\n")
		b.WriteString("\tauth.Use(middleware.BearerAuth(\"secret\"))\n\n")
	}

	if doc != nil {
		for pathStr, pathItem := range doc.Paths.Map() {
			for method, op := range pathItem.Operations() {
				if op.OperationID == "" {
					continue
				}
				ginPath := convertPathParamsGin(pathStr)
				handlerName := op.OperationID

				// Determine target: s.Domain.Method or s.Method.
				domain := opDomains[handlerName]
				var target string
				if domain != "" {
					target = fmt.Sprintf("s.%s.%s", ucFirst(domain), handlerName)
				} else {
					target = fmt.Sprintf("s.%s", handlerName)
				}

				// Determine route group from OpenAPI security field.
				needsAuth := opHasSecurity(op)
				ginMethod := strings.ToUpper(method)
				var routerVar string
				if needsAuth && hasBearer {
					routerVar = "auth"
				} else {
					routerVar = "r"
				}

				switch ginMethod {
				case "GET":
					b.WriteString(fmt.Sprintf("\t%s.GET(%q, %s)\n", routerVar, ginPath, target))
				case "POST":
					b.WriteString(fmt.Sprintf("\t%s.POST(%q, %s)\n", routerVar, ginPath, target))
				case "PUT":
					b.WriteString(fmt.Sprintf("\t%s.PUT(%q, %s)\n", routerVar, ginPath, target))
				case "DELETE":
					b.WriteString(fmt.Sprintf("\t%s.DELETE(%q, %s)\n", routerVar, ginPath, target))
				case "PATCH":
					b.WriteString(fmt.Sprintf("\t%s.PATCH(%q, %s)\n", routerVar, ginPath, target))
				default:
					b.WriteString(fmt.Sprintf("\t%s.Handle(%q, %q, %s)\n", routerVar, ginMethod, ginPath, target))
				}
			}
		}
	}

	b.WriteString("\n\treturn r\n")
	b.WriteString("}\n")

	// Build imports.
	var imports []string
	imports = append(imports, "\"github.com/gin-gonic/gin\"")
	if hasBearer {
		imports = append(imports, "\"github.com/geul-org/fullend/pkg/middleware\"")
	}
	if len(flatModels) > 0 || hasFlatFuncs {
		imports = append(imports, fmt.Sprintf("\"%s/internal/model\"", modulePath))
	}
	for _, d := range domains {
		imports = append(imports, fmt.Sprintf("%ssvc \"%s/internal/service/%s\"", d, modulePath, d))
	}

	var header strings.Builder
	header.WriteString("package service\n\n")
	header.WriteString("import (\n")
	for _, imp := range imports {
		header.WriteString("\t" + imp + "\n")
	}
	header.WriteString(")\n\n")

	// Replace package+import block.
	body := b.String()
	if idx := strings.Index(body, "// Server composes"); idx > 0 {
		body = body[idx:]
	}
	final := header.String() + body

	path := filepath.Join(serviceDir, "server.go")
	return os.WriteFile(path, []byte(final), 0644)
}

// generateMainWithDomains creates cmd/main.go with domain handler initialization.
func generateMainWithDomains(artifactsDir string, serviceFuncs []ssacparser.ServiceFunc, modulePath string) error {
	if modulePath == "" {
		base := filepath.Base(artifactsDir)
		modulePath = base + "/backend"
	}

	goModPath := filepath.Join(artifactsDir, "backend", "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(artifactsDir, "backend"), 0755); err != nil {
			return err
		}
		goModContent := fmt.Sprintf("module %s\n\ngo 1.22\n\nrequire github.com/gin-gonic/gin v1.10.0\n", modulePath)
		if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Join(artifactsDir, "backend", "cmd"), 0755); err != nil {
		return err
	}

	domains := uniqueDomains(serviceFuncs)
	flatModels := collectModelsForDomain(serviceFuncs, "")
	anyNeedsAuth := false
	for _, d := range domains {
		if domainNeedsAuth(serviceFuncs, d) {
			anyNeedsAuth = true
			break
		}
	}

	// Build init block.
	var initLines []string

	// Flat model fields.
	for _, m := range flatModels {
		fieldName := ucFirst(lcFirst(m) + "Model")
		initLines = append(initLines, fmt.Sprintf("\t\t%s: model.New%sModel(conn),", fieldName, m))
	}

	// Domain handler fields.
	for _, domain := range domains {
		domainModels := collectModelsForDomain(serviceFuncs, domain)
		fieldName := ucFirst(domain)

		var handlerLines []string
		for _, m := range domainModels {
			mFieldName := ucFirst(lcFirst(m) + "Model")
			handlerLines = append(handlerLines, fmt.Sprintf("\t\t\t%s: model.New%sModel(conn),", mFieldName, m))
		}
		if domainNeedsAuth(serviceFuncs, domain) {
			handlerLines = append(handlerLines, "\t\t\tAuthz: az,")
		}

		initLines = append(initLines, fmt.Sprintf("\t\t%s: &%ssvc.Handler{", fieldName, domain))
		initLines = append(initLines, handlerLines...)
		initLines = append(initLines, "\t\t},")
	}

	initBlock := strings.Join(initLines, "\n")
	if initBlock == "" {
		initBlock = "\t\t// No models detected"
	}

	// Build domain imports.
	var extraImports []string
	extraImports = append(extraImports, fmt.Sprintf("\n\t\"%s/internal/model\"", modulePath))
	extraImports = append(extraImports, fmt.Sprintf("\t\"%s/internal/service\"", modulePath))
	if anyNeedsAuth {
		extraImports = append(extraImports, fmt.Sprintf("\t\"%s/internal/authz\"", modulePath))
	}
	for _, d := range domains {
		extraImports = append(extraImports, fmt.Sprintf("\t%ssvc \"%s/internal/service/%s\"", d, modulePath, d))
	}
	importBlock := strings.Join(extraImports, "\n")

	// Authz init block.
	authzBlock := ""
	if anyNeedsAuth {
		authzBlock = `
	az, err := authz.New(conn)
	if err != nil {
		log.Fatalf("authz init failed: %v", err)
	}
`
	}

	src := fmt.Sprintf(`package main

import (
	"database/sql"
	"flag"
	"log"

	_ "github.com/lib/pq"
%s
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dsn := flag.String("dsn", "postgres://localhost:5432/app?sslmode=disable", "database connection string")
	dbDriver := flag.String("db", "postgres", "database driver (postgres, mysql)")
	flag.Parse()

	conn, err := sql.Open(*dbDriver, *dsn)
	if err != nil {
		log.Fatalf("database connection failed: %%v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("database ping failed: %%v", err)
	}
%s
	server := &service.Server{
%s
	}

	r := service.SetupRouter(server)
	log.Printf("server listening on %%s", *addr)
	log.Fatal(r.Run(*addr))
}
`, importBlock, authzBlock, initBlock)

	path := filepath.Join(artifactsDir, "backend", "cmd", "main.go")
	return os.WriteFile(path, []byte(src), 0644)
}
