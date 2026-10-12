-- +goose Up
CREATE TABLE saved_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    team_id UUID,
    owner_id UUID,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    columns JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT saved_templates_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE,
    CONSTRAINT saved_templates_owner_fkey FOREIGN KEY (org_id, owner_id) REFERENCES organisation_members (org_id, user_id) ON DELETE CASCADE,
    CONSTRAINT saved_templates_one_scope CHECK ((team_id IS NULL) <> (owner_id IS NULL)),
    CONSTRAINT saved_templates_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    CONSTRAINT saved_templates_description_length CHECK (char_length(description) <= 300),
    CONSTRAINT saved_templates_columns_shape CHECK (jsonb_typeof(columns) = 'array' AND jsonb_array_length(columns) >= 1)
);
CREATE INDEX idx_saved_templates_org ON saved_templates (org_id, created_at, id);

-- +goose Down
DROP TABLE saved_templates;
