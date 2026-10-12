package dto

import (
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	ClientIDCarrier interface {
		ClientIDs() []uuid.UUID
	}

	BoardCommandRequest struct {
		OpID uuid.UUID `json:"op_id"`
	}

	CreateBoardRequest struct {
		Title           string          `json:"title"`
		TemplateKey     string          `json:"template_key"`
		SavedTemplateID *uuid.UUID      `json:"saved_template_id"`
		Context         string          `json:"context"`
		Anonymity       model.Anonymity `json:"anonymity"`
		CardsShown      bool            `json:"cards_shown"`
	}

	CreateOrgBoardRequest struct {
		CreateBoardRequest

		TeamID *uuid.UUID `json:"team_id"`
	}

	DescribeBoardRequest struct {
		BoardCommandRequest

		Context string `json:"context"`
	}

	ColumnFieldsRequest struct {
		BoardCommandRequest

		Title       string       `json:"title"`
		Description string       `json:"description"`
		Colour      model.Colour `json:"colour"`
	}

	CreateColumnRequest struct {
		ColumnFieldsRequest

		ID uuid.UUID `json:"id"`
	}

	MoveColumnRequest struct {
		BoardCommandRequest

		AfterColumnID *uuid.UUID `json:"after_column_id"`
	}

	CreateCardRequest struct {
		BoardCommandRequest

		ID       uuid.UUID     `json:"id"`
		ColumnID uuid.UUID     `json:"column_id"`
		Body     string        `json:"body"`
		Colour   *model.Colour `json:"colour"`
		Mentions []uuid.UUID   `json:"mentions"`
	}

	EditCardRequest struct {
		BoardCommandRequest

		Body     string      `json:"body"`
		Mentions []uuid.UUID `json:"mentions"`
	}

	RecolourCardRequest struct {
		BoardCommandRequest

		Colour *model.Colour `json:"colour"`
	}

	MoveCardRequest struct {
		BoardCommandRequest

		ColumnID    uuid.UUID  `json:"column_id"`
		AfterCardID *uuid.UUID `json:"after_card_id"`
	}

	MergeCardRequest struct {
		BoardCommandRequest

		TargetCardID uuid.UUID `json:"target_card_id"`
	}

	CreateCommentRequest struct {
		BoardCommandRequest

		ID       uuid.UUID   `json:"id"`
		Body     string      `json:"body"`
		Mentions []uuid.UUID `json:"mentions"`
	}

	EditCommentRequest struct {
		BoardCommandRequest

		Body     string      `json:"body"`
		Mentions []uuid.UUID `json:"mentions"`
	}

	BoardSettingsRequest struct {
		BoardCommandRequest

		VotingOpen       *bool           `json:"voting_open"`
		VoteCountsHidden *bool           `json:"vote_counts_hidden"`
		MaxVotes         *int            `json:"max_votes"`
		MaxVotesPerCard  *int            `json:"max_votes_per_card"`
		CommentsEnabled  *bool           `json:"comments_enabled"`
		MergingEnabled   *bool           `json:"merging_enabled"`
		SortMode         *model.SortMode `json:"sort_mode"`
		CardsLocked      *bool           `json:"cards_locked"`
		MovesLocked      *bool           `json:"moves_locked"`
		BoardLocked      *bool           `json:"board_locked"`
		TimerPlaySound   *bool           `json:"timer_play_sound"`
		TimerAutoLock    *bool           `json:"timer_auto_lock"`
		SortEnforced     *bool           `json:"sort_enforced"`
		AnyoneCanEdit    *bool           `json:"anyone_can_edit"`
		ShowCardDates    *bool           `json:"show_card_dates"`
	}

	CardSpotlightRequest struct {
		BoardCommandRequest

		CardID *uuid.UUID `json:"card_id"`
	}

	ColumnFocusRequest struct {
		BoardCommandRequest

		ColumnID *uuid.UUID `json:"column_id"`
	}

	ColumnVoteLimitRequest struct {
		BoardCommandRequest

		MaxVotes *int `json:"max_votes"`
	}

	TimerRequest struct {
		BoardCommandRequest

		Action     model.TimerAction `json:"action"`
		DurationMS int               `json:"duration_ms"`
	}

	ParticipantRoleRequest struct {
		BoardCommandRequest

		Role model.BoardRole `json:"role"`
	}

	StageRequest struct {
		BoardCommandRequest

		Stage model.Stage `json:"stage"`
	}

	SetLinkAccessRequest struct {
		BoardCommandRequest

		LinkAccess model.LinkAccess `json:"link_access"`
	}

	SetBoardPasswordRequest struct {
		BoardCommandRequest

		Password *string `json:"password"`
	}

	JoinBoardRequest struct {
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}

	JoinPreview struct {
		RequiresPassword bool `json:"requires_password"`
		RequiresAccount  bool `json:"requires_account"`
		CanJoin          bool `json:"can_join"`
	}

	LinkJoinResponse struct {
		BoardID uuid.UUID  `json:"board_id"`
		OrgID   *uuid.UUID `json:"org_id,omitempty"`
	}

	SharingView struct {
		ShareSlug   string           `json:"share_slug"`
		LinkAccess  model.LinkAccess `json:"link_access"`
		HasPassword bool             `json:"has_password"`
	}

	GuestClaimRequest struct {
		BoardIDs []uuid.UUID `json:"board_ids"`
	}

	GuestClaimOffer struct {
		Boards []GuestClaimBoard `json:"boards"`
	}

	GuestClaimBoard struct {
		BoardID     uuid.UUID `json:"board_id"`
		Title       string    `json:"title"`
		DisplayName string    `json:"display_name"`
	}

	GuestClaimResult struct {
		Claimed []uuid.UUID `json:"claimed"`
	}

	BoardCommandResponse struct {
		Version        int64               `json:"version"`
		Replayed       bool                `json:"replayed,omitzero"`
		ID             *uuid.UUID          `json:"id,omitempty"`
		Card           *CardView           `json:"card,omitempty"`
		Column         *ColumnView         `json:"column,omitempty"`
		Comment        *CommentView        `json:"comment,omitempty"`
		Me             *MeView             `json:"me,omitempty"`
		Sharing        *SharingView        `json:"sharing,omitempty"`
		ActionItem     *ActionItemView     `json:"action_item,omitempty"`
		PreviousAction *PreviousActionView `json:"previous_action,omitempty"`
		Reactions      *CardReactionsView  `json:"reactions,omitempty"`
		Survey         *SurveyView         `json:"survey,omitempty"`
	}

	BoardSnapshot struct {
		BoardID         uuid.UUID            `json:"board_id"`
		Version         int64                `json:"version"`
		Board           BoardView            `json:"board"`
		Settings        SettingsView         `json:"settings"`
		Viewer          ViewerView           `json:"viewer"`
		Me              MeView               `json:"me"`
		Columns         []ColumnView         `json:"columns"`
		Cards           []CardView           `json:"cards"`
		Comments        []CommentView        `json:"comments"`
		HiddenCounts    map[uuid.UUID]int    `json:"hidden_counts"`
		Participants    []ParticipantView    `json:"participants"`
		Timer           *TimerView           `json:"timer"`
		ActionItems     []ActionItemView     `json:"action_items"`
		PreviousActions []PreviousActionView `json:"previous_actions"`
		Spotlight       SpotlightView        `json:"spotlight"`
		Surveys         []SurveyView         `json:"surveys"`
		ServerNow       string               `json:"server_now,omitempty"`
	}

	SpotlightView struct {
		PresentingCardID  *uuid.UUID `json:"presenting_card_id"`
		HighlightedCardID *uuid.UUID `json:"highlighted_card_id"`
		FocusedColumnID   *uuid.UUID `json:"focused_column_id"`
	}

	SettingsView struct {
		VotingOpen       bool           `json:"voting_open"`
		VoteCountsHidden bool           `json:"vote_counts_hidden"`
		MaxVotes         int            `json:"max_votes"`
		MaxVotesPerCard  int            `json:"max_votes_per_card"`
		CommentsEnabled  bool           `json:"comments_enabled"`
		MergingEnabled   bool           `json:"merging_enabled"`
		SortMode         model.SortMode `json:"sort_mode"`
		CardsLocked      bool           `json:"cards_locked"`
		MovesLocked      bool           `json:"moves_locked"`
		BoardLocked      bool           `json:"board_locked"`
		TimerPlaySound   bool           `json:"timer_play_sound"`
		TimerAutoLock    bool           `json:"timer_auto_lock"`
		SortEnforced     bool           `json:"sort_enforced"`
		AnyoneCanEdit    bool           `json:"anyone_can_edit"`
		ShowCardDates    bool           `json:"show_card_dates"`
	}

	TimerView struct {
		DurationMS        int     `json:"duration_ms"`
		EndsAt            *string `json:"ends_at"`
		PausedRemainingMS *int    `json:"paused_remaining_ms"`
	}

	MeView struct {
		VotesRemaining int               `json:"votes_remaining"`
		MyVotes        map[uuid.UUID]int `json:"my_votes"`
	}

	VotesView struct {
		Total *int `json:"total"`
	}

	CommentView struct {
		ID        uuid.UUID    `json:"id"`
		CardID    uuid.UUID    `json:"card_id"`
		Body      string       `json:"body"`
		Author    *AuthorView  `json:"author"`
		IsMine    bool         `json:"is_mine,omitzero"`
		Mentions  []PersonView `json:"mentions,omitempty"`
		CreatedAt string       `json:"created_at,omitempty"`
		UpdatedAt string       `json:"updated_at,omitempty"`
	}

	BoardView struct {
		ID                 uuid.UUID       `json:"id"`
		TeamID             *uuid.UUID      `json:"team_id,omitempty"`
		Title              string          `json:"title"`
		Context            string          `json:"context"`
		TemplateKey        string          `json:"template_key"`
		Anonymity          model.Anonymity `json:"anonymity"`
		Stage              model.Stage     `json:"stage"`
		CardsHidden        bool            `json:"cards_hidden"`
		Closed             bool            `json:"closed"`
		ShowPrimeDirective bool            `json:"show_prime_directive"`
		CreatorName        string          `json:"creator_name,omitempty"`
		CreatedAt          string          `json:"created_at,omitempty"`
		ArchivedAt         string          `json:"archived_at,omitempty"`
		BackgroundPath     string          `json:"background_path,omitempty"`
	}

	ViewerView struct {
		ParticipantID *uuid.UUID              `json:"participant_id"`
		Role          model.BoardRole         `json:"role"`
		Capabilities  []model.BoardCapability `json:"capabilities"`
	}

	ColumnView struct {
		ID            uuid.UUID    `json:"id"`
		Title         string       `json:"title"`
		Description   string       `json:"description"`
		Colour        model.Colour `json:"colour"`
		Position      string       `json:"position"`
		MaxVotes      *int         `json:"max_votes"`
		CardsRevealed bool         `json:"cards_revealed"`
		Hidden        bool         `json:"hidden"`
	}

	CardView struct {
		ID           uuid.UUID           `json:"id"`
		ColumnID     uuid.UUID           `json:"column_id"`
		ParentCardID *uuid.UUID          `json:"parent_card_id"`
		Position     string              `json:"position"`
		Body         string              `json:"body"`
		Colour       *model.Colour       `json:"colour"`
		Author       *AuthorView         `json:"author"`
		IsMine       bool                `json:"is_mine,omitzero"`
		Hidden       bool                `json:"hidden"`
		Votes        VotesView           `json:"votes"`
		Mentions     []PersonView        `json:"mentions,omitempty"`
		CreatedAt    string              `json:"created_at,omitempty"`
		UpdatedAt    string              `json:"updated_at,omitempty"`
		Pinned       bool                `json:"pinned,omitzero"`
		ImagePath    string              `json:"image_path,omitempty"`
		Reactions    []ReactionCountView `json:"reactions,omitempty"`
		MyReactions  []model.Reaction    `json:"my_reactions,omitempty"`
	}

	ReactionCountView struct {
		Reaction model.Reaction `json:"reaction"`
		Count    int            `json:"count"`
	}

	CardReactionsView struct {
		CardID      uuid.UUID           `json:"card_id"`
		Reactions   []ReactionCountView `json:"reactions"`
		MyReactions []model.Reaction    `json:"my_reactions"`
	}

	AuthorView struct {
		ParticipantID uuid.UUID `json:"participant_id"`
		DisplayName   string    `json:"display_name"`
	}

	ParticipantView struct {
		ParticipantID uuid.UUID       `json:"participant_id"`
		DisplayName   string          `json:"display_name"`
		Role          model.BoardRole `json:"role"`
	}

	BoardListItem struct {
		ID          uuid.UUID  `json:"id"`
		TeamID      *uuid.UUID `json:"team_id,omitempty"`
		Title       string     `json:"title"`
		CreatorName string     `json:"creator_name,omitempty"`
		CreatedAt   string     `json:"created_at,omitempty"`
		ArchivedAt  string     `json:"archived_at,omitempty"`
		Ended       bool       `json:"ended,omitempty"`
	}

	BoardListResponse struct {
		Boards []BoardListItem `json:"boards"`
	}

	OrgBoardListItem struct {
		BoardListItem

		TeamName    string `json:"team_name,omitempty"`
		TemplateKey string `json:"template_key"`
		Pinned      bool   `json:"pinned,omitzero"`
	}

	OrgBoardListResponse struct {
		Boards []OrgBoardListItem `json:"boards"`
	}
)

func (r CreateColumnRequest) ClientIDs() []uuid.UUID {
	return []uuid.UUID{r.ID}
}

func (r CreateCardRequest) ClientIDs() []uuid.UUID {
	return []uuid.UUID{r.ID}
}

func (r CreateCommentRequest) ClientIDs() []uuid.UUID {
	return []uuid.UUID{r.ID}
}
