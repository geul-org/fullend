package crosscheck

import (
	"fmt"

	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CheckSSaCOpenAPI validates SSaC function names match OpenAPI operationIds and vice versa.
func CheckSSaCOpenAPI(funcs []ssacparser.ServiceFunc, st *ssacvalidator.SymbolTable) []CrossError {
	var errs []CrossError

	funcNames := make(map[string]string) // funcName → fileName
	for _, fn := range funcs {
		funcNames[fn.Name] = fn.FileName
	}

	// Rule 3: Every SSaC function must have a matching operationId.
	for name, fileName := range funcNames {
		if _, ok := st.Operations[name]; !ok {
			errs = append(errs, CrossError{
				Rule:       "SSaC → OpenAPI",
				Context:    fmt.Sprintf("%s:%s", fileName, name),
				Message:    fmt.Sprintf("SSaC function %q has no matching OpenAPI operationId", name),
				Suggestion: fmt.Sprintf("OpenAPI에 추가: operationId: %s", name),
			})
		}
	}

	// Rule 4: Every operationId should have a matching SSaC function.
	for opID := range st.Operations {
		if _, ok := funcNames[opID]; !ok {
			errs = append(errs, CrossError{
				Rule:       "OpenAPI → SSaC",
				Context:    opID,
				Message:    fmt.Sprintf("OpenAPI operationId %q has no matching SSaC function", opID),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("SSaC에 추가: func %s(w http.ResponseWriter, r *http.Request) {}", opID),
			})
		}
	}

	return errs
}
