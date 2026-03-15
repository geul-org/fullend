-- name: ExecutionCreate :one
INSERT INTO executions (workflow_id, org_id, log_status, credits_spent)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ExecutionListByWorkflow :many
SELECT * FROM executions WHERE workflow_id = $1 ORDER BY executed_at DESC;
