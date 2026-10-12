-- +goose Up

CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organisations (id) ON DELETE CASCADE,
    team_id UUID NOT NULL,
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
    CONSTRAINT rooms_team_fkey FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id),
    CONSTRAINT rooms_kind_check CHECK (kind IN ('board')),
    CONSTRAINT rooms_link_access_check CHECK (link_access IN ('off', 'guests', 'accounts')),
    CONSTRAINT rooms_title_length CHECK (char_length(title) BETWEEN 1 AND 200)
) WITH (fillfactor = 70);
CREATE INDEX idx_rooms_team_created ON rooms (team_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_rooms_deleted_at ON rooms (deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE room_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms (id) ON DELETE CASCADE,
    user_id UUID REFERENCES users (id) ON DELETE SET NULL,
    display_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'participant',
    access_epoch INT NOT NULL DEFAULT 0,
    removed_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT room_participants_room_id_id_key UNIQUE (room_id, id),
    CONSTRAINT room_participants_room_user_key UNIQUE (room_id, user_id),
    CONSTRAINT room_participants_role_check CHECK (role IN ('facilitator', 'participant')),
    CONSTRAINT room_participants_display_name_length CHECK (char_length(display_name) <= 40)
);
CREATE INDEX idx_room_participants_user ON room_participants (user_id) WHERE user_id IS NOT NULL;

CREATE TABLE room_ops (
    room_id UUID NOT NULL REFERENCES rooms (id) ON DELETE CASCADE,
    op_id UUID NOT NULL,
    version BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, op_id)
);
CREATE INDEX idx_room_ops_created_at ON room_ops (created_at);

CREATE TABLE boards (
    id UUID PRIMARY KEY REFERENCES rooms (id) ON DELETE CASCADE,
    context TEXT NOT NULL DEFAULT '',
    template_key TEXT NOT NULL DEFAULT '',
    anonymity TEXT NOT NULL DEFAULT 'named',
    cards_hidden BOOLEAN NOT NULL DEFAULT TRUE,
    cards_locked BOOLEAN NOT NULL DEFAULT FALSE,
    moves_locked BOOLEAN NOT NULL DEFAULT FALSE,
    voting_open BOOLEAN NOT NULL DEFAULT TRUE,
    vote_counts_hidden BOOLEAN NOT NULL DEFAULT FALSE,
    max_votes SMALLINT NOT NULL DEFAULT 6,
    max_votes_per_card SMALLINT NOT NULL DEFAULT 0,
    comments_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    merging_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    guest_names_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    show_prime_directive BOOLEAN NOT NULL DEFAULT TRUE,
    sort_mode TEXT NOT NULL DEFAULT 'manual',
    timer_duration_ms INT,
    timer_ends_at TIMESTAMPTZ,
    timer_paused_remaining_ms INT,
    timer_play_sound BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT boards_anonymity_check CHECK (anonymity IN ('named', 'anonymous')),
    CONSTRAINT boards_max_votes_check CHECK (max_votes BETWEEN 0 AND 99),
    CONSTRAINT boards_max_votes_per_card_check CHECK (max_votes_per_card BETWEEN 0 AND 99),
    CONSTRAINT boards_sort_mode_check CHECK (sort_mode IN ('manual', 'votes')),
    CONSTRAINT boards_timer_state CHECK (timer_ends_at IS NULL OR timer_paused_remaining_ms IS NULL),
    CONSTRAINT boards_timer_duration CHECK (timer_duration_ms IS NULL OR timer_duration_ms BETWEEN 1000 AND 3600000)
);

CREATE TABLE board_columns (
    id UUID PRIMARY KEY,
    board_id UUID NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    colour TEXT NOT NULL DEFAULT 'grey',
    position TEXT COLLATE "C" NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT board_columns_board_id_id_key UNIQUE (board_id, id),
    CONSTRAINT board_columns_colour_check CHECK (colour IN ('green', 'red', 'blue', 'amber', 'purple', 'teal', 'pink', 'grey')),
    CONSTRAINT board_columns_title_length CHECK (char_length(title) BETWEEN 1 AND 80)
);
CREATE INDEX idx_board_columns_order ON board_columns (board_id, position);

CREATE TABLE cards (
    id UUID PRIMARY KEY,
    board_id UUID NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    column_id UUID NOT NULL,
    parent_card_id UUID,
    author_participant_id UUID,
    body TEXT NOT NULL,
    colour TEXT,
    position TEXT COLLATE "C" NOT NULL,
    vote_total INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT cards_board_id_id_key UNIQUE (board_id, id),
    CONSTRAINT cards_column_fkey FOREIGN KEY (board_id, column_id) REFERENCES board_columns (board_id, id) ON DELETE CASCADE,
    CONSTRAINT cards_parent_fkey FOREIGN KEY (board_id, parent_card_id) REFERENCES cards (board_id, id) ON DELETE SET NULL (parent_card_id),
    CONSTRAINT cards_author_fkey FOREIGN KEY (board_id, author_participant_id) REFERENCES room_participants (room_id, id) ON DELETE SET NULL (author_participant_id),
    CONSTRAINT cards_not_own_parent CHECK (parent_card_id IS NULL OR parent_card_id <> id),
    CONSTRAINT cards_body_length CHECK (char_length(body) BETWEEN 1 AND 2000),
    CONSTRAINT cards_colour_check CHECK (colour IN ('green', 'red', 'blue', 'amber', 'purple', 'teal', 'pink', 'grey')),
    CONSTRAINT cards_vote_total_check CHECK (vote_total >= 0)
);
CREATE INDEX idx_cards_column_order ON cards (board_id, column_id, position);
CREATE INDEX idx_cards_parent ON cards (board_id, parent_card_id) WHERE parent_card_id IS NOT NULL;

-- +goose Down

DROP TABLE cards;
DROP TABLE board_columns;
DROP TABLE boards;
DROP TABLE room_ops;
DROP TABLE room_participants;
DROP TABLE rooms;
