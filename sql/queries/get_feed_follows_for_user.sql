-- name: GetFeedFollowsForUser :many
SELECT 
    feed_follows.*,
    users.name as user_name,
    feeds.name as feed_name,
    feeds.url as feed_url
FROM feed_follows 
INNER JOIN users ON feed_follows.user_id = users.id
INNER JOIN feeds ON feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;