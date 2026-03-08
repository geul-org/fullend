-- name: FindByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: FindByID :one
SELECT * FROM users WHERE id = $1;
