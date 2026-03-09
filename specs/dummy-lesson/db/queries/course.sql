-- name: CourseFindByID :one
SELECT * FROM courses WHERE id = $1;

-- name: CourseList :many
SELECT * FROM courses WHERE published = TRUE ORDER BY created_at DESC;

-- name: CourseCreate :one
INSERT INTO courses (instructor_id, title, description, category, level, price)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: CourseUpdate :exec
UPDATE courses SET title = $2, description = $3, category = $4, level = $5, price = $6
WHERE id = $1;

-- name: CoursePublish :exec
UPDATE courses SET published = TRUE WHERE id = $1;

-- name: CourseDelete :exec
DELETE FROM courses WHERE id = $1;
