package funcspec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "auth")
	os.MkdirAll(pkgDir, 0755)

	src := `package auth

import "golang.org/x/crypto/bcrypt"

// @func hashPassword
// @description 평문 비밀번호를 bcrypt 해시로 변환한다

type HashPasswordRequest struct {
	Password string
}

type HashPasswordResponse struct {
	HashedPassword string
}

func HashPassword(req HashPasswordRequest) (HashPasswordResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	return HashPasswordResponse{HashedPassword: string(hash)}, err
}
`
	path := filepath.Join(pkgDir, "hash_password.go")
	os.WriteFile(path, []byte(src), 0644)

	spec, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}

	if spec.Name != "hashPassword" {
		t.Errorf("Name = %q, want %q", spec.Name, "hashPassword")
	}
	if spec.Description != "평문 비밀번호를 bcrypt 해시로 변환한다" {
		t.Errorf("Description = %q", spec.Description)
	}
	if len(spec.RequestFields) != 1 {
		t.Fatalf("RequestFields count = %d, want 1", len(spec.RequestFields))
	}
	if spec.RequestFields[0].Name != "Password" || spec.RequestFields[0].Type != "string" {
		t.Errorf("RequestFields[0] = %+v", spec.RequestFields[0])
	}
	if len(spec.ResponseFields) != 1 {
		t.Fatalf("ResponseFields count = %d, want 1", len(spec.ResponseFields))
	}
	if spec.ResponseFields[0].Name != "HashedPassword" || spec.ResponseFields[0].Type != "string" {
		t.Errorf("ResponseFields[0] = %+v", spec.ResponseFields[0])
	}
	if !spec.HasBody {
		t.Error("HasBody = false, want true")
	}
}

func TestParseFileStub(t *testing.T) {
	dir := t.TempDir()

	src := `package billing

// @func calculateRefund
// @description 환불 금액을 계산한다

type CalculateRefundRequest struct {
	Amount int
}

type CalculateRefundResponse struct {
	Refund int
}

func CalculateRefund(req CalculateRefundRequest) (CalculateRefundResponse, error) {
	return CalculateRefundResponse{}, nil
}
`
	path := filepath.Join(dir, "calculate_refund.go")
	os.WriteFile(path, []byte(src), 0644)

	spec, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}

	if spec.HasBody {
		t.Error("HasBody = true, want false (stub)")
	}
}

func TestParseDir(t *testing.T) {
	dir := t.TempDir()

	authDir := filepath.Join(dir, "auth")
	os.MkdirAll(authDir, 0755)

	hash := `package auth

// @func hashPassword
// @description hash

type HashPasswordRequest struct {
	Password string
}
type HashPasswordResponse struct {
	HashedPassword string
}
func HashPassword(req HashPasswordRequest) (HashPasswordResponse, error) {
	return HashPasswordResponse{HashedPassword: "hashed"}, nil
}
`
	verify := `package auth

// @func verifyPassword
// @description verify

type VerifyPasswordRequest struct {
	PasswordHash string
	Password     string
}
type VerifyPasswordResponse struct{}
func VerifyPassword(req VerifyPasswordRequest) (VerifyPasswordResponse, error) {
	return VerifyPasswordResponse{}, nil
}
`
	os.WriteFile(filepath.Join(authDir, "hash_password.go"), []byte(hash), 0644)
	os.WriteFile(filepath.Join(authDir, "verify_password.go"), []byte(verify), 0644)

	specs, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("ParseDir count = %d, want 2", len(specs))
	}

	for _, s := range specs {
		if s.Package != "auth" {
			t.Errorf("Package = %q, want %q", s.Package, "auth")
		}
	}
}

func TestParseFileNoAnnotation(t *testing.T) {
	dir := t.TempDir()
	src := `package foo

func Foo() {}
`
	path := filepath.Join(dir, "foo.go")
	os.WriteFile(path, []byte(src), 0644)

	spec, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec != nil {
		t.Error("expected nil for file without @func annotation")
	}
}
