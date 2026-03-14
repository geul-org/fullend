package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/gig/create_gig.ssac contract=5167701
func (h *Handler) CreateGig(c *gin.Context) {
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	var req struct {
		Description string `json:"description"`
		Budget      int64  `json:"budget"`
		Title       string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	description := req.Description
	budget := req.Budget
	title := req.Title

	tx, err := h.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback()

	gig, err := h.GigModel.WithTx(tx).Create(currentUser.ID, title, description, budget)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 생성 실패"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gig": gig,
	})

}
