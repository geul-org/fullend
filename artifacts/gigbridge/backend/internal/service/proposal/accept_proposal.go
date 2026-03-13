package proposal

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/geul-org/fullend/pkg/authz"
	"github.com/gigbridge/api/internal/billing"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/gigbridge/api/internal/states/gigstate"
	"github.com/gigbridge/api/internal/states/proposalstate"
	"strconv"
)

//fullend:gen ssot=service/proposal/accept_proposal.ssac contract=097ba74
func (h *Handler) AcceptProposal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	tx, err := h.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback()

	proposal, err := h.ProposalModel.WithTx(tx).FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 조회 실패"})
		return
	}

	if proposal == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proposal not found"})
		return
	}

	gig, err := h.GigModel.WithTx(tx).FindByID(proposal.GigID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	if gig == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gig not found"})
		return
	}

	if _, err = authz.Check(authz.CheckRequest{Action: "AcceptProposal", Resource: "gig", UserID: currentUser.ID, Role: currentUser.Role, ResourceID: gig.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
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

	err = h.ProposalModel.WithTx(tx).UpdateStatus("accepted", proposal.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 수정 실패"})
		return
	}

	err = h.GigModel.WithTx(tx).UpdateStatus("in_progress", gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	err = h.GigModel.WithTx(tx).UpdateFreelancerID(proposal.FreelancerID, gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	_, err = billing.HoldEscrow(billing.HoldEscrowRequest{Amount: gig.Budget, ClientID: gig.ClientID, GigID: gig.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	_, err = h.TransactionModel.WithTx(tx).Create(gig.ID, "hold", gig.Budget)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction 생성 실패"})
		return
	}

	updatedProposal, err := h.ProposalModel.WithTx(tx).FindByID(proposal.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 조회 실패"})
		return
	}

	if updatedProposal == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proposal not found"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"proposal": updatedProposal,
	})

}
