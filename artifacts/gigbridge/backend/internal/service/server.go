package service

import (
	"github.com/gin-gonic/gin"
	"github.com/gigbridge/api/internal/middleware"
	authsvc "github.com/gigbridge/api/internal/service/auth"
	gigsvc "github.com/gigbridge/api/internal/service/gig"
)

// Server composes domain handlers.
type Server struct {
	Auth *authsvc.Handler
	Gig *gigsvc.Handler
	JWTSecret string
}

// SetupRouter creates a gin.Engine that routes requests to the Server.
func SetupRouter(s *Server) *gin.Engine {
	r := gin.Default()

	// Auth group — JWT middleware extracts currentUser into context.
	auth := r.Group("/")
	auth.Use(middleware.BearerAuth(s.JWTSecret))

	auth.POST("/gigs/:id/proposals", s.Gig.SubmitProposal)
	auth.PUT("/gigs/:id/publish", s.Gig.PublishGig)
	r.GET("/gigs", s.Gig.ListGigs)
	auth.POST("/gigs", s.Gig.CreateGig)
	r.POST("/auth/register", s.Auth.Register)
	auth.POST("/gigs/:id/approve", s.Gig.ApproveWork)
	auth.POST("/gigs/:id/dispute", s.Gig.RaiseDispute)
	auth.POST("/gigs/:id/submit-work", s.Gig.SubmitWork)
	auth.POST("/proposals/:id/accept", s.Gig.AcceptProposal)
	auth.POST("/proposals/:id/reject", s.Gig.RejectProposal)
	r.POST("/auth/login", s.Auth.Login)
	r.GET("/gigs/:id", s.Gig.GetGig)

	return r
}
