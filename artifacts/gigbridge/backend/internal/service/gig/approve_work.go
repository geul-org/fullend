package gig

import (
	"github.com/gigbridge/api/internal/model"
	"github.com/gigbridge/api/internal/authz"
	"github.com/geul-org/fullend/pkg/mail"
	"github.com/gigbridge/api/internal/billing"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/gigbridge/api/internal/states/gigstate"
	"strconv"
)

func (h *Handler) ApproveWork(c *gin.Context) {
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

	if _, err = authz.Check(authz.CheckRequest{Action: "ApproveWork", Resource: "gig", ResourceID: gig.ClientID, Role: currentUser.Role, UserID: currentUser.ID}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if err := gigstate.CanTransition(gigstate.Input{Status: gig.Status}, "ApproveWork"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	err = h.GigModel.UpdateStatus(gig.ID, "completed")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gig 수정 실패"})
		return
	}

	_, err = billing.ReleaseFunds(billing.ReleaseFundsRequest{Amount: gig.Budget, FreelancerID: gig.FreelancerID, GigID: gig.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
		return
	}

	_, err = h.TransactionModel.Create(gig.Budget, gig.ID, "release")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction 생성 실패"})
		return
	}

	freelancerUser, err := h.UserModel.FindByID(gig.FreelancerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if _, err = mail.SendTemplateEmail(mail.SendTemplateEmailRequest{Subject: "Work Approved", TemplateName: "work_approved", To: freelancerUser.Email}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "호출 실패"})
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
