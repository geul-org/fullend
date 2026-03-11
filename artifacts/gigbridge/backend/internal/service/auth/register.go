package auth

import (
	"github.com/geul-org/fullend/pkg/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	email := req.Email
	password := req.Password
	name := req.Name
	role := req.Role

	existingUser, err := h.UserModel.FindByEmail(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	hp, err := auth.HashPassword(auth.HashPasswordRequest{Password: password})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	user, err := h.UserModel.Create(email, name, hp.HashedPassword, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 생성 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})

}
