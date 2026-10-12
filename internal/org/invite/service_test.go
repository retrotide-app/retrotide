package invite

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/config"
	"retrotide/internal/email"
	"retrotide/internal/entitlements"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/settings"
	"retrotide/internal/token"
	"retrotide/internal/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		orgs         *repository.MockOrganisationRepository
		teams        *repository.MockTeamRepository
		users        *repository.MockUserRepository
		auditLog     *repository.MockAuditLogRepository
		accounts     *user.MockService
		orgAuthz     *orgauthz.MockService
		entitlements *entitlements.MockService
		settings     *settings.MockService
		email        *email.MockService
	}
)

const (
	testBaseURL  = "https://retrotide.test"
	testSiteName = "RetroTide"
	testIP       = "203.0.113.9"
	testToken    = "a-presented-invite-token"
	testOrgName  = "Acme Retros"
	testOrgSlug  = "acme-retros"
	testInviter  = "Dana"
	testAddress  = "carol@example.com"
	testPassword = "correct horse battery staple"
	testName     = "Carol"
	freeMembers  = 10
)

var (
	inviteLink       = regexp.MustCompile(`https://retrotide\.test/invite#([0-9a-f]{64})`)
	verificationLink = regexp.MustCompile(`https://retrotide\.test/verify-email#([0-9a-f]{64})`)
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		orgs:         repository.NewMockOrganisationRepository(t),
		teams:        repository.NewMockTeamRepository(t),
		users:        repository.NewMockUserRepository(t),
		auditLog:     repository.NewMockAuditLogRepository(t),
		accounts:     user.NewMockService(t),
		orgAuthz:     orgauthz.NewMockService(t),
		entitlements: entitlements.NewMockService(t),
		settings:     settings.NewMockService(t),
		email:        email.NewMockService(t),
	}

	svc := NewService(Deps{
		Organisations: m.orgs,
		Teams:         m.teams,
		Users:         m.users,
		AuditLog:      m.auditLog,
		Accounts:      m.accounts,
		OrgAuthz:      m.orgAuthz,
		Entitlements:  m.entitlements,
		Settings:      m.settings,
		Email:         m.email,
	}).(*service)

	return svc, m
}

func freePlan(t *testing.T) *entitlements.Plan {
	t.Helper()

	plan, ok := entitlements.PlanFor(model.PlanFree)
	require.True(t, ok)

	return plan
}

func (m *testMocks) readsSite() {
	m.settings.EXPECT().Get(mock.Anything, config.SettingBaseURL).Return(testBaseURL+"/", nil)
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteName).Return(testSiteName, nil)
}

func (m *testMocks) audits(err error) *[]audit.NewEntry {
	recorded := &[]audit.NewEntry{}

	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, entry audit.NewEntry, _ ...*sql.Tx) error {
		*recorded = append(*recorded, entry)

		return err
	})

	return recorded
}

func (m *testMocks) sends(err error) *[]email.Message {
	sent := &[]email.Message{}

	m.email.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, message email.Message) error {
		*sent = append(*sent, message)

		return err
	})

	return sent
}

func (m *testMocks) hasSpace(t *testing.T, orgID uuid.UUID, members int) {
	t.Helper()

	m.orgs.EXPECT().CountMembers(mock.Anything, orgID).Return(members, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, spec.LimitCheck{OrgID: orgID, Limit: model.LimitMembers, Current: members}).Return(nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(t), nil)
}

func (m *testMocks) isFull(orgID uuid.UUID, members int) {
	m.orgs.EXPECT().CountMembers(mock.Anything, orgID).Return(members, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, spec.LimitCheck{OrgID: orgID, Limit: model.LimitMembers, Current: members}).Return(&entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitMembers, Allowed: freeMembers, Current: members})
}

func (m *testMocks) notAMember(orgID uuid.UUID, userID uuid.UUID) {
	key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}

	m.orgs.EXPECT().GetMember(mock.Anything, key).Return(nil, errMissing)
}

func (m *testMocks) accepts(details *model.InviteDetails, err error) *spec.InviteAcceptance {
	captured := &spec.InviteAcceptance{}

	m.orgs.EXPECT().AcceptInvite(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.InviteAcceptance, _ ...*sql.Tx) (*model.AcceptedInvite, error) {
		*captured = s

		accepted := &model.AcceptedInvite{
			Invite: details.OrganisationInvite,
			Member: model.OrganisationMember{OrgID: details.OrgID, UserID: s.UserID, Role: details.Role, JoinedAt: time.Now()},
		}
		if details.TeamID != nil {
			accepted.TeamMember = &model.TeamMember{OrgID: details.OrgID, TeamID: *details.TeamID, UserID: s.UserID, Role: model.TeamRoleMember}
		}

		return accepted, err
	})

	return captured
}

func (m *testMocks) finds(details *model.InviteDetails) {
	hash := token.Hash(testToken)

	m.orgs.EXPECT().GetLiveInviteByTokenHash(mock.Anything, hash).Return(details, nil)
}

func liveDetails(orgID uuid.UUID) *model.InviteDetails {
	return &model.InviteDetails{
		ID:          uuid.New(),
		OrgID:       orgID,
		TokenHash:   token.Hash(testToken),
		Role:        model.OrgRoleMember,
		ExpiresAt:   time.Now().Add(time.Hour),
		CreatedAt:   time.Now().Add(-time.Hour),
		OrgName:     testOrgName,
		OrgSlug:     testOrgSlug,
		InviterName: testInviter,
	}
}

func boundDetails(orgID uuid.UUID, address string) *model.InviteDetails {
	details := liveDetails(orgID)
	details.Email = new(address)
	details.MaxUses = new(1)

	return details
}

func linkedToken(t *testing.T, pattern *regexp.Regexp, message email.Message) string {
	t.Helper()

	match := pattern.FindStringSubmatch(message.HTML)
	require.Len(t, match, 2, "the email carries no link matching %s", pattern)

	return match[1]
}
