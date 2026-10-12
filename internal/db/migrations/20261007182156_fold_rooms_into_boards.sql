-- +goose Up

DELETE FROM rooms WHERE NOT EXISTS (SELECT 1 FROM boards WHERE boards.id = rooms.id);

ALTER TABLE boards
    ADD COLUMN org_id UUID,
    ADD COLUMN team_id UUID,
    ADD COLUMN title TEXT,
    ADD COLUMN share_slug TEXT,
    ADD COLUMN link_access TEXT NOT NULL DEFAULT 'off',
    ADD COLUMN password_hash TEXT,
    ADD COLUMN access_epoch INT NOT NULL DEFAULT 0,
    ADD COLUMN version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN created_by UUID,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN locked_at TIMESTAMPTZ,
    ADD COLUMN closed_at TIMESTAMPTZ,
    ADD COLUMN archived_at TIMESTAMPTZ,
    ADD COLUMN deleted_at TIMESTAMPTZ;

UPDATE boards SET
    org_id = rooms.org_id,
    team_id = rooms.team_id,
    title = rooms.title,
    share_slug = rooms.share_slug,
    link_access = rooms.link_access,
    password_hash = rooms.password_hash,
    access_epoch = rooms.access_epoch,
    version = rooms.version,
    created_by = rooms.created_by,
    created_at = rooms.created_at,
    updated_at = rooms.updated_at,
    locked_at = rooms.locked_at,
    closed_at = rooms.closed_at,
    archived_at = rooms.archived_at,
    deleted_at = rooms.deleted_at
FROM rooms
WHERE rooms.id = boards.id;

ALTER TABLE room_participants DROP CONSTRAINT room_participants_room_id_fkey;
ALTER TABLE room_ops DROP CONSTRAINT room_ops_room_id_fkey;
ALTER TABLE boards DROP CONSTRAINT boards_id_fkey;

DROP TABLE rooms;

ALTER TABLE boards ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE boards ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE boards ALTER COLUMN title SET NOT NULL;
ALTER TABLE boards ALTER COLUMN share_slug SET NOT NULL;

ALTER TABLE boards
    ADD CONSTRAINT boards_org_id_fkey FOREIGN KEY (org_id) REFERENCES organisations (id) ON DELETE CASCADE,
    ADD CONSTRAINT boards_created_by_fkey FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL,
    ADD CONSTRAINT boards_org_id_id_key UNIQUE (org_id, id),
    ADD CONSTRAINT boards_share_slug_key UNIQUE (share_slug),
    ADD CONSTRAINT boards_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT boards_link_access_check CHECK (link_access IN ('off', 'guests', 'accounts')),
    ADD CONSTRAINT boards_title_length CHECK (char_length(title) BETWEEN 1 AND 200);

ALTER TABLE boards SET (fillfactor = 70);

