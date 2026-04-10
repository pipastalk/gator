-- name: CreateFeedFollow :one
WITH inserted_feed_follows AS (
    INSERT INTO feed_follows (
        id,
        created_at,
        updated_at,
        user_id,
        feed_id
    )
    VALUES (
        DEFAULT,
        DEFAULT,
        DEFAULT,
        $1,
        $2
    )
    RETURNING *
)
SELECT 
    inserted_feed_follows.*,
    feeds.name as feed_name,
    users.name as user_name
FROM inserted_feed_follows
INNER JOIN users ON inserted_feed_follows.user_id = users.id
INNER JOIN feeds ON inserted_feed_follows.feed_id = feeds.id;