package dto

import (
	"encoding/json"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	BoardEventsFrame struct {
		BoardID uuid.UUID         `json:"board_id"`
		From    int64             `json:"from"`
		To      int64             `json:"to"`
		Events  []json.RawMessage `json:"events"`
	}

	BoardEventHead struct {
		Type    string `json:"type"`
		Version int64  `json:"version"`
	}

	BoardUpdatedEvent struct {
		BoardEventHead

		Title   string `json:"title"`
		Context string `json:"context"`
	}

	ParticipantUpsertedEvent struct {
		BoardEventHead

		Participant ParticipantView `json:"participant"`
	}

	ColumnUpsertedEvent struct {
		BoardEventHead

		Column ColumnView `json:"column"`
	}

	ColumnMovedEvent struct {
		BoardEventHead

		ColumnID  uuid.UUID      `json:"column_id"`
		Position  string         `json:"position"`
		Rewritten []PositionView `json:"rewritten"`
	}

	ColumnRemovedEvent struct {
		BoardEventHead

		ColumnID uuid.UUID `json:"column_id"`
	}

	CardUpsertedEvent struct {
		BoardEventHead

		Card CardView `json:"card"`
	}

	CardMovedEvent struct {
		BoardEventHead

		CardID    uuid.UUID      `json:"card_id"`
		ColumnID  uuid.UUID      `json:"column_id"`
		Position  string         `json:"position"`
		Rewritten []PositionView `json:"rewritten"`
	}

	CardDeletedEvent struct {
		BoardEventHead

		CardID uuid.UUID `json:"card_id"`
	}

	HiddenCountsEvent struct {
		BoardEventHead

		HiddenCounts map[uuid.UUID]int `json:"hidden_counts"`
	}

	SettingsChangedEvent struct {
		BoardEventHead

		Settings SettingsView `json:"settings"`
	}

	VotesChangedEvent struct {
		BoardEventHead

		CardID uuid.UUID `json:"card_id"`
		Total  *int      `json:"total"`
	}

	CommentUpsertedEvent struct {
		BoardEventHead

		Comment CommentView `json:"comment"`
	}

	CommentDeletedEvent struct {
		BoardEventHead

		CommentID uuid.UUID `json:"comment_id"`
		CardID    uuid.UUID `json:"card_id"`
	}

	CardsMergedEvent struct {
		BoardEventHead

		CardID       uuid.UUID   `json:"card_id"`
		ParentCardID uuid.UUID   `json:"parent_card_id"`
		ColumnID     uuid.UUID   `json:"column_id"`
		Position     string      `json:"position"`
		Restacked    []uuid.UUID `json:"restacked"`
	}

	CardUnmergedEvent struct {
		BoardEventHead

		CardID    uuid.UUID      `json:"card_id"`
		ColumnID  uuid.UUID      `json:"column_id"`
		Position  string         `json:"position"`
		Rewritten []PositionView `json:"rewritten"`
	}

	CardsRevealedEvent struct {
		BoardEventHead

		Stage    model.Stage   `json:"stage"`
		Cards    []CardView    `json:"cards"`
		Comments []CommentView `json:"comments"`
	}

	StageChangedEvent struct {
		BoardEventHead

		Stage model.Stage `json:"stage"`
	}

	CardsHiddenEvent struct {
		BoardEventHead

		HiddenCounts map[uuid.UUID]int `json:"hidden_counts"`
	}

	ParticipantRemovedEvent struct {
		BoardEventHead

		ParticipantID uuid.UUID `json:"participant_id"`
	}

	TimerChangedEvent struct {
		BoardEventHead

		Timer     *TimerView `json:"timer"`
		ServerNow string     `json:"server_now"`
	}

	ColumnRevealedEvent struct {
		BoardEventHead

		ColumnID uuid.UUID     `json:"column_id"`
		Cards    []CardView    `json:"cards"`
		Comments []CommentView `json:"comments"`
	}

	ColumnHiddenEvent struct {
		BoardEventHead

		ColumnID uuid.UUID `json:"column_id"`
	}

	ColumnVotesResetEvent struct {
		BoardEventHead

		ColumnID uuid.UUID `json:"column_id"`
	}

	SpotlightChangedEvent struct {
		BoardEventHead

		Spotlight SpotlightView `json:"spotlight"`
	}

	CardReactionsChangedEvent struct {
		BoardEventHead
		CardReactionsView
	}

	CardPinnedEvent struct {
		BoardEventHead

		CardID uuid.UUID `json:"card_id"`
		Pinned bool      `json:"pinned"`
	}

	CardImageChangedEvent struct {
		BoardEventHead

		CardID    uuid.UUID `json:"card_id"`
		ImagePath *string   `json:"image_path"`
	}

	BoardBackgroundChangedEvent struct {
		BoardEventHead

		BackgroundPath *string `json:"background_path"`
	}

	SurveyUpsertedEvent struct {
		BoardEventHead

		Survey SurveyView `json:"survey"`
	}

	SurveyRemovedEvent struct {
		BoardEventHead

		SurveyID uuid.UUID `json:"survey_id"`
	}

	BoardMeFrame struct {
		BoardID        uuid.UUID         `json:"board_id"`
		Version        int64             `json:"version"`
		VotesRemaining int               `json:"votes_remaining"`
		MyVotes        map[uuid.UUID]int `json:"my_votes"`
	}

	PositionView struct {
		ID       uuid.UUID `json:"id"`
		Position string    `json:"position"`
	}

	BoardPresenceFrame struct {
		BoardID uuid.UUID      `json:"board_id"`
		Viewers []PresenceView `json:"viewers"`
	}

	PresenceView struct {
		ParticipantID uuid.UUID `json:"participant_id"`
		DisplayName   string    `json:"display_name"`
		State         string    `json:"state,omitempty"`
	}
)
