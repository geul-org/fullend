package auth

import (
	"database/sql"

	"github.com/zenflow/zenflow/internal/model"
)

// Handler handles requests for the auth domain.
type Handler struct {
	DB *sql.DB
	UserModel model.UserModel
	JWTSecret string
}
