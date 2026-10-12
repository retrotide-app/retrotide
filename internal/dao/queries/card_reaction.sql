-- name: AddCardReaction :execrows
INSERT INTO card_reactions (org_id, board_id, card_id, participant_id, reaction)
VALUES (sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(card_id), sqlc.arg(participant_id)::uuid, sqlc.arg(reaction))
ON CONFLICT (card_id, participant_id, reaction) DO NOTHING;

-- name: RemoveCardReaction :execrows
DELETE FROM card_reactions
WHERE card_reactions.org_id = sqlc.arg(org_id) AND card_reactions.board_id = sqlc.arg(board_id) AND card_reactions.card_id = sqlc.arg(card_id)
    AND card_reactions.participant_id = sqlc.arg(participant_id)::uuid AND card_reactions.reaction = sqlc.arg(reaction);

-- name: ListCardReactionsForCard :many
SELECT card_reactions.board_id, card_reactions.card_id, card_reactions.participant_id, card_reactions.reaction
FROM card_reactions
WHERE card_reactions.board_id = sqlc.arg(board_id) AND card_reactions.card_id = sqlc.arg(card_id)
ORDER BY card_reactions.reaction, card_reactions.id;

-- name: ListCardReactions :many
SELECT card_reactions.board_id, card_reactions.card_id, card_reactions.participant_id, card_reactions.reaction
FROM card_reactions
WHERE card_reactions.board_id = $1
ORDER BY card_reactions.card_id, card_reactions.reaction, card_reactions.id;

-- name: HandOverCardReactions :one
WITH moved AS (
    INSERT INTO card_reactions (org_id, board_id, card_id, participant_id, reaction)
    SELECT held.org_id, held.board_id, held.card_id, sqlc.arg(to_participant_id)::uuid, held.reaction
    FROM card_reactions AS held
    WHERE held.board_id = sqlc.arg(board_id) AND held.participant_id = sqlc.arg(from_participant_id)::uuid
    ON CONFLICT (card_id, participant_id, reaction) DO NOTHING
    RETURNING card_reactions.id
), dropped AS (
    DELETE FROM card_reactions
    WHERE card_reactions.board_id = sqlc.arg(board_id) AND card_reactions.participant_id = sqlc.arg(from_participant_id)::uuid
    RETURNING card_reactions.id
)
SELECT (SELECT COUNT(*) FROM moved)::bigint AS moved, (SELECT COUNT(*) FROM dropped)::bigint AS dropped;

-- name: SeverCardReactions :execrows
UPDATE card_reactions SET participant_id = NULL
WHERE card_reactions.board_id = $1 AND card_reactions.participant_id IS NOT NULL;
