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

	r.POST("/auth/login", s.Auth.Login)
	auth.POST("/proposals/:id/accept", s.Gig.AcceptProposal)
	r.POST("/auth/register", s.Auth.Register)
	r.GET("/gigs/:id", s.Gig.GetGig)
	auth.POST("/gigs/:id/proposals", s.Gig.SubmitProposal)
	auth.POST("/proposals/:id/reject", s.Gig.RejectProposal)
	r.GET("/gigs", s.Gig.ListGigs)
	auth.POST("/gigs", s.Gig.CreateGig)
	auth.POST("/gigs/:id/approve", s.Gig.ApproveWork)
	auth.PUT("/gigs/:id/publish", s.Gig.PublishGig)
	auth.POST("/gigs/:id/submit-work", s.Gig.SubmitWork)
	auth.POST("/gigs/:id/dispute", s.Gig.RaiseDispute)

	return r
}
