package dao_test

import (
	"strings"
	"testing"

	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	tenancyFixture struct {
		orgA     uuid.UUID
		teamA    uuid.UUID
		teamB    uuid.UUID
		owner    uuid.UUID
		member   uuid.UUID
		stranger uuid.UUID
	}

	polishFixture struct {
		boardFixture

		cardID     uuid.UUID
		surveyID   uuid.UUID
		questionID uuid.UUID
		stranger   uuid.UUID
	}
)

const (
	identityInsert      = `INSERT INTO user_identities (provider, subject, user_id, email) VALUES ($1, $2, $3, 'someone@gmail.com')`
	flowInsert          = `INSERT INTO oidc_flows (token_hash, provider, purpose, state, nonce, code_verifier, user_id, expires_at) VALUES ('flow-hash', $1, $2, $3, $4, $5, $6::uuid, NOW() + INTERVAL '10 minutes')`
	organisationInsert  = `INSERT INTO organisations (slug, name) VALUES ($1, $2)`
	orgMemberInsert     = `INSERT INTO organisation_members (org_id, user_id, role) VALUES ($1, $2, $3)`
	teamInsert          = `INSERT INTO teams (org_id, name) VALUES ($1, $2)`
	teamMemberInsert    = `INSERT INTO team_members (org_id, team_id, user_id, role) VALUES ($1, $2, $3, $4)`
	inviteInsert        = `INSERT INTO organisation_invites (org_id, team_id, token_hash, email, role, max_uses, use_count, expires_at) VALUES ($1, $2::uuid, $3, $4::citext, $5, $6::int, $7, NOW() + INTERVAL '7 days')`
	savedTemplateInsert = `INSERT INTO saved_templates (org_id, team_id, owner_id, name, columns) VALUES ($1, $2::uuid, $3::uuid, $4, $5::jsonb)`
)

var (
	flowValue43 = strings.Repeat("f", 43)
)

func newTenancyFixture(t *testing.T, repos *repository.Repositories) tenancyFixture {
	t.Helper()

	owner := daotest.CreateUser(t, repos)
	a := daotest.CreateOrg(t, repos, owner.ID)
	b := daotest.CreateOrg(t, repos, daotest.CreateUser(t, repos).ID)
	member := daotest.CreateUser(t, repos)
	daotest.AddOrgMember(t, repos, a.Organisation.ID, member.ID, model.OrgRoleMember)

	return tenancyFixture{
		orgA:     a.Organisation.ID,
		teamA:    a.Team.ID,
		teamB:    b.Team.ID,
		owner:    owner.ID,
		member:   member.ID,
		stranger: daotest.CreateUser(t, repos).ID,
	}
}

