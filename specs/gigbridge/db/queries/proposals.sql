-- name: ProposalCreate :one
INSERT INTO proposals (gig_id, freelancer_id, bid_amount)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ProposalFindByID :one
SELECT * FROM proposals WHERE id = $1;

-- name: ProposalUpdateStatus :exec
UPDATE proposals SET status = $1 WHERE id = $2;
