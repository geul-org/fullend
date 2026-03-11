package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/policy"
	ssacparser "github.com/geul-org/ssac/parser"
)

// transformServiceFilesWithDomains transforms service files in both flat and domain subdirectories.
func transformServiceFilesWithDomains(intDir string, serviceFuncs []ssacparser.ServiceFunc, models, funcs []string, modulePath string, xConfigs map[string]string) error {
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
		transformed := transformSource(string(src), models, funcs, modulePath, xConfigs, false)
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
			transformed := transformSource(string(src), models, funcs, modulePath, xConfigs, true)
			if err := os.WriteFile(path, []byte(transformed), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateAuthStubWithDomains creates model/auth.go with CurrentUser type and Authorizer interface.
// CurrentUser fields are derived from fullend.yaml claims config if available.
func generateAuthStubWithDomains(intDir string, modulePath string, claims map[string]string) error {
	modelDir := filepath.Join(intDir, "model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("package model\n\n")

	if claims != nil && len(claims) > 0 {
		// Generate CurrentUser from claims config — no imports needed.
		b.WriteString("// CurrentUser is the authenticated user extracted by JWT middleware.\n")
		b.WriteString("type CurrentUser struct {\n")
		// Maintain deterministic field order.
		var fields []string
		for field := range claims {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			goType := inferClaimGoType(field)
			b.WriteString(fmt.Sprintf("\t%s %s\n", field, goType))
		}
		b.WriteString("}\n\n")
	} else {
		// Fallback: alias to pkg/middleware.CurrentUser.
		b.WriteString("import \"github.com/geul-org/fullend/pkg/middleware\"\n\n")
		b.WriteString("// CurrentUser is the authenticated user extracted by JWT middleware.\n")
		b.WriteString("type CurrentUser = middleware.CurrentUser\n\n")
	}

	b.WriteString("// Authorizer checks permissions.\n")
	b.WriteString("type Authorizer interface {\n")
	b.WriteString("\tCheck(user *CurrentUser, action, resource string, input interface{}) error\n")
	b.WriteString("}\n")

	return os.WriteFile(filepath.Join(modelDir, "auth.go"), []byte(b.String()), 0644)
}

// inferClaimGoType infers Go type from claim field name convention.
func inferClaimGoType(fieldName string) string {
	lower := strings.ToLower(fieldName)
	if strings.HasSuffix(lower, "id") {
		return "int64"
	}
	return "string"
}

// generateServerStructWithDomains creates per-domain handler.go files and central server.go.
func generateServerStructWithDomains(intDir string, serviceFuncs []ssacparser.ServiceFunc, modulePath string, doc *openapi3.T, claims map[string]string) error {
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

// domainNeedsJWTSecret checks if any service function in the domain calls auth.IssueToken.
func domainNeedsJWTSecret(serviceFuncs []ssacparser.ServiceFunc, domain string) bool {
	for _, fn := range serviceFuncs {
		if fn.Domain != domain {
			continue
		}
		for _, seq := range fn.Sequences {
			if seq.Model == "auth.IssueToken" {
				return true
			}
		}
	}
	return false
}

// generateDomainHandler creates service/{domain}/handler.go with the Handler struct.
func generateDomainHandler(serviceDir, domain string, serviceFuncs []ssacparser.ServiceFunc, modulePath string) error {
	domainDir := filepath.Join(serviceDir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return err
	}

	models := collectModelsForDomain(serviceFuncs, domain)
	funcs := collectFuncsForDomain(serviceFuncs, domain)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("package %s\n\n", domain))
	b.WriteString(fmt.Sprintf("import \"%s/internal/model\"\n\n", modulePath))

	b.WriteString("// Handler handles requests for the " + domain + " domain.\n")
	b.WriteString("type Handler struct {\n")

	for _, m := range models {
		fieldName := ucFirst(lcFirst(m) + "Model")
		b.WriteString(fmt.Sprintf("\t%s model.%sModel\n", fieldName, m))
	}

	for _, f := range funcs {
		fieldName := ucFirst(f)
		b.WriteString(fmt.Sprintf("\t%s func(args ...interface{}) (interface{}, error)\n", fieldName))
	}

	if domainNeedsJWTSecret(serviceFuncs, domain) {
		b.WriteString("\tJWTSecret string\n")
	}

	b.WriteString("}\n")

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
	hasFlatFuncs := len(flatModels) > 0 || len(flatFuncs) > 0

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
	for _, f := range flatFuncs {
		fieldName := ucFirst(f)
		b.WriteString(fmt.Sprintf("\t%s func(args ...interface{}) (interface{}, error)\n", fieldName))
	}

	// Detect security schemes from OpenAPI.
	hasBearer := hasBearerScheme(doc)

	if hasBearer {
		b.WriteString("\tJWTSecret string\n")
	}

	b.WriteString("}\n\n")

	// SetupRouter creates a gin.Engine with routes.
	b.WriteString("// SetupRouter creates a gin.Engine that routes requests to the Server.\n")
	b.WriteString("func SetupRouter(s *Server) *gin.Engine {\n")
	b.WriteString("\tr := gin.Default()\n\n")

	if hasBearer {
		b.WriteString("\t// Auth group — JWT middleware extracts currentUser into context.\n")
		b.WriteString("\tauth := r.Group(\"/\")\n")
		b.WriteString("\tauth.Use(middleware.BearerAuth(s.JWTSecret))\n\n")
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
		imports = append(imports, fmt.Sprintf("\"%s/internal/middleware\"", modulePath))
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
func generateMainWithDomains(artifactsDir string, serviceFuncs []ssacparser.ServiceFunc, modulePath string, queueBackend string, policies []*policy.Policy) error {
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
		if domainNeedsJWTSecret(serviceFuncs, domain) {
			handlerLines = append(handlerLines, "\t\t\tJWTSecret: *jwtSecret,")
		}
		initLines = append(initLines, fmt.Sprintf("\t\t%s: &%ssvc.Handler{", fieldName, domain))
		initLines = append(initLines, handlerLines...)
		initLines = append(initLines, "\t\t},")
	}

	// Add JWTSecret to Server struct if bearer auth is used.
	if anyNeedsAuth {
		initLines = append(initLines, "\t\tJWTSecret: *jwtSecret,")
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
		extraImports = append(extraImports, "\t\"github.com/geul-org/fullend/pkg/authz\"")
	}
	for _, d := range domains {
		extraImports = append(extraImports, fmt.Sprintf("\t%ssvc \"%s/internal/service/%s\"", d, modulePath, d))
	}
	importBlock := strings.Join(extraImports, "\n")

	// Authz init block.
	authzBlock := ""
	if anyNeedsAuth {
		ownershipsCode := buildOwnershipsLiteral(policies)
		authzBlock = fmt.Sprintf(`
	os.Setenv("JWT_SECRET", *jwtSecret)

	if err := authz.Init(conn, %s); err != nil {
		log.Fatalf("authz init failed: %%v", err)
	}
`, ownershipsCode)
	}

	// Queue code blocks.
	queueImport := ""
	queueInitBlock := ""
	queueSubscribeBlock := ""
	if queueBackend != "" {
		subscribers := collectSubscribers(serviceFuncs)
		if len(subscribers) > 0 || hasPublishSequence(serviceFuncs) {
			queueImport = "\n\t\"context\"\n\t\"encoding/json\"\n\t\"github.com/geul-org/fullend/pkg/queue\"\n\t\"fmt\""
			queueInitBlock = fmt.Sprintf(`
	if err := queue.Init(context.Background(), %q, conn); err != nil {
		log.Fatalf("queue init failed: %%v", err)
	}
	defer queue.Close()
`, queueBackend)

			var subLines []string
			for _, fn := range subscribers {
				if fn.Param == nil {
					continue
				}
				// Determine the service package for this subscriber.
				svcPkg := "service"
				if fn.Domain != "" {
					svcPkg = fn.Domain + "svc"
				}
				subLines = append(subLines, fmt.Sprintf(`
	queue.Subscribe(%q, func(ctx context.Context, msg []byte) error {
		var message %s.%s
		if err := json.Unmarshal(msg, &message); err != nil {
			return fmt.Errorf("unmarshal: %%w", err)
		}
		return server.%s.%s(ctx, message)
	})`, fn.Subscribe.Topic, svcPkg, fn.Param.TypeName, ucFirst(fn.Domain), fn.Name))
			}
			if len(subLines) > 0 {
				queueSubscribeBlock = strings.Join(subLines, "\n") + "\n\n\tgo queue.Start(context.Background())\n"
			}
		}
	}

	// JWT flag line.
	jwtFlagLine := ""
	if anyNeedsAuth {
		jwtFlagLine = `
	jwtSecretDefault := os.Getenv("JWT_SECRET")
	if jwtSecretDefault == "" {
		jwtSecretDefault = "secret"
	}
	jwtSecret := flag.String("jwt-secret", jwtSecretDefault, "JWT signing secret")`
	}

	osImport := ""
	if anyNeedsAuth {
		osImport = "\n\t\"os\""
	}

	src := fmt.Sprintf(`package main

import (
	"database/sql"
	"flag"
	"log"%s

	_ "github.com/lib/pq"
%s%s
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dsn := flag.String("dsn", "postgres://localhost:5432/app?sslmode=disable", "database connection string")
	dbDriver := flag.String("db", "postgres", "database driver (postgres, mysql)")%s
	flag.Parse()

	conn, err := sql.Open(*dbDriver, *dsn)
	if err != nil {
		log.Fatalf("database connection failed: %%v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("database ping failed: %%v", err)
	}
%s%s
	server := &service.Server{
%s
	}
%s
	r := service.SetupRouter(server)
	log.Printf("server listening on %%s", *addr)
	log.Fatal(r.Run(*addr))
}
`, osImport, importBlock, queueImport, jwtFlagLine, authzBlock, queueInitBlock, initBlock, queueSubscribeBlock)

	path := filepath.Join(artifactsDir, "backend", "cmd", "main.go")
	return os.WriteFile(path, []byte(src), 0644)
}
