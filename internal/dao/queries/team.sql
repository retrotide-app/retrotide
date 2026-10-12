-- name: CreateTeamWithinQuota :one
INSERT INTO teams (org_id, name)
SELECT sqlc.arg(org_id)::uuid, sqlc.arg(name)::text
WHERE (
    SELECT COUNT(*) FROM teams WHERE org_id = sqlc.arg(org_id)::uuid AND archived_at IS NULL
) < sqlc.arg(max_teams)::int
RETURNING *;

-- name: GetTeamHeader :one
-- @sqlc-vet-disable org-scoped
SELECT * FROM teams WHERE id = $1;

-- name: GetTeam :one
SELECT * FROM teams WHERE org_id = $1 AND id = $2;

-- name: ListTeamsForOrganisation :many
SELECT * FROM teams WHERE org_id = $1 ORDER BY created_at, id;

-- name: ListTeamsWithCountsForOrganisation :many
SELECT sqlc.embed(teams),
       (SELECT COUNT(*) FROM team_members WHERE team_members.org_id = teams.org_id AND team_members.team_id = teams.id)::bigint AS member_count,
       (SELECT COUNT(*) FROM boards
           WHERE boards.org_id = teams.org_id AND boards.team_id = teams.id AND boards.deleted_at IS NULL)::bigint AS board_count
FROM teams
WHERE teams.org_id = $1
ORDER BY teams.created_at, teams.id;

-- name: CountActiveTeams :one
SELECT COUNT(*) FROM teams WHERE org_id = $1 AND archived_at IS NULL;

-- name: RenameTeam :one
UPDATE teams SET name = sqlc.arg(name)::text
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: ArchiveTeam :one
UPDATE teams SET archived_at = NOW()
WHERE org_id = $1 AND id = $2 AND archived_at IS NULL
RETURNING *;

-- name: DeleteTeam :one
DELETE FROM teams WHERE org_id = $1 AND id = $2 RETURNING *;

-- name: AddTeamMember :one
INSERT INTO team_members (org_id, team_id, user_id, role) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetTeamMember :one
SELECT * FROM team_members WHERE org_id = $1 AND team_id = $2 AND user_id = $3;

-- name: ListTeamMembers :many
SELECT sqlc.embed(team_members), users.email, users.display_name,
       organisation_members.role AS org_role,
       COALESCE(user_roles.role, '')::text AS site_role
FROM team_members
JOIN users ON users.id = team_members.user_id
JOIN organisation_members ON organisation_members.org_id = team_members.org_id AND organisation_members.user_id = team_members.user_id
LEFT JOIN user_roles ON user_roles.user_id = team_members.user_id
WHERE team_members.org_id = $1 AND team_members.team_id = $2
ORDER BY team_members.joined_at, team_members.user_id;

-- name: ListTeamsForMember :many
SELECT sqlc.embed(teams), team_members.role AS member_role, team_members.joined_at AS member_joined_at
FROM team_members
JOIN teams ON teams.org_id = team_members.org_id AND teams.id = team_members.team_id
WHERE team_members.org_id = $1 AND team_members.user_id = $2
ORDER BY teams.created_at, teams.id;

-- name: SetTeamMemberRole :one
UPDATE team_members SET role = $4
WHERE org_id = $1 AND team_id = $2 AND user_id = $3
RETURNING *;

-- name: RemoveTeamMember :one
DELETE FROM team_members WHERE org_id = $1 AND team_id = $2 AND user_id = $3 RETURNING *;
