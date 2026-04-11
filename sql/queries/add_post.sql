-- name: CreatePost :one
INSERT INTO posts (id, title, created_at, updated_at, url, description, feed_id, published_at) 
VALUES (DEFAULT, $1, DEFAULT, DEFAULT, $2, $3, $4, $5) 
RETURNING *;