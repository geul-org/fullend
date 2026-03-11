package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gigbridge/api/internal/authz"
	"github.com/gigbridge/api/internal/billing"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/gigbridge/api/internal/states/gigstate"
	"github.com/gigbridge/api/internal/states/proposalstate"
	"strconv"
)

func (h *Handler) AcceptProposal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	proposal, err := h.ProposalModel.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 조회 실패"})
		return
	}

	if proposal == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proposal not found"})
		return
	}

	gig, err := h.GigModel.FindByID(proposal.GigID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	if gig == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gig not found"})
		return
	}

	if _, err = authz.Check(authz.CheckRequest{Action: "AcceptProposal", Resource: "gig", ResourceID: gig.ClientID, UserID: currentUser.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if err := proposalstate.CanTransition(proposalstate.Input{Status: proposal.Status}, "AcceptProposal"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if err := gigstate.CanTransition(gigstate.Input{Status: gig.Status}, "AcceptProposal"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	err = h.ProposalModel.UpdateStatus(proposal.ID, "accepted")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 수정 실패"})
		return
	}

	err = h.GigModel.UpdateStatus(gig.ID, "in_progress")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	err = h.GigModel.AssignFreelancer(proposal.FreelancerID, gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	_, err = billing.HoldEscrow(billing.HoldEscrowRequest{Amount: gig.Budget, ClientID: gig.ClientID, GigID: gig.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	_, err = h.TransactionModel.Create(gig.Budget, gig.ID, "hold")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction 생성 실패"})
		return
	}

	updatedProposal, err := h.ProposalModel.FindByID(proposal.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 조회 실패"})
		return
	}

	updatedGig, err := h.GigModel.FindByID(gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gig":      updatedGig,
		"proposal": updatedProposal,
	})

}
