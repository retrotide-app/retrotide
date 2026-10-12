-- name: CreateMedia :one
INSERT INTO media (org_id, board_id, kind, width, height, byte_size, data)
VALUES (sqlc.arg(org_id), sqlc.arg(board_id), sqlc.arg(kind), sqlc.arg(width), sqlc.arg(height), sqlc.arg(byte_size), sqlc.arg(data))
RETURNING media.id, media.org_id, media.board_id, media.kind, media.content_type, media.width, media.height, media.byte_size, media.created_at;

-- name: GetMedia :one
SELECT media.id, media.org_id, media.board_id, media.kind, media.content_type, media.width, media.height, media.byte_size, media.created_at
FROM media
WHERE media.board_id = sqlc.arg(board_id) AND media.id = sqlc.arg(id);

-- name: OpenMedia :one
SELECT media.id, media.org_id, media.board_id, media.kind, media.content_type, media.width, media.height, media.byte_size, media.created_at, media.data
FROM media
WHERE media.board_id = sqlc.arg(board_id) AND media.id = sqlc.arg(id);

-- name: PurgeUnattachedMedia :execrows
-- @sqlc-vet-disable board-child-scoped org-scoped
DELETE FROM media
WHERE media.created_at < sqlc.arg(cutoff)::timestamptz
    AND NOT EXISTS (SELECT 1 FROM cards WHERE cards.board_id = media.board_id AND cards.image_media_id = media.id)
    AND NOT EXISTS (SELECT 1 FROM boards WHERE boards.id = media.board_id AND boards.background_media_id = media.id);
