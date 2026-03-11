package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geul-org/fullend/internal/policy"
)

// GenerateAuthzPackage generates the OPA-based authz package.
// If authzPackage is empty, generates default pkg/authz-style code.
// If authzPackage is set (custom), only copies .rego files (user provides Go code).
func GenerateAuthzPackage(policies []*policy.Policy, artifactsDir, modulePath, authzPackage string) error {
	authzDir := filepath.Join(artifactsDir, "backend", "internal", "authz")
	if err := os.MkdirAll(authzDir, 0755); err != nil {
		return fmt.Errorf("create authz dir: %w", err)
	}

	// 1. Copy .rego file(s) for embedding.
	regoFileName := "authz.rego"
	for _, p := range policies {
		data, err := os.ReadFile(p.File)
		if err != nil {
			return fmt.Errorf("read rego file: %w", err)
		}
		dest := filepath.Join(authzDir, filepath.Base(p.File))
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("copy rego file: %w", err)
		}
		regoFileName = filepath.Base(p.File)
	}

	// 2. If custom authz package, skip Go code generation (user provides it).
	if authzPackage != "" {
		return nil
	}

	// 3. Generate default authz.go (pkg/authz pattern: CheckRequest + Check function).
	src := generateDefaultAuthzSource(regoFileName)
	path := filepath.Join(authzDir, "authz.go")
	return os.WriteFile(path, []byte(src), 0644)
}

func generateDefaultAuthzSource(regoFileName string) string {
	var b strings.Builder

	b.WriteString(`package authz

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
)

`)
	b.WriteString(fmt.Sprintf("//go:embed %s\n", regoFileName))
	b.WriteString(`var policyRego string

// CheckRequest holds the inputs for an authorization check.
type CheckRequest struct {
	Action     string
	Resource   string
	UserID     int64
	Role       string
	ResourceID int64
}

// CheckResponse is the result of an authorization check.
type CheckResponse struct{}

var globalEval *rego.PreparedEvalQuery
var globalDB *sql.DB

// Init initializes the global authz evaluator with the embedded Rego policy.
// Skips OPA initialization when DISABLE_AUTHZ=1.
func Init(db *sql.DB) error {
	if os.Getenv("DISABLE_AUTHZ") == "1" {
		globalDB = db
		return nil
	}
	query, err := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("policy.rego", policyRego),
	).PrepareForEval(context.Background())
	if err != nil {
		return fmt.Errorf("OPA init failed: %w", err)
	}
	globalEval = &query
	globalDB = db
	return nil
}

// Check evaluates the OPA policy. Returns error if denied or evaluation fails.
// Set DISABLE_AUTHZ=1 to bypass authorization checks.
func Check(req CheckRequest) (CheckResponse, error) {
	if os.Getenv("DISABLE_AUTHZ") == "1" {
		return CheckResponse{}, nil
	}
	if globalEval == nil {
		return CheckResponse{}, fmt.Errorf("authz not initialized")
	}

	opaInput := map[string]interface{}{
		"claims":            map[string]interface{}{"user_id": req.UserID, "role": req.Role},
		"action":            req.Action,
		"resource":          req.Resource,
		"resource_owner_id": req.ResourceID,
	}

	results, err := globalEval.Eval(context.Background(), rego.EvalInput(opaInput))
	if err != nil {
		return CheckResponse{}, fmt.Errorf("OPA eval failed: %w", err)
	}
	if len(results) == 0 {
		return CheckResponse{}, fmt.Errorf("forbidden")
	}
	allowed, ok := results[0].Expressions[0].Value.(bool)
	if !ok || !allowed {
		return CheckResponse{}, fmt.Errorf("forbidden")
	}
	return CheckResponse{}, nil
}
`)

	return b.String()
}
