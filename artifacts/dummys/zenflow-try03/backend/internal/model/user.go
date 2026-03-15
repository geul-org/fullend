package model

import (
	"context"
	"database/sql"
)

type userModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *userModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewUserModel(db *sql.DB) UserModel {
	return &userModelImpl{db: db}
}

func scanUser(row interface{ Scan(...interface{}) error }) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

//fullend:gen ssot=db/users.sql contract=89b1094
func (m *userModelImpl) WithTx(tx *sql.Tx) UserModel {
	return &userModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/users.sql contract=2d80f0c
func (m *userModelImpl) Create(orgID int64, email string, passwordHash string, role string) (*User, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"INSERT INTO users (org_id, email, password_hash, role)\nVALUES ($1, $2, $3, $4)\nRETURNING *;",
		orgID, email, passwordHash, role)
	return scanUser(row)
}

//fullend:gen ssot=db/users.sql contract=35374f0
func (m *userModelImpl) FindByEmail(email string) (*User, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"SELECT * FROM users WHERE email = $1;",
		email)
	v, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

//fullend:gen ssot=db/users.sql contract=2d995a5
func (m *userModelImpl) FindByID(id int64) (*User, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"SELECT * FROM users WHERE id = $1;",
		id)
	v, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}
