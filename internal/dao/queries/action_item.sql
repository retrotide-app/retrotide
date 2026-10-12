-- name: CreateActionItem :exec
INSERT INTO action_items (id, org_id, team_id, board_id, source_card_id, title, assignee_user_id, due_on, created_by)
VALUES (
    sqlc.arg(id), sqlc.arg(org_id), sqlc.narg(team_id), sqlc.narg(board_id), sqlc.narg(source_card_id), sqlc.arg(title),
    sqlc.narg(assignee_user_id), sqlc.narg(due_on), sqlc.narg(created_by)
);

-- name: GetActionItem :one
SELECT sqlc.embed(action_items),
       COALESCE(assignees.display_name, '')::text AS assignee_name,
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM action_item_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.org_id = action_items.org_id AND mentions.action_item_id = action_items.id
       ), '[]')::text AS mentions
FROM action_items
LEFT JOIN users AS assignees ON assignees.id = action_items.assignee_user_id
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.id = sqlc.arg(id);

-- name: ListActionItemsForBoard :many
SELECT sqlc.embed(action_items),
       COALESCE(assignees.display_name, '')::text AS assignee_name,
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM action_item_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.org_id = action_items.org_id AND mentions.action_item_id = action_items.id
       ), '[]')::text AS mentions
FROM action_items
LEFT JOIN users AS assignees ON assignees.id = action_items.assignee_user_id
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.board_id = sqlc.arg(board_id)::uuid
ORDER BY action_items.created_at, action_items.id;

-- name: ListPreviousActionItems :many
SELECT sqlc.embed(action_items),
       COALESCE(assignees.display_name, '')::text AS assignee_name,
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM action_item_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.org_id = action_items.org_id AND mentions.action_item_id = action_items.id
       ), '[]')::text AS mentions,
       COALESCE(reviews.outcome, '')::text AS review_outcome
FROM action_items
LEFT JOIN users AS assignees ON assignees.id = action_items.assignee_user_id
LEFT JOIN boards AS sources ON sources.org_id = action_items.org_id AND sources.id = action_items.board_id
LEFT JOIN action_item_reviews AS reviews
    ON reviews.org_id = action_items.org_id AND reviews.action_item_id = action_items.id AND reviews.board_id = sqlc.arg(board_id)::uuid
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.team_id = sqlc.arg(team_id)::uuid
    AND (action_items.board_id IS NULL OR action_items.board_id <> sqlc.arg(board_id)::uuid)
    AND (sources.id IS NULL OR sources.created_at < sqlc.arg(before)::timestamptz)
    AND (action_items.status = 'open' OR reviews.board_id IS NOT NULL)
ORDER BY action_items.created_at, action_items.id;

-- name: ListActionItemsForOrganisation :many
SELECT sqlc.embed(action_items),
       COALESCE(assignees.display_name, '')::text AS assignee_name,
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM action_item_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.org_id = action_items.org_id AND mentions.action_item_id = action_items.id
       ), '[]')::text AS mentions,
       COALESCE(sources.title, '')::text AS board_title,
       COALESCE(sources.anonymity, '')::text AS board_anonymity,
       COALESCE(teams.name, '')::text AS team_name
FROM action_items
LEFT JOIN users AS assignees ON assignees.id = action_items.assignee_user_id
LEFT JOIN boards AS sources ON sources.org_id = action_items.org_id AND sources.id = action_items.board_id
LEFT JOIN teams ON teams.org_id = action_items.org_id AND teams.id = action_items.team_id
WHERE action_items.org_id = sqlc.arg(org_id)
    AND (sources.id IS NULL OR sources.deleted_at IS NULL)
    AND (
        action_items.team_id IS NULL
        OR sqlc.arg(every_team)::boolean
        OR EXISTS (
            SELECT 1 FROM team_members
            WHERE team_members.org_id = action_items.org_id AND team_members.team_id = action_items.team_id
                AND team_members.user_id = sqlc.arg(viewer_id)::uuid
        )
    )
    AND (sqlc.narg(status)::text IS NULL OR action_items.status = sqlc.narg(status)::text)
    AND (sqlc.narg(assignee_user_id)::uuid IS NULL OR action_items.assignee_user_id = sqlc.narg(assignee_user_id)::uuid)
    AND (NOT sqlc.arg(unassigned)::boolean OR action_items.assignee_user_id IS NULL)
    AND (sqlc.narg(team_id)::uuid IS NULL OR action_items.team_id = sqlc.narg(team_id)::uuid)
    AND (NOT sqlc.arg(organisation_boards)::boolean OR action_items.team_id IS NULL)
