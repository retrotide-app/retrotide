package spec

import "time"

type (
	SiteActivityWindows struct {
		DaySince   time.Time
		WeekSince  time.Time
		MonthSince time.Time
	}

	SiteDailySeries struct {
		FirstDay time.Time
		Days     int
	}
)
