package model

import (
	"context"
	"database/sql"
)

type executionModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *executionModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewExecutionModel(db *sql.DB) ExecutionModel {
	return &executionModelImpl{db: db}
}

func scanExecution(row interface{ Scan(...interface{}) error }) (*Execution, error) {
	var e Execution
	err := row.Scan(&e.ID, &e.WorkflowID, &e.OrgID, &e.LogStatus, &e.CreditsSpent, &e.ExecutedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

//fullend:gen ssot=db/executions.sql contract=070ab6a
func (m *executionModelImpl) WithTx(tx *sql.Tx) ExecutionModel {
	return &executionModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/executions.sql contract=22d4376
func (m *executionModelImpl) Create(workflowID int64, orgID int64, logStatus string, creditsSpent int64) (*Execution, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"INSERT INTO executions (workflow_id, org_id, log_status, credits_spent)\nVALUES ($1, $2, $3, $4)\nRETURNING *;",
		workflowID, orgID, logStatus, creditsSpent)
	return scanExecution(row)
}
