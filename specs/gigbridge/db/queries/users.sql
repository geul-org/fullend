-- name: UserCreate :one
INSERT INTO users (email, password_hash, role, name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UserFindByID :one
SELECT * FROM users WHERE id = $1;

-- name: UserFindByEmail :one
SELECT * FROM users WHERE email = $1;