func TestSchema_TenancyConstraintsRefuseBadRows(t *testing.T) {
	tests := []struct {
		name       string
		statement  string
		args       func(f tenancyFixture) []any
		constraint string
	}{
		{
			name:       "a slug another organisation holds",
			statement:  `INSERT INTO organisations (slug, name) SELECT slug, 'Copy' FROM organisations WHERE id = $1`,
			args:       func(f tenancyFixture) []any { return []any{f.orgA} },
			constraint: "organisations_slug_key",
		},
		{
			name:       "a slug with capitals",
			statement:  organisationInsert,
			args:       func(tenancyFixture) []any { return []any{"Acme", "Acme"} },
			constraint: "organisations_slug_format",
		},
		{
			name:       "a slug that starts with a hyphen",
			statement:  organisationInsert,
			args:       func(tenancyFixture) []any { return []any{"-acme", "Acme"} },
			constraint: "organisations_slug_format",
		},
		{
			name:       "a slug of two characters",
			statement:  organisationInsert,
			args:       func(tenancyFixture) []any { return []any{"ab", "Acme"} },
			constraint: "organisations_slug_format",
		},
		{
			name:       "a slug of 41 characters",
			statement:  organisationInsert,
			args:       func(tenancyFixture) []any { return []any{strings.Repeat("a", 41), "Acme"} },
			constraint: "organisations_slug_format",
		},
		{
			name:       "an empty organisation name",
			statement:  organisationInsert,
			args:       func(tenancyFixture) []any { return []any{"acme", ""} },
			constraint: "organisations_name_length",
		},
		{
			name:       "an organisation name over 80 characters",
			statement:  organisationInsert,
			args:       func(tenancyFixture) []any { return []any{"acme", strings.Repeat("n", 81)} },
			constraint: "organisations_name_length",
		},
		{
			name:       "an organisation role outside the catalogue",
			statement:  orgMemberInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.stranger, "guest"} },
			constraint: "organisation_members_role_check",
		},
		{
			name:       "a second owner",
			statement:  orgMemberInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.stranger, "owner"} },
			constraint: "organisation_members_one_owner",
		},
		{
			name:       "a second membership of one organisation",
			statement:  orgMemberInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.member, "admin"} },
			constraint: "organisation_members_pkey",
		},
		{
			name:       "a team name the organisation holds, in another case",
			statement:  `INSERT INTO teams (org_id, name) SELECT org_id, UPPER(name) FROM teams WHERE id = $1`,
			args:       func(f tenancyFixture) []any { return []any{f.teamA} },
			constraint: "teams_org_name_key",
		},
		{
			name:       "an empty team name",
			statement:  teamInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, ""} },
			constraint: "teams_name_length",
		},
		{
			name:       "a team name over 80 characters",
			statement:  teamInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, strings.Repeat("n", 81)} },
			constraint: "teams_name_length",
		},
		{
			name:       "a team role outside the catalogue",
			statement:  teamMemberInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.teamA, f.member, "owner"} },
			constraint: "team_members_role_check",
		},
		{
			name:       "a team of another organisation",
			statement:  teamMemberInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.teamB, f.member, "member"} },
			constraint: "team_members_team_fkey",
		},
		{
			name:       "a team member who is not in the organisation",
			statement:  teamMemberInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.teamA, f.stranger, "member"} },
			constraint: "team_members_org_member_fkey",
		},
		{
			name:       "one token hash for two invites",
			statement:  `INSERT INTO organisation_invites (org_id, token_hash, expires_at) VALUES ($1, 'shared-hash', NOW() + INTERVAL '7 days'), ($1, 'shared-hash', NOW() + INTERVAL '7 days')`,
			args:       func(f tenancyFixture) []any { return []any{f.orgA} },
			constraint: "organisation_invites_token_hash_key",
		},
		{
			name:       "an invite that would make an owner",
			statement:  inviteInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, "owner-hash", nil, "owner", nil, 0} },
			constraint: "organisation_invites_role_check",
		},
		{
			name:       "an invite to a team of another organisation",
			statement:  inviteInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.teamB, "team-hash", nil, "member", nil, 0} },
			constraint: "organisation_invites_team_fkey",
		},
		{
			name:       "an invite with no uses",
			statement:  inviteInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, "zero-hash", nil, "member", 0, 0} },
			constraint: "organisation_invites_uses_check",
		},
		{
			name:       "an invite used more often than it allows",
			statement:  inviteInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, "over-hash", nil, "member", 2, 3} },
			constraint: "organisation_invites_uses_check",
		},
		{
			name:      "an email invite with unlimited uses",
			statement: inviteInsert,
			args: func(f tenancyFixture) []any {
				return []any{f.orgA, nil, "email-hash", "dana@example.com", "member", nil, 0}
			},
			constraint: "organisation_invites_email_single_use",
		},
		{
			name:      "an email invite for two people",
			statement: inviteInsert,
			args: func(f tenancyFixture) []any {
				return []any{f.orgA, nil, "email-hash", "dana@example.com", "member", 2, 0}
			},
			constraint: "organisation_invites_email_single_use",
		},
		{
			name:       "an invited address too short to be one",
			statement:  inviteInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, "short-hash", "a@", "member", 1, 0} },
			constraint: "organisation_invites_email_length",
		},
		{
			name:      "a saved template for both a team and a person",
			statement: savedTemplateInsert,
			args: func(f tenancyFixture) []any {
				return []any{f.orgA, f.teamA, f.member, "Both", `[{"title":"Went well"}]`}
			},
			constraint: "saved_templates_one_scope",
		},
		{
			name:       "a saved template for neither a team nor a person",
			statement:  savedTemplateInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, nil, "Neither", `[{"title":"Went well"}]`} },
			constraint: "saved_templates_one_scope",
		},
		{
			name:       "a saved template for a team of another organisation",
			statement:  savedTemplateInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, f.teamB, nil, "Foreign", `[{"title":"Went well"}]`} },
			constraint: "saved_templates_team_fkey",
		},
		{
			name:      "a personal saved template for someone outside the organisation",
			statement: savedTemplateInsert,
			args: func(f tenancyFixture) []any {
				return []any{f.orgA, nil, f.stranger, "Stranger", `[{"title":"Went well"}]`}
			},
			constraint: "saved_templates_owner_fkey",
		},
		{
			name:       "an empty saved template name",
			statement:  savedTemplateInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, f.member, "", `[{"title":"Went well"}]`} },
			constraint: "saved_templates_name_length",
		},
		{
			name:      "a saved template name over 80 characters",
			statement: savedTemplateInsert,
			args: func(f tenancyFixture) []any {
				return []any{f.orgA, nil, f.member, strings.Repeat("n", 81), `[{"title":"Went well"}]`}
			},
			constraint: "saved_templates_name_length",
		},
		{
			name:       "a saved template with no columns",
			statement:  savedTemplateInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, f.member, "Empty", `[]`} },
			constraint: "saved_templates_columns_shape",
		},
		{
			name:       "saved template columns that are not a list",
			statement:  savedTemplateInsert,
			args:       func(f tenancyFixture) []any { return []any{f.orgA, nil, f.member, "Object", `{"title":"Went well"}`} },
			constraint: "saved_templates_columns_shape",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			fixture := newTenancyFixture(t, repos)

			// when
			_, err := repos.DB().ExecContext(t.Context(), tc.statement, tc.args(fixture)...)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func TestSchema_DeletingAnOrganisationCascadesToEverythingBelowIt(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	fixture := newTenancyFixture(t, repos)
	_, err := repos.DB().ExecContext(t.Context(), inviteInsert, fixture.orgA, fixture.teamA, "cascade-hash", nil, "member", nil, 0)
	require.NoError(t, err)

	// when
	_, err = repos.DB().ExecContext(t.Context(), `DELETE FROM organisations WHERE id = $1`, fixture.orgA)

	// then
	require.NoError(t, err)
	for _, table := range []string{"organisation_members", "teams", "team_members", "organisation_invites"} {
		assert.Zero(t, countWhere(t, repos, "SELECT COUNT(*) FROM "+table+" WHERE org_id = $1", fixture.orgA), table)
	}
}

func TestSchema_DeletingAUserLeavesTheOrganisationsTheyCreatedAndInvitedTo(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	fixture := newTenancyFixture(t, repos)
	_, err := repos.DB().ExecContext(t.Context(), `INSERT INTO organisation_invites (org_id, token_hash, invited_by, expires_at) VALUES ($1, 'kept-hash', $2, NOW() + INTERVAL '7 days')`, fixture.orgA, fixture.member)
	require.NoError(t, err)

	// when
	_, ownerErr := repos.DB().ExecContext(t.Context(), `DELETE FROM users WHERE id = $1`, fixture.owner)
	_, memberErr := repos.DB().ExecContext(t.Context(), `DELETE FROM users WHERE id = $1`, fixture.member)

	// then the rows stay with their author cleared, and the memberships go
	require.NoError(t, ownerErr)
	require.NoError(t, memberErr)
	assert.Equal(t, 1, countWhere(t, repos, `SELECT COUNT(*) FROM organisations WHERE id = $1 AND created_by IS NULL`, fixture.orgA))
	assert.Equal(t, 1, countWhere(t, repos, `SELECT COUNT(*) FROM organisation_invites WHERE org_id = $1 AND invited_by IS NULL`, fixture.orgA))
	assert.Zero(t, countWhere(t, repos, `SELECT COUNT(*) FROM organisation_members WHERE org_id = $1`, fixture.orgA))
	assert.Zero(t, countWhere(t, repos, `SELECT COUNT(*) FROM team_members WHERE org_id = $1`, fixture.orgA))
}

func TestSchema_NamedConstraintsRefuseBadRows(t *testing.T) {
	tests := []struct {
		name       string
		statement  string
		args       func(userID uuid.UUID) []any
		constraint string
	}{
		{
			name:       "an address already held, in another case",
			statement:  `INSERT INTO users (email) VALUES ($1)`,
			args:       func(uuid.UUID) []any { return []any{"HOLDER@example.com"} },
			constraint: "users_email_key",
		},
		{
			name:       "an address too short to be one",
			statement:  `INSERT INTO users (email) VALUES ($1)`,
			args:       func(uuid.UUID) []any { return []any{"a@"} },
			constraint: "users_email_length",
		},
		{
			name:       "a display name over 40 characters",
			statement:  `INSERT INTO users (email, display_name) VALUES ($1, $2)`,
			args:       func(uuid.UUID) []any { return []any{"long@example.com", strings.Repeat("n", 41)} },
			constraint: "users_display_name_length",
		},
		{
			name:       "a notification type outside the catalogue",
			statement:  `INSERT INTO notifications (user_id, type) VALUES ($1, 'card_created')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "notifications_type_check",
		},
		{
			name:       "a staff role outside the catalogue",
			statement:  `INSERT INTO user_roles (user_id, role) VALUES ($1, 'moderator')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "user_roles_role_check",
		},
		{
			name:       "a second staff role for one account",
			statement:  `INSERT INTO user_roles (user_id, role) VALUES ($1, 'support'), ($1, 'admin')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "user_roles_pkey",
		},
		{
			name:       "a stored permission for an immutable role",
			statement:  `INSERT INTO role_permissions (role, permission) VALUES ('admin', 'view_users')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "role_permissions_editable_role_only",
		},
		{
			name:       "an audit actor kind outside the catalogue",
			statement:  `INSERT INTO audit_log (actor_kind, action, target_type) VALUES ('robot', 'ban_user', 'user')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "audit_log_actor_kind_check",
		},
		{
			name:       "audit details that are not an object",
			statement:  `INSERT INTO audit_log (actor_kind, action, target_type, details) VALUES ('staff', 'ban_user', 'user', '["spam"]')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "audit_log_details_object",
		},
		{
			name:       "an identity from a provider outside the catalogue",
			statement:  identityInsert,
			args:       func(userID uuid.UUID) []any { return []any{"github", "sub-1", userID} },
			constraint: "user_identities_provider_check",
		},
		{
			name:       "an identity with an empty subject",
			statement:  identityInsert,
			args:       func(userID uuid.UUID) []any { return []any{"google", "", userID} },
			constraint: "user_identities_subject_length",
		},
		{
			name:       "an identity with a subject over 255 characters",
			statement:  identityInsert,
			args:       func(userID uuid.UUID) []any { return []any{"google", strings.Repeat("s", 256), userID} },
			constraint: "user_identities_subject_length",
		},
		{
			name:       "a second Google identity for one account",
			statement:  `INSERT INTO user_identities (provider, subject, user_id, email) VALUES ('google', 'first', $1, 'first@gmail.com'), ('google', 'second', $1, 'second@gmail.com')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "user_identities_user_provider_key",
		},
		{
			name: "one Google identity linked to two accounts",
			statement: `WITH other AS (INSERT INTO users (email) VALUES ('other@example.com') RETURNING id)
				INSERT INTO user_identities (provider, subject, user_id, email)
				SELECT 'google', 'shared', $1::uuid, 'shared@gmail.com'
				UNION ALL SELECT 'google', 'shared', id, 'shared@gmail.com' FROM other`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "user_identities_pkey",
		},
		{
			name:       "a flow for a provider outside the catalogue",
			statement:  flowInsert,
			args:       func(uuid.UUID) []any { return []any{"github", "sign_in", flowValue43, flowValue43, flowValue43, nil} },
			constraint: "oidc_flows_provider_check",
		},
		{
			name:       "a flow for a purpose outside the catalogue",
			statement:  flowInsert,
			args:       func(uuid.UUID) []any { return []any{"google", "sign_up", flowValue43, flowValue43, flowValue43, nil} },
			constraint: "oidc_flows_purpose_check",
		},
		{
			name:      "a sign-in flow that names a user",
			statement: flowInsert,
			args: func(userID uuid.UUID) []any {
				return []any{"google", "sign_in", flowValue43, flowValue43, flowValue43, userID}
			},
			constraint: "oidc_flows_link_has_user",
		},
		{
			name:      "a link flow bound to no session",
			statement: flowInsert,
			args: func(userID uuid.UUID) []any {
				return []any{"google", "link", flowValue43, flowValue43, flowValue43, userID}
			},
			constraint: "oidc_flows_link_has_session",
		},
		{
			name:       "an empty state",
			statement:  flowInsert,
			args:       func(uuid.UUID) []any { return []any{"google", "sign_in", "", flowValue43, flowValue43, nil} },
			constraint: "oidc_flows_state_length",
		},
		{
			name:      "a truncated state",
			statement: flowInsert,
			args: func(uuid.UUID) []any {
				return []any{"google", "sign_in", flowValue43[:42], flowValue43, flowValue43, nil}
			},
			constraint: "oidc_flows_state_length",
		},
		{
			name:       "an empty nonce",
			statement:  flowInsert,
			args:       func(uuid.UUID) []any { return []any{"google", "sign_in", flowValue43, "", flowValue43, nil} },
			constraint: "oidc_flows_nonce_length",
		},
		{
			name:      "a truncated nonce",
			statement: flowInsert,
			args: func(uuid.UUID) []any {
				return []any{"google", "sign_in", flowValue43, flowValue43[:42], flowValue43, nil}
			},
			constraint: "oidc_flows_nonce_length",
		},
		{
			name:       "an empty code verifier",
			statement:  flowInsert,
			args:       func(uuid.UUID) []any { return []any{"google", "sign_in", flowValue43, flowValue43, "", nil} },
			constraint: "oidc_flows_code_verifier_length",
		},
		{
			name:      "a code verifier shorter than RFC 7636 allows",
			statement: flowInsert,
			args: func(uuid.UUID) []any {
				return []any{"google", "sign_in", flowValue43, flowValue43, flowValue43[:42], nil}
			},
			constraint: "oidc_flows_code_verifier_length",
		},
		{
			name:      "a code verifier longer than RFC 7636 allows",
			statement: flowInsert,
			args: func(uuid.UUID) []any {
				return []any{"google", "sign_in", flowValue43, flowValue43, strings.Repeat("v", 129), nil}
			},
			constraint: "oidc_flows_code_verifier_length",
		},
		{
			name:       "a second pending invitation for one address, in another case",
			statement:  `INSERT INTO invitations (email, token_hash, expires_at) VALUES ('carol@example.com', 'first-hash', NOW() + INTERVAL '7 days'), ('CAROL@example.com', 'second-hash', NOW() + INTERVAL '7 days')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "invitations_email_key",
		},
		{
			name:       "one token hash for two invitations",
			statement:  `INSERT INTO invitations (email, token_hash, expires_at) VALUES ('carol@example.com', 'shared-hash', NOW() + INTERVAL '7 days'), ('dave@example.com', 'shared-hash', NOW() + INTERVAL '7 days')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "invitations_token_hash_key",
		},
		{
			name:       "an invited address too short to be one",
			statement:  `INSERT INTO invitations (email, token_hash, expires_at) VALUES ('a@', 'short-hash', NOW() + INTERVAL '7 days')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "invitations_email_length",
		},
		{
			name:       "a guest device whose hash is not a SHA-256",
			statement:  `INSERT INTO guest_devices (token_hash, expires_at) VALUES ('raw-token', NOW() + INTERVAL '90 days')`,
			args:       func(uuid.UUID) []any { return nil },
			constraint: "guest_devices_token_hash_length",
		},
		{
			name:       "one token hash for two guest devices",
			statement:  `INSERT INTO guest_devices (token_hash, expires_at) VALUES ($1, NOW() + INTERVAL '90 days'), ($1, NOW() + INTERVAL '90 days')`,
			args:       func(uuid.UUID) []any { return []any{strings.Repeat("a", 64)} },
			constraint: "guest_devices_token_hash_key",
		},
		{
			name:       "a two-factor secret that is not 32 base32 characters",
			statement:  `INSERT INTO user_totp (user_id, secret) VALUES ($1, 'short')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "user_totp_secret_length",
		},
		{
			name:       "a negative last used two-factor step",
			statement:  `INSERT INTO user_totp (user_id, secret, last_used_step) VALUES ($1, $2, -1)`,
			args:       func(userID uuid.UUID) []any { return []any{userID, strings.Repeat("A", 32)} },
			constraint: "user_totp_last_used_step_check",
		},
		{
			name:       "a two-factor challenge whose hash is not a SHA-256",
			statement:  `INSERT INTO two_factor_challenges (token_hash, user_id, expires_at) VALUES ('raw-token', $1, NOW() + INTERVAL '5 minutes')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "two_factor_challenges_token_hash_length",
		},
		{
			name:       "a two-factor challenge with negative attempts",
			statement:  `INSERT INTO two_factor_challenges (token_hash, user_id, attempts, expires_at) VALUES ($1, $2, -1, NOW() + INTERVAL '5 minutes')`,
			args:       func(userID uuid.UUID) []any { return []any{strings.Repeat("a", 64), userID} },
			constraint: "two_factor_challenges_attempts_check",
		},
		{
			name:       "a recovery code stored bare rather than as a SHA-256",
			statement:  `INSERT INTO two_factor_recovery_codes (user_id, code_hash) VALUES ($1, 'abcde-fgh23')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "two_factor_recovery_codes_code_hash_length",
		},
		{
			name:       "a trusted device stored bare rather than as a SHA-256",
			statement:  `INSERT INTO two_factor_trusted_devices (token_hash, user_id, expires_at) VALUES ('raw-device-token', $1, NOW() + INTERVAL '30 days')`,
			args:       func(userID uuid.UUID) []any { return []any{userID} },
			constraint: "two_factor_trusted_devices_token_hash_length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			repos := daotest.NewRepos(t)
			holder := daotest.CreateUser(t, repos, daotest.WithEmail("holder@example.com"))

			// when
			_, err := repos.DB().ExecContext(t.Context(), tc.statement, tc.args(holder.ID)...)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func TestSchema_UserDeletionCascadesToSessionsTokensNotificationsAndRoles(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	user := daotest.CreateUser(t, repos)
	statements := []string{
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ('cascade-session', $1, NOW() + INTERVAL '1 hour')`,
		`INSERT INTO email_verification_tokens (token_hash, user_id, expires_at) VALUES ('cascade-verify', $1, NOW() + INTERVAL '1 hour')`,
		`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ('cascade-reset', $1, NOW() + INTERVAL '1 hour')`,
		`INSERT INTO notifications (user_id, type) VALUES ($1, 'org_invite')`,
		`INSERT INTO user_roles (user_id, role) VALUES ($1, 'support')`,
	}
	for _, statement := range statements {
		_, err := repos.DB().ExecContext(t.Context(), statement, user.ID)
		require.NoError(t, err)
	}

	// when
	_, err := repos.DB().ExecContext(t.Context(), `DELETE FROM users WHERE id = $1`, user.ID)

	// then
	require.NoError(t, err)
	var remaining int
	require.NoError(t, repos.DB().QueryRowContext(t.Context(), `
		SELECT (SELECT COUNT(*) FROM sessions) + (SELECT COUNT(*) FROM email_verification_tokens)
		     + (SELECT COUNT(*) FROM password_reset_tokens) + (SELECT COUNT(*) FROM notifications)
		     + (SELECT COUNT(*) FROM user_roles)`).Scan(&remaining))
	assert.Zero(t, remaining)
}

func newPolishFixture(t *testing.T) polishFixture {
	t.Helper()

	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	surveyID := uuid.New()
	questionID := uuid.New()

	_, err := f.repos.DB().ExecContext(t.Context(), `INSERT INTO board_surveys (id, org_id, board_id, title) VALUES ($1, $2, $3, 'Pulse')`, surveyID, f.orgID(), f.boardID())
	require.NoError(t, err)

	_, err = f.repos.DB().ExecContext(t.Context(), `INSERT INTO survey_questions (id, org_id, board_id, survey_id, position, kind, prompt, scale_min, scale_max) VALUES ($1, $2, $3, $4, 0, 'scale', 'Mood?', 1, 5)`, questionID, f.orgID(), f.boardID(), surveyID)
	require.NoError(t, err)

	_, err = f.repos.DB().ExecContext(t.Context(), `INSERT INTO survey_responses (org_id, board_id, survey_id, question_id, participant_id, answer) VALUES ($1, $2, $3, $4, $5, '[3]')`, f.orgID(), f.boardID(), surveyID, questionID, f.facilitator())
	require.NoError(t, err)

	_, err = f.repos.DB().ExecContext(t.Context(), `INSERT INTO card_reactions (org_id, board_id, card_id, participant_id, reaction) VALUES ($1, $2, $3, $4, 'heart')`, f.orgID(), f.boardID(), card.ID, f.facilitator())
	require.NoError(t, err)

	return polishFixture{boardFixture: f, cardID: card.ID, surveyID: surveyID, questionID: questionID, stranger: daotest.CreateUser(t, f.repos).ID}
}

func TestSchema_CardPolishMediaPinAndSurveyConstraintsRefuseBadRows(t *testing.T) {
	const (
		surveyInsert   = `INSERT INTO board_surveys (id, org_id, board_id, title, status, closed_at) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5::timestamptz)`
		questionInsert = `INSERT INTO survey_questions (id, org_id, board_id, survey_id, position, kind, prompt, options, scale_min, scale_max) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7::jsonb, $8::smallint, $9::smallint)`
		responseInsert = `INSERT INTO survey_responses (org_id, board_id, survey_id, question_id, participant_id, answer) VALUES ($1, $2, $3, $4, $5, $6::jsonb)`
		reactionInsert = `INSERT INTO card_reactions (org_id, board_id, card_id, participant_id, reaction) VALUES ($1, $2, $3, $4, $5)`
		mediaInsert    = `INSERT INTO media (org_id, board_id, kind, content_type, width, height, byte_size, data) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	)

	tests := []struct {
		name       string
		statement  string
		args       func(f polishFixture) []any
		constraint string
	}{
		{
			name:       "a survey with no title",
			statement:  surveyInsert,
			args:       func(f polishFixture) []any { return []any{f.orgID(), f.boardID(), "", "open", nil} },
			constraint: "board_surveys_title_length",
		},
		{
			name:       "a survey in an unknown status",
			statement:  surveyInsert,
			args:       func(f polishFixture) []any { return []any{f.orgID(), f.boardID(), "Pulse", "draft", nil} },
			constraint: "board_surveys_status_check",
		},
		{
			name:       "a closed survey with no close time",
			statement:  surveyInsert,
			args:       func(f polishFixture) []any { return []any{f.orgID(), f.boardID(), "Pulse", "closed", nil} },
			constraint: "board_surveys_closed_check",
		},
		{
			name:      "a question of an unknown kind",
			statement: questionInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, 1, "text", "Why?", `[]`, nil, nil}
			},
			constraint: "survey_questions_kind_check",
		},
		{
			name:      "a question with no prompt",
			statement: questionInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, 1, "scale", "", `[]`, 1, 5}
			},
			constraint: "survey_questions_prompt_length",
		},
		{
			name:      "a scale reaching past ten",
			statement: questionInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, 1, "scale", "Mood?", `[]`, 0, 11}
			},
			constraint: "survey_questions_shape_check",
		},
		{
			name:      "a choice with eleven options",
			statement: questionInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, 1, "single_choice", "Pick", `["1","2","3","4","5","6","7","8","9","10","11"]`, nil, nil}
			},
			constraint: "survey_questions_shape_check",
		},
		{
			name:      "a position the survey already uses",
			statement: questionInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, 0, "scale", "Again?", `[]`, 1, 5}
			},
			constraint: "survey_questions_position_key",
		},
		{
			name:      "a question of a survey on no board",
			statement: questionInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), uuid.New(), 1, "scale", "Mood?", `[]`, 1, 5}
			},
			constraint: "survey_questions_survey_fkey",
		},
		{
			name:      "an empty answer",
			statement: responseInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, f.questionID, f.facilitator(), `[]`}
			},
			constraint: "survey_responses_answer_array",
		},
		{
			name:      "a second answer to the same question",
			statement: responseInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), f.surveyID, f.questionID, f.facilitator(), `[4]`}
			},
			constraint: "survey_responses_once",
		},
		{
			name:       "an unknown reaction",
			statement:  reactionInsert,
			args:       func(f polishFixture) []any { return []any{f.orgID(), f.boardID(), f.cardID, f.facilitator(), "skull"} },
			constraint: "card_reactions_reaction_check",
		},
		{
			name:       "the same reaction twice",
			statement:  reactionInsert,
			args:       func(f polishFixture) []any { return []any{f.orgID(), f.boardID(), f.cardID, f.facilitator(), "heart"} },
			constraint: "card_reactions_once",
		},
		{
			name:      "media that is not webp",
			statement: mediaInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), "card", "image/png", 1, 1, 3, []byte("png")}
			},
			constraint: "media_content_type_check",
		},
		{
			name:      "media whose size disagrees with its bytes",
			statement: mediaInsert,
			args: func(f polishFixture) []any {
				return []any{f.orgID(), f.boardID(), "card", "image/webp", 1, 1, 5, []byte("abc")}
			},
			constraint: "media_byte_size_check",
		},
		{
			name:       "a card image that does not exist",
			statement:  `UPDATE cards SET image_media_id = $3 WHERE board_id = $1 AND id = $2`,
			args:       func(f polishFixture) []any { return []any{f.boardID(), f.cardID, uuid.New()} },
			constraint: "cards_image_fkey",
		},
		{
			name:       "a board background that does not exist",
			statement:  `UPDATE boards SET background_media_id = $2 WHERE id = $1`,
			args:       func(f polishFixture) []any { return []any{f.boardID(), uuid.New()} },
			constraint: "boards_background_fkey",
		},
		{
			name:       "a pin by someone outside the organisation",
			statement:  `INSERT INTO board_pins (org_id, board_id, user_id) VALUES ($1, $2, $3)`,
			args:       func(f polishFixture) []any { return []any{f.orgID(), f.boardID(), f.stranger} },
			constraint: "board_pins_member_fkey",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fixture := newPolishFixture(t)

			// when
			_, err := fixture.repos.DB().ExecContext(t.Context(), tc.statement, tc.args(fixture)...)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func TestSchema_DeletingABoardCascadesToItsMediaReactionsPinsAndSurveys(t *testing.T) {
	// given
	fixture := newPolishFixture(t)
	_, err := fixture.repos.DB().ExecContext(t.Context(), `INSERT INTO board_pins (org_id, board_id, user_id) VALUES ($1, $2, $3)`, fixture.orgID(), fixture.boardID(), fixture.workspace.Owner.UserID)
	require.NoError(t, err)
	_, err = fixture.repos.DB().ExecContext(t.Context(), `INSERT INTO media (org_id, board_id, kind, width, height, byte_size, data) VALUES ($1, $2, 'card', 1, 1, 1, 'x')`, fixture.orgID(), fixture.boardID())
	require.NoError(t, err)

	// when
	_, err = fixture.repos.DB().ExecContext(t.Context(), `DELETE FROM boards WHERE id = $1`, fixture.boardID())

	// then
	require.NoError(t, err)
	for _, table := range []string{"media", "card_reactions", "board_pins", "board_surveys", "survey_questions", "survey_responses"} {
		assert.Zero(t, countWhere(t, fixture.repos, "SELECT COUNT(*) FROM "+table+" WHERE board_id = $1", fixture.boardID()), table)
	}
}
