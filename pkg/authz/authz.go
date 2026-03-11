package authz

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// OwnershipMapping represents a resource-to-table ownership mapping from @ownership annotations.
type OwnershipMapping struct {
	Resource string // "gig", "proposal"
	Table    string // "gigs", "proposals"
	Column   string // "client_id", "freelancer_id"
}

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

var globalPolicy string
var globalDB *sql.DB
var globalOwnerships []OwnershipMapping

// Init initializes the global authz state.
// Reads OPA policy from OPA_POLICY_PATH environment variable.
// Skips initialization when DISABLE_AUTHZ=1.
func Init(db *sql.DB, ownerships []OwnershipMapping) error {
	globalDB = db
	globalOwnerships = ownerships

	if os.Getenv("DISABLE_AUTHZ") == "1" {
		return nil
	}

	policyPath := os.Getenv("OPA_POLICY_PATH")
	if policyPath == "" {
		return fmt.Errorf("OPA_POLICY_PATH environment variable is required (set DISABLE_AUTHZ=1 to skip)")
	}

	policyData, err := os.ReadFile(policyPath)
	if err != nil {
		return fmt.Errorf("read OPA policy %s: %w", policyPath, err)
	}

	globalPolicy = string(policyData)
	return nil
}

// Check evaluates the OPA policy. Returns error if denied or evaluation fails.
// Set DISABLE_AUTHZ=1 to bypass authorization checks.
func Check(req CheckRequest) (CheckResponse, error) {
	if os.Getenv("DISABLE_AUTHZ") == "1" {
		return CheckResponse{}, nil
	}
	if globalPolicy == "" {
		return CheckResponse{}, fmt.Errorf("authz not initialized")
	}

	// Build data.owners by querying DB for matching ownership mappings.
	owners, err := loadOwners(req)
	if err != nil {
		return CheckResponse{}, fmt.Errorf("load owners: %w", err)
	}

	opaInput := map[string]interface{}{
		"claims":      map[string]interface{}{"user_id": req.UserID, "role": req.Role},
		"action":      req.Action,
		"resource":    req.Resource,
		"resource_id": req.ResourceID,
	}

	// Build in-memory store with owners data for OPA evaluation.
	store := inmem.NewFromObject(map[string]interface{}{
		"owners": owners,
	})

	query, err := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("policy.rego", globalPolicy),
		rego.Store(store),
		rego.Input(opaInput),
	).Eval(context.Background())
	if err != nil {
		return CheckResponse{}, fmt.Errorf("OPA eval failed: %w", err)
	}
	if len(query) == 0 {
		return CheckResponse{}, fmt.Errorf("forbidden")
	}
	allowed, ok := query[0].Expressions[0].Value.(bool)
	if !ok || !allowed {
		return CheckResponse{}, fmt.Errorf("forbidden")
	}
	return CheckResponse{}, nil
}

// loadOwners queries DB for ownership data based on registered mappings.
func loadOwners(req CheckRequest) (map[string]interface{}, error) {
	owners := make(map[string]interface{})
	if globalDB == nil || len(globalOwnerships) == 0 {
		return owners, nil
	}

	for _, om := range globalOwnerships {
		var ownerID int64
		query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", om.Column, om.Table)
		err := globalDB.QueryRow(query, req.ResourceID).Scan(&ownerID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("query %s.%s: %w", om.Table, om.Column, err)
		}

		resMap, ok := owners[om.Resource].(map[string]interface{})
		if !ok {
			resMap = make(map[string]interface{})
			owners[om.Resource] = resMap
		}
		resMap[fmt.Sprint(req.ResourceID)] = ownerID
	}

	return owners, nil
}
