package admin

import (
	"encoding/json"
	"testing"
	"time"

	"retrotide/internal/authz"
	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/email"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	statsNow = time.Date(2026, 10, 5, 10, 30, 0, 0, time.UTC)
)

func (m *testMocks) siteCounts() {
	m.stats.EXPECT().CountTotals(mock.Anything, time.Date(2026, 9, 28, 10, 30, 0, 0, time.UTC)).Return(&model.SiteTotals{
		Users: 40, UsersAdded: 4, VerifiedUsers: 30, VerifiedUsersAdded: 3,
		Organisations: 12, OrganisationsAdded: 2, Teams: 20, TeamsAdded: 1,
		Boards: 50, BoardsAdded: 6, Cards: 900, CardsAdded: 120,
	}, nil)
	m.stats.EXPECT().CountActivity(mock.Anything, spec.SiteActivityWindows{
		DaySince:   time.Date(2026, 10, 4, 10, 30, 0, 0, time.UTC),
		WeekSince:  time.Date(2026, 9, 28, 10, 30, 0, 0, time.UTC),
		MonthSince: time.Date(2026, 9, 5, 10, 30, 0, 0, time.UTC),
	}).Return(&model.SiteActivity{
		Day:   model.SiteActivityCounts{Users: 1, Organisations: 0, Boards: 2, Cards: 30},
		Week:  model.SiteActivityCounts{Users: 4, Organisations: 2, Boards: 6, Cards: 120},
		Month: model.SiteActivityCounts{Users: 9, Organisations: 5, Boards: 15, Cards: 400},
	}, nil)
	m.stats.EXPECT().ListDaily(mock.Anything, spec.SiteDailySeries{FirstDay: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), Days: statsSeriesDays}).Return([]model.SiteDay{
		{Day: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), Users: 0, Boards: 1, Cards: 7},
		{Day: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Users: 2, Boards: 0, Cards: 0},
	}, nil)
}

func (m *testMocks) healthySite() {
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePrivate), nil)
	m.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(true, nil)
	m.email.EXPECT().Status(mock.Anything).Return(email.StatusReady, nil)
	m.google.EXPECT().ID().Return(model.ProviderGoogle)
	m.google.EXPECT().Enabled(mock.Anything).Return(false, nil)
}

func newStatsService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	svc, m := newTestService(t)
	svc.now = func() time.Time {
		return statsNow
	}

	return svc, m
}

func TestGetStats_BuildsEveryPartAndShowsTheNewestSignupsToAUserViewer(t *testing.T) {
	// given
	svc, m := newStatsService(t)
	viewerID, newest := uuid.New(), uuid.New()
	m.siteCounts()
	m.healthySite()
	m.authz.EXPECT().Can(mock.Anything, viewerID, authz.PermViewUsers).Return(true, nil)
	m.users.EXPECT().ListWithRoles(mock.Anything, spec.UserListing{Limit: recentSignupsShown}).Return([]model.UserWithRole{{User: *account(newest)}}, nil)

	// when
	stats, err := svc.GetStats(t.Context(), viewerID)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.AdminStatsResponse{
		GeneratedAt: "2026-10-05T10:30:00Z",
		Totals: dto.AdminStatTotals{
			Users:         dto.AdminStatTotal{Total: 40, Last7Days: 4},
			VerifiedUsers: dto.AdminStatTotal{Total: 30, Last7Days: 3},
			Organisations: dto.AdminStatTotal{Total: 12, Last7Days: 2},
			Teams:         dto.AdminStatTotal{Total: 20, Last7Days: 1},
			Boards:        dto.AdminStatTotal{Total: 50, Last7Days: 6},
			Cards:         dto.AdminStatTotal{Total: 900, Last7Days: 120},
		},
		Daily: []dto.AdminStatDay{
			{Date: "2026-09-06", Users: 0, Boards: 1, Cards: 7},
			{Date: "2026-10-05", Users: 2, Boards: 0, Cards: 0},
		},
		Activity: dto.AdminActivity{
			Last24Hours: dto.AdminActivityCounts{Users: 1, Organisations: 0, Boards: 2, Cards: 30},
			Last7Days:   dto.AdminActivityCounts{Users: 4, Organisations: 2, Boards: 6, Cards: 120},
			Last30Days:  dto.AdminActivityCounts{Users: 9, Organisations: 5, Boards: 15, Cards: 400},
		},
		RecentSignups: &[]dto.AdminRecentSignup{
			{ID: newest, DisplayName: testDisplayName, Email: testEmail, EmailVerified: true, CreatedAt: "2026-09-01T12:00:00Z"},
		},
		Health: dto.AdminSiteHealth{SiteMode: config.SiteModePrivate, MaintenanceMode: true, EmailConfigured: true, EmailStatus: "ready", GoogleSignInConfigured: false},
	}, stats)
}

