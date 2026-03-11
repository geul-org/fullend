package gig

import "github.com/gigbridge/api/internal/model"

// Handler handles requests for the gig domain.
type Handler struct {
	GigModel model.GigModel
	ProposalModel model.ProposalModel
	TransactionModel model.TransactionModel
	UserModel model.UserModel
}
