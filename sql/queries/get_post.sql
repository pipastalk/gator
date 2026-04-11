-- name: GetPost :one
SELECT * FROM posts
WHERE url = $1 LIMIT 1;
