-- name: GigCreate :one
INSERT INTO gigs (client_id, title, description, budget)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GigFindByID :one
SELECT * FROM gigs WHERE id = $1;

-- name: GigList :many
SELECT * FROM gigs ORDER BY created_at DESC;

-- name: GigUpdateStatus :exec
UPDATE gigs SET status = $1 WHERE id = $2;

-- name: GigUpdateFreelancerID :exec
UPDATE gigs SET freelancer_id = $1 WHERE id = $2;
