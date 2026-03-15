package workflow

import (
	"github.com/zenflow/zenflow/internal/model"
	"github.com/geul-org/fullend/pkg/authz"
	"github.com/gin-gonic/gin"
	"net/http"
)

//fullend:gen ssot=service/workflow/list_workflows.ssac contract=841bbd3
func (h *Handler) ListWorkflows(c *gin.Context) {
	currentUser := c.MustGet("currentUser").(*model.CurrentUser)

	opts := model.ParseQueryOpts(c, model.QueryOptsConfig{
		Pagination: &model.PaginationConfig{Style: "offset", DefaultLimit: 20, MaxLimit: 100},
		Sort:       &model.SortConfig{Allowed: []string{"created_at"}, Default: "created_at", Direction: "desc"},
		Filter:     &model.FilterConfig{Allowed: []string{"status"}},
	})

	if _, err := authz.Check(authz.CheckRequest{Action: "ListWorkflows", Resource: "workflow", UserID: currentUser.ID, Role: currentUser.Role}); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	me, err := h.UserModel.FindByID(currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User 조회 실패"})
		return
	}

	if me == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	workflowPage, err := h.WorkflowModel.List(me.OrgID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Workflow 조회 실패"})
		return
	}

	c.JSON(http.StatusOK, workflowPage)

}
