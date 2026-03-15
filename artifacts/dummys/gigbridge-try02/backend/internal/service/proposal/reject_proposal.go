package proposal

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/geul-org/fullend/pkg/authz"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/gigbridge/api/internal/states/proposalstate"
	"strconv"
)

//fullend:gen ssot=service/proposal/reject_proposal.ssac contract=a560536
func (h *Handler) RejectProposal(c *gin.Context) {
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

	if _, err = authz.Check(authz.CheckRequest{Action: "RejectProposal", Resource: "gig", UserID: currentUser.ID, Role: currentUser.Role, ResourceID: gig.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	if err := proposalstate.CanTransition(proposalstate.Input{Status: proposal.Status}, "RejectProposal"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	err = h.ProposalModel.WithTx(tx).UpdateStatus("rejected", proposal.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Proposal 수정 실패"})
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
