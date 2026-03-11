package authz

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
)

//go:embed authz.rego
var policyRego string

// CheckRequest holds the inputs for an authorization check.
type CheckRequest struct {
	Action     string
	Resource   string
	UserID     int64
	ResourceID int64
}

// CheckResponse is the result of an authorization check.
type CheckResponse struct{}

var globalEval *rego.PreparedEvalQuery
var globalDB *sql.DB

// Init initializes the global authz evaluator with the embedded Rego policy.
func Init(db *sql.DB) error {
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
		"user":        map[string]interface{}{"id": req.UserID},
		"action":      req.Action,
		"resource":    req.Resource,
		"resource_id": req.ResourceID,
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
