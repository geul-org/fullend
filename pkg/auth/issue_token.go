package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// @func issueToken
// @description 인증된 사용자 정보로 JWT 액세스 토큰을 발급한다

type IssueTokenRequest struct {
	UserID int64
	Email  string
	Role   string
}

type IssueTokenResponse struct {
	AccessToken string
}

func IssueToken(req IssueTokenRequest) (IssueTokenResponse, error) {
	claims := jwt.MapClaims{
		"user_id": req.UserID,
		"email":   req.Email,
		"role":    req.Role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	return IssueTokenResponse{AccessToken: signed}, err
}
