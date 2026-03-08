-- name: FindByID :one
SELECT * FROM rooms WHERE id = $1;

-- name: Delete :exec
DELETE FROM rooms WHERE id = $1;

-- name: Update :exec
UPDATE rooms SET name = $2, capacity = $3, location = $4 WHERE id = $1;
