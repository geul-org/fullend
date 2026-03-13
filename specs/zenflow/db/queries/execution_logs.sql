-- name: ExecutionLogCreate :one
INSERT INTO execution_logs (workflow_id, org_id, status)
VALUES ($1, $2, $3)
RETURNING *;
