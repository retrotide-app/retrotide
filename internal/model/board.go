package model

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type (
	LinkAccess string

	BoardRole string

	JoinPath string

	Anonymity string

	SortMode string

	Colour string

	BoardCapability string

	TimerAction string

	Stage string

	Reaction string

	BoardTimer struct {
		DurationMS        *int
		EndsAt            *time.Time
		PausedRemainingMS *int
	}

	BoardRef struct {
		ID    uuid.UUID
		OrgID uuid.UUID
	}

	BoardSpotlight struct {
		PresentingCardID  *uuid.UUID
		HighlightedCardID *uuid.UUID
		FocusedColumnID   *uuid.UUID
	}

	TimerDue struct {
		BoardID uuid.UUID
		OrgID   uuid.UUID
		EndsAt  time.Time
	}

	Board struct {
		ID                     uuid.UUID
		OrgID                  uuid.UUID
		TeamID                 *uuid.UUID
		Title                  string
		ShareSlug              string
		LinkAccess             LinkAccess
		PasswordHash           *string
		AccessEpoch            int
		Version                int64
		CreatedBy              *uuid.UUID
		CreatedAt              time.Time
		UpdatedAt              time.Time
		LockedAt               *time.Time
		ClosedAt               *time.Time
		ArchivedAt             *time.Time
		DeletedAt              *time.Time
		Context                string
		TemplateKey            string
		Anonymity              Anonymity
		Stage                  Stage
		CardsHidden            bool
		CardsLocked            bool
		MovesLocked            bool
		VotingOpen             bool
		VoteCountsHidden       bool
		MaxVotes               int
		MaxVotesPerCard        int
		CommentsEnabled        bool
		MergingEnabled         bool
		GuestNamesEnabled      bool
		ShowPrimeDirective     bool
		SortMode               SortMode
		TimerDurationMS        *int
		TimerEndsAt            *time.Time
		TimerPausedRemainingMS *int
		TimerPlaySound         bool
		RevealedColumnIDs      []uuid.UUID
		HiddenColumnIDs        []uuid.UUID
		Spotlight              BoardSpotlight
		TimerAutoLock          bool
		SortEnforced           bool
		AnyoneCanEdit          bool
		ShowCardDates          bool
		BackgroundMediaID      *uuid.UUID
	}

	BoardHeader struct {
		Board

		OrgDeletedAt    *time.Time
		OrgAllowsGuests bool
		CreatorName     string
	}

	BoardParticipant struct {
		ID            uuid.UUID
		BoardID       uuid.UUID
		UserID        *uuid.UUID
		GuestDeviceID *uuid.UUID
		DisplayName   string
		Role          BoardRole
		AccessEpoch   int
		JoinedVia     JoinPath
		RemovedAt     *time.Time
		JoinedAt      time.Time
	}

	BoardParticipantProfile struct {
		BoardParticipant

		AccountName string
	}

	BoardOp struct {
		BoardID   uuid.UUID
		OpID      uuid.UUID
		Version   int64
		CreatedAt time.Time
	}

	Versioned[T any] struct {
		Value   T
		Version int64
	}

	BoardColumn struct {
		ID          uuid.UUID
		BoardID     uuid.UUID
		Title       string
		Description string
		Colour      Colour
		Position    string
		CreatedAt   time.Time
		MaxVotes    *int
	}

	Card struct {
		ID                  uuid.UUID
		BoardID             uuid.UUID
		ColumnID            uuid.UUID
		ParentCardID        *uuid.UUID
		AuthorParticipantID *uuid.UUID
		Body                string
		Colour              *Colour
		Position            string
		VoteTotal           int
		CreatedAt           time.Time
		UpdatedAt           *time.Time
		Mentions            []ActionItemPerson
		PinnedAt            *time.Time
		ImageMediaID        *uuid.UUID
	}

	CardReaction struct {
		BoardID       uuid.UUID
		CardID        uuid.UUID
		ParticipantID *uuid.UUID
		Reaction      Reaction
	}

	CardImport struct {
		Columns []BoardColumn
		Cards   []Card
	}

	BoardState struct {
		Header          BoardHeader
		Columns         []BoardColumn
		Cards           []Card
		Participants    []BoardParticipantProfile
		Votes           []CardVote
		Comments        []CardComment
		ActionItems     []ActionItem
		PreviousActions []PreviousAction
		Reactions       []CardReaction
		Surveys         []Survey
		SurveyResponses []SurveyResponse
	}

	BoardAccess struct {
		Principal     Principal
		Header        BoardHeader
		ParticipantID *uuid.UUID
		Role          BoardRole
		OrgMember     bool
		Capabilities  []BoardCapability
	}
)

