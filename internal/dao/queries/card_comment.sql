-- name: CreateCardComment :one
INSERT INTO card_comments (id, board_id, card_id, author_participant_id, body)
VALUES (sqlc.arg(id), sqlc.arg(board_id), sqlc.arg(card_id), sqlc.narg(author_participant_id), sqlc.arg(body))
RETURNING *;

-- name: GetCardComment :one
SELECT sqlc.embed(card_comments),
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM card_comment_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.board_id = card_comments.board_id AND mentions.comment_id = card_comments.id
       ), '[]')::text AS mentions
FROM card_comments
WHERE card_comments.board_id = sqlc.arg(board_id) AND card_comments.id = sqlc.arg(id);

-- name: ListCardComments :many
SELECT sqlc.embed(card_comments),
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM card_comment_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.board_id = card_comments.board_id AND mentions.comment_id = card_comments.id
       ), '[]')::text AS mentions
FROM card_comments
WHERE card_comments.board_id = sqlc.arg(board_id)
ORDER BY card_comments.created_at, card_comments.id;

-- name: EditCardComment :one
UPDATE card_comments SET body = sqlc.arg(body), updated_at = NOW()
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteCardComment :one
DELETE FROM card_comments WHERE board_id = $1 AND id = $2 RETURNING *;

-- name: SeverCardCommentAuthors :execrows
UPDATE card_comments SET author_participant_id = NULL
WHERE board_id = $1 AND author_participant_id IS NOT NULL;

-- name: ReassignCardCommentAuthor :execrows
UPDATE card_comments SET author_participant_id = sqlc.arg(to_participant_id)::uuid
WHERE board_id = sqlc.arg(board_id) AND author_participant_id = sqlc.arg(from_participant_id)::uuid;

-- name: AddCardCommentMention :exec
INSERT INTO card_comment_mentions (org_id, board_id, comment_id, user_id)
VALUES (sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(comment_id), sqlc.arg(user_id))
ON CONFLICT (board_id, comment_id, user_id) DO NOTHING;

-- name: RemoveCardCommentMention :exec
DELETE FROM card_comment_mentions
WHERE card_comment_mentions.board_id = sqlc.arg(board_id) AND card_comment_mentions.comment_id = sqlc.arg(comment_id)
    AND card_comment_mentions.user_id = sqlc.arg(user_id);

-- name: ListCardCommentMentionsForUser :many
-- @sqlc-vet-disable board-child-scoped org-scoped
SELECT comments.id AS comment_id, comments.card_id, comments.board_id, comments.body, mentions.org_id, mentions.created_at AS mentioned_at,
       organisations.slug AS org_slug, organisations.name AS org_name,
       boards.title AS board_title, boards.team_id AS board_team_id, boards.cards_hidden,
       EXISTS (
           SELECT 1 FROM board_participants AS writers
           WHERE writers.board_id = cards.board_id AND writers.id = cards.author_participant_id AND writers.user_id = mentions.user_id
       ) AS own_card
FROM card_comment_mentions AS mentions
JOIN card_comments AS comments ON comments.board_id = mentions.board_id AND comments.id = mentions.comment_id
JOIN cards ON cards.board_id = comments.board_id AND cards.id = comments.card_id
JOIN boards ON boards.org_id = mentions.org_id AND boards.id = mentions.board_id
JOIN organisations ON organisations.id = mentions.org_id
WHERE mentions.user_id = sqlc.arg(user_id) AND organisations.deleted_at IS NULL AND boards.deleted_at IS NULL
ORDER BY mentions.created_at DESC, comments.id
LIMIT sqlc.arg(row_limit);

-- name: ListCardCommentsWrittenByUser :many
-- @sqlc-vet-disable board-child-scoped org-scoped
SELECT comments.body, comments.created_at, comments.updated_at,
       organisations.name AS org_name, boards.title AS board_title
FROM card_comments AS comments
JOIN board_participants AS writers ON writers.board_id = comments.board_id AND writers.id = comments.author_participant_id
JOIN boards ON boards.id = comments.board_id
JOIN organisations ON organisations.id = boards.org_id
WHERE writers.user_id = sqlc.arg(user_id)::uuid
ORDER BY comments.created_at, comments.id;
