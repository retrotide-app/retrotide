-- name: CreateOrganisationWithinOwnerQuota :one
INSERT INTO organisations (slug, name, created_by)
SELECT sqlc.arg(slug)::text, sqlc.arg(name)::text, sqlc.arg(created_by)::uuid
WHERE (
    SELECT COUNT(*) FROM organisation_members
    JOIN organisations owned ON owned.id = organisation_members.org_id
    WHERE organisation_members.user_id = sqlc.arg(created_by)::uuid
      AND organisation_members.role = 'owner'
      AND owned.deleted_at IS NULL
) < sqlc.arg(max_owned)::int
RETURNING *;

-- name: GetOrganisation :one
SELECT * FROM organisations WHERE id = $1;

-- name: LockOrganisation :one
SELECT id FROM organisations WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: UpdateOrganisation :one
UPDATE organisations SET slug = $2, name = $3, allow_guests = $4, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteOrganisation :one
UPDATE organisations SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: RestoreOrganisationWithinOwnerQuota :one
-- @sqlc-vet-disable org-scoped
UPDATE organisations SET deleted_at = NULL, updated_at = NOW()
WHERE organisations.id = sqlc.arg(id)
  AND organisations.deleted_at > sqlc.arg(deleted_after)::timestamptz
  AND (
      SELECT COUNT(*) FROM organisation_members
      JOIN organisations owned ON owned.id = organisation_members.org_id
      WHERE organisation_members.user_id = sqlc.arg(owner_id)::uuid
        AND organisation_members.role = 'owner'
        AND owned.deleted_at IS NULL
  ) < sqlc.arg(max_owned)::int
RETURNING *;

-- name: PurgeOrganisationsDeletedBefore :execrows
DELETE FROM organisations WHERE deleted_at < sqlc.arg(cutoff)::timestamptz;

-- name: ListOrganisationsForUser :many
-- @sqlc-vet-disable org-scoped
SELECT sqlc.embed(organisations), organisation_members.role AS member_role, organisation_members.joined_at AS member_joined_at
FROM organisation_members
JOIN organisations ON organisations.id = organisation_members.org_id
WHERE organisation_members.user_id = $1
  AND (organisations.deleted_at IS NULL OR organisation_members.role = 'owner')
ORDER BY organisations.name, organisations.id;

-- name: CountOrganisationsOwnedByUser :one
-- @sqlc-vet-disable org-scoped
SELECT COUNT(*) FROM organisation_members
JOIN organisations ON organisations.id = organisation_members.org_id
WHERE organisation_members.user_id = $1
  AND organisation_members.role = 'owner'
  AND organisations.deleted_at IS NULL;

-- name: ListOrganisationsOwnedWithOtherMembers :many
-- @sqlc-vet-disable org-scoped
SELECT organisations.* FROM organisations
JOIN organisation_members owners ON owners.org_id = organisations.id
WHERE owners.user_id = sqlc.arg(user_id)
  AND owners.role = 'owner'
  AND organisations.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM organisation_members others
      WHERE others.org_id = organisations.id AND others.user_id <> sqlc.arg(user_id)
  )
ORDER BY organisations.name, organisations.id;

-- name: LockOrganisationsOfMember :many
-- @sqlc-vet-disable org-scoped
SELECT organisations.id FROM organisations
JOIN organisation_members ON organisation_members.org_id = organisations.id
WHERE organisation_members.user_id = $1
ORDER BY organisations.id
FOR UPDATE OF organisations;

-- name: SoftDeleteOrganisationsOwnedAlone :execrows
-- @sqlc-vet-disable org-scoped
UPDATE organisations SET deleted_at = NOW(), updated_at = NOW()
WHERE deleted_at IS NULL
  AND id IN (
      SELECT owners.org_id FROM organisation_members owners
      WHERE owners.user_id = sqlc.arg(user_id) AND owners.role = 'owner'
  )
  AND NOT EXISTS (
      SELECT 1 FROM organisation_members others
      WHERE others.org_id = organisations.id AND others.user_id <> sqlc.arg(user_id)
  );

-- name: ListOrganisationsForStaff :many
-- @sqlc-vet-disable org-scoped
SELECT sqlc.embed(organisations),
       owners.user_id AS owner_id,
       owner_users.display_name AS owner_display_name,
       owner_users.email AS owner_email,
       (SELECT COUNT(*) FROM organisation_members members WHERE members.org_id = organisations.id)::bigint AS member_count,
       (SELECT COUNT(*) FROM teams WHERE teams.org_id = organisations.id)::bigint AS team_count,
       (SELECT COUNT(*) FROM boards WHERE boards.org_id = organisations.id AND boards.deleted_at IS NULL)::bigint AS board_count
FROM organisations
LEFT JOIN organisation_members owners ON owners.org_id = organisations.id AND owners.role = 'owner'
LEFT JOIN users owner_users ON owner_users.id = owners.user_id
WHERE (sqlc.arg(pattern)::text = '' OR organisations.name ILIKE sqlc.arg(pattern)::text OR organisations.slug ILIKE sqlc.arg(pattern)::text)
ORDER BY organisations.created_at DESC, organisations.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountOrganisationsForStaff :one
SELECT COUNT(*) FROM organisations
WHERE (sqlc.arg(pattern)::text = '' OR name ILIKE sqlc.arg(pattern)::text OR slug ILIKE sqlc.arg(pattern)::text);

