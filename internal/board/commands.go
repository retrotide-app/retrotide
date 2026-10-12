package board

import (
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	CreateCommand struct {
		Principal       model.Principal
		OrgID           uuid.UUID
		TeamID          *uuid.UUID
		Title           string
		TemplateKey     string
		SavedTemplateID *uuid.UUID
		Context         string
		Anonymity       model.Anonymity
		CardsShown      bool
	}

	Command struct {
		Principal model.Principal
		BoardID   uuid.UUID
		OpID      uuid.UUID
	}

	DescribeCommand struct {
		Command

		Context string
	}

	SetLinkAccessCommand struct {
		Command

		LinkAccess model.LinkAccess
	}

	SetPasswordCommand struct {
		Command

		Password *string
	}

	ColumnFields struct {
		Title       string
		Description string
		Colour      model.Colour
	}

	AddColumnCommand struct {
		Command
		ColumnFields

		ColumnID uuid.UUID
	}

	EditColumnCommand struct {
		Command
		ColumnFields

		ColumnID uuid.UUID
	}

	MoveColumnCommand struct {
		Command

		ColumnID      uuid.UUID
		AfterColumnID *uuid.UUID
	}

	ColumnCommand struct {
		Command

		ColumnID uuid.UUID
	}

	AddCardCommand struct {
		Command

		CardID   uuid.UUID
		ColumnID uuid.UUID
		Body     string
		Colour   *model.Colour
		Mentions []uuid.UUID
	}

	EditCardCommand struct {
		Command

		CardID   uuid.UUID
		Body     string
		Mentions []uuid.UUID
	}

	RecolourCardCommand struct {
		Command

		CardID uuid.UUID
		Colour *model.Colour
	}

	MoveCardCommand struct {
		Command

		CardID      uuid.UUID
		ColumnID    uuid.UUID
		AfterCardID *uuid.UUID
	}

	CardCommand struct {
		Command

		CardID uuid.UUID
	}

	MergeCardCommand struct {
		Command

		CardID       uuid.UUID
		TargetCardID uuid.UUID
	}

	AddCommentCommand struct {
		Command

		CardID    uuid.UUID
		CommentID uuid.UUID
		Body      string
		Mentions  []uuid.UUID
	}

	EditCommentCommand struct {
		Command

		CommentID uuid.UUID
		Body      string
		Mentions  []uuid.UUID
	}

	CommentCommand struct {
		Command

		CommentID uuid.UUID
	}

	SettingsCommand struct {
		Command

		Settings spec.BoardSettingsPatch
	}

	TimerCommand struct {
		Command

		Action     model.TimerAction
		DurationMS int
	}

	ParticipantCommand struct {
		Command

		ParticipantID uuid.UUID
	}

	ParticipantRoleCommand struct {
		Command

		ParticipantID uuid.UUID
		Role          model.BoardRole
	}

	StageCommand struct {
		Command

		Stage model.Stage
	}

	ColumnVoteLimitCommand struct {
		Command

		ColumnID uuid.UUID
		MaxVotes *int
	}

	CardSpotlightCommand struct {
		Command

		CardID *uuid.UUID
	}

	ColumnFocusCommand struct {
		Command

		ColumnID *uuid.UUID
	}
)
