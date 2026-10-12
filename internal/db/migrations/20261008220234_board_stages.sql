-- +goose Up
ALTER TABLE boards ADD COLUMN stage TEXT NOT NULL DEFAULT 'write';
ALTER TABLE boards ADD CONSTRAINT boards_stage_check CHECK (stage IN ('write', 'vote', 'discuss', 'actions', 'end'));

UPDATE boards SET stage = CASE
    WHEN closed_at IS NOT NULL THEN 'end'
    WHEN NOT cards_hidden THEN 'discuss'
    ELSE 'write'
END;

-- +goose Down
ALTER TABLE boards DROP CONSTRAINT boards_stage_check;
ALTER TABLE boards DROP COLUMN stage;
