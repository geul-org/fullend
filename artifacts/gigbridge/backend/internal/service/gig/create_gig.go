package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gigbridge/api/internal/authz"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) CreateGig(c *gin.Context) {
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Budget      int64  `json:"budget"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	title := req.Title
	description := req.Description
	budget := req.Budget

	if _, err := authz.Check(authz.CheckRequest{Action: "CreateGig", Resource: "gig", Role: currentUser.Role, UserID: currentUser.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only clients can create gigs"})
		return
	}

	gig, err := h.GigModel.Create(budget, currentUser.ID, description, "draft", title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 생성 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gig": gig,
	})

}
