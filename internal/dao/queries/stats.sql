-- name: CountSiteTotals :one
-- @sqlc-vet-disable org-scoped board-child-scoped
SELECT
    (SELECT COUNT(*) FROM users)::bigint AS users,
    (SELECT COUNT(*) FROM users WHERE users.created_at >= sqlc.arg(since)::timestamptz)::bigint AS users_added,
    (SELECT COUNT(*) FROM users WHERE users.email_verified)::bigint AS verified_users,
    (SELECT COUNT(*) FROM users WHERE users.email_verified AND users.created_at >= sqlc.arg(since)::timestamptz)::bigint AS verified_users_added,
    (SELECT COUNT(*) FROM organisations WHERE organisations.deleted_at IS NULL)::bigint AS organisations,
    (SELECT COUNT(*) FROM organisations WHERE organisations.deleted_at IS NULL AND organisations.created_at >= sqlc.arg(since)::timestamptz)::bigint AS organisations_added,
    (SELECT COUNT(*) FROM teams JOIN organisations ON organisations.id = teams.org_id
        WHERE organisations.deleted_at IS NULL)::bigint AS teams,
    (SELECT COUNT(*) FROM teams JOIN organisations ON organisations.id = teams.org_id
        WHERE organisations.deleted_at IS NULL AND teams.created_at >= sqlc.arg(since)::timestamptz)::bigint AS teams_added,
    (SELECT COUNT(*) FROM boards JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL)::bigint AS boards,
    (SELECT COUNT(*) FROM boards JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND boards.created_at >= sqlc.arg(since)::timestamptz)::bigint AS boards_added,
    (SELECT COUNT(*) FROM cards JOIN boards ON boards.id = cards.board_id JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL)::bigint AS cards,
    (SELECT COUNT(*) FROM cards JOIN boards ON boards.id = cards.board_id JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND cards.created_at >= sqlc.arg(since)::timestamptz)::bigint AS cards_added;

-- name: CountSiteActivity :one
-- @sqlc-vet-disable org-scoped board-child-scoped
SELECT
    (SELECT COUNT(*) FROM users WHERE users.created_at >= sqlc.arg(day_since)::timestamptz)::bigint AS users_day,
    (SELECT COUNT(*) FROM users WHERE users.created_at >= sqlc.arg(week_since)::timestamptz)::bigint AS users_week,
    (SELECT COUNT(*) FROM users WHERE users.created_at >= sqlc.arg(month_since)::timestamptz)::bigint AS users_month,
    (SELECT COUNT(*) FROM organisations WHERE organisations.deleted_at IS NULL AND organisations.created_at >= sqlc.arg(day_since)::timestamptz)::bigint AS organisations_day,
    (SELECT COUNT(*) FROM organisations WHERE organisations.deleted_at IS NULL AND organisations.created_at >= sqlc.arg(week_since)::timestamptz)::bigint AS organisations_week,
    (SELECT COUNT(*) FROM organisations WHERE organisations.deleted_at IS NULL AND organisations.created_at >= sqlc.arg(month_since)::timestamptz)::bigint AS organisations_month,
    (SELECT COUNT(*) FROM boards JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND boards.created_at >= sqlc.arg(day_since)::timestamptz)::bigint AS boards_day,
    (SELECT COUNT(*) FROM boards JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND boards.created_at >= sqlc.arg(week_since)::timestamptz)::bigint AS boards_week,
    (SELECT COUNT(*) FROM boards JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND boards.created_at >= sqlc.arg(month_since)::timestamptz)::bigint AS boards_month,
    (SELECT COUNT(*) FROM cards JOIN boards ON boards.id = cards.board_id JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND cards.created_at >= sqlc.arg(day_since)::timestamptz)::bigint AS cards_day,
    (SELECT COUNT(*) FROM cards JOIN boards ON boards.id = cards.board_id JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND cards.created_at >= sqlc.arg(week_since)::timestamptz)::bigint AS cards_week,
    (SELECT COUNT(*) FROM cards JOIN boards ON boards.id = cards.board_id JOIN organisations ON organisations.id = boards.org_id
        WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND cards.created_at >= sqlc.arg(month_since)::timestamptz)::bigint AS cards_month;

-- name: ListDailySiteActivity :many
-- @sqlc-vet-disable org-scoped board-child-scoped
WITH days AS (
    SELECT ((sqlc.arg(first_day)::timestamptz AT TIME ZONE 'UTC')::date + offsets.n)::date AS day
    FROM generate_series(0, sqlc.arg(day_count)::int - 1) AS offsets(n)
),
new_users AS (
    SELECT (users.created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS total
    FROM users
    WHERE users.created_at >= sqlc.arg(first_day)::timestamptz
    GROUP BY 1
),
new_boards AS (
    SELECT (boards.created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS total
    FROM boards
    JOIN organisations ON organisations.id = boards.org_id
    WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND boards.created_at >= sqlc.arg(first_day)::timestamptz
    GROUP BY 1
),
new_cards AS (
    SELECT (cards.created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS total
    FROM cards
    JOIN boards ON boards.id = cards.board_id
    JOIN organisations ON organisations.id = boards.org_id
    WHERE boards.deleted_at IS NULL AND organisations.deleted_at IS NULL AND cards.created_at >= sqlc.arg(first_day)::timestamptz
    GROUP BY 1
)
SELECT days.day::date AS day,
       COALESCE(new_users.total, 0)::bigint AS users,
       COALESCE(new_boards.total, 0)::bigint AS boards,
       COALESCE(new_cards.total, 0)::bigint AS cards
FROM days
LEFT JOIN new_users ON new_users.day = days.day
LEFT JOIN new_boards ON new_boards.day = days.day
LEFT JOIN new_cards ON new_cards.day = days.day
ORDER BY days.day;
