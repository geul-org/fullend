package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/geul-org/fullend/pkg/authz"
	"github.com/geul-org/fullend/pkg/mail"
	"github.com/gigbridge/api/internal/billing"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/gigbridge/api/internal/states/gigstate"
	"strconv"
)

//fullend:gen ssot=service/gig/approve_work.ssac contract=dbcf48a
func (h *Handler) ApproveWork(c *gin.Context) {
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

	gig, err := h.GigModel.WithTx(tx).FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	if gig == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gig not found"})
		return
	}

	if _, err = authz.Check(authz.CheckRequest{Action: "ApproveWork", Resource: "gig", UserID: currentUser.ID, Role: currentUser.Role, ResourceID: gig.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	if err := gigstate.CanTransition(gigstate.Input{Status: gig.Status}, "ApproveWork"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	err = h.GigModel.WithTx(tx).UpdateStatus("completed", gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	freelancer, err := h.UserModel.WithTx(tx).FindByID(gig.FreelancerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if freelancer == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "freelancer not found"})
		return
	}

	_, err = billing.ReleaseFunds(billing.ReleaseFundsRequest{Amount: gig.Budget, FreelancerID: gig.FreelancerID, GigID: gig.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	_, err = h.TransactionModel.WithTx(tx).Create(gig.ID, "release", gig.Budget)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction 생성 실패"})
		return
	}

	if _, err = mail.SendTemplateEmail(mail.SendTemplateEmailRequest{Subject: "Work approved", TemplateName: "work_approved", To: freelancer.Email}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	updatedGig, err := h.GigModel.WithTx(tx).FindByID(gig.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 조회 실패"})
		return
	}

	if updatedGig == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "gig not found"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gig": updatedGig,
	})

}
