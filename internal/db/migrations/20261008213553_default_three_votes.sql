-- +goose Up
ALTER TABLE boards ALTER COLUMN max_votes SET DEFAULT 3;

-- +goose Down
ALTER TABLE boards ALTER COLUMN max_votes SET DEFAULT 6;
