package model

import (
	"context"
	"database/sql"
)

type transactionModelImpl struct {
	db *sql.DB
}

func NewTransactionModel(db *sql.DB) TransactionModel {
	return &transactionModelImpl{db: db}
}

func scanTransaction(s interface{ Scan(...interface{}) error }) (*Transaction, error) {
	var t Transaction
	err := s.Scan(&t.ID, &t.GigID, &t.TxType, &t.Amount, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (m *transactionModelImpl) Create(amount int64, gigID int64, txType string) (*Transaction, error) {
	row := m.db.QueryRowContext(context.Background(),
		"INSERT INTO transactions (gig_id, tx_type, amount)\nVALUES ($1, $2, $3)\nRETURNING *;",
		gigID, txType, amount)
	return scanTransaction(row)
}
