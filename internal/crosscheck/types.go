package crosscheck

import (
	"github.com/geul-org/fullend/internal/genapi"
)

// CrossValidateInput holds the pre-loaded data from individual validations.
type CrossValidateInput struct {
	*genapi.ParsedSSOTs
	DTOTypes        map[string]bool            // model types marked with @dto (skip DDL matching)
	Middleware      []string                   // from fullend.yaml backend.middleware
	Archived        *ArchivedInfo              // @archived tables/columns from DDL
	Claims          map[string]string          // from fullend.yaml backend.auth.claims (FieldName → claim key)
	QueueBackend    string                     // from fullend.yaml queue.backend ("postgres", "memory", "")
	AuthzPackage    string                     // from fullend.yaml authz.package ("" = default pkg/authz)
	SensitiveCols   map[string]map[string]bool // @sensitive columns per table (table → column → true)
	NoSensitiveCols map[string]map[string]bool // @nosensitive columns per table (suppress WARNING)
	Roles           []string                   // from fullend.yaml auth.roles
}

// Rule represents a single cross-validation rule with metadata.
type Rule struct {
	Name     string // e.g. "OpenAPI ↔ DDL", "SSaC → OpenAPI"
	Source   string // "OpenAPI", "SSaC", "Policy", "States", "Config", "DDL"
	Target   string // "DDL", "OpenAPI", ... ("" = standalone)
	Requires func(*CrossValidateInput) bool
	Check    func(*CrossValidateInput) []CrossError
}

// CrossError represents a cross-validation error between two SSOT layers.
type CrossError struct {
	Rule       string // e.g. "x-sort ↔ DDL", "SSaC @result ↔ DDL"
	Context    string // e.g. operationId or funcName
	Message    string
	Level      string // "ERROR" or "WARNING" (empty = ERROR)
	Suggestion string // fix suggestion (empty if none)
}
