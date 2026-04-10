-- name: GetUser :one
SELECT * FROM users
WHERE name = $1 LIMIT 1;
