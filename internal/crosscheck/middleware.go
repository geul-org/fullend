package crosscheck

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// CheckMiddleware validates that fullend.yaml middleware matches OpenAPI securitySchemes.
func CheckMiddleware(middleware []string, doc *openapi3.T) []CrossError {
	var errs []CrossError

	// Collect OpenAPI securitySchemes names.
	schemeNames := make(map[string]bool)
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		for name := range doc.Components.SecuritySchemes {
			schemeNames[name] = true
		}
	}

	// Middleware set.
	mwSet := make(map[string]bool)
	for _, m := range middleware {
		mwSet[m] = true
	}

	// Rule 1: Each OpenAPI securityScheme must have a matching middleware.
	for name := range schemeNames {
		if !mwSet[name] {
			errs = append(errs, CrossError{
				Rule:       "Config ↔ OpenAPI",
				Context:    fmt.Sprintf("securitySchemes.%s", name),
				Message:    fmt.Sprintf("OpenAPI securityScheme %q has no matching middleware in fullend.yaml", name),
				Suggestion: fmt.Sprintf("fullend.yaml backend.middleware에 %q 추가", name),
			})
		}
	}

	// Rule 2: Each middleware must have a matching OpenAPI securityScheme.
	for _, m := range middleware {
		if !schemeNames[m] {
			errs = append(errs, CrossError{
				Rule:       "Config ↔ OpenAPI",
				Context:    fmt.Sprintf("middleware.%s", m),
				Message:    fmt.Sprintf("middleware %q has no matching OpenAPI securityScheme", m),
				Suggestion: fmt.Sprintf("OpenAPI components.securitySchemes에 %q 추가", m),
			})
		}
	}

	// Rule 3: Endpoints referencing security names not in middleware.
	for pathStr, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if op.Security == nil {
				continue
			}
			for _, req := range *op.Security {
				for name := range req {
					if !mwSet[name] {
						errs = append(errs, CrossError{
							Rule:       "Config ↔ OpenAPI",
							Context:    fmt.Sprintf("%s %s", strings.ToUpper(method), pathStr),
							Message:    fmt.Sprintf("endpoint references security %q not in fullend.yaml middleware", name),
							Suggestion: fmt.Sprintf("fullend.yaml backend.middleware에 %q 추가", name),
						})
					}
				}
			}
		}
	}

	return errs
}
