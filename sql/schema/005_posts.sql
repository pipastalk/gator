-- +goose Up
CREATE TABLE posts ( 
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    title TEXT,
    url TEXT NOT NULL UNIQUE,
    description TEXT,
    feed_id UUID NOT NULL, 
    published_at TIMESTAMPTZ,
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE posts;