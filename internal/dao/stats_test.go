package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	statsSeriesDays = 30
)

type (
	statsSite struct {
		repos     *repository.Repositories
		now       time.Time
		liveBoard *model.BoardState
	}
)

func backdate(t *testing.T, repos *repository.Repositories, table string, id uuid.UUID, at time.Time) {
	t.Helper()

	_, err := repos.DB().ExecContext(t.Context(), "UPDATE "+table+" SET created_at = $1 WHERE id = $2", at, id)
	require.NoError(t, err)
}

func addStatsCard(t *testing.T, repos *repository.Repositories, board *model.BoardState, position string) *model.Card {
	t.Helper()

	card, err := repos.Card.Create(t.Context(), spec.NewCard{ID: uuid.New(), BoardID: board.Header.ID, ColumnID: board.Columns[0].ID, Body: "Counted, never read", Position: position})
	require.NoError(t, err)

	return card
}

func newStatsSite(t *testing.T) statsSite {
	t.Helper()

	repos := daotest.NewRepos(t)
	now := time.Now().UTC()

	owner := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	daotest.CreateUser(t, repos)
	oldVerified := daotest.CreateUser(t, repos, daotest.WithVerifiedEmail())
	backdate(t, repos, "users", oldVerified.ID, now.Add(-10*24*time.Hour))

	live := daotest.CreateOrg(t, repos, owner.ID)
	extraTeam := daotest.CreateTeam(t, repos, live.Organisation.ID)
	backdate(t, repos, "teams", extraTeam.ID, now.Add(-12*24*time.Hour))

	liveBoard := daotest.CreateBoard(t, repos, live.Team, owner.ID)
	addStatsCard(t, repos, liveBoard, "a0")
	oldCard := addStatsCard(t, repos, liveBoard, "a1")
	backdate(t, repos, "cards", oldCard.ID, now.Add(-3*24*time.Hour))

	olderBoard := daotest.CreateBoard(t, repos, live.Team, owner.ID)
	backdate(t, repos, "boards", olderBoard.Header.ID, now.Add(-20*24*time.Hour))

	deletedBoard := daotest.CreateBoard(t, repos, live.Team, owner.ID)
	addStatsCard(t, repos, deletedBoard, "a0")
	_, err := repos.DB().ExecContext(t.Context(), "UPDATE boards SET deleted_at = NOW() WHERE id = $1", deletedBoard.Header.ID)
	require.NoError(t, err)

	gone := daotest.CreateOrg(t, repos, owner.ID)
	goneBoard := daotest.CreateBoard(t, repos, gone.Team, owner.ID)
	addStatsCard(t, repos, goneBoard, "a0")
	_, err = repos.Organisation.SoftDelete(t.Context(), gone.Organisation.ID)
	require.NoError(t, err)

	return statsSite{repos: repos, now: now, liveBoard: liveBoard}
}

func TestStatsDAO_CountTotals_CountsLiveRowsAndTheWeeksAdditions(t *testing.T) {
	// given three accounts, one live organisation with two teams, two live boards and two live cards, and a deleted board and a deleted organisation that each hold a card
	site := newStatsSite(t)

	// when
	totals, err := site.repos.Stats.CountTotals(t.Context(), site.now.Add(-7*24*time.Hour))

	// then the deleted board and organisation are gone, and the backdated rows fall outside the week
	require.NoError(t, err)
	assert.Equal(t, &model.SiteTotals{
		Users: 3, UsersAdded: 2,
		VerifiedUsers: 2, VerifiedUsersAdded: 1,
		Organisations: 1, OrganisationsAdded: 1,
		Teams: 2, TeamsAdded: 1,
		Boards: 2, BoardsAdded: 1,
		Cards: 2, CardsAdded: 2,
	}, totals)
}

func TestStatsDAO_CountActivity_CountsEachWindowSeparately(t *testing.T) {
	// given
	site := newStatsSite(t)

	// when
	activity, err := site.repos.Stats.CountActivity(t.Context(), spec.SiteActivityWindows{
		DaySince:   site.now.Add(-24 * time.Hour),
		WeekSince:  site.now.Add(-7 * 24 * time.Hour),
		MonthSince: site.now.Add(-30 * 24 * time.Hour),
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, &model.SiteActivity{
		Day:   model.SiteActivityCounts{Users: 2, Organisations: 1, Boards: 1, Cards: 1},
		Week:  model.SiteActivityCounts{Users: 2, Organisations: 1, Boards: 1, Cards: 2},
		Month: model.SiteActivityCounts{Users: 3, Organisations: 1, Boards: 2, Cards: 2},
	}, activity)
}

func TestStatsDAO_ListDaily_FillsEveryUTCDayIncludingTheEmptyOnes(t *testing.T) {
	// given
	site := newStatsSite(t)
	today := time.Date(site.now.Year(), site.now.Month(), site.now.Day(), 0, 0, 0, 0, time.UTC)
	firstDay := today.AddDate(0, 0, 1-statsSeriesDays)

	// when
	days, err := site.repos.Stats.ListDaily(t.Context(), spec.SiteDailySeries{FirstDay: firstDay, Days: statsSeriesDays})

	// then every day is present once, in order, at UTC midnight
	require.NoError(t, err)
	require.Len(t, days, statsSeriesDays)
	for index, day := range days {
		assert.Equal(t, firstDay.AddDate(0, 0, index), day.Day)
	}

	byDay := make(map[time.Time]model.SiteDay, len(days))
	totalUsers, totalBoards, totalCards := 0, 0, 0
	for _, day := range days {
		byDay[day.Day] = day
		totalUsers += day.Users
		totalBoards += day.Boards
		totalCards += day.Cards
	}

	assert.Equal(t, 3, totalUsers)
	assert.Equal(t, 2, totalBoards)
	assert.Equal(t, 2, totalCards)
	assert.Equal(t, 2, byDay[today].Users+byDay[today.AddDate(0, 0, -1)].Users, "the two accounts made just now fall on today, or on yesterday if the test crossed midnight")

	tenDaysAgo := site.now.Add(-10 * 24 * time.Hour)
	assert.Equal(t, 1, byDay[time.Date(tenDaysAgo.Year(), tenDaysAgo.Month(), tenDaysAgo.Day(), 0, 0, 0, 0, time.UTC)].Users)
}

func TestStatsDAO_ListDaily_AnEmptySiteIsAllZeroes(t *testing.T) {
	// given
	repos := daotest.NewRepos(t)
	firstDay := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)

	// when
	days, err := repos.Stats.ListDaily(t.Context(), spec.SiteDailySeries{FirstDay: firstDay, Days: statsSeriesDays})

	// then
	require.NoError(t, err)
	require.Len(t, days, statsSeriesDays)
	assert.Equal(t, model.SiteDay{Day: firstDay}, days[0])
	assert.Equal(t, model.SiteDay{Day: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}, days[statsSeriesDays-1])
}
