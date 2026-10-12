package spec

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewBoard struct {
		OrgID       uuid.UUID
		TeamID      *uuid.UUID
		Title       string
		ShareSlug   string
		CreatedBy   uuid.UUID
		Context     string
		TemplateKey string
		Anonymity   model.Anonymity
		CardsHidden bool
		Columns     []BoardColumnSeed
	}

	NewBoardRecord struct {
		OrgID       uuid.UUID
		TeamID      *uuid.UUID
		Title       string
		ShareSlug   string
		LinkAccess  model.LinkAccess
		CreatedBy   *uuid.UUID
		Context     string
		TemplateKey string
		Anonymity   model.Anonymity
		CardsHidden bool
	}

	BoardKey struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
	}

	BoardWrite struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		Version int64
		OpID    uuid.UUID
	}

	NewBoardOp struct {
		BoardID uuid.UUID
		OpID    uuid.UUID
		Version int64
	}

	BoardOpKey struct {
		BoardID uuid.UUID
		OpID    uuid.UUID
	}

	NewBoardParticipant struct {
		BoardID       uuid.UUID
		UserID        *uuid.UUID
		GuestDeviceID *uuid.UUID
		DisplayName   string
		Role          model.BoardRole
		AccessEpoch   int
		JoinedVia     model.JoinPath
	}

	BoardParticipantUser struct {
		BoardID uuid.UUID
		UserID  uuid.UUID
	}

	BoardParticipantGuest struct {
		BoardID       uuid.UUID
		GuestDeviceID uuid.UUID
	}

	BoardSeatCount struct {
		BoardID     uuid.UUID
		AccessEpoch int
	}

	BoardParticipantKey struct {
		BoardID       uuid.UUID
		ParticipantID uuid.UUID
	}

	LinkParticipant struct {
		BoardID       uuid.UUID
		UserID        *uuid.UUID
		GuestDeviceID *uuid.UUID
		DisplayName   string
		AccessEpoch   int
	}

	LinkParticipantJoin struct {
		BoardWrite

		UserID        *uuid.UUID
		GuestDeviceID *uuid.UUID
		DisplayName   string
		AccessEpoch   int
	}

	ParticipantReassignment struct {
		BoardID       uuid.UUID
		ParticipantID uuid.UUID
		UserID        uuid.UUID
	}

	GuestClaimWrite struct {
		BoardWrite

		GuestParticipantID uuid.UUID
		UserID             uuid.UUID
		UserParticipantID  *uuid.UUID
	}

	BoardLinkAccessChange struct {
		OrgID      uuid.UUID
		BoardID    uuid.UUID
		LinkAccess model.LinkAccess
	}

	BoardLinkAccessWrite struct {
		BoardWrite

		LinkAccess model.LinkAccess
	}

	BoardPasswordChange struct {
		OrgID        uuid.UUID
		BoardID      uuid.UUID
		PasswordHash *string
	}

	BoardPasswordWrite struct {
		BoardWrite

		PasswordHash *string
	}

	BoardLinkRotation struct {
		OrgID     uuid.UUID
		BoardID   uuid.UUID
		ShareSlug string
	}

	BoardLinkRotationWrite struct {
		BoardWrite

		ShareSlug string
	}

	BoardParticipantJoin struct {
		BoardWrite

		UserID      uuid.UUID
		DisplayName string
		Role        model.BoardRole
	}

	BoardAccessQuery struct {
		Principal model.Principal
		BoardID   uuid.UUID
	}

	BoardContextChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		Context string
	}

	BoardContextWrite struct {
		BoardWrite

		Context string
	}

	BoardSnapshotQuery struct {
		Principal    model.Principal
		BoardID      uuid.UUID
		KnownVersion *int64
	}

	PositionChange struct {
		ID       uuid.UUID
		Position string
	}

	BoardSettingsPatch struct {
		VotingOpen       *bool
		VoteCountsHidden *bool
		MaxVotes         *int
		MaxVotesPerCard  *int
		CommentsEnabled  *bool
		MergingEnabled   *bool
		SortMode         *model.SortMode
		CardsLocked      *bool
		MovesLocked      *bool
		BoardLocked      *bool
		TimerPlaySound   *bool
		TimerAutoLock    *bool
		SortEnforced     *bool
		AnyoneCanEdit    *bool
		ShowCardDates    *bool
	}

	BoardCardsHiddenChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		Hidden  bool
	}

	CardsRevealWrite struct {
		BoardWrite

		Positions []PositionChange
	}

	BoardCloseWrite struct {
		BoardWrite

		Positions []PositionChange
		Sever     bool
	}

	BoardStageChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		From    model.Stage
		To      model.Stage
	}

	BoardStageWrite struct {
		BoardWrite

		From      model.Stage
		To        model.Stage
		Positions []PositionChange
		Reveal    bool
	}

	BoardTimerChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		Timer   model.BoardTimer
	}

	BoardTimerWrite struct {
		BoardWrite

		Timer model.BoardTimer
	}

	IdleBoardsQuery struct {
		IdleSince time.Time
		Limit     int
	}

	ParticipantRoleChange struct {
		BoardID       uuid.UUID
		ParticipantID uuid.UUID
		Role          model.BoardRole
	}

	ParticipantRoleWrite struct {
		BoardWrite

		ParticipantID uuid.UUID
		Role          model.BoardRole
	}

	ParticipantRemovalWrite struct {
		BoardWrite

		ParticipantID uuid.UUID
	}

	BoardSettingsChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		Patch   BoardSettingsPatch
	}

	BoardSettingsWrite struct {
		BoardWrite

		Patch BoardSettingsPatch
	}

	BoardFacilitationChange struct {
		OrgID             uuid.UUID
		BoardID           uuid.UUID
		RevealedColumnIDs []uuid.UUID
		HiddenColumnIDs   []uuid.UUID
		Spotlight         model.BoardSpotlight
	}

	BoardFacilitationWrite struct {
		BoardWrite

		RevealedColumnIDs []uuid.UUID
		HiddenColumnIDs   []uuid.UUID
		Spotlight         model.BoardSpotlight
		RevealedColumnID  *uuid.UUID
		Positions         []PositionChange
	}

	BoardTimerLock struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		EndsAt  time.Time
	}

	BoardTimerLockWrite struct {
		BoardWrite

		EndsAt time.Time
	}

	BoardBackgroundChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		MediaID *uuid.UUID
	}

	BoardBackgroundWrite struct {
		BoardWrite

		MediaID *uuid.UUID
	}

	BoardArchiveChange struct {
		OrgID    uuid.UUID
		BoardID  uuid.UUID
		Archived bool
	}

	BoardArchiveWrite struct {
		BoardWrite

		Archived bool
	}

	BoardRestore struct {
		OrgID        uuid.UUID
		BoardID      uuid.UUID
		DeletedSince time.Time
	}

	DeletedBoardsQuery struct {
		OrgID        uuid.UUID
		DeletedSince time.Time
	}

	BoardTeamChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		TeamID  *uuid.UUID
	}

	BoardTeamWrite struct {
		BoardWrite

		TeamID *uuid.UUID
	}

	ActionItemTeamChange struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		TeamID  *uuid.UUID
	}

	BoardPinKey struct {
		OrgID   uuid.UUID
		BoardID uuid.UUID
		UserID  uuid.UUID
	}

	ParticipantHandover struct {
		BoardID           uuid.UUID
		FromParticipantID uuid.UUID
		ToParticipantID   uuid.UUID
	}
)
