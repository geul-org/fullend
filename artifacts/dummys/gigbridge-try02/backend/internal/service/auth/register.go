package auth

import (
	"github.com/geul-org/fullend/pkg/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/auth/register.ssac contract=075ca27
func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
		Role     string `json:"role"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	password := req.Password
	role := req.Role
	name := req.Name
	email := req.Email

	tx, err := h.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback()

	hp, err := auth.HashPassword(auth.HashPasswordRequest{Password: password})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	user, err := h.UserModel.WithTx(tx).Create(email, hp.HashedPassword, role, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 생성 실패"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})

}