-- name: GetOrganisationForStaff :one
SELECT sqlc.embed(organisations),
       owners.user_id AS owner_id,
       owner_users.display_name AS owner_display_name,
       owner_users.email AS owner_email,
       (SELECT COUNT(*) FROM organisation_members members WHERE members.org_id = organisations.id)::bigint AS member_count,
       (SELECT COUNT(*) FROM teams WHERE teams.org_id = organisations.id)::bigint AS team_count,
       (SELECT COUNT(*) FROM boards WHERE boards.org_id = organisations.id AND boards.deleted_at IS NULL)::bigint AS board_count
FROM organisations
LEFT JOIN organisation_members owners ON owners.org_id = organisations.id AND owners.role = 'owner'
LEFT JOIN users owner_users ON owner_users.id = owners.user_id
WHERE organisations.id = sqlc.arg(org_id);

-- name: AddOrganisationMemberWithinQuota :one
INSERT INTO organisation_members (org_id, user_id, role)
SELECT sqlc.arg(org_id)::uuid, sqlc.arg(user_id)::uuid, sqlc.arg(role)::text
WHERE (
    SELECT COUNT(*) FROM organisation_members WHERE org_id = sqlc.arg(org_id)::uuid
) < sqlc.arg(max_members)::int
RETURNING *;

-- name: GetOrganisationMember :one
SELECT * FROM organisation_members WHERE org_id = $1 AND user_id = $2;

-- name: ListOrganisationMembers :many
SELECT sqlc.embed(organisation_members), users.email, users.display_name, COALESCE(user_roles.role, '')::text AS site_role
FROM organisation_members
JOIN users ON users.id = organisation_members.user_id
LEFT JOIN user_roles ON user_roles.user_id = organisation_members.user_id
WHERE organisation_members.org_id = $1
ORDER BY organisation_members.joined_at, organisation_members.user_id;

-- name: CountOrganisationMembers :one
SELECT COUNT(*) FROM organisation_members WHERE org_id = $1;

-- name: SetOrganisationMemberRole :one
UPDATE organisation_members SET role = sqlc.arg(role)::text
WHERE org_id = sqlc.arg(org_id)
  AND user_id = sqlc.arg(user_id)
  AND role <> 'owner'
  AND sqlc.arg(role)::text <> 'owner'
RETURNING *;

-- name: RemoveOrganisationMember :one
DELETE FROM organisation_members
WHERE org_id = $1 AND user_id = $2 AND role <> 'owner'
RETURNING *;

-- name: DemoteOrganisationOwner :execrows
UPDATE organisation_members SET role = 'admin'
WHERE org_id = $1 AND user_id = $2 AND role = 'owner';

-- name: PromoteOrganisationOwnerWithinOwnerQuota :execrows
UPDATE organisation_members SET role = 'owner'
WHERE organisation_members.org_id = sqlc.arg(org_id)
  AND organisation_members.user_id = sqlc.arg(user_id)
  AND organisation_members.role <> 'owner'
  AND (
      SELECT COUNT(*) FROM organisation_members owned_members
      JOIN organisations owned ON owned.id = owned_members.org_id
      WHERE owned_members.user_id = sqlc.arg(user_id)
        AND owned_members.role = 'owner'
        AND owned.deleted_at IS NULL
  ) < sqlc.arg(max_owned)::int;

-- name: CreateOrganisationInvite :one
INSERT INTO organisation_invites (org_id, team_id, token_hash, email, role, max_uses, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOrganisationInvite :one
SELECT * FROM organisation_invites WHERE org_id = $1 AND id = $2;

-- name: ListOrganisationInvites :many
SELECT * FROM organisation_invites
WHERE org_id = $1
  AND revoked_at IS NULL
  AND (max_uses IS NULL OR use_count < max_uses)
ORDER BY created_at DESC, id DESC;

-- name: RevokeOrganisationInvite :one
UPDATE organisation_invites SET revoked_at = NOW()
WHERE org_id = $1 AND id = $2 AND revoked_at IS NULL
RETURNING *;

-- name: GetLiveOrganisationInviteByTokenHash :one
-- @sqlc-vet-disable org-scoped
SELECT sqlc.embed(organisation_invites),
       organisations.name AS org_name,
       organisations.slug AS org_slug,
       teams.name AS team_name,
       COALESCE(users.display_name, '')::text AS inviter_name
FROM organisation_invites
JOIN organisations ON organisations.id = organisation_invites.org_id
LEFT JOIN teams ON teams.org_id = organisation_invites.org_id AND teams.id = organisation_invites.team_id AND teams.archived_at IS NULL
LEFT JOIN users ON users.id = organisation_invites.invited_by
WHERE organisation_invites.token_hash = $1
  AND organisation_invites.revoked_at IS NULL
  AND organisation_invites.expires_at > NOW()
  AND (organisation_invites.max_uses IS NULL OR organisation_invites.use_count < organisation_invites.max_uses)
  AND organisations.deleted_at IS NULL;

-- name: ConsumeOrganisationInvite :one
-- @sqlc-vet-disable org-scoped
UPDATE organisation_invites SET use_count = use_count + 1
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL
  AND expires_at > NOW()
  AND (max_uses IS NULL OR use_count < max_uses)
  AND (email IS NULL OR email = sqlc.arg(email)::citext)
RETURNING *;

-- name: DeleteExpiredOrganisationInvites :execrows
-- @sqlc-vet-disable org-scoped
DELETE FROM organisation_invites WHERE expires_at <= NOW();
