-- name: GetFeed :one
SELECT * FROM feeds
WHERE url = $1 LIMIT 1;