const (
	LinkAccessOff      LinkAccess = "off"
	LinkAccessGuests   LinkAccess = "guests"
	LinkAccessAccounts LinkAccess = "accounts"
)

const (
	BoardRoleFacilitator BoardRole = "facilitator"
	BoardRoleParticipant BoardRole = "participant"
)

const (
	JoinPathMembership JoinPath = "membership"
	JoinPathLink       JoinPath = "link"
)

const (
	AnonymityNamed     Anonymity = "named"
	AnonymityAnonymous Anonymity = "anonymous"
)

const (
	SortModeManual SortMode = "manual"
	SortModeVotes  SortMode = "votes"
)

const (
	ColourGreen  Colour = "green"
	ColourRed    Colour = "red"
	ColourBlue   Colour = "blue"
	ColourAmber  Colour = "amber"
	ColourPurple Colour = "purple"
	ColourTeal   Colour = "teal"
	ColourPink   Colour = "pink"
	ColourGrey   Colour = "grey"
)

const (
	BoardCapabilityView        BoardCapability = "board.view"
	BoardCapabilityParticipate BoardCapability = "board.participate"
	BoardCapabilityFacilitate  BoardCapability = "board.facilitate"
	BoardCapabilityManage      BoardCapability = "board.manage"
	BoardCapabilityExport      BoardCapability = "board.export"
)

const (
	TimerActionStart     TimerAction = "start"
	TimerActionPause     TimerAction = "pause"
	TimerActionResume    TimerAction = "resume"
	TimerActionAddMinute TimerAction = "add_minute"
	TimerActionStop      TimerAction = "stop"
)

const (
	StageWrite   Stage = "write"
	StageVote    Stage = "vote"
	StageDiscuss Stage = "discuss"
	StageActions Stage = "actions"
	StageEnd     Stage = "end"
)

const (
	ReactionThumbsUp   Reaction = "thumbs_up"
	ReactionThumbsDown Reaction = "thumbs_down"
	ReactionHeart      Reaction = "heart"
	ReactionLaugh      Reaction = "laugh"
	ReactionWow        Reaction = "wow"
	ReactionSad        Reaction = "sad"
	ReactionAngry      Reaction = "angry"
	ReactionParty      Reaction = "party"
	ReactionRocket     Reaction = "rocket"
	ReactionFire       Reaction = "fire"
	ReactionClap       Reaction = "clap"
	ReactionThinking   Reaction = "thinking"
	ReactionEyes       Reaction = "eyes"
	ReactionHundred    Reaction = "hundred"
	ReactionStar       Reaction = "star"
)

const (
	TimerMinDuration   = time.Second
	TimerMaxDuration   = time.Hour
	TimerExtension     = time.Minute
	RetroIdleLimit     = 14 * 24 * time.Hour
	BoardRestoreWindow = 30 * 24 * time.Hour
)

const (
	CSVImportMaxCards = 500
	BoardMaxColumns   = 20
)

const (
	BoardTitleMaxRunes           = 200
	BoardParticipantNameMaxRunes = 40
	BoardColumnTitleMaxRunes     = 80
	CardBodyMaxRunes             = 2000
	BoardPasswordMaxBytes        = 72
)

