package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gigbridge/api/internal/authz"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h *Handler) SubmitProposal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	var req struct {
		bid_amount int64 `json:"bid_amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	bid_amount := req.bid_amount

	gig, err := h.GigModel.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	if gig == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gig not found"})
		return
	}

	if _, err = authz.Check(authz.CheckRequest{Action: "SubmitProposal", Resource: "gig", ResourceID: gig.ClientID, UserID: currentUser.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot submit proposal to own gig"})
		return
	}

	proposal, err := h.ProposalModel.Create(bid_amount, currentUser.ID, gig.ID, "pending")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 생성 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"proposal": proposal,
	})

}
