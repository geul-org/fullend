package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// @func refreshToken
// @description 리프레시 토큰을 발급한다 (7일 만료)

type RefreshTokenRequest struct {
	UserID int64
	Email  string
	Role   string
}

type RefreshTokenResponse struct {
	RefreshToken string
}

func RefreshToken(req RefreshTokenRequest) (RefreshTokenResponse, error) {
	claims := jwt.MapClaims{
		"user_id": req.UserID,
		"email":   req.Email,
		"role":    req.Role,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	return RefreshTokenResponse{RefreshToken: signed}, err
}
