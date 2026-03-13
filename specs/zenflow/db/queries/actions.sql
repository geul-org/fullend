-- name: ActionCreate :one
INSERT INTO actions (workflow_id, action_type, sequence_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ActionListByWorkflowID :many
SELECT * FROM actions WHERE workflow_id = $1 ORDER BY sequence_order ASC;
