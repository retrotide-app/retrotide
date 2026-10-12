-- name: CreateBoard :one
INSERT INTO boards (org_id, team_id, title, share_slug, link_access, created_by, context, template_key, anonymity, cards_hidden)
VALUES (
    sqlc.arg(org_id), sqlc.narg(team_id), sqlc.arg(title), sqlc.arg(share_slug), sqlc.arg(link_access), sqlc.narg(created_by),
    sqlc.arg(context), sqlc.arg(template_key), sqlc.arg(anonymity), sqlc.arg(cards_hidden)
)
RETURNING *;

-- name: GetBoardHeader :one
-- @sqlc-vet-disable org-scoped
SELECT sqlc.embed(boards), organisations.deleted_at AS org_deleted_at, organisations.allow_guests AS org_allow_guests
FROM boards
JOIN organisations ON organisations.id = boards.org_id
WHERE boards.id = $1;

-- name: GetBoardHeaderBySlug :one
-- @sqlc-vet-disable org-scoped
SELECT sqlc.embed(boards), organisations.deleted_at AS org_deleted_at, organisations.allow_guests AS org_allow_guests
FROM boards
JOIN organisations ON organisations.id = boards.org_id
WHERE boards.share_slug = $1;

-- name: GetBoard :one
SELECT sqlc.embed(boards), organisations.deleted_at AS org_deleted_at, organisations.allow_guests AS org_allow_guests, creators.display_name AS creator_name
FROM boards
JOIN organisations ON organisations.id = boards.org_id
LEFT JOIN users AS creators ON creators.id = boards.created_by
WHERE boards.org_id = $1 AND boards.id = $2;

-- name: ListBoardsForTeam :many
SELECT sqlc.embed(boards), creators.display_name AS creator_name
FROM boards
LEFT JOIN users AS creators ON creators.id = boards.created_by
WHERE boards.org_id = sqlc.arg(org_id) AND boards.team_id = sqlc.arg(team_id)::uuid AND boards.deleted_at IS NULL
ORDER BY boards.created_at DESC, boards.id;

-- name: ListBoardsForOrganisation :many
SELECT sqlc.embed(boards), creators.display_name AS creator_name
FROM boards
LEFT JOIN users AS creators ON creators.id = boards.created_by
WHERE boards.org_id = $1 AND boards.deleted_at IS NULL
ORDER BY boards.created_at DESC, boards.id;

-- name: SetBoardContext :one
UPDATE boards SET context = sqlc.arg(context)
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SetBoardSettings :one
UPDATE boards SET
    voting_open = COALESCE(sqlc.narg(voting_open)::boolean, boards.voting_open),
    vote_counts_hidden = COALESCE(sqlc.narg(vote_counts_hidden)::boolean, boards.vote_counts_hidden),
    max_votes = COALESCE(sqlc.narg(max_votes)::smallint, boards.max_votes),
    max_votes_per_card = COALESCE(sqlc.narg(max_votes_per_card)::smallint, boards.max_votes_per_card),
    comments_enabled = COALESCE(sqlc.narg(comments_enabled)::boolean, boards.comments_enabled),
    merging_enabled = COALESCE(sqlc.narg(merging_enabled)::boolean, boards.merging_enabled),
    sort_mode = COALESCE(sqlc.narg(sort_mode)::text, boards.sort_mode),
    cards_locked = COALESCE(sqlc.narg(cards_locked)::boolean, boards.cards_locked),
    moves_locked = COALESCE(sqlc.narg(moves_locked)::boolean, boards.moves_locked),
    timer_play_sound = COALESCE(sqlc.narg(timer_play_sound)::boolean, boards.timer_play_sound),
    timer_auto_lock = COALESCE(sqlc.narg(timer_auto_lock)::boolean, boards.timer_auto_lock),
    sort_enforced = COALESCE(sqlc.narg(sort_enforced)::boolean, boards.sort_enforced),
    anyone_can_edit = COALESCE(sqlc.narg(anyone_can_edit)::boolean, boards.anyone_can_edit),
    show_card_dates = COALESCE(sqlc.narg(show_card_dates)::boolean, boards.show_card_dates),
    locked_at = CASE
        WHEN sqlc.narg(board_locked)::boolean IS NULL THEN boards.locked_at
        WHEN sqlc.narg(board_locked)::boolean THEN COALESCE(boards.locked_at, NOW())
        ELSE NULL
    END
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SetBoardCardsHidden :one
UPDATE boards SET
    cards_hidden = sqlc.arg(cards_hidden),
    revealed_column_ids = '[]'::jsonb,
    presenting_card_id = CASE WHEN sqlc.arg(cards_hidden) THEN NULL ELSE boards.presenting_card_id END,
    highlighted_card_id = CASE WHEN sqlc.arg(cards_hidden) THEN NULL ELSE boards.highlighted_card_id END
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SetBoardStage :one
UPDATE boards SET
    stage = sqlc.arg(stage)::text,
    cards_hidden = boards.cards_hidden AND sqlc.arg(stage)::text = 'write',
    revealed_column_ids = CASE WHEN sqlc.arg(stage)::text = 'write' THEN boards.revealed_column_ids ELSE '[]'::jsonb END
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL AND boards.closed_at IS NULL
    AND boards.stage = sqlc.arg(from_stage)::text