ORDER BY action_items.created_at DESC, action_items.id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountActionItemsForOrganisation :many
SELECT action_items.status, COUNT(*)::bigint AS item_count
FROM action_items
LEFT JOIN boards AS sources ON sources.org_id = action_items.org_id AND sources.id = action_items.board_id
WHERE action_items.org_id = sqlc.arg(org_id)
    AND (sources.id IS NULL OR sources.deleted_at IS NULL)
    AND (
        action_items.team_id IS NULL
        OR sqlc.arg(every_team)::boolean
        OR EXISTS (
            SELECT 1 FROM team_members
            WHERE team_members.org_id = action_items.org_id AND team_members.team_id = action_items.team_id
                AND team_members.user_id = sqlc.arg(viewer_id)::uuid
        )
    )
    AND (sqlc.narg(assignee_user_id)::uuid IS NULL OR action_items.assignee_user_id = sqlc.narg(assignee_user_id)::uuid)
    AND (NOT sqlc.arg(unassigned)::boolean OR action_items.assignee_user_id IS NULL)
    AND (sqlc.narg(team_id)::uuid IS NULL OR action_items.team_id = sqlc.narg(team_id)::uuid)
    AND (NOT sqlc.arg(organisation_boards)::boolean OR action_items.team_id IS NULL)
GROUP BY action_items.status;

-- name: EditActionItem :execrows
UPDATE action_items SET
    title = sqlc.arg(title),
    assignee_user_id = sqlc.narg(assignee_user_id),
    due_on = sqlc.narg(due_on),
    updated_at = NOW()
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.id = sqlc.arg(id) AND action_items.board_id = sqlc.arg(board_id)::uuid;

-- name: SetActionItemStatus :execrows
UPDATE action_items SET
    status = sqlc.arg(status)::text,
    completed_at = CASE WHEN sqlc.arg(status)::text = 'done' THEN COALESCE(action_items.completed_at, NOW()) ELSE NULL END,
    updated_at = NOW()
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.id = sqlc.arg(id);

-- name: SetActionItemTeamForBoard :execrows
UPDATE action_items SET team_id = sqlc.narg(team_id)
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.board_id = sqlc.arg(board_id)::uuid;

-- name: DeleteActionItem :execrows
DELETE FROM action_items
WHERE action_items.org_id = sqlc.arg(org_id) AND action_items.id = sqlc.arg(id) AND action_items.board_id = sqlc.arg(board_id)::uuid;

-- name: AddActionItemMention :exec
INSERT INTO action_item_mentions (org_id, action_item_id, user_id)
VALUES (sqlc.arg(org_id), sqlc.arg(action_item_id), sqlc.arg(user_id))
ON CONFLICT (action_item_id, user_id) DO NOTHING;

-- name: RemoveActionItemMention :exec
DELETE FROM action_item_mentions
WHERE action_item_mentions.org_id = sqlc.arg(org_id) AND action_item_mentions.action_item_id = sqlc.arg(action_item_id)
    AND action_item_mentions.user_id = sqlc.arg(user_id);

-- name: RecordActionItemReview :exec
INSERT INTO action_item_reviews (org_id, action_item_id, board_id, outcome)
VALUES (sqlc.arg(org_id), sqlc.arg(action_item_id), sqlc.arg(board_id), sqlc.arg(outcome))
ON CONFLICT (action_item_id, board_id) DO UPDATE SET outcome = EXCLUDED.outcome, reviewed_at = NOW();

-- name: ListMentionsForUser :many
-- @sqlc-vet-disable org-scoped
SELECT action_items.id, action_items.org_id, action_items.team_id, action_items.board_id, action_items.title, action_items.status,
       action_items.due_on, mentions.created_at AS mentioned_at, organisations.slug AS org_slug, organisations.name AS org_name,
       COALESCE(sources.title, '')::text AS board_title, sources.team_id AS board_team_id
FROM action_item_mentions AS mentions
JOIN action_items ON action_items.org_id = mentions.org_id AND action_items.id = mentions.action_item_id
JOIN organisations ON organisations.id = mentions.org_id
LEFT JOIN boards AS sources ON sources.org_id = action_items.org_id AND sources.id = action_items.board_id
WHERE mentions.user_id = sqlc.arg(user_id) AND organisations.deleted_at IS NULL
    AND (sources.id IS NULL OR sources.deleted_at IS NULL)
ORDER BY mentions.created_at DESC, action_items.id
LIMIT sqlc.arg(row_limit);

-- name: ListActionItemsForUser :many
-- @sqlc-vet-disable org-scoped
SELECT action_items.title, action_items.status, action_items.due_on, action_items.created_by, action_items.assignee_user_id,
       organisations.name AS org_name, COALESCE(teams.name, '')::text AS team_name, COALESCE(sources.title, '')::text AS board_title
FROM action_items
JOIN organisations ON organisations.id = action_items.org_id
LEFT JOIN teams ON teams.org_id = action_items.org_id AND teams.id = action_items.team_id
LEFT JOIN boards AS sources ON sources.org_id = action_items.org_id AND sources.id = action_items.board_id
WHERE action_items.created_by = sqlc.arg(user_id)::uuid OR action_items.assignee_user_id = sqlc.arg(user_id)::uuid
ORDER BY action_items.created_at, action_items.id;
