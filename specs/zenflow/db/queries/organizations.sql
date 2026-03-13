-- name: OrganizationCreate :one
INSERT INTO organizations (name, plan_type, credits_balance)
VALUES ($1, $2, $3)
RETURNING *;

-- name: OrganizationFindByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: OrganizationFindByIDWithCredits :one
SELECT * FROM organizations WHERE id = $1 AND credits_balance > 0;

-- name: OrganizationDeductCredit :exec
UPDATE organizations SET credits_balance = credits_balance - 1 WHERE id = $1;
