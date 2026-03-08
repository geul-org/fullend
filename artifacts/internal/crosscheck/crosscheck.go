package crosscheck

import (
	"github.com/getkin/kin-openapi/openapi3"

	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CrossValidateInput holds the pre-loaded data from individual validations.
type CrossValidateInput struct {
	OpenAPIDoc   *openapi3.T
	SymbolTable  *ssacvalidator.SymbolTable
	ServiceFuncs []ssacparser.ServiceFunc
}

// Run executes all cross-validation rules and returns collected errors.
func Run(input *CrossValidateInput) []CrossError {
	var errs []CrossError

	// OpenAPI x-extensions ↔ DDL
	if input.OpenAPIDoc != nil && input.SymbolTable != nil {
		errs = append(errs, CheckOpenAPIDDL(input.OpenAPIDoc, input.SymbolTable)...)
	}

	// SSaC ↔ DDL
	if input.ServiceFuncs != nil && input.SymbolTable != nil {
		errs = append(errs, CheckSSaCDDL(input.ServiceFuncs, input.SymbolTable)...)
	}

	return errs
}
