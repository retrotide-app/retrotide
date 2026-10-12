package model

import (
	"time"
)

type (
	WrittenCard struct {
		OrgName     string
		BoardTitle  string
		ColumnTitle string
		Body        string
		CreatedAt   time.Time
		UpdatedAt   *time.Time
	}

	WrittenComment struct {
		OrgName    string
		BoardTitle string
		Body       string
		CreatedAt  time.Time
		UpdatedAt  *time.Time
	}

	UserActionItem struct {
		OrgName        string
		TeamName       string
		BoardTitle     string
		Title          string
		Status         ActionItemStatus
		DueOn          *time.Time
		RaisedByUser   bool
		AssignedToUser bool
	}
)
