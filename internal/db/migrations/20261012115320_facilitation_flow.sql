-- +goose Up
ALTER TABLE boards
    ADD COLUMN revealed_column_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN hidden_column_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN presenting_card_id UUID,
    ADD COLUMN highlighted_card_id UUID,
    ADD COLUMN focused_column_id UUID,
    ADD COLUMN timer_auto_lock BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN sort_enforced BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN anyone_can_edit BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT boards_revealed_column_ids_array CHECK (jsonb_typeof(revealed_column_ids) = 'array'),
    ADD CONSTRAINT boards_hidden_column_ids_array CHECK (jsonb_typeof(hidden_column_ids) = 'array'),
    ADD CONSTRAINT boards_presenting_card_fkey FOREIGN KEY (id, presenting_card_id) REFERENCES cards (board_id, id) ON DELETE SET NULL (presenting_card_id),
    ADD CONSTRAINT boards_highlighted_card_fkey FOREIGN KEY (id, highlighted_card_id) REFERENCES cards (board_id, id) ON DELETE SET NULL (highlighted_card_id),
    ADD CONSTRAINT boards_focused_column_fkey FOREIGN KEY (id, focused_column_id) REFERENCES board_columns (board_id, id) ON DELETE SET NULL (focused_column_id);

CREATE INDEX idx_boards_auto_lock_timers ON boards (timer_ends_at) WHERE timer_auto_lock AND timer_ends_at IS NOT NULL AND locked_at IS NULL AND closed_at IS NULL;

ALTER TABLE board_columns
    ADD COLUMN max_votes SMALLINT,
    ADD CONSTRAINT board_columns_max_votes_check CHECK (max_votes IS NULL OR max_votes BETWEEN 1 AND 99);

-- +goose Down
ALTER TABLE board_columns
    DROP CONSTRAINT board_columns_max_votes_check,
    DROP COLUMN max_votes;

DROP INDEX idx_boards_auto_lock_timers;

ALTER TABLE boards
    DROP CONSTRAINT boards_focused_column_fkey,
    DROP CONSTRAINT boards_highlighted_card_fkey,
    DROP CONSTRAINT boards_presenting_card_fkey,
    DROP CONSTRAINT boards_hidden_column_ids_array,
    DROP CONSTRAINT boards_revealed_column_ids_array,
    DROP COLUMN anyone_can_edit,
    DROP COLUMN sort_enforced,
    DROP COLUMN timer_auto_lock,
    DROP COLUMN focused_column_id,
    DROP COLUMN highlighted_card_id,
    DROP COLUMN presenting_card_id,
    DROP COLUMN hidden_column_ids,
    DROP COLUMN revealed_column_ids;
