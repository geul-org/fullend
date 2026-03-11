package model

import (
	"context"
	"database/sql"

	"github.com/geul-org/fullend/pkg/pagination"
)

type gigModelImpl struct {
	db *sql.DB
}

func NewGigModel(db *sql.DB) GigModel {
	return &gigModelImpl{db: db}
}

func scanGig(s interface{ Scan(...interface{}) error }) (*Gig, error) {
	var g Gig
	err := s.Scan(&g.ID, &g.ClientID, &g.Title, &g.Description, &g.Budget, &g.Status, &g.FreelancerID, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (m *gigModelImpl) AssignFreelancer(freelancerID int64, id int64) error {
	_, err := m.db.ExecContext(context.Background(),
		"UPDATE gigs SET freelancer_id = $1 WHERE id = $2;",
		freelancerID, id)
	return err
}

func (m *gigModelImpl) Create(budget int64, clientID int64, description string, status string, title string) (*Gig, error) {
	row := m.db.QueryRowContext(context.Background(),
		"INSERT INTO gigs (client_id, title, description, budget, status)\nVALUES ($1, $2, $3, $4, $5)\nRETURNING *;",
		clientID, title, description, budget, status)
	return scanGig(row)
}

func (m *gigModelImpl) FindByID(id int64) (*Gig, error) {
	row := m.db.QueryRowContext(context.Background(),
		"SELECT * FROM gigs WHERE id = $1;",
		id)
	v, err := scanGig(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

func (m *gigModelImpl) List(opts QueryOpts) (*pagination.Page[Gig], error) {
	countSQL, countArgs := BuildCountQuery("gigs", "", 0, opts)
	var total int64
	if err := m.db.QueryRowContext(context.Background(), countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	selectSQL, selectArgs := BuildSelectQuery("gigs", "", 0, opts)
	rows, err := m.db.QueryContext(context.Background(), selectSQL, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Gig, 0)
	for rows.Next() {
		v, err := scanGig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *v)
	}
	if err := m.includeClient(items); err != nil {
		return nil, err
	}
	return &pagination.Page[Gig]{Items: items, Total: total}, nil
}

func (m *gigModelImpl) UpdateStatus(id int64, status string) error {
	_, err := m.db.ExecContext(context.Background(),
		"UPDATE gigs SET status = $1 WHERE id = $2;",
		status, id)
	return err
}

func (m *gigModelImpl) includeClient(items []Gig) error {
	ids := make(map[int64]bool)
	for _, item := range items {
		ids[item.ClientID] = true
	}
	if len(ids) == 0 {
		return nil
	}
	keys := collectInt64s(ids)
	placeholders := buildPlaceholders(len(keys))
	args := int64sToArgs(keys)
	rows, err := m.db.QueryContext(context.Background(),
		"SELECT * FROM users WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	lookup := make(map[int64]*User)
	for rows.Next() {
		v, err := scanUser(rows)
		if err != nil {
			return err
		}
		lookup[v.ID] = v
	}
	for i := range items {
		items[i].Client = lookup[items[i].ClientID]
	}
	return nil
}
