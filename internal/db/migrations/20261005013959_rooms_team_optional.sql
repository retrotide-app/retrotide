-- +goose Up
ALTER TABLE rooms ALTER COLUMN team_id DROP NOT NULL;
CREATE INDEX idx_rooms_org_created ON rooms (org_id, created_at DESC) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_rooms_org_created;
ALTER TABLE rooms ALTER COLUMN team_id SET NOT NULL;
