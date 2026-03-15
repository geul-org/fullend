package workflow

import (
	"database/sql"

	"github.com/zenflow/zenflow/internal/model"
)

// Handler handles requests for the workflow domain.
type Handler struct {
	DB *sql.DB
	ActionModel model.ActionModel
	ExecutionModel model.ExecutionModel
	OrganizationModel model.OrganizationModel
	UserModel model.UserModel
	WorkflowModel model.WorkflowModel
}