CREATE INDEX idx_boards_team_created ON boards (team_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_boards_org_created ON boards (org_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_boards_deleted_at ON boards (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_boards_created_at ON boards (created_at);

ALTER TABLE room_participants RENAME TO board_participants;
ALTER TABLE board_participants RENAME COLUMN room_id TO board_id;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_pkey TO board_participants_pkey;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_user_id_fkey TO board_participants_user_id_fkey;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_room_id_id_key TO board_participants_board_id_id_key;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_room_user_key TO board_participants_board_user_key;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_role_check TO board_participants_role_check;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_display_name_length TO board_participants_display_name_length;
ALTER TABLE board_participants RENAME CONSTRAINT room_participants_joined_via_check TO board_participants_joined_via_check;
ALTER INDEX idx_room_participants_user RENAME TO idx_board_participants_user;
ALTER TABLE board_participants
    ADD CONSTRAINT board_participants_board_id_fkey FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE CASCADE;

ALTER TABLE room_ops RENAME TO board_ops;
ALTER TABLE board_ops RENAME COLUMN room_id TO board_id;
ALTER TABLE board_ops RENAME CONSTRAINT room_ops_pkey TO board_ops_pkey;
ALTER INDEX idx_room_ops_created_at RENAME TO idx_board_ops_created_at;
ALTER TABLE board_ops
    ADD CONSTRAINT board_ops_board_id_fkey FOREIGN KEY (board_id) REFERENCES boards (id) ON DELETE CASCADE;

-- +goose Down

ALTER TABLE board_ops DROP CONSTRAINT board_ops_board_id_fkey;
ALTER INDEX idx_board_ops_created_at RENAME TO idx_room_ops_created_at;
ALTER TABLE board_ops RENAME CONSTRAINT board_ops_pkey TO room_ops_pkey;
ALTER TABLE board_ops RENAME COLUMN board_id TO room_id;
ALTER TABLE board_ops RENAME TO room_ops;

ALTER TABLE board_participants DROP CONSTRAINT board_participants_board_id_fkey;
ALTER INDEX idx_board_participants_user RENAME TO idx_room_participants_user;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_joined_via_check TO room_participants_joined_via_check;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_display_name_length TO room_participants_display_name_length;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_role_check TO room_participants_role_check;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_board_user_key TO room_participants_room_user_key;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_board_id_id_key TO room_participants_room_id_id_key;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_user_id_fkey TO room_participants_user_id_fkey;
ALTER TABLE board_participants RENAME CONSTRAINT board_participants_pkey TO room_participants_pkey;
ALTER TABLE board_participants RENAME COLUMN board_id TO room_id;
ALTER TABLE board_participants RENAME TO room_participants;

CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    team_id UUID,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    share_slug TEXT NOT NULL,
    link_access TEXT NOT NULL DEFAULT 'off',
    password_hash TEXT,
    access_epoch INT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT rooms_org_id_id_key UNIQUE (org_id, id),
    CONSTRAINT rooms_share_slug_key UNIQUE (share_slug),
    CONSTRAINT rooms_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id) ON DELETE CASCADE,
    CONSTRAINT rooms_kind_check CHECK (kind IN ('board')),
    CONSTRAINT rooms_link_access_check CHECK (link_access IN ('off', 'guests', 'accounts')),
    CONSTRAINT rooms_title_length CHECK (char_length(title) BETWEEN 1 AND 200)
) WITH (fillfactor = 70);

INSERT INTO rooms (
    id, org_id, team_id, kind, title, share_slug, link_access, password_hash, access_epoch, version,
    created_by, created_at, updated_at, locked_at, closed_at, archived_at, deleted_at
)
SELECT
    id, org_id, team_id, 'board', title, share_slug, link_access, password_hash, access_epoch, version,
    created_by, created_at, updated_at, locked_at, closed_at, archived_at, deleted_at
FROM boards;

CREATE INDEX idx_rooms_team_created ON rooms (team_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_rooms_org_created ON rooms (org_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_rooms_deleted_at ON rooms (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_rooms_created_at ON rooms (created_at);

ALTER TABLE room_participants
    ADD CONSTRAINT room_participants_room_id_fkey FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE;
ALTER TABLE room_ops
    ADD CONSTRAINT room_ops_room_id_fkey FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE;

DROP INDEX idx_boards_created_at;
DROP INDEX idx_boards_deleted_at;
DROP INDEX idx_boards_org_created;
DROP INDEX idx_boards_team_created;

ALTER TABLE boards RESET (fillfactor);

ALTER TABLE boards
    DROP CONSTRAINT boards_title_length,
    DROP CONSTRAINT boards_link_access_check,
    DROP CONSTRAINT boards_team_fkey,
    DROP CONSTRAINT boards_share_slug_key,
    DROP CONSTRAINT boards_org_id_id_key,
    DROP CONSTRAINT boards_created_by_fkey,
    DROP CONSTRAINT boards_org_id_fkey;

ALTER TABLE boards
    DROP COLUMN deleted_at,
    DROP COLUMN archived_at,
    DROP COLUMN closed_at,
    DROP COLUMN locked_at,
    DROP COLUMN updated_at,
    DROP COLUMN created_at,
    DROP COLUMN created_by,
    DROP COLUMN version,
    DROP COLUMN access_epoch,
    DROP COLUMN password_hash,
    DROP COLUMN link_access,
    DROP COLUMN share_slug,
    DROP COLUMN title,
    DROP COLUMN team_id,
    DROP COLUMN org_id;

ALTER TABLE boards ALTER COLUMN id DROP DEFAULT;
ALTER TABLE boards
    ADD CONSTRAINT boards_id_fkey FOREIGN KEY (id) REFERENCES rooms (id) ON DELETE CASCADE;
