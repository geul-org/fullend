-- name: OrganizationCreate :one
INSERT INTO organizations (name, plan_type)
VALUES ($1, $2)
RETURNING *;

-- name: OrganizationFindByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: OrganizationDeductOneCredit :exec
UPDATE organizations SET credits_balance = credits_balance - 1 WHERE id = $1;
