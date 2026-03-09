package crosscheck

import (
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/artifacts/internal/funcspec"
	"github.com/geul-org/fullend/artifacts/internal/policy"
	"github.com/geul-org/fullend/artifacts/internal/scenario"
	"github.com/geul-org/fullend/artifacts/internal/statemachine"
	ssacparser "github.com/geul-org/ssac/parser"
	ssacvalidator "github.com/geul-org/ssac/validator"
)

// CrossValidateInput holds the pre-loaded data from individual validations.
type CrossValidateInput struct {
	OpenAPIDoc       *openapi3.T
	SymbolTable      *ssacvalidator.SymbolTable
	ServiceFuncs     []ssacparser.ServiceFunc
	StateDiagrams    []*statemachine.StateDiagram
	Policies         []*policy.Policy
	Features         []*scenario.Feature
	ProjectFuncSpecs []funcspec.FuncSpec
	FullendPkgSpecs  []funcspec.FuncSpec
}

// Run executes all cross-validation rules and returns collected errors.
func Run(input *CrossValidateInput) []CrossError {
	var errs []CrossError

	// OpenAPI x-extensions ↔ DDL
	if input.OpenAPIDoc != nil && input.SymbolTable != nil {
		errs = append(errs, CheckOpenAPIDDL(input.OpenAPIDoc, input.SymbolTable, input.ServiceFuncs)...)
	}

	// SSaC ↔ DDL
	if input.ServiceFuncs != nil && input.SymbolTable != nil {
		errs = append(errs, CheckSSaCDDL(input.ServiceFuncs, input.SymbolTable)...)
	}

	// SSaC ↔ OpenAPI (function name ↔ operationId)
	if input.ServiceFuncs != nil && input.SymbolTable != nil {
		errs = append(errs, CheckSSaCOpenAPI(input.ServiceFuncs, input.SymbolTable)...)
	}

	// States ↔ SSaC/DDL/OpenAPI
	if len(input.StateDiagrams) > 0 {
		errs = append(errs, CheckStates(input.StateDiagrams, input.ServiceFuncs, input.SymbolTable, input.OpenAPIDoc)...)
	}

	// Policy ↔ SSaC/DDL/States
	if len(input.Policies) > 0 {
		errs = append(errs, CheckPolicy(input.Policies, input.ServiceFuncs, input.SymbolTable, input.StateDiagrams)...)
	}

	// Scenario ↔ OpenAPI/States
	if len(input.Features) > 0 {
		errs = append(errs, CheckScenarios(input.Features, input.OpenAPIDoc, input.StateDiagrams)...)
	}

	// Func ↔ SSaC
	if input.ServiceFuncs != nil {
		errs = append(errs, CheckFuncs(input.ServiceFuncs, input.FullendPkgSpecs, input.ProjectFuncSpecs, input.SymbolTable, input.OpenAPIDoc)...)
	}

	return errs
}
