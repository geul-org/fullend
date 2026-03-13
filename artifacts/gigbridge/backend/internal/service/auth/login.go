package auth

import (
	"github.com/geul-org/fullend/pkg/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/auth/login.ssac contract=1e99033
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	email := req.Email
	password := req.Password

	user, err := h.UserModel.FindByEmail(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if _, err = auth.VerifyPassword(auth.VerifyPasswordRequest{Password: password, PasswordHash: user.PasswordHash}); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "호출 실패"})
		return
	}

	token, err := auth.IssueToken(auth.IssueTokenRequest{Email: user.Email, Role: user.Role, UserID: user.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	c.JSON(http.StatusOK, token)

}
