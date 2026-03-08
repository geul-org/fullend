-- name: FindByID :one
SELECT * FROM reservations WHERE id = $1;

-- name: FindConflict :one
SELECT * FROM reservations
WHERE room_id = $1
  AND status = 'confirmed'
  AND start_at < $3
  AND end_at > $2
LIMIT 1;

-- name: Create :one
INSERT INTO reservations (user_id, room_id, start_at, end_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListByUserID :many
SELECT * FROM reservations WHERE user_id = $1 ORDER BY start_at DESC;

-- name: CountByRoomID :one
SELECT COUNT(*) FROM reservations WHERE room_id = $1 AND status = 'confirmed';

-- name: UpdateStatus :exec
UPDATE reservations SET status = $2 WHERE id = $1;
