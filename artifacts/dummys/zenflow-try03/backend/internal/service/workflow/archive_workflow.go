package workflow

import (
	"github.com/zenflow/zenflow/internal/model"
	"github.com/geul-org/fullend/pkg/authz"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/zenflow/zenflow/internal/states/workflowstate"
	"strconv"
)

//fullend:gen ssot=service/workflow/archive_workflow.ssac contract=4942586
func (h *Handler) ArchiveWorkflow(c *gin.Context) {
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

	if _, err = authz.Check(authz.CheckRequest{Action: "ActivateWorkflow", Resource: "workflow", UserID: currentUser.ID, Role: currentUser.Role, ResourceID: id}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can archive workflows"})
		return
	}

	workflow, err := h.WorkflowModel.WithTx(tx).FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Workflow 조회 실패"})
		return
	}

	if workflow == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	if err := workflowstate.CanTransition(workflowstate.Input{Status: workflow.Status}, "ArchiveWorkflow"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	err = h.WorkflowModel.WithTx(tx).UpdateStatus(workflow.ID, "archived")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Workflow 수정 실패"})
		return
	}

	updated, err := h.WorkflowModel.WithTx(tx).FindByID(workflow.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Workflow 조회 실패"})
		return
	}

	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow": updated,
	})

}
