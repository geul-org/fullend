package model

import (
	"github.com/geul-org/fullend/pkg/pagination"
)

type GigModel interface {
	AssignFreelancer(freelancerID int64, id int64) error
	Create(budget int64, clientID int64, description string, status string, title string) (*Gig, error)
	FindByID(id int64) (*Gig, error)
	List(opts QueryOpts) (*pagination.Page[Gig], error)
	UpdateStatus(id int64, status string) error
}

type ProposalModel interface {
	Create(bidAmount int64, freelancerID int64, gigID int64, status string) (*Proposal, error)
	FindByID(id int64) (*Proposal, error)
	UpdateStatus(id int64, status string) error
}

type TransactionModel interface {
	Create(amount int64, gigID int64, txType string) (*Transaction, error)
}

type UserModel interface {
	Create(email string, name string, passwordHash string, role string) (*User, error)
	FindByEmail(email string) (*User, error)
	FindByID(id int64) (*User, error)
}

type QueryOpts struct {
	Limit   int
	Offset  int
	Cursor  string
	SortCol string
	SortDir string
	Filters map[string]string
}
