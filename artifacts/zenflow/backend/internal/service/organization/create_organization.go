package organization

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/organization/create_organization.ssac contract=f5103f6
func (h *Handler) CreateOrganization(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		PlanType       string `json:"plan_type"`
		CreditsBalance int64  `json:"credits_balance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	name := req.Name
	planType := req.PlanType
	creditsBalance := req.CreditsBalance

	tx, err := h.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback()

	org, err := h.OrganizationModel.WithTx(tx).Create(name, planType, creditsBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization 생성 실패"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"organization": org,
	})

}
