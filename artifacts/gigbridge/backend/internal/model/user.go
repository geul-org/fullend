package model

import (
	"context"
	"database/sql"
)

type userModelImpl struct {
	db *sql.DB
}

func NewUserModel(db *sql.DB) UserModel {
	return &userModelImpl{db: db}
}

func scanUser(s interface{ Scan(...interface{}) error }) (*User, error) {
	var u User
	err := s.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (m *userModelImpl) Create(email string, name string, passwordHash string, role string) (*User, error) {
	row := m.db.QueryRowContext(context.Background(),
		"INSERT INTO users (email, password_hash, role, name)\nVALUES ($1, $2, $3, $4)\nRETURNING *;",
		email, passwordHash, role, name)
	return scanUser(row)
}

func (m *userModelImpl) FindByEmail(email string) (*User, error) {
	row := m.db.QueryRowContext(context.Background(),
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

func (m *userModelImpl) FindByID(id int64) (*User, error) {
	row := m.db.QueryRowContext(context.Background(),
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