RETURNING *;

-- name: CloseBoard :one
UPDATE boards SET
    closed_at = NOW(),
    stage = 'end',
    cards_hidden = FALSE,
    timer_duration_ms = NULL,
    timer_ends_at = NULL,
    timer_paused_remaining_ms = NULL,
    revealed_column_ids = '[]'::jsonb,
    hidden_column_ids = '[]'::jsonb,
    presenting_card_id = NULL,
    highlighted_card_id = NULL,
    focused_column_id = NULL
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL AND boards.closed_at IS NULL
RETURNING *;

-- name: SetBoardFacilitationState :one
UPDATE boards SET
    revealed_column_ids = sqlc.arg(revealed_column_ids)::jsonb,
    hidden_column_ids = sqlc.arg(hidden_column_ids)::jsonb,
    presenting_card_id = sqlc.narg(presenting_card_id),
    highlighted_card_id = sqlc.narg(highlighted_card_id),
    focused_column_id = sqlc.narg(focused_column_id)
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL AND boards.closed_at IS NULL
RETURNING *;

-- name: LockBoardAtTimerEnd :one
UPDATE boards SET locked_at = sqlc.arg(ends_at)::timestamptz
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL AND boards.closed_at IS NULL
    AND boards.archived_at IS NULL AND boards.locked_at IS NULL AND boards.timer_auto_lock
    AND boards.timer_ends_at = sqlc.arg(ends_at)::timestamptz
RETURNING *;

-- name: ListAutoLockTimers :many
-- @sqlc-vet-disable org-scoped
SELECT boards.id, boards.org_id, boards.timer_ends_at::timestamptz AS ends_at
FROM boards
JOIN organisations ON organisations.id = boards.org_id
WHERE boards.timer_auto_lock AND boards.timer_ends_at IS NOT NULL AND boards.locked_at IS NULL AND boards.closed_at IS NULL
    AND boards.archived_at IS NULL AND boards.deleted_at IS NULL AND organisations.deleted_at IS NULL
ORDER BY boards.timer_ends_at, boards.id;

-- name: SetBoardTimer :one
UPDATE boards SET
    timer_duration_ms = sqlc.narg(timer_duration_ms),
    timer_ends_at = sqlc.narg(timer_ends_at),
    timer_paused_remaining_ms = sqlc.narg(timer_paused_remaining_ms)
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: ListIdleBoards :many
-- @sqlc-vet-disable org-scoped
SELECT boards.id, boards.org_id FROM boards
JOIN organisations ON organisations.id = boards.org_id
WHERE boards.closed_at IS NULL AND boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND boards.updated_at < sqlc.arg(idle_since)
ORDER BY boards.id
LIMIT sqlc.arg(row_limit);

-- name: BumpBoardVersion :one
WITH target AS (
    SELECT boards.id FROM boards
    WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
), bumped AS (
    UPDATE boards SET version = boards.version + 1, updated_at = NOW()
    FROM target
    WHERE boards.id = target.id AND boards.version = sqlc.arg(expected_version)::bigint
    RETURNING boards.version
)
SELECT bumped.version AS bumped_version FROM target LEFT JOIN bumped ON TRUE;

-- name: RecordBoardOp :exec
INSERT INTO board_ops (board_id, op_id, version) VALUES ($1, $2, $3);

-- name: GetBoardOp :one
SELECT * FROM board_ops WHERE board_id = $1 AND op_id = $2;

-- name: SetBoardLinkAccess :one
UPDATE boards SET link_access = sqlc.arg(link_access)
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SetBoardPassword :one
UPDATE boards SET password_hash = sqlc.narg(password_hash), access_epoch = boards.access_epoch + 1
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: RotateBoardLink :one
UPDATE boards SET share_slug = sqlc.arg(share_slug), access_epoch = boards.access_epoch + 1
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SetBoardBackground :one
UPDATE boards SET background_media_id = sqlc.narg(background_media_id)
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SetBoardArchived :one
UPDATE boards SET archived_at = CASE WHEN sqlc.arg(archived)::boolean THEN COALESCE(boards.archived_at, NOW()) ELSE NULL END
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteBoard :one
UPDATE boards SET deleted_at = NOW()
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: RestoreBoard :one
UPDATE boards SET deleted_at = NULL
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NOT NULL
    AND boards.deleted_at >= sqlc.arg(deleted_since)::timestamptz
RETURNING *;

-- name: PurgeDeletedBoards :execrows
-- @sqlc-vet-disable org-scoped
DELETE FROM boards WHERE boards.deleted_at IS NOT NULL AND boards.deleted_at < sqlc.arg(cutoff)::timestamptz;

