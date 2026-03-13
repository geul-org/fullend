package model

import (
	"context"
	"database/sql"

	"github.com/geul-org/fullend/pkg/pagination"
)

type gigModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *gigModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewGigModel(db *sql.DB) GigModel {
	return &gigModelImpl{db: db}
}

func scanGig(row interface{ Scan(...interface{}) error }) (*Gig, error) {
	var g Gig
	err := row.Scan(&g.ID, &g.ClientID, &g.Title, &g.Description, &g.Budget, &g.Status, &g.FreelancerID, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

//fullend:gen ssot=db/gigs.sql contract=f551e3b
func (m *gigModelImpl) WithTx(tx *sql.Tx) GigModel {
	return &gigModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/gigs.sql contract=63fc191
func (m *gigModelImpl) Create(clientID int64, title string, description string, budget int64) (*Gig, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"INSERT INTO gigs (client_id, title, description, budget)\nVALUES ($1, $2, $3, $4)\nRETURNING *;",
		clientID, title, description, budget)
	return scanGig(row)
}

//fullend:gen ssot=db/gigs.sql contract=b3e0514
func (m *gigModelImpl) FindByID(id int64) (*Gig, error) {
	row := m.conn().QueryRowContext(context.Background(),
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

//fullend:gen ssot=db/gigs.sql contract=d5c4d5d
func (m *gigModelImpl) List(opts QueryOpts) (*pagination.Page[Gig], error) {
	countSQL, countArgs := BuildCountQuery("gigs", "", 0, opts)
	var total int64
	if err := m.conn().QueryRowContext(context.Background(), countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	selectSQL, selectArgs := BuildSelectQuery("gigs", "", 0, opts)
	rows, err := m.conn().QueryContext(context.Background(), selectSQL, selectArgs...)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := m.includeClient(items); err != nil {
		return nil, err
	}
	return &pagination.Page[Gig]{Items: items, Total: total}, nil
}

//fullend:gen ssot=db/gigs.sql contract=eaf3796
func (m *gigModelImpl) UpdateFreelancerID(freelancerID int64, id int64) error {
	_, err := m.conn().ExecContext(context.Background(),
		"UPDATE gigs SET freelancer_id = $1 WHERE id = $2;",
		freelancerID, id)
	return err
}

//fullend:gen ssot=db/gigs.sql contract=dddd157
func (m *gigModelImpl) UpdateStatus(status string, id int64) error {
	_, err := m.conn().ExecContext(context.Background(),
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
	rows, err := m.conn().QueryContext(context.Background(),
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
