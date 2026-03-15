package model

import (
	"context"
	"database/sql"
)

type organizationModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *organizationModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewOrganizationModel(db *sql.DB) OrganizationModel {
	return &organizationModelImpl{db: db}
}

func scanOrganization(row interface{ Scan(...interface{}) error }) (*Organization, error) {
	var o Organization
	err := row.Scan(&o.ID, &o.Name, &o.PlanType, &o.CreditsBalance)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

//fullend:gen ssot=db/organizations.sql contract=66924c5
func (m *organizationModelImpl) WithTx(tx *sql.Tx) OrganizationModel {
	return &organizationModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/organizations.sql contract=95083cb
func (m *organizationModelImpl) FindByID(id int64) (*Organization, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"SELECT * FROM organizations WHERE id = $1;",
		id)
	v, err := scanOrganization(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}
