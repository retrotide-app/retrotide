-- name: CreateCard :one
INSERT INTO cards (id, board_id, column_id, author_participant_id, body, colour, position)
VALUES (sqlc.arg(id), sqlc.arg(board_id), sqlc.arg(column_id), sqlc.narg(author_participant_id), sqlc.arg(body), sqlc.narg(colour), sqlc.arg(position))
RETURNING *;

-- name: GetCard :one
SELECT sqlc.embed(cards),
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM card_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.board_id = cards.board_id AND mentions.card_id = cards.id
       ), '[]')::text AS mentions
FROM cards
WHERE cards.board_id = sqlc.arg(board_id) AND cards.id = sqlc.arg(id);

-- name: ListCards :many
SELECT sqlc.embed(cards),
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM card_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.board_id = cards.board_id AND mentions.card_id = cards.id
       ), '[]')::text AS mentions
FROM cards
WHERE cards.board_id = sqlc.arg(board_id)
ORDER BY cards.column_id, cards.position, cards.id;

-- name: ListCardsInColumn :many
SELECT sqlc.embed(cards),
       COALESCE((
           SELECT json_agg(json_build_object('user_id', mentions.user_id, 'display_name', mentioned.display_name) ORDER BY mentioned.display_name, mentions.user_id)::text
           FROM card_mentions AS mentions
           JOIN users AS mentioned ON mentioned.id = mentions.user_id
           WHERE mentions.board_id = cards.board_id AND mentions.card_id = cards.id
       ), '[]')::text AS mentions
FROM cards
WHERE cards.board_id = sqlc.arg(board_id) AND cards.column_id = sqlc.arg(column_id)
ORDER BY cards.position, cards.id;

-- name: EditCard :one
UPDATE cards SET body = sqlc.arg(body), updated_at = NOW()
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: SetCardColour :one
UPDATE cards SET colour = sqlc.narg(colour)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: MoveCard :one
UPDATE cards SET column_id = sqlc.arg(column_id), position = sqlc.arg(position), parent_card_id = NULL
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: MergeCard :one
UPDATE cards SET parent_card_id = sqlc.arg(parent_card_id)::uuid, column_id = sqlc.arg(column_id), position = sqlc.arg(position)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: RestackCards :many
UPDATE cards SET parent_card_id = sqlc.arg(to_parent_card_id)::uuid, column_id = sqlc.arg(column_id)
WHERE board_id = sqlc.arg(board_id) AND parent_card_id = sqlc.arg(from_parent_card_id)::uuid
RETURNING id;

-- name: SetCardPinned :one
UPDATE cards SET pinned_at = CASE WHEN sqlc.arg(pinned)::boolean THEN COALESCE(cards.pinned_at, NOW()) ELSE NULL END
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: SetCardImage :one
UPDATE cards SET image_media_id = sqlc.narg(image_media_id)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: AddCardVoteTotal :one
UPDATE cards SET vote_total = cards.vote_total + sqlc.arg(delta)::int
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: ResetCardVoteTotals :execrows
UPDATE cards SET vote_total = 0 WHERE board_id = $1 AND vote_total <> 0;

-- name: ResetColumnVoteTotals :execrows
UPDATE cards SET vote_total = 0 WHERE board_id = sqlc.arg(board_id) AND column_id = sqlc.arg(column_id);

-- name: DeleteCard :one
DELETE FROM cards WHERE board_id = $1 AND id = $2 RETURNING *;

-- name: ReassignCardAuthor :execrows
UPDATE cards SET author_participant_id = sqlc.arg(to_participant_id)::uuid
WHERE board_id = sqlc.arg(board_id) AND author_participant_id = sqlc.arg(from_participant_id)::uuid;

-- name: SetCardPosition :execrows
UPDATE cards SET position = sqlc.arg(position)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id);

-- name: SeverCardAuthors :execrows
UPDATE cards SET author_participant_id = NULL
WHERE board_id = $1 AND author_participant_id IS NOT NULL;

-- name: ClearBoardColumnCards :execrows
DELETE FROM cards WHERE board_id = $1 AND column_id = $2;

-- name: AddCardMention :exec
INSERT INTO card_mentions (org_id, board_id, card_id, user_id)
VALUES (sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(card_id), sqlc.arg(user_id))
ON CONFLICT (board_id, card_id, user_id) DO NOTHING;

-- name: RemoveCardMention :exec
DELETE FROM card_mentions
WHERE card_mentions.board_id = sqlc.arg(board_id) AND card_mentions.card_id = sqlc.arg(card_id) AND card_mentions.user_id = sqlc.arg(user_id);

-- name: ListCardMentionsForUser :many
-- @sqlc-vet-disable board-child-scoped org-scoped
SELECT cards.id AS card_id, cards.board_id, cards.body, mentions.org_id, mentions.created_at AS mentioned_at,
       organisations.slug AS org_slug, organisations.name AS org_name,
       boards.title AS board_title, boards.team_id AS board_team_id, boards.cards_hidden,
       EXISTS (
           SELECT 1 FROM board_participants AS writers
           WHERE writers.board_id = cards.board_id AND writers.id = cards.author_participant_id AND writers.user_id = mentions.user_id
       ) AS own_card
FROM card_mentions AS mentions
JOIN cards ON cards.board_id = mentions.board_id AND cards.id = mentions.card_id
JOIN boards ON boards.org_id = mentions.org_id AND boards.id = mentions.board_id
JOIN organisations ON organisations.id = mentions.org_id
WHERE mentions.user_id = sqlc.arg(user_id) AND organisations.deleted_at IS NULL AND boards.deleted_at IS NULL
ORDER BY mentions.created_at DESC, cards.id
LIMIT sqlc.arg(row_limit);

-- name: ListCardsWrittenByUser :many
-- @sqlc-vet-disable board-child-scoped org-scoped
SELECT cards.body, cards.created_at, cards.updated_at,
       organisations.name AS org_name, boards.title AS board_title, board_columns.title AS column_title
FROM cards
JOIN board_participants AS writers ON writers.board_id = cards.board_id AND writers.id = cards.author_participant_id
JOIN boards ON boards.id = cards.board_id
JOIN board_columns ON board_columns.board_id = cards.board_id AND board_columns.id = cards.column_id
JOIN organisations ON organisations.id = boards.org_id
WHERE writers.user_id = sqlc.arg(user_id)::uuid
ORDER BY cards.created_at, cards.id;
