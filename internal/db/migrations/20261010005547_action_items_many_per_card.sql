-- +goose Up
CREATE INDEX idx_action_items_source_card ON action_items (board_id, source_card_id) WHERE source_card_id IS NOT NULL;
ALTER TABLE action_items DROP CONSTRAINT action_items_source_card_key;

-- +goose Down
ALTER TABLE action_items ADD CONSTRAINT action_items_source_card_key UNIQUE (source_card_id);
DROP INDEX idx_action_items_source_card;
