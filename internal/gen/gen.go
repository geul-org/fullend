package gen

import (
	"github.com/geul-org/fullend/internal/gen/gogin"
	"github.com/geul-org/fullend/internal/gen/hurl"
	"github.com/geul-org/fullend/internal/gen/react"
	"github.com/geul-org/fullend/internal/genapi"
	"github.com/geul-org/fullend/internal/projectconfig"
)

// Generate creates all artifacts from parsed SSOTs.
func Generate(parsed *genapi.ParsedSSOTs, cfg *genapi.GenConfig, stmlOut *genapi.STMLGenOutput) error {
	// 1. Backend code generation.
	backend := selectBackend(parsed.Config)
	if err := backend.Generate(parsed, cfg); err != nil {
		return err
	}
	// 2. React frontend (OpenAPI contract-based, backend-independent).
	if err := react.Generate(parsed, cfg, stmlOut); err != nil {
		return err
	}
	// 3. Hurl smoke tests (OpenAPI contract-based, backend-independent).
	if err := hurl.Generate(parsed, cfg); err != nil {
		return err
	}
	return nil
}

func selectBackend(cfg *projectconfig.ProjectConfig) genapi.Backend {
	// Future: branch on cfg.Backend field.
	return &gogin.GoGin{}
}
