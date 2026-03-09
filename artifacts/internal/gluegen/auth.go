package gluegen

import (
	"os"
	"path/filepath"
)

// generateAuthStub creates auth.go with CurrentUser type and extraction stub.
func generateAuthStub(intDir string) error {
	src := `package internal

import "net/http"

// CurrentUser represents the authenticated user.
type CurrentUser struct {
	UserID int64
	Email  string
	Name   string
	Role   string
}

// currentUser extracts the authenticated user from the request.
// TODO: Implement JWT token parsing.
func (s *Server) currentUser(r *http.Request) *CurrentUser {
	// Placeholder: extract from Authorization header.
	return &CurrentUser{}
}
`
	path := filepath.Join(intDir, "auth.go")
	return os.WriteFile(path, []byte(src), 0644)
}
