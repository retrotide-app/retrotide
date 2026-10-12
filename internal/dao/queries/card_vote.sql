-- name: SpendVoteBudget :one
INSERT INTO board_vote_budgets (board_id, participant_id, used)
SELECT sqlc.arg(board_id)::uuid, sqlc.arg(participant_id)::uuid, 1
WHERE sqlc.arg(max_votes)::int > 0
ON CONFLICT (board_id, participant_id) DO UPDATE SET used = board_vote_budgets.used + 1
WHERE board_vote_budgets.used < sqlc.arg(max_votes)::int
RETURNING used;

-- name: RefundVoteBudget :one
UPDATE board_vote_budgets SET used = board_vote_budgets.used - 1
WHERE board_vote_budgets.board_id = sqlc.arg(board_id) AND board_vote_budgets.participant_id = sqlc.arg(participant_id) AND board_vote_budgets.used > 0
RETURNING used;

-- name: IncrementCardVote :one
INSERT INTO card_votes (board_id, card_id, participant_id, count)
VALUES (sqlc.arg(board_id), sqlc.arg(card_id), sqlc.arg(participant_id), 1)
ON CONFLICT (card_id, participant_id) DO UPDATE SET count = card_votes.count + 1
WHERE card_votes.board_id = sqlc.arg(board_id) AND (sqlc.arg(max_per_card)::int = 0 OR card_votes.count < sqlc.arg(max_per_card)::int)
RETURNING count;

-- name: LowerCardVote :one
UPDATE card_votes SET count = card_votes.count - 1
WHERE card_votes.board_id = sqlc.arg(board_id) AND card_votes.card_id = sqlc.arg(card_id) AND card_votes.participant_id = sqlc.arg(participant_id) AND card_votes.count > 1
RETURNING count;

-- name: DropCardVote :execrows
DELETE FROM card_votes
WHERE card_votes.board_id = sqlc.arg(board_id) AND card_votes.card_id = sqlc.arg(card_id) AND card_votes.participant_id = sqlc.arg(participant_id) AND card_votes.count = 1;

-- name: ListCardVotes :many
SELECT card_votes.card_id, card_votes.participant_id, card_votes.count, cards.author_participant_id AS card_author_participant_id, cards.column_id AS card_column_id
FROM card_votes
JOIN cards ON cards.board_id = card_votes.board_id AND cards.id = card_votes.card_id
WHERE card_votes.board_id = $1
ORDER BY card_votes.card_id, card_votes.participant_id;

-- name: ListParticipantCardVotes :many
SELECT card_votes.card_id, card_votes.participant_id, card_votes.count, cards.author_participant_id AS card_author_participant_id, cards.column_id AS card_column_id
FROM card_votes
JOIN cards ON cards.board_id = card_votes.board_id AND cards.id = card_votes.card_id
WHERE card_votes.board_id = $1 AND card_votes.participant_id = $2
ORDER BY card_votes.card_id;

-- name: CountColumnVotes :one
SELECT COALESCE(SUM(card_votes.count), 0)::int AS used
FROM card_votes
JOIN cards ON cards.board_id = card_votes.board_id AND cards.id = card_votes.card_id
WHERE card_votes.board_id = sqlc.arg(board_id) AND card_votes.participant_id = sqlc.arg(participant_id) AND cards.column_id = sqlc.arg(column_id);

-- name: DeleteColumnCardVotes :execrows
DELETE FROM card_votes USING cards
WHERE card_votes.board_id = sqlc.arg(board_id) AND cards.board_id = card_votes.board_id AND cards.id = card_votes.card_id
    AND cards.column_id = sqlc.arg(column_id);

-- name: RefundCardVoteBudgets :execrows
UPDATE board_vote_budgets SET used = board_vote_budgets.used - spent.total
FROM (
    SELECT card_votes.participant_id, SUM(card_votes.count)::smallint AS total
    FROM card_votes
    WHERE card_votes.board_id = sqlc.arg(board_id) AND card_votes.card_id = sqlc.arg(card_id)
    GROUP BY card_votes.participant_id
) AS spent
WHERE board_vote_budgets.board_id = sqlc.arg(board_id) AND board_vote_budgets.participant_id = spent.participant_id;

-- name: RefundColumnVoteBudgets :execrows
UPDATE board_vote_budgets SET used = board_vote_budgets.used - spent.total
FROM (
    SELECT card_votes.participant_id, SUM(card_votes.count)::smallint AS total
    FROM card_votes
    JOIN cards ON cards.board_id = card_votes.board_id AND cards.id = card_votes.card_id
    WHERE card_votes.board_id = sqlc.arg(board_id) AND cards.column_id = sqlc.arg(column_id)
    GROUP BY card_votes.participant_id
) AS spent
WHERE board_vote_budgets.board_id = sqlc.arg(board_id) AND board_vote_budgets.participant_id = spent.participant_id;

-- name: DeleteBoardCardVotes :execrows
DELETE FROM card_votes WHERE card_votes.board_id = $1;

-- name: DeleteBoardVoteBudgets :execrows
DELETE FROM board_vote_budgets WHERE board_vote_budgets.board_id = $1;

-- name: MergeParticipantCardVotes :execrows
INSERT INTO card_votes (board_id, card_id, participant_id, count)
SELECT card_votes.board_id, card_votes.card_id, sqlc.arg(to_participant_id)::uuid, card_votes.count
FROM card_votes
WHERE card_votes.board_id = sqlc.arg(board_id) AND card_votes.participant_id = sqlc.arg(from_participant_id)::uuid
ON CONFLICT (card_id, participant_id) DO UPDATE SET count = card_votes.count + EXCLUDED.count;

-- name: MergeParticipantVoteBudget :execrows
INSERT INTO board_vote_budgets (board_id, participant_id, used)
SELECT board_vote_budgets.board_id, sqlc.arg(to_participant_id)::uuid, board_vote_budgets.used
FROM board_vote_budgets
WHERE board_vote_budgets.board_id = sqlc.arg(board_id) AND board_vote_budgets.participant_id = sqlc.arg(from_participant_id)::uuid
ON CONFLICT (board_id, participant_id) DO UPDATE SET used = board_vote_budgets.used + EXCLUDED.used;
