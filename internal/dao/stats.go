package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
)

type (
	StatsDAO interface {
		CountTotals(ctx context.Context, since time.Time, tx ...*sql.Tx) (*model.SiteTotals, error)
		CountActivity(ctx context.Context, s spec.SiteActivityWindows, tx ...*sql.Tx) (*model.SiteActivity, error)
		ListDaily(ctx context.Context, s spec.SiteDailySeries, tx ...*sql.Tx) ([]model.SiteDay, error)
	}

	statsDAO struct {
		db *sql.DB
	}
)

func (r *statsDAO) CountTotals(ctx context.Context, since time.Time, tx ...*sql.Tx) (*model.SiteTotals, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountSiteTotals(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("count the site totals since %s: %w", since.Format(time.RFC3339), err)
	}

	return &model.SiteTotals{
		Users:              int(row.Users),
		UsersAdded:         int(row.UsersAdded),
		VerifiedUsers:      int(row.VerifiedUsers),
		VerifiedUsersAdded: int(row.VerifiedUsersAdded),
		Organisations:      int(row.Organisations),
		OrganisationsAdded: int(row.OrganisationsAdded),
		Teams:              int(row.Teams),
		TeamsAdded:         int(row.TeamsAdded),
		Boards:             int(row.Boards),
		BoardsAdded:        int(row.BoardsAdded),
		Cards:              int(row.Cards),
		CardsAdded:         int(row.CardsAdded),
	}, nil
}

func (r *statsDAO) CountActivity(ctx context.Context, s spec.SiteActivityWindows, tx ...*sql.Tx) (*model.SiteActivity, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountSiteActivity(ctx, sqlcgen.CountSiteActivityParams{
		DaySince:   s.DaySince,
		WeekSince:  s.WeekSince,
		MonthSince: s.MonthSince,
	})
	if err != nil {
		return nil, fmt.Errorf("count the site activity: %w", err)
	}

	return &model.SiteActivity{
		Day:   model.SiteActivityCounts{Users: int(row.UsersDay), Organisations: int(row.OrganisationsDay), Boards: int(row.BoardsDay), Cards: int(row.CardsDay)},
		Week:  model.SiteActivityCounts{Users: int(row.UsersWeek), Organisations: int(row.OrganisationsWeek), Boards: int(row.BoardsWeek), Cards: int(row.CardsWeek)},
		Month: model.SiteActivityCounts{Users: int(row.UsersMonth), Organisations: int(row.OrganisationsMonth), Boards: int(row.BoardsMonth), Cards: int(row.CardsMonth)},
	}, nil
}

func (r *statsDAO) ListDaily(ctx context.Context, s spec.SiteDailySeries, tx ...*sql.Tx) ([]model.SiteDay, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListDailySiteActivity(ctx, sqlcgen.ListDailySiteActivityParams{
		FirstDay: s.FirstDay,
		DayCount: clampInt32(s.Days),
	})
	if err != nil {
		return nil, fmt.Errorf("list the daily site activity from %s: %w", s.FirstDay.Format(time.DateOnly), err)
	}

	days := make([]model.SiteDay, 0, len(rows))
	for _, row := range rows {
		day := time.Date(row.Day.Year(), row.Day.Month(), row.Day.Day(), 0, 0, 0, 0, time.UTC)
		days = append(days, model.SiteDay{Day: day, Users: int(row.Users), Boards: int(row.Boards), Cards: int(row.Cards)})
	}

	return days, nil
}