func TestGetStats_FlagsEmailThatIsSetUpButFailing(t *testing.T) {
	cases := []struct {
		name           string
		status         email.Status
		wantConfigured bool
	}{
		{name: "off", status: email.StatusOff, wantConfigured: false},
		{name: "ready", status: email.StatusReady, wantConfigured: true},
		{name: "failing", status: email.StatusFailing, wantConfigured: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newStatsService(t)
			viewerID := uuid.New()
			m.siteCounts()
			m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
			m.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, nil)
			m.email.EXPECT().Status(mock.Anything).Return(tc.status, nil)
			m.google.EXPECT().ID().Return(model.ProviderGoogle)
			m.google.EXPECT().Enabled(mock.Anything).Return(false, nil)
			m.authz.EXPECT().Can(mock.Anything, viewerID, authz.PermViewUsers).Return(false, nil)

			// when
			stats, err := svc.GetStats(t.Context(), viewerID)

			// then
			require.NoError(t, err)
			assert.Equal(t, string(tc.status), stats.Health.EmailStatus)
			assert.Equal(t, tc.wantConfigured, stats.Health.EmailConfigured)
		})
	}
}

func TestGetStats_LeavesOutTheSignupsForAViewerWithoutViewUsers(t *testing.T) {
	// given
	svc, m := newStatsService(t)
	viewerID := uuid.New()
	m.siteCounts()
	m.healthySite()
	m.authz.EXPECT().Can(mock.Anything, viewerID, authz.PermViewUsers).Return(false, nil)

	// when
	stats, err := svc.GetStats(t.Context(), viewerID)

	// then the user list is never read, and the key is absent rather than empty
	require.NoError(t, err)
	assert.Nil(t, stats.RecentSignups)
	encoded, err := json.Marshal(stats)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "recent_signups")
}

func TestGetStats_ReturnsAFailedCountInsteadOfAPartialDashboard(t *testing.T) {
	// given
	svc, m := newStatsService(t)
	m.stats.EXPECT().CountTotals(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	stats, err := svc.GetStats(t.Context(), uuid.New())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, stats)
}

func TestGetStats_ReturnsAFailedPermissionCheckInsteadOfHidingTheSignups(t *testing.T) {
	// given
	svc, m := newStatsService(t)
	viewerID := uuid.New()
	m.siteCounts()
	m.healthySite()
	m.authz.EXPECT().Can(mock.Anything, viewerID, authz.PermViewUsers).Return(false, errDatabaseDown)

	// when
	stats, err := svc.GetStats(t.Context(), viewerID)

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, stats)
}

func TestGetStats_ReturnsAFailedHealthRead(t *testing.T) {
	// given
	svc, m := newStatsService(t)
	m.siteCounts()
	m.settings.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(config.SiteModePublic), nil)
	m.settings.EXPECT().GetBool(mock.Anything, config.SettingMaintenanceMode).Return(false, nil)
	m.email.EXPECT().Status(mock.Anything).Return("", errDatabaseDown)

	// when
	stats, err := svc.GetStats(t.Context(), uuid.New())

	// then
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, stats)
}

func TestProviderConfigured_IsFalseWhenTheProviderIsNotComposed(t *testing.T) {
	// given
	svc, _ := newStatsService(t)
	svc.signInProviders = nil

	// when
	configured, err := svc.providerConfigured(t.Context(), model.ProviderGoogle)

	// then
	require.NoError(t, err)
	assert.False(t, configured)
}

func TestProviderConfigured_ReportsTheProvidersOwnAnswer(t *testing.T) {
	// given
	svc, m := newStatsService(t)
	m.google.EXPECT().ID().Return(model.ProviderGoogle)
	m.google.EXPECT().Enabled(mock.Anything).Return(true, nil)

	// when
	configured, err := svc.providerConfigured(t.Context(), model.ProviderGoogle)

	// then
	require.NoError(t, err)
	assert.True(t, configured)
}
