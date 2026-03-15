package model

import (
	"context"
	"database/sql"
	"encoding/json"
)

type actionModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *actionModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewActionModel(db *sql.DB) ActionModel {
	return &actionModelImpl{db: db}
}

func scanAction(row interface{ Scan(...interface{}) error }) (*Action, error) {
	var a Action
	err := row.Scan(&a.ID, &a.WorkflowID, &a.ActionType, &a.PayloadTemplate, &a.SequenceOrder)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

//fullend:gen ssot=db/actions.sql contract=1ec3bad
func (m *actionModelImpl) WithTx(tx *sql.Tx) ActionModel {
	return &actionModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/actions.sql contract=482f54a
func (m *actionModelImpl) Create(workflowID int64, actionType string, payloadTemplate json.RawMessage, sequenceOrder int64) (*Action, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"INSERT INTO actions (workflow_id, action_type, payload_template, sequence_order)\nVALUES ($1, $2, $3, $4)\nRETURNING *;",
		workflowID, actionType, payloadTemplate, sequenceOrder)
	return scanAction(row)
}

//fullend:gen ssot=db/actions.sql contract=05cc25c
func (m *actionModelImpl) ListByWorkflow(workflowID int64) ([]Action, error) {
	rows, err := m.conn().QueryContext(context.Background(),
		"SELECT * FROM actions WHERE workflow_id = $1 ORDER BY sequence_order ASC;",
		workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Action, 0)
	for rows.Next() {
		v, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
