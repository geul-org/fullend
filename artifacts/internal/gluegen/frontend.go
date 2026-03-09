package gluegen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// generateFrontendSetup creates React + Vite project files.
func generateFrontendSetup(artifactsDir string, doc *openapi3.T, stmlDeps map[string]string, stmlPages []string, stmlPageOps map[string]string) error {
	frontendDir := filepath.Join(artifactsDir, "frontend")
	srcDir := filepath.Join(frontendDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return err
	}

	if err := writePackageJSON(frontendDir, stmlDeps); err != nil {
		return err
	}
	if err := writeViteConfig(frontendDir); err != nil {
		return err
	}
	if err := writeTSConfig(frontendDir); err != nil {
		return err
	}
	if err := writeIndexHTML(frontendDir); err != nil {
		return err
	}
	if err := writeMainTSX(srcDir, stmlDeps); err != nil {
		return err
	}
	if err := writeAppTSX(srcDir, doc, stmlPages, stmlPageOps); err != nil {
		return err
	}
	if err := writeAPIClient(srcDir, doc); err != nil {
		return err
	}
	return nil
}

func writePackageJSON(dir string, stmlDeps map[string]string) error {
	deps := map[string]string{
		"react":            "^18",
		"react-dom":        "^18",
		"react-router-dom": "^6",
	}
	// Merge stml dependencies.
	for k, v := range stmlDeps {
		deps[k] = v
	}

	pkg := map[string]interface{}{
		"private": true,
		"type":    "module",
		"scripts": map[string]string{
			"dev":   "vite",
			"build": "tsc && vite build",
		},
		"dependencies": deps,
		"devDependencies": map[string]string{
			"@types/react":          "^18",
			"@types/react-dom":      "^18",
			"@vitejs/plugin-react":  "^4",
			"typescript":            "^5",
			"vite":                  "^5",
			"tailwindcss":           "^3",
			"postcss":               "^8",
			"autoprefixer":          "^10",
		},
	}
	b, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "package.json"), b, 0644)
}

func writeViteConfig(dir string) error {
	src := `import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080'
    }
  }
})
`
	return os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte(src), 0644)
}

func writeTSConfig(dir string) error {
	src := `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedSideEffectImports": true
  },
  "include": ["src"]
}
`
	return os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(src), 0644)
}

