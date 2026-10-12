-- name: CreateBoardColumn :one
INSERT INTO board_columns (id, board_id, title, description, colour, position)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetBoardColumn :one
SELECT * FROM board_columns WHERE board_id = $1 AND id = $2;

-- name: ListBoardColumns :many
SELECT * FROM board_columns WHERE board_id = $1 ORDER BY position, id;

-- name: UpdateBoardColumn :one
UPDATE board_columns SET title = sqlc.arg(title), description = sqlc.arg(description), colour = sqlc.arg(colour)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: MoveBoardColumn :one
UPDATE board_columns SET position = sqlc.arg(position)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteBoardColumn :one
DELETE FROM board_columns WHERE board_id = $1 AND id = $2 RETURNING *;

-- name: SetBoardColumnMaxVotes :one
UPDATE board_columns SET max_votes = sqlc.narg(max_votes)
WHERE board_id = sqlc.arg(board_id) AND id = sqlc.arg(id)
RETURNING *;