var (
	Colours = []Colour{ColourGreen, ColourRed, ColourBlue, ColourAmber, ColourPurple, ColourTeal, ColourPink, ColourGrey}

	LinkAccessLevels = []LinkAccess{LinkAccessOff, LinkAccessGuests, LinkAccessAccounts}

	SortModes = []SortMode{SortModeManual, SortModeVotes}

	BoardRoles = []BoardRole{BoardRoleFacilitator, BoardRoleParticipant}

	TimerActions = []TimerAction{TimerActionStart, TimerActionPause, TimerActionResume, TimerActionAddMinute, TimerActionStop}

	Stages = []Stage{StageWrite, StageVote, StageDiscuss, StageActions, StageEnd}

	Reactions = []Reaction{
		ReactionThumbsUp, ReactionThumbsDown, ReactionHeart, ReactionLaugh, ReactionWow, ReactionSad, ReactionAngry, ReactionParty,
		ReactionRocket, ReactionFire, ReactionClap, ReactionThinking, ReactionEyes, ReactionHundred, ReactionStar,
	}
)

func (b Board) HasPassword() bool {
	return b.PasswordHash != nil
}

func (b Board) Admits(kind PrincipalKind) bool {
	if b.LinkAccess == LinkAccessGuests {
		return true
	}

	return b.LinkAccess == LinkAccessAccounts && kind == PrincipalUser
}

func (b Board) Anonymous() bool {
	return b.Anonymity == AnonymityAnonymous
}

func (b Board) Archived() bool {
	return b.ArchivedAt != nil
}

func (b Board) Deleted() bool {
	return b.DeletedAt != nil
}

func (b Board) Locked() bool {
	return b.LockedAt != nil
}

func (b Board) Closed() bool {
	return b.ClosedAt != nil
}

func (b Board) TotalsHidden() bool {
	return b.VoteCountsHidden && (b.Stage == StageWrite || b.Stage == StageVote)
}

func (b Board) CardsHiddenIn(columnID uuid.UUID) bool {
	return b.CardsHidden && !slices.Contains(b.RevealedColumnIDs, columnID)
}

func (b Board) HasHiddenColumns() bool {
	return len(b.HiddenColumnIDs) > 0
}

func (b Board) TimerDue() (TimerDue, bool) {
	if !b.TimerAutoLock || b.TimerEndsAt == nil || b.Locked() || b.Closed() || b.Archived() || b.Deleted() {
		return TimerDue{}, false
	}

	return TimerDue{BoardID: b.ID, OrgID: b.OrgID, EndsAt: *b.TimerEndsAt}, true
}

func (s Stage) Next() (Stage, bool) {
	at := slices.Index(Stages, s)
	if at < 0 || at == len(Stages)-1 {
		return "", false
	}

	return Stages[at+1], true
}

func (s Stage) Previous() (Stage, bool) {
	at := slices.Index(Stages, s)
	if at <= 0 || s == StageEnd {
		return "", false
	}

	return Stages[at-1], true
}

func (b Board) Timer() BoardTimer {
	return BoardTimer{DurationMS: b.TimerDurationMS, EndsAt: b.TimerEndsAt, PausedRemainingMS: b.TimerPausedRemainingMS}
}

func (t BoardTimer) Set() bool {
	return t.DurationMS != nil
}

func (t BoardTimer) Paused() bool {
	return t.PausedRemainingMS != nil
}

func (h BoardHeader) Gone() bool {
	return h.Deleted() || h.OrgDeletedAt != nil
}

func (p BoardParticipant) Removed() bool {
	return p.RemovedAt != nil
}

func (p BoardParticipant) JoinedThroughLink() bool {
	return p.JoinedVia == JoinPathLink
}

func (p BoardParticipant) Guest() bool {
	return p.UserID == nil && (p.GuestDeviceID != nil || p.JoinedThroughLink())
}
