package model

import (
	"context"
	"database/sql"
)

type transactionModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *transactionModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewTransactionModel(db *sql.DB) TransactionModel {
	return &transactionModelImpl{db: db}
}

func scanTransaction(row interface{ Scan(...interface{}) error }) (*Transaction, error) {
	var t Transaction
	err := row.Scan(&t.ID, &t.GigID, &t.TxType, &t.Amount, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

//fullend:gen ssot=db/transactions.sql contract=4742234
func (m *transactionModelImpl) WithTx(tx *sql.Tx) TransactionModel {
	return &transactionModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/transactions.sql contract=3febcab
func (m *transactionModelImpl) Create(gigID int64, txType string, amount int64) (*Transaction, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"INSERT INTO transactions (gig_id, tx_type, amount)\nVALUES ($1, $2, $3)\nRETURNING *;",
		gigID, txType, amount)
	return scanTransaction(row)
}
