package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gigbridge/api/internal/authz"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/gigbridge/api/internal/states/gigstate"
	"strconv"
)

func (h *Handler) SubmitWork(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	gig, err := h.GigModel.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	if gig == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gig not found"})
		return
	}

	if _, err = authz.Check(authz.CheckRequest{Action: "SubmitWork", Resource: "gig_assignee", ResourceID: gig.FreelancerID, UserID: currentUser.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if err := gigstate.CanTransition(gigstate.Input{Status: gig.Status}, "SubmitWork"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	err = h.GigModel.UpdateStatus(gig.ID, "under_review")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	updatedGig, err := h.GigModel.FindByID(gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gig": updatedGig,
	})

}
