package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geul-org/fullend/internal/policy"
)

// GenerateAuthzPackage generates the OPA-based Authorizer implementation.
func GenerateAuthzPackage(policies []*policy.Policy, artifactsDir, modulePath string) error {
	authzDir := filepath.Join(artifactsDir, "backend", "internal", "authz")
	if err := os.MkdirAll(authzDir, 0755); err != nil {
		return fmt.Errorf("create authz dir: %w", err)
	}

	// 1. Copy .rego file(s) for embedding.
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

	// 2. Collect all ownership mappings.
	var ownerships []policy.OwnershipMapping
	for _, p := range policies {
		ownerships = append(ownerships, p.Ownerships...)
	}

	// 3. Generate authz.go.
	regoFileName := "authz.rego"
	if len(policies) > 0 {
		regoFileName = filepath.Base(policies[0].File)
	}

	src := generateAuthzSource(ownerships, modulePath, regoFileName)
	path := filepath.Join(authzDir, "authz.go")
	return os.WriteFile(path, []byte(src), 0644)
}

func generateAuthzSource(ownerships []policy.OwnershipMapping, modulePath, regoFileName string) string {
	var b strings.Builder

	b.WriteString(`package authz

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/open-policy-agent/opa/v1/rego"

`)
	b.WriteString(fmt.Sprintf("\t\"%s/internal/model\"\n", modulePath))
	b.WriteString(`)

`)
	b.WriteString(fmt.Sprintf("//go:embed %s\n", regoFileName))
	b.WriteString(`var policyRego string

// OPAAuthorizer implements model.Authorizer using OPA Rego.
type OPAAuthorizer struct {
	query rego.PreparedEvalQuery
	db    *sql.DB
}

// New creates an OPA-based Authorizer.
func New(db *sql.DB) (*OPAAuthorizer, error) {
	query, err := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("policy.rego", policyRego),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("OPA init failed: %w", err)
	}
	return &OPAAuthorizer{query: query, db: db}, nil
}

// Check evaluates the OPA policy.
func (a *OPAAuthorizer) Check(user *model.CurrentUser, action, resource string, id interface{}) (bool, error) {
	input := map[string]interface{}{
		"user":        map[string]interface{}{"id": user.UserID, "role": user.Role},
		"action":      action,
		"resource":    resource,
		"resource_id": id,
	}

	if ownerID, err := a.lookupOwner(resource, id); err == nil {
		input["resource_owner"] = ownerID
	}

	results, err := a.query.Eval(context.Background(), rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("OPA eval failed: %w", err)
	}
	if len(results) == 0 {
		return false, nil
	}
	allowed, ok := results[0].Expressions[0].Value.(bool)
	return ok && allowed, nil
}

`)

	// Generate lookupOwner.
	b.WriteString("func (a *OPAAuthorizer) lookupOwner(resource string, id interface{}) (int64, error) {\n")
	if len(ownerships) == 0 {
		b.WriteString("\treturn 0, fmt.Errorf(\"no ownership mapping for resource: %s\", resource)\n")
	} else {
		b.WriteString("\tswitch resource {\n")
		for _, om := range ownerships {
			b.WriteString(fmt.Sprintf("\tcase %q:\n", om.Resource))
			b.WriteString("\t\tvar ownerID int64\n")
			if om.JoinTable != "" {
				// JOIN query.
				b.WriteString(fmt.Sprintf("\t\terr := a.db.QueryRow(\n"))
				b.WriteString(fmt.Sprintf("\t\t\t\"SELECT t.%s FROM %s t JOIN %s j ON t.id = j.%s WHERE j.id = $1\",\n",
					om.Column, om.Table, om.JoinTable, om.JoinFK))
				b.WriteString("\t\t\tid,\n")
				b.WriteString("\t\t).Scan(&ownerID)\n")
			} else {
				// Direct query.
				b.WriteString(fmt.Sprintf("\t\terr := a.db.QueryRow(\"SELECT %s FROM %s WHERE id = $1\", id).Scan(&ownerID)\n",
					om.Column, om.Table))
			}
			b.WriteString("\t\treturn ownerID, err\n")
		}
		b.WriteString("\tdefault:\n")
		b.WriteString("\t\treturn 0, fmt.Errorf(\"no ownership mapping for resource: %s\", resource)\n")
		b.WriteString("\t}\n")
	}
	b.WriteString("}\n")

	return b.String()
}
