-- name: EnrollmentFindByID :one
SELECT * FROM enrollments WHERE id = $1;

-- name: EnrollmentFindByCourseAndUser :one
SELECT * FROM enrollments WHERE course_id = $1 AND user_id = $2;

-- name: EnrollmentListByUser :many
SELECT * FROM enrollments WHERE user_id = $1 ORDER BY created_at DESC;

-- name: EnrollmentCreate :one
INSERT INTO enrollments (user_id, course_id)
VALUES ($1, $2) RETURNING *;
