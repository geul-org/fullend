package gogin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/geul-org/fullend/internal/policy"
)

// GenerateAuthzPackage copies .rego files to the artifacts directory for runtime loading.
// Go authz code is provided by fullend/pkg/authz — no code generation needed.
func GenerateAuthzPackage(policies []*policy.Policy, artifactsDir string) error {
	authzDir := filepath.Join(artifactsDir, "backend", "internal", "authz")
	if err := os.MkdirAll(authzDir, 0755); err != nil {
		return fmt.Errorf("create authz dir: %w", err)
	}

	for _, p := range policies {
		data, err := os.ReadFile(p.File)
		if err != nil {
			return fmt.Errorf("read rego file: %w", err)
		}
		dest := filepath.Join(authzDir, filepath.Base(p.File))
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("copy rego file: %w", err)
		}
	}

	return nil
}