-- name: ListDeletedBoardsForOrganisation :many
SELECT sqlc.embed(boards), creators.display_name AS creator_name
FROM boards
LEFT JOIN users AS creators ON creators.id = boards.created_by
WHERE boards.org_id = sqlc.arg(org_id) AND boards.deleted_at IS NOT NULL AND boards.deleted_at >= sqlc.arg(deleted_since)::timestamptz
ORDER BY boards.deleted_at DESC, boards.id;

-- name: SetBoardTeam :one
UPDATE boards SET team_id = sqlc.narg(team_id)
WHERE boards.org_id = sqlc.arg(org_id) AND boards.id = sqlc.arg(id) AND boards.deleted_at IS NULL
RETURNING *;

-- name: PinBoard :exec
INSERT INTO board_pins (org_id, board_id, user_id)
VALUES (sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(user_id))
ON CONFLICT (user_id, board_id) DO NOTHING;

-- name: UnpinBoard :exec
DELETE FROM board_pins
WHERE board_pins.org_id = sqlc.arg(org_id) AND board_pins.board_id = sqlc.arg(board_id) AND board_pins.user_id = sqlc.arg(user_id);

-- name: ListPinnedBoards :many
SELECT board_pins.board_id
FROM board_pins
WHERE board_pins.org_id = sqlc.arg(org_id) AND board_pins.user_id = sqlc.arg(user_id)
ORDER BY board_pins.created_at, board_pins.board_id;

-- name: AddBoardParticipant :one
INSERT INTO board_participants (board_id, user_id, guest_device_id, display_name, role, access_epoch, joined_via)
VALUES (
    sqlc.arg(board_id), sqlc.narg(user_id), sqlc.narg(guest_device_id), sqlc.arg(display_name), sqlc.arg(role),
    sqlc.arg(access_epoch), sqlc.arg(joined_via)
)
RETURNING *;

-- name: UpsertLinkParticipantForUser :one
INSERT INTO board_participants (board_id, user_id, display_name, role, access_epoch, joined_via)
VALUES (sqlc.arg(board_id), sqlc.arg(user_id)::uuid, '', 'participant', sqlc.arg(access_epoch), 'link')
ON CONFLICT ON CONSTRAINT board_participants_board_user_key
DO UPDATE SET access_epoch = EXCLUDED.access_epoch, joined_via = 'link'
RETURNING *;

-- name: UpsertLinkParticipantForGuest :one
INSERT INTO board_participants (board_id, guest_device_id, display_name, role, access_epoch, joined_via)
VALUES (sqlc.arg(board_id), sqlc.arg(guest_device_id)::uuid, sqlc.arg(display_name), 'participant', sqlc.arg(access_epoch), 'link')
ON CONFLICT ON CONSTRAINT board_participants_board_guest_key
DO UPDATE SET access_epoch = EXCLUDED.access_epoch, display_name = EXCLUDED.display_name
RETURNING *;

-- name: GetBoardParticipantForUser :one
SELECT * FROM board_participants WHERE board_id = $1 AND user_id = $2;

-- name: GetBoardParticipantForGuest :one
SELECT * FROM board_participants WHERE board_id = $1 AND guest_device_id = $2;

-- name: GetBoardParticipant :one
SELECT * FROM board_participants WHERE board_id = $1 AND id = $2;

-- name: SetBoardParticipantRole :one
UPDATE board_participants SET role = sqlc.arg(role)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id) AND removed_at IS NULL
RETURNING *;

-- name: RemoveBoardParticipant :one
UPDATE board_participants SET removed_at = NOW()
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id) AND removed_at IS NULL
RETURNING *;

-- name: CountLiveBoardParticipants :one
SELECT COUNT(*) FROM board_participants
WHERE board_id = sqlc.arg(board_id)
    AND removed_at IS NULL
    AND (joined_via <> 'link' OR access_epoch = sqlc.arg(access_epoch))
    AND (user_id IS NOT NULL OR guest_device_id IS NOT NULL);

-- name: ReassignBoardParticipantToUser :one
UPDATE board_participants SET user_id = sqlc.arg(user_id)::uuid, guest_device_id = NULL, display_name = ''
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteBoardParticipant :execrows
DELETE FROM board_participants WHERE board_id = $1 AND id = $2;

-- name: ListBoardsForGuestDevice :many
-- @sqlc-vet-disable org-scoped board-child-scoped
SELECT sqlc.embed(boards), sqlc.embed(board_participants)
FROM board_participants
JOIN boards ON boards.id = board_participants.board_id
JOIN organisations ON organisations.id = boards.org_id
WHERE board_participants.guest_device_id = $1
    AND board_participants.removed_at IS NULL
    AND boards.deleted_at IS NULL
    AND organisations.deleted_at IS NULL
ORDER BY boards.id;

-- name: ListBoardParticipants :many
SELECT sqlc.embed(board_participants), COALESCE(users.display_name, '')::text AS account_name
FROM board_participants
LEFT JOIN users ON users.id = board_participants.user_id
WHERE board_participants.board_id = $1
ORDER BY board_participants.joined_at, board_participants.id;
