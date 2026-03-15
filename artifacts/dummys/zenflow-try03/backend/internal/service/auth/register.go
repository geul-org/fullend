package auth

import (
	"github.com/geul-org/fullend/pkg/auth"
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/auth/register.ssac contract=9fde818
func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		OrgID    int64  `json:"org_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	email := req.Email
	password := req.Password
	role := req.Role
	orgID := req.OrgID

	tx, err := h.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback()

	existing, err := h.UserModel.WithTx(tx).FindByEmail(email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	hp, err := auth.HashPassword(auth.HashPasswordRequest{Password: password})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	user, err := h.UserModel.WithTx(tx).Create(orgID, email, hp.HashedPassword, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 생성 실패"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": user,
	})

}
