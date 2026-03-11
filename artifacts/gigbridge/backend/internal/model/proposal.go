package model

import (
	"context"
	"database/sql"
)

type proposalModelImpl struct {
	db *sql.DB
}

func NewProposalModel(db *sql.DB) ProposalModel {
	return &proposalModelImpl{db: db}
}

func scanProposal(s interface{ Scan(...interface{}) error }) (*Proposal, error) {
	var p Proposal
	err := s.Scan(&p.ID, &p.GigID, &p.FreelancerID, &p.BidAmount, &p.Status, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (m *proposalModelImpl) Create(bidAmount int64, freelancerID int64, gigID int64, status string) (*Proposal, error) {
	row := m.db.QueryRowContext(context.Background(),
		"INSERT INTO proposals (gig_id, freelancer_id, bid_amount, status)\nVALUES ($1, $2, $3, $4)\nRETURNING *;",
		gigID, freelancerID, bidAmount, status)
	return scanProposal(row)
}

func (m *proposalModelImpl) FindByID(id int64) (*Proposal, error) {
	row := m.db.QueryRowContext(context.Background(),
		"SELECT * FROM proposals WHERE id = $1;",
		id)
	v, err := scanProposal(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

func (m *proposalModelImpl) UpdateStatus(id int64, status string) error {
	_, err := m.db.ExecContext(context.Background(),
		"UPDATE proposals SET status = $1 WHERE id = $2;",
		status, id)
	return err
}
