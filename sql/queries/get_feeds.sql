-- name: GetFeeds :many
Select feeds.name, feeds.url, users.name AS username
FROM feeds
INNER JOIN users ON feeds.user_id = users.id
LIMIT 50;