package crosscheck

import (
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/scenario"
	"github.com/geul-org/fullend/internal/statemachine"
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
	DTOTypes         map[string]bool   // model types marked with @dto (skip DDL matching)
	Middleware       []string          // from fullend.yaml backend.middleware
	Archived         *ArchivedInfo     // @archived tables/columns from DDL
	Claims           map[string]string // from fullend.yaml backend.auth.claims (FieldName → claim key)
	QueueBackend     string            // from fullend.yaml queue.backend ("postgres", "memory", "")
	AuthzPackage     string            // from fullend.yaml authz.package ("" = default pkg/authz)
	SensitiveCols    map[string]map[string]bool // @sensitive columns per table (table → column → true)
	NoSensitiveCols  map[string]map[string]bool // @nosensitive columns per table (suppress WARNING)
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
		errs = append(errs, CheckSSaCDDL(input.ServiceFuncs, input.SymbolTable, input.DTOTypes)...)
	}

	// SSaC ↔ OpenAPI (function name ↔ operationId + @response fields ↔ response schema)
	if input.ServiceFuncs != nil && input.SymbolTable != nil {
		errs = append(errs, CheckSSaCOpenAPI(input.ServiceFuncs, input.SymbolTable, input.OpenAPIDoc)...)
	}

	// States ↔ SSaC/DDL/OpenAPI
	if len(input.StateDiagrams) > 0 {
		errs = append(errs, CheckStates(input.StateDiagrams, input.ServiceFuncs, input.SymbolTable, input.OpenAPIDoc)...)
	}

	// Policy ↔ SSaC/DDL/States
	if len(input.Policies) > 0 {
		errs = append(errs, CheckPolicy(input.Policies, input.ServiceFuncs, input.SymbolTable, input.StateDiagrams)...)
	}

	// Scenario ↔ OpenAPI/States/Policy
	if len(input.Features) > 0 {
		errs = append(errs, CheckScenarios(input.Features, input.OpenAPIDoc, input.StateDiagrams, input.Policies, input.ServiceFuncs)...)
	}

	// Func ↔ SSaC
	if input.ServiceFuncs != nil {
		errs = append(errs, CheckFuncs(input.ServiceFuncs, input.FullendPkgSpecs, input.ProjectFuncSpecs, input.SymbolTable, input.OpenAPIDoc)...)
	}

	// Middleware ↔ OpenAPI securitySchemes
	if input.OpenAPIDoc != nil && input.Middleware != nil {
		errs = append(errs, CheckMiddleware(input.Middleware, input.OpenAPIDoc)...)
	}

	// Claims ↔ SSaC currentUser
	if input.ServiceFuncs != nil {
		errs = append(errs, CheckClaims(input.ServiceFuncs, input.Claims)...)
	}

	// DDL → SSaC coverage
	if input.SymbolTable != nil && input.ServiceFuncs != nil {
		errs = append(errs, CheckDDLCoverage(input.SymbolTable, input.ServiceFuncs, input.Archived)...)
	}

	// Queue: publish ↔ subscribe
	if input.ServiceFuncs != nil {
		errs = append(errs, CheckQueue(input.ServiceFuncs, input.QueueBackend)...)
	}

	// Authz: @auth inputs ↔ CheckRequest fields
	if input.ServiceFuncs != nil {
		errs = append(errs, CheckAuthz(input.ServiceFuncs, input.AuthzPackage)...)
	}

	// DDL: sensitive column name pattern without @sensitive annotation
	if input.SymbolTable != nil {
		errs = append(errs, CheckSensitiveColumns(input.SymbolTable, input.SensitiveCols, input.NoSensitiveCols)...)
	}

	return errs
}
