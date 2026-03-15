-- name: UserCreate :one
INSERT INTO users (org_id, email, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UserFindByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UserFindByID :one
SELECT * FROM users WHERE id = $1;
