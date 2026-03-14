package genapi

import (
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/geul-org/fullend/internal/funcspec"
	"github.com/geul-org/fullend/internal/policy"
	"github.com/geul-org/fullend/internal/projectconfig"
	"github.com/geul-org/fullend/internal/statemachine"
	ssacparser "github.com/geul-org/fullend/internal/ssac/parser"
	ssacvalidator "github.com/geul-org/fullend/internal/ssac/validator"
	stmlparser "github.com/geul-org/fullend/internal/stml/parser"
)

// ParsedSSOTs holds all SSOT parsing results.
// orchestrator.ParseAll() populates this; crosscheck and gen consume it.
type ParsedSSOTs struct {
	Config           *projectconfig.ProjectConfig
	OpenAPIDoc       *openapi3.T
	SymbolTable      *ssacvalidator.SymbolTable
	ServiceFuncs     []ssacparser.ServiceFunc
	STMLPages        []stmlparser.PageSpec
	StateDiagrams    []*statemachine.StateDiagram
	Policies         []*policy.Policy
	ProjectFuncSpecs []funcspec.FuncSpec
	FullendPkgSpecs  []funcspec.FuncSpec
	HurlFiles        []string
	ModelDir         string
	StatesErr        error
}

// GenConfig holds generation settings (not parsing results).
type GenConfig struct {
	ArtifactsDir string
	SpecsDir     string
	ModulePath   string
}

// STMLGenOutput holds STML generator output (not parse results).
// Populated by orchestrator after stml.Generate(), consumed by react gen.
type STMLGenOutput struct {
	Deps    map[string]string // npm dependencies
	Pages   []string          // page names
	PageOps map[string]string // page file → primary operationID
}

// Backend generates backend code from parsed SSOTs.
type Backend interface {
	Generate(parsed *ParsedSSOTs, cfg *GenConfig) error
}
