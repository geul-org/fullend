package service

import (
	"github.com/gin-gonic/gin"
	"github.com/zenflow/zenflow/internal/middleware"
	actionsvc "github.com/zenflow/zenflow/internal/service/action"
	authsvc "github.com/zenflow/zenflow/internal/service/auth"
	organizationsvc "github.com/zenflow/zenflow/internal/service/organization"
	workflowsvc "github.com/zenflow/zenflow/internal/service/workflow"
)

// Server composes domain handlers.
type Server struct {
	Action *actionsvc.Handler
	Auth *authsvc.Handler
	Organization *organizationsvc.Handler
	Workflow *workflowsvc.Handler
	JWTSecret string
}

// SetupRouter creates a gin.Engine that routes requests to the Server.
func SetupRouter(s *Server) *gin.Engine {
	r := gin.Default()

	// Auth group — JWT middleware extracts currentUser into context.
	auth := r.Group("/")
	auth.Use(middleware.BearerAuth(s.JWTSecret))

	r.POST("/organizations", s.Organization.CreateOrganization)
	auth.POST("/workflows/:id/pause", s.Workflow.PauseWorkflow)
	r.POST("/auth/register", s.Auth.Register)
	auth.GET("/workflows", s.Workflow.ListWorkflows)
	auth.POST("/workflows", s.Workflow.CreateWorkflow)
	auth.GET("/workflows/:id", s.Workflow.GetWorkflow)
	auth.GET("/workflows/:id/actions", s.Action.ListActions)
	auth.POST("/workflows/:id/actions", s.Action.CreateAction)
	auth.POST("/workflows/:id/archive", s.Workflow.ArchiveWorkflow)
	auth.POST("/workflows/:id/execute", s.Workflow.ExecuteWorkflow)
	auth.POST("/workflows/:id/activate", s.Workflow.ActivateWorkflow)
	r.POST("/auth/login", s.Auth.Login)

	return r
}
