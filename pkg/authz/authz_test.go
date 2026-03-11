package authz

import (
	"os"
	"testing"
)

func TestCheckDisabled(t *testing.T) {
	os.Setenv("DISABLE_AUTHZ", "1")
	defer os.Unsetenv("DISABLE_AUTHZ")

	resp, err := Check(CheckRequest{
		Action:   "read",
		Resource: "gig",
		UserID:   1,
		Role:     "client",
	})
	if err != nil {
		t.Fatalf("expected no error with DISABLE_AUTHZ=1, got: %v", err)
	}
	_ = resp
}

func TestCheckNotInitialized(t *testing.T) {
	os.Unsetenv("DISABLE_AUTHZ")
	globalPolicy = ""

	_, err := Check(CheckRequest{
		Action:   "read",
		Resource: "gig",
		UserID:   1,
		Role:     "client",
	})
	if err == nil {
		t.Fatal("expected error when not initialized")
	}
	if err.Error() != "authz not initialized" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckRequestFields(t *testing.T) {
	req := CheckRequest{
		Action:     "AcceptProposal",
		Resource:   "gig",
		UserID:     42,
		Role:       "client",
		ResourceID: 99,
	}
	if req.Action != "AcceptProposal" {
		t.Fatal("Action mismatch")
	}
	if req.Resource != "gig" {
		t.Fatal("Resource mismatch")
	}
	if req.UserID != 42 {
		t.Fatal("UserID mismatch")
	}
	if req.Role != "client" {
		t.Fatal("Role mismatch")
	}
	if req.ResourceID != 99 {
		t.Fatal("ResourceID mismatch")
	}
}

func TestInitRequiresOPAPolicyPath(t *testing.T) {
	os.Unsetenv("DISABLE_AUTHZ")
	os.Unsetenv("OPA_POLICY_PATH")

	err := Init(nil, nil)
	if err == nil {
		t.Fatal("expected error when OPA_POLICY_PATH is not set")
	}
}

func TestInitSkipsWithDisableAuthz(t *testing.T) {
	os.Setenv("DISABLE_AUTHZ", "1")
	defer os.Unsetenv("DISABLE_AUTHZ")

	err := Init(nil, nil)
	if err != nil {
		t.Fatalf("expected no error with DISABLE_AUTHZ=1, got: %v", err)
	}
}
