package utils

import (
	"fmt"
	"strings"

	"retrotide/internal/dto"
)

const (
	HiddenMark        = " (hidden: only you can see this)"
	OtherActionsTitle = "Other actions"
	MergedPrefix      = "Merged: "
	ActionPrefix      = "Action: "
)

func VotesText(votes *int) string {
	if votes == nil {
		return ""
	}

	if *votes == 1 {
		return " (1 vote)"
	}

	return fmt.Sprintf(" (%d votes)", *votes)
}

func HiddenText(count int) string {
	if count == 1 {
		return "1 hidden card"
	}

	return fmt.Sprintf("%d hidden cards", count)
}

func ActionDetails(item dto.ExportActionItem, assignee string) string {
	details := []string{string(item.Status)}

	if assignee != "" {
		details = append(details, "assigned to "+assignee)
	}

	if item.DueOn != nil {
		details = append(details, "due "+*item.DueOn)
	}

	return "(" + strings.Join(details, ", ") + ")"
}
