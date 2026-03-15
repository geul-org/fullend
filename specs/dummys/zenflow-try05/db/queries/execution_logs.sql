-- name: ExecutionLogCreate :one
INSERT INTO execution_logs (workflow_id, org_id)
VALUES ($1, $2)
RETURNING *;

-- name: ExecutionLogListByOrg :many
SELECT * FROM execution_logs WHERE org_id = $1 ORDER BY executed_at DESC;
