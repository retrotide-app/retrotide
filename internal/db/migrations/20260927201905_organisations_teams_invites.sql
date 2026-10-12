-- +goose Up

CREATE TABLE organisations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    allow_guests BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT organisations_slug_key UNIQUE (slug),
    CONSTRAINT organisations_slug_format CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$'),
    CONSTRAINT organisations_name_length CHECK (char_length(name) BETWEEN 1 AND 80)
);
CREATE INDEX idx_organisations_created_by ON organisations (created_by) WHERE created_by IS NOT NULL;
CREATE INDEX idx_organisations_deleted_at ON organisations (deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE organisation_members (
    org_id UUID NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, user_id),
    CONSTRAINT organisation_members_role_check CHECK (role IN ('owner', 'admin', 'member'))
);
CREATE UNIQUE INDEX organisation_members_one_owner ON organisation_members (org_id) WHERE role = 'owner';
CREATE INDEX idx_organisation_members_user ON organisation_members (user_id);

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    name CITEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT teams_org_id_id_key UNIQUE (org_id, id),
    CONSTRAINT teams_org_name_key UNIQUE (org_id, name),
    CONSTRAINT teams_name_length CHECK (char_length(name) BETWEEN 1 AND 80)
);

CREATE TABLE team_members (
    org_id UUID NOT NULL,
    team_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id),
    CONSTRAINT team_members_role_check CHECK (role IN ('lead', 'member')),
    CONSTRAINT team_members_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE,
    CONSTRAINT team_members_org_member_fkey FOREIGN KEY (org_id, user_id) REFERENCES organisation_members (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX idx_team_members_org_user ON team_members (org_id, user_id);

CREATE TABLE organisation_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    team_id UUID,
    token_hash TEXT NOT NULL,
    email CITEXT,
    role TEXT NOT NULL DEFAULT 'member',
    max_uses INT,
    use_count INT NOT NULL DEFAULT 0,
    invited_by UUID REFERENCES users (id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT organisation_invites_token_hash_key UNIQUE (token_hash),
    CONSTRAINT organisation_invites_role_check CHECK (role IN ('admin', 'member')),
    CONSTRAINT organisation_invites_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE,
    CONSTRAINT organisation_invites_uses_check CHECK (max_uses IS NULL OR (max_uses > 0 AND use_count <= max_uses)),
    CONSTRAINT organisation_invites_email_single_use CHECK (email IS NULL OR (max_uses IS NOT NULL AND max_uses = 1)),
    CONSTRAINT organisation_invites_email_length CHECK (email IS NULL OR char_length(email) BETWEEN 3 AND 254)
);
CREATE INDEX idx_organisation_invites_org_created ON organisation_invites (org_id, created_at DESC);
CREATE INDEX idx_organisation_invites_team ON organisation_invites (org_id, team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_organisation_invites_invited_by ON organisation_invites (invited_by) WHERE invited_by IS NOT NULL;
CREATE INDEX idx_organisation_invites_expires_at ON organisation_invites (expires_at);

-- +goose Down

DROP TABLE organisation_invites;
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE organisation_members;
DROP TABLE organisations;
