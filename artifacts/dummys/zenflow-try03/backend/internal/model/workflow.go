package model

import (
	"context"
	"database/sql"

	"github.com/geul-org/fullend/pkg/pagination"
)

type workflowModelImpl struct {
	db *sql.DB
	tx *sql.Tx
}

func (m *workflowModelImpl) conn() interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if m.tx != nil {
		return m.tx
	}
	return m.db
}

func NewWorkflowModel(db *sql.DB) WorkflowModel {
	return &workflowModelImpl{db: db}
}

func scanWorkflow(row interface{ Scan(...interface{}) error }) (*Workflow, error) {
	var w Workflow
	err := row.Scan(&w.ID, &w.OrgID, &w.Title, &w.TriggerEvent, &w.Status, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

//fullend:gen ssot=db/workflows.sql contract=94ac658
func (m *workflowModelImpl) WithTx(tx *sql.Tx) WorkflowModel {
	return &workflowModelImpl{db: m.db, tx: tx}
}

//fullend:gen ssot=db/workflows.sql contract=14b8a49
func (m *workflowModelImpl) Create(orgID int64, title string, triggerEvent string) (*Workflow, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"INSERT INTO workflows (org_id, title, trigger_event, status)\nVALUES ($1, $2, $3, 'draft')\nRETURNING *;",
		orgID, title, triggerEvent)
	return scanWorkflow(row)
}

//fullend:gen ssot=db/workflows.sql contract=8e417ed
func (m *workflowModelImpl) FindByID(id int64) (*Workflow, error) {
	row := m.conn().QueryRowContext(context.Background(),
		"SELECT * FROM workflows WHERE id = $1;",
		id)
	v, err := scanWorkflow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

//fullend:gen ssot=db/workflows.sql contract=f5974f9
func (m *workflowModelImpl) List(orgID int64, opts QueryOpts) (*pagination.Page[Workflow], error) {
	baseArgs := []interface{}{orgID}
	countSQL, countArgs := BuildCountQuery("workflows", "org_id = $1", 1, opts)
	countArgs = append(baseArgs, countArgs...)
	var total int64
	if err := m.conn().QueryRowContext(context.Background(), countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	selectSQL, selectArgs := BuildSelectQuery("workflows", "org_id = $1", 1, opts)
	selectArgs = append(baseArgs, selectArgs...)
	rows, err := m.conn().QueryContext(context.Background(), selectSQL, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Workflow, 0)
	for rows.Next() {
		v, err := scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &pagination.Page[Workflow]{Items: items, Total: total}, nil
}

//fullend:gen ssot=db/workflows.sql contract=2b9704e
func (m *workflowModelImpl) UpdateStatus(id int64, status string) error {
	_, err := m.conn().ExecContext(context.Background(),
		"UPDATE workflows SET status = $2 WHERE id = $1;",
		id, status)
	return err
}
