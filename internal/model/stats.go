package model

import "time"

type (
	SiteTotals struct {
		Users              int
		UsersAdded         int
		VerifiedUsers      int
		VerifiedUsersAdded int
		Organisations      int
		OrganisationsAdded int
		Teams              int
		TeamsAdded         int
		Boards             int
		BoardsAdded        int
		Cards              int
		CardsAdded         int
	}

	SiteActivityCounts struct {
		Users         int
		Organisations int
		Boards        int
		Cards         int
	}

	SiteActivity struct {
		Day   SiteActivityCounts
		Week  SiteActivityCounts
		Month SiteActivityCounts
	}

	SiteDay struct {
		Day    time.Time
		Users  int
		Boards int
		Cards  int
	}
)
