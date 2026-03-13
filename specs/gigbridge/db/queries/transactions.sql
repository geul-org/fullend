-- name: TransactionCreate :one
INSERT INTO transactions (gig_id, tx_type, amount)
VALUES ($1, $2, $3)
RETURNING *;
