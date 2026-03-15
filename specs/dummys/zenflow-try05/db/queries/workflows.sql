-- name: WorkflowCreate :one
INSERT INTO workflows (org_id, title, trigger_event, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: WorkflowFindByID :one
SELECT * FROM workflows WHERE id = $1;

-- name: WorkflowFindByIDAndOrg :one
SELECT * FROM workflows WHERE id = $1 AND org_id = $2;

-- name: WorkflowListByOrg :many
SELECT * FROM workflows WHERE org_id = $1;

-- name: WorkflowUpdateStatus :exec
UPDATE workflows SET status = $2 WHERE id = $1;
