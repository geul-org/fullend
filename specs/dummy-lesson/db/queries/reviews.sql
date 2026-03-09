-- name: ReviewFindByID :one
SELECT * FROM reviews WHERE id = $1;

-- name: ReviewFindByCourseAndUser :one
SELECT * FROM reviews WHERE course_id = $1 AND user_id = $2;

-- name: ReviewListByCourse :many
SELECT * FROM reviews WHERE course_id = $1 ORDER BY created_at DESC;

-- name: ReviewCreate :one
INSERT INTO reviews (user_id, course_id, rating, comment)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ReviewDelete :exec
DELETE FROM reviews WHERE id = $1;
