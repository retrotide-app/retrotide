-- +goose Up
CREATE INDEX idx_users_created_at ON users (created_at);
CREATE INDEX idx_organisations_created_at ON organisations (created_at);
CREATE INDEX idx_teams_created_at ON teams (created_at);
CREATE INDEX idx_rooms_created_at ON rooms (created_at);
CREATE INDEX idx_cards_created_at ON cards (created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_cards_created_at;
DROP INDEX IF EXISTS idx_rooms_created_at;
DROP INDEX IF EXISTS idx_teams_created_at;
DROP INDEX IF EXISTS idx_organisations_created_at;
DROP INDEX IF EXISTS idx_users_created_at;
