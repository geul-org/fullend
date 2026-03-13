package action

import (
	"github.com/zenflow/zenflow/internal/model"
	"github.com/geul-org/fullend/pkg/authz"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

//fullend:gen ssot=service/action/create_action.ssac contract=738d5e3
func (h *Handler) CreateAction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path parameter"})
		return
	}

	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	var req struct {
		ActionType      string `json:"action_type"`
		PayloadTemplate string `json:"payload_template"`
		SequenceOrder   int64  `json:"sequence_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	actionType := req.ActionType
	payloadTemplate := req.PayloadTemplate
	sequenceOrder := req.SequenceOrder

	tx, err := h.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback()

	if _, err = authz.Check(authz.CheckRequest{Action: "CreateAction", Resource: "action", UserID: currentUser.ID, Role: currentUser.Role}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	user, err := h.UserModel.WithTx(tx).FindByID(currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	workflow, err := h.WorkflowModel.WithTx(tx).FindByIDAndOrg(id, user.OrgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Workflow 조회 실패"})
		return
	}

	if workflow == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	action, err := h.ActionModel.WithTx(tx).Create(workflow.ID, actionType, payloadTemplate, sequenceOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Action 생성 실패"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"action": action,
	})

}
