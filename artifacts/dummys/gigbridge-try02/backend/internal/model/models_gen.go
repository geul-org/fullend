package model

import (
	"database/sql"

	"github.com/geul-org/fullend/pkg/pagination"
)

type GigModel interface {
	WithTx(tx *sql.Tx) GigModel
	Create(clientID int64, title string, description string, budget int64) (*Gig, error)
	FindByID(id int64) (*Gig, error)
	List(opts QueryOpts) (*pagination.Page[Gig], error)
	UpdateFreelancerID(freelancerID int64, id int64) error
	UpdateStatus(status string, id int64) error
}

type ProposalModel interface {
	WithTx(tx *sql.Tx) ProposalModel
	Create(gigID int64, freelancerID int64, bidAmount int64) (*Proposal, error)
	FindByID(id int64) (*Proposal, error)
	UpdateStatus(status string, id int64) error
}

type TransactionModel interface {
	WithTx(tx *sql.Tx) TransactionModel
	Create(gigID int64, txType string, amount int64) (*Transaction, error)
}

type UserModel interface {
	WithTx(tx *sql.Tx) UserModel
	Create(email string, passwordHash string, role string, name string) (*User, error)
	FindByEmail(email string) (*User, error)
	FindByID(id int64) (*User, error)
}
