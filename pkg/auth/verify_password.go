package auth

import "golang.org/x/crypto/bcrypt"

// @func verifyPassword
// @description 저장된 해시와 평문 비밀번호가 일치하는지 검증한다

type VerifyPasswordRequest struct {
	PasswordHash string
	Password     string
}

type VerifyPasswordResponse struct{}

func VerifyPassword(req VerifyPasswordRequest) (VerifyPasswordResponse, error) {
	err := bcrypt.CompareHashAndPassword([]byte(req.PasswordHash), []byte(req.Password))
	return VerifyPasswordResponse{}, err
}
