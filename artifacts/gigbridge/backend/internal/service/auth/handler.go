package auth

import "github.com/gigbridge/api/internal/model"

// Handler handles requests for the auth domain.
type Handler struct {
	UserModel model.UserModel
	JWTSecret string
}