func writeIndexHTML(dir string) error {
	src := `<!DOCTYPE html>
<html lang="ko">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>App</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(src), 0644)
}

func writeMainTSX(srcDir string, stmlDeps map[string]string) error {
	var src string
	if _, ok := stmlDeps["@tanstack/react-query"]; ok {
		src = `import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'

const queryClient = new QueryClient()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
`
	} else {
		src = `import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </React.StrictMode>,
)
`
	}
	return os.WriteFile(filepath.Join(srcDir, "main.tsx"), []byte(src), 0644)
}

// writeAppTSX generates App.tsx by scanning actual page files on disk
// and matching them to OpenAPI paths via stml page→operationID mapping.
func writeAppTSX(srcDir string, doc *openapi3.T, stmlPages []string, stmlPageOps map[string]string) error {
	var b strings.Builder

	type route struct {
		path      string
		component string
		fileName  string
	}

	// Scan pages directory for actual .tsx files.
	pagesDir := filepath.Join(srcDir, "pages")
	var pageFiles []string
	if entries, err := os.ReadDir(pagesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".tsx") {
				pageFiles = append(pageFiles, strings.TrimSuffix(e.Name(), ".tsx"))
			}
		}
	}
	sort.Strings(pageFiles)

	// Build OpenAPI operationID → path mapping.
	opPaths := make(map[string]string) // operationID → OpenAPI path
	if doc != nil && doc.Paths != nil {
		for path, pi := range doc.Paths.Map() {
			for _, op := range pi.Operations() {
				if op != nil && op.OperationID != "" {
					opPaths[op.OperationID] = path
				}
			}
		}
	}

	// For each page file, determine the route via its primary operationID.
	var routes []route
	for _, fileName := range pageFiles {
		component := fileNameToComponent(fileName)

		var matchedPath string

		// 1. Use stml page→operationID mapping (most reliable).
		if opID, ok := stmlPageOps[fileName]; ok {
			if apiPath, ok := opPaths[opID]; ok {
				matchedPath = openAPIPathToReactRoute(apiPath)
			}
		}

		// 2. Fallback: derive path from file name.
		if matchedPath == "" {
			matchedPath = "/" + strings.ReplaceAll(strings.TrimSuffix(fileName, "-page"), "-", "/")
		}

		routes = append(routes, route{path: matchedPath, component: component, fileName: fileName})
	}

	// Deduplicate by path (keep first).
	seen := make(map[string]bool)
	var uniqueRoutes []route
	for _, r := range routes {
		if !seen[r.path] {
			seen[r.path] = true
			uniqueRoutes = append(uniqueRoutes, r)
		}
	}
	sort.Slice(uniqueRoutes, func(i, j int) bool {
		return uniqueRoutes[i].path < uniqueRoutes[j].path
	})

	// Write imports.
	b.WriteString("import { Routes, Route } from 'react-router-dom'\n")
	for _, r := range uniqueRoutes {
		b.WriteString(fmt.Sprintf("import %s from './pages/%s'\n", r.component, r.fileName))
	}
	b.WriteString("\n")

	b.WriteString("export default function App() {\n")
	b.WriteString("  return (\n")
	b.WriteString("    <Routes>\n")
	for _, r := range uniqueRoutes {
		b.WriteString(fmt.Sprintf("      <Route path=\"%s\" element={<%s />} />\n", r.path, r.component))
	}
	b.WriteString("    </Routes>\n")
	b.WriteString("  )\n")
	b.WriteString("}\n")

	return os.WriteFile(filepath.Join(srcDir, "App.tsx"), []byte(b.String()), 0644)
}

// fileNameToComponent converts "course-list-page" → "CourseListPage"
func fileNameToComponent(fileName string) string {
	parts := strings.Split(fileName, "-")
	var result strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			result.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return result.String()
}

// writeAPIClient generates api.ts with fetch wrappers using object parameters.
// stml generates calls like api.GetCourse({ courseid: CourseID, include: '...' }),
// so all functions accept a single params object.
func writeAPIClient(srcDir string, doc *openapi3.T) error {
	var b strings.Builder
	b.WriteString("const BASE = '/api'\n\n")

	if doc == nil || doc.Paths == nil {
		b.WriteString("export const api = {}\n")
		return os.WriteFile(filepath.Join(srcDir, "api.ts"), []byte(b.String()), 0644)
	}

	type endpoint struct {
		method     string
		path       string
		opID       string
		pathParams []string // camelCase param names
	}
	var endpoints []endpoint

	for path, pi := range doc.Paths.Map() {
		for method, op := range pi.Operations() {
			if op == nil || op.OperationID == "" {
				continue
			}
			var pathParams []string
			parts := strings.Split(path, "/")
			for _, p := range parts {
				if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
					pathParams = append(pathParams, lcFirst(p[1:len(p)-1]))
				}
			}
			endpoints = append(endpoints, endpoint{
				method:     method,
				path:       path,
				opID:       op.OperationID,
				pathParams: pathParams,
			})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].opID < endpoints[j].opID
	})

	// Generate individual functions — all accept a single params object.
	for _, ep := range endpoints {
		funcName := lcFirst(ep.opID)
		method := strings.ToUpper(ep.method)

		b.WriteString(fmt.Sprintf("async function %s(params?: Record<string, any>) {\n", funcName))

		// Build URL with path param substitution.
		fetchPath := openAPIPathToTemplateLiteral(ep.path)
		if len(ep.pathParams) > 0 {
			// Extract path params from the object.
			for _, pp := range ep.pathParams {
				b.WriteString(fmt.Sprintf("  const %s = params?.%s\n", pp, pp))
			}
		}

		if method == "GET" {
			// Build query string from remaining params.
			b.WriteString("  const query = new URLSearchParams()\n")
			b.WriteString("  if (params) {\n")
			if len(ep.pathParams) > 0 {
				excluded := make([]string, len(ep.pathParams))
				for i, pp := range ep.pathParams {
					excluded[i] = fmt.Sprintf("'%s'", pp)
				}
				b.WriteString(fmt.Sprintf("    const exclude = new Set([%s])\n", strings.Join(excluded, ", ")))
				b.WriteString("    for (const [k, v] of Object.entries(params)) {\n")
				b.WriteString("      if (v != null && !exclude.has(k)) query.set(k, String(v))\n")
				b.WriteString("    }\n")
			} else {
				b.WriteString("    for (const [k, v] of Object.entries(params)) {\n")
				b.WriteString("      if (v != null) query.set(k, String(v))\n")
				b.WriteString("    }\n")
			}
			b.WriteString("  }\n")
			b.WriteString("  const qs = query.toString()\n")
			b.WriteString(fmt.Sprintf("  const res = await fetch(`${BASE}%s${qs ? '?' + qs : ''}`)\n", fetchPath))
			b.WriteString("  return res.json()\n")
		} else {
			// POST/PUT/DELETE — path params extracted, rest goes to body.
			if len(ep.pathParams) > 0 {
				excluded := make([]string, len(ep.pathParams))
				for i, pp := range ep.pathParams {
					excluded[i] = fmt.Sprintf("'%s'", pp)
				}
				b.WriteString(fmt.Sprintf("  const exclude = new Set([%s])\n", strings.Join(excluded, ", ")))
				b.WriteString("  const body: Record<string, any> = {}\n")
				b.WriteString("  if (params) {\n")
				b.WriteString("    for (const [k, v] of Object.entries(params)) {\n")
				b.WriteString("      if (!exclude.has(k)) body[k] = v\n")
				b.WriteString("    }\n")
				b.WriteString("  }\n")
			} else {
				b.WriteString("  const body = params ?? {}\n")
			}
			b.WriteString(fmt.Sprintf("  const res = await fetch(`${BASE}%s`, {\n", fetchPath))
			b.WriteString(fmt.Sprintf("    method: '%s',\n", method))
			b.WriteString("    headers: { 'Content-Type': 'application/json' },\n")
			b.WriteString("    body: JSON.stringify(body),\n")
			b.WriteString("  })\n")
			b.WriteString("  return res.json()\n")
		}
		b.WriteString("}\n\n")
	}

	// Generate api namespace object (PascalCase keys for stml compatibility).
	b.WriteString("export const api = {\n")
	for i, ep := range endpoints {
		funcName := lcFirst(ep.opID)
		b.WriteString(fmt.Sprintf("  %s: %s", ep.opID, funcName))
		if i < len(endpoints)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")

	return os.WriteFile(filepath.Join(srcDir, "api.ts"), []byte(b.String()), 0644)
}

// openAPIPathToReactRoute converts "/courses/{CourseID}" → "/courses/:courseID"
func openAPIPathToReactRoute(path string) string {
	result := path
	for {
		start := strings.Index(result, "{")
		if start < 0 {
			break
		}
		end := strings.Index(result, "}")
		if end < 0 {
			break
		}
		paramName := result[start+1 : end]
		result = result[:start] + ":" + lcFirst(paramName) + result[end+1:]
	}
	return result
}

// openAPIPathToTemplateLiteral converts "/courses/{CourseID}" → "/courses/${courseID}"
func openAPIPathToTemplateLiteral(path string) string {
	var b strings.Builder
	i := 0
	for i < len(path) {
		start := strings.Index(path[i:], "{")
		if start < 0 {
			b.WriteString(path[i:])
			break
		}
		b.WriteString(path[i : i+start])
		end := strings.Index(path[i+start:], "}")
		if end < 0 {
			b.WriteString(path[i+start:])
			break
		}
		paramName := path[i+start+1 : i+start+end]
		b.WriteString("${" + lcFirst(paramName) + "}")
		i = i + start + end + 1
	}
	return b.String()
}

// operationIDToComponent converts "ListCourses" → "ListCoursesPage"
func operationIDToComponent(opID string) string {
	return opID + "Page"
}

// componentToFileName converts "ListCoursesPage" → "list-courses-page"
func componentToFileName(component string) string {
	var result []byte
	for i, r := range component {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '-')
		}
		result = append(result, byte(r|0x20)) // toLower
	}
	return string(result)
}
