package dto

import (
	"time"

	"retrotide/internal/model"
)

type (
	SelfExport struct {
		Account       SelfExportAccount      `json:"account"`
		Identities    []SelfExportIdentity   `json:"identities"`
		Organisations []SelfExportMembership `json:"organisations"`
		Cards         []SelfExportCard       `json:"cards"`
		Comments      []SelfExportComment    `json:"comments"`
		ActionItems   []SelfExportActionItem `json:"action_items"`
	}

	SelfExportAccount struct {
		DisplayName   string `json:"display_name"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		CreatedAt     string `json:"created_at"`
	}

	SelfExportIdentity struct {
		Provider string `json:"provider"`
		Subject  string `json:"subject"`
		Email    string `json:"email"`
		LinkedAt string `json:"linked_at"`
	}

	SelfExportMembership struct {
		Organisation string           `json:"organisation"`
		Role         model.OrgRole    `json:"role"`
		JoinedAt     string           `json:"joined_at"`
		Teams        []SelfExportTeam `json:"teams"`
	}

	SelfExportTeam struct {
		Name     string         `json:"name"`
		Role     model.TeamRole `json:"role"`
		JoinedAt string         `json:"joined_at"`
	}

	SelfExportCard struct {
		Organisation string `json:"organisation"`
		Board        string `json:"board"`
		Column       string `json:"column"`
		Body         string `json:"body"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at,omitempty"`
	}

	SelfExportComment struct {
		Organisation string `json:"organisation"`
		Board        string `json:"board"`
		Body         string `json:"body"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at,omitempty"`
	}

	SelfExportActionItem struct {
		Organisation  string                 `json:"organisation"`
		Team          string                 `json:"team,omitempty"`
		Board         string                 `json:"board,omitempty"`
		Title         string                 `json:"title"`
		Status        model.ActionItemStatus `json:"status"`
		DueOn         *string                `json:"due_on"`
		RaisedByYou   bool                   `json:"raised_by_you"`
		AssignedToYou bool                   `json:"assigned_to_you"`
	}

	OrganisationExportHeader struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}

	OrganisationExportBoard struct {
		Team     string      `json:"team,omitempty"`
		Archived bool        `json:"archived"`
		Board    BoardExport `json:"board"`
	}
)

func NewSelfExportAccount(user model.User) SelfExportAccount {
	return SelfExportAccount{
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func NewSelfExportIdentities(identities []model.UserIdentity) []SelfExportIdentity {
	views := make([]SelfExportIdentity, 0, len(identities))
	for _, identity := range identities {
		views = append(views, SelfExportIdentity{
			Provider: string(identity.Provider),
			Subject:  identity.Subject,
			Email:    identity.Email,
			LinkedAt: identity.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	return views
}

func NewSelfExportMembership(organisation model.UserOrganisation, teams []model.MemberTeam) SelfExportMembership {
	views := make([]SelfExportTeam, 0, len(teams))
	for _, team := range teams {
		views = append(views, SelfExportTeam{Name: team.Name, Role: team.Role, JoinedAt: team.JoinedAt.UTC().Format(time.RFC3339)})
	}

	return SelfExportMembership{
		Organisation: organisation.Name,
		Role:         organisation.Role,
		JoinedAt:     organisation.JoinedAt.UTC().Format(time.RFC3339),
		Teams:        views,
	}
}

func NewSelfExportCards(cards []model.WrittenCard) []SelfExportCard {
	views := make([]SelfExportCard, 0, len(cards))
	for _, card := range cards {
		views = append(views, SelfExportCard{
			Organisation: card.OrgName,
			Board:        card.BoardTitle,
			Column:       card.ColumnTitle,
			Body:         card.Body,
			CreatedAt:    card.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:    optionalTime(card.UpdatedAt),
		})
	}

	return views
}

func NewSelfExportComments(comments []model.WrittenComment) []SelfExportComment {
	views := make([]SelfExportComment, 0, len(comments))
	for _, comment := range comments {
		views = append(views, SelfExportComment{
			Organisation: comment.OrgName,
			Board:        comment.BoardTitle,
			Body:         comment.Body,
			CreatedAt:    comment.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:    optionalTime(comment.UpdatedAt),
		})
	}

	return views
}

func NewSelfExportActionItems(items []model.UserActionItem) []SelfExportActionItem {
	views := make([]SelfExportActionItem, 0, len(items))
	for _, item := range items {
		view := SelfExportActionItem{
			Organisation:  item.OrgName,
			Team:          item.TeamName,
			Board:         item.BoardTitle,
			Title:         item.Title,
			Status:        item.Status,
			RaisedByYou:   item.RaisedByUser,
			AssignedToYou: item.AssignedToUser,
		}

		if item.DueOn != nil {
			view.DueOn = new(item.DueOn.Format(model.DueDateLayout))
		}

		views = append(views, view)
	}

	return views
}
