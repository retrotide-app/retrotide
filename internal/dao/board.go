package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	BoardDAO interface {
		Create(ctx context.Context, s spec.NewBoardRecord, tx ...*sql.Tx) (*model.Board, error)
		GetHeader(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.BoardHeader, error)
		GetHeaderBySlug(ctx context.Context, slug string, tx ...*sql.Tx) (*model.BoardHeader, error)
		GetByID(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.BoardHeader, error)
		ListForTeam(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) ([]model.BoardHeader, error)
		ListForOrganisation(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.BoardHeader, error)
		ListForGuestDevice(ctx context.Context, deviceID uuid.UUID, tx ...*sql.Tx) ([]model.GuestBoard, error)
		SetContext(ctx context.Context, s spec.BoardContextChange, tx ...*sql.Tx) (*model.Board, error)
		SetLinkAccess(ctx context.Context, s spec.BoardLinkAccessChange, tx ...*sql.Tx) (*model.Board, error)
		SetPassword(ctx context.Context, s spec.BoardPasswordChange, tx ...*sql.Tx) (*model.Board, error)
		RotateLink(ctx context.Context, s spec.BoardLinkRotation, tx ...*sql.Tx) (*model.Board, error)
		SetSettings(ctx context.Context, s spec.BoardSettingsChange, tx ...*sql.Tx) (*model.Board, error)
		SetCardsHidden(ctx context.Context, s spec.BoardCardsHiddenChange, tx ...*sql.Tx) (*model.Board, error)
		SetStage(ctx context.Context, s spec.BoardStageChange, tx ...*sql.Tx) (*model.Board, error)
		Close(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.Board, error)
		SetTimer(ctx context.Context, s spec.BoardTimerChange, tx ...*sql.Tx) (*model.Board, error)
		ListIdle(ctx context.Context, s spec.IdleBoardsQuery, tx ...*sql.Tx) ([]model.BoardRef, error)
		SetFacilitation(ctx context.Context, s spec.BoardFacilitationChange, tx ...*sql.Tx) (*model.Board, error)
		LockAtTimerEnd(ctx context.Context, s spec.BoardTimerLock, tx ...*sql.Tx) (*model.Board, error)
		ListAutoLockTimers(ctx context.Context, tx ...*sql.Tx) ([]model.TimerDue, error)
		BumpVersion(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (int64, error)
		RecordOp(ctx context.Context, s spec.NewBoardOp, tx ...*sql.Tx) error
		GetOp(ctx context.Context, s spec.BoardOpKey, tx ...*sql.Tx) (*model.BoardOp, error)
		SetBackground(ctx context.Context, s spec.BoardBackgroundChange, tx ...*sql.Tx) (*model.Board, error)
		SetArchived(ctx context.Context, s spec.BoardArchiveChange, tx ...*sql.Tx) (*model.Board, error)
		SoftDelete(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.Board, error)
		Restore(ctx context.Context, s spec.BoardRestore, tx ...*sql.Tx) (*model.Board, error)
		PurgeDeletedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
		ListDeletedForOrganisation(ctx context.Context, s spec.DeletedBoardsQuery, tx ...*sql.Tx) ([]model.BoardHeader, error)
		SetTeam(ctx context.Context, s spec.BoardTeamChange, tx ...*sql.Tx) (*model.Board, error)
		Pin(ctx context.Context, s spec.BoardPinKey, tx ...*sql.Tx) error
		Unpin(ctx context.Context, s spec.BoardPinKey, tx ...*sql.Tx) error
		ListPinned(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) ([]uuid.UUID, error)

		AddParticipant(ctx context.Context, s spec.NewBoardParticipant, tx ...*sql.Tx) (*model.BoardParticipant, error)
		UpsertLinkParticipant(ctx context.Context, s spec.LinkParticipant, tx ...*sql.Tx) (*model.BoardParticipant, error)
		GetParticipantForUser(ctx context.Context, s spec.BoardParticipantUser, tx ...*sql.Tx) (*model.BoardParticipant, error)
		GetParticipantForGuest(ctx context.Context, s spec.BoardParticipantGuest, tx ...*sql.Tx) (*model.BoardParticipant, error)
		GetParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) (*model.BoardParticipant, error)
		SetParticipantRole(ctx context.Context, s spec.ParticipantRoleChange, tx ...*sql.Tx) (*model.BoardParticipant, error)
		RemoveParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) (*model.BoardParticipant, error)
		CountLiveParticipants(ctx context.Context, s spec.BoardSeatCount, tx ...*sql.Tx) (int, error)
		ReassignParticipant(ctx context.Context, s spec.ParticipantReassignment, tx ...*sql.Tx) (*model.BoardParticipant, error)
		DeleteParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) error
		ListParticipants(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.BoardParticipantProfile, error)
	}

	boardDAO struct {
		db *sql.DB
	}

	organisationState struct {
		deletedAt   sql.NullTime
		allowGuests bool
	}
)

func decodeColumnIDs(raw json.RawMessage, field string, boardID uuid.UUID) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var ids []uuid.UUID
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, fmt.Errorf("decode the %s of board %s: %w", field, boardID, err)
	}

	if len(ids) == 0 {
		return nil, nil
	}

	return ids, nil
}

func encodeColumnIDs(ids []uuid.UUID) (json.RawMessage, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}

	encoded, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("encode column ids: %w", err)
	}

	return encoded, nil
}

func toBoard(row sqlcgen.Board) (*model.Board, error) {
	stage := model.Stage(row.Stage)
	if row.ClosedAt.Valid {
		stage = model.StageEnd
	}

	revealed, err := decodeColumnIDs(row.RevealedColumnIds, "revealed column ids", row.ID)
	if err != nil {
		return nil, err
	}

	hidden, err := decodeColumnIDs(row.HiddenColumnIds, "hidden column ids", row.ID)
	if err != nil {
		return nil, err
	}

	return &model.Board{
		ID:                     row.ID,
		OrgID:                  row.OrgID,
		TeamID:                 row.TeamID,
		Title:                  row.Title,
		ShareSlug:              row.ShareSlug,
		LinkAccess:             model.LinkAccess(row.LinkAccess),
		PasswordHash:           nullableString(row.PasswordHash),
		AccessEpoch:            int(row.AccessEpoch),
		Version:                row.Version,
		CreatedBy:              row.CreatedBy,
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
		LockedAt:               nullableTime(row.LockedAt),
		ClosedAt:               nullableTime(row.ClosedAt),
		ArchivedAt:             nullableTime(row.ArchivedAt),
		DeletedAt:              nullableTime(row.DeletedAt),
		Context:                row.Context,
		TemplateKey:            row.TemplateKey,
		Anonymity:              model.Anonymity(row.Anonymity),
		Stage:                  stage,
		CardsHidden:            row.CardsHidden,
		CardsLocked:            row.CardsLocked,
		MovesLocked:            row.MovesLocked,
		VotingOpen:             row.VotingOpen,
		VoteCountsHidden:       row.VoteCountsHidden,
		MaxVotes:               int(row.MaxVotes),
		MaxVotesPerCard:        int(row.MaxVotesPerCard),
		CommentsEnabled:        row.CommentsEnabled,
		MergingEnabled:         row.MergingEnabled,
		GuestNamesEnabled:      row.GuestNamesEnabled,
		ShowPrimeDirective:     row.ShowPrimeDirective,
		SortMode:               model.SortMode(row.SortMode),
		TimerDurationMS:        nullableInt(row.TimerDurationMs),
		TimerEndsAt:            nullableTime(row.TimerEndsAt),
		TimerPausedRemainingMS: nullableInt(row.TimerPausedRemainingMs),
		TimerPlaySound:         row.TimerPlaySound,
		RevealedColumnIDs:      revealed,
		HiddenColumnIDs:        hidden,
		Spotlight: model.BoardSpotlight{
			PresentingCardID:  row.PresentingCardID,
			HighlightedCardID: row.HighlightedCardID,
			FocusedColumnID:   row.FocusedColumnID,
		},
		TimerAutoLock:     row.TimerAutoLock,
		SortEnforced:      row.SortEnforced,
		AnyoneCanEdit:     row.AnyoneCanEdit,
		ShowCardDates:     row.ShowCardDates,
		BackgroundMediaID: row.BackgroundMediaID,
	}, nil
}

func toBoardHeader(row sqlcgen.Board, organisation organisationState, creatorName sql.NullString) (*model.BoardHeader, error) {
	board, err := toBoard(row)
	if err != nil {
		return nil, err
	}

	return &model.BoardHeader{
		Board:           *board,
		OrgDeletedAt:    nullableTime(organisation.deletedAt),
		OrgAllowsGuests: organisation.allowGuests,
		CreatorName:     creatorName.String,
	}, nil
}

func toBoardParticipant(row sqlcgen.BoardParticipant) *model.BoardParticipant {
	return &model.BoardParticipant{
		ID:            row.ID,
		BoardID:       row.BoardID,
		UserID:        row.UserID,
		GuestDeviceID: row.GuestDeviceID,
		DisplayName:   row.DisplayName,
		Role:          model.BoardRole(row.Role),
		AccessEpoch:   int(row.AccessEpoch),
		JoinedVia:     model.JoinPath(row.JoinedVia),
		RemovedAt:     nullableTime(row.RemovedAt),
		JoinedAt:      row.JoinedAt,
	}
}

func boardParticipant(row sqlcgen.BoardParticipant, err error, action string) (*model.BoardParticipant, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return toBoardParticipant(row), nil
}

func boardRow(row sqlcgen.Board, err error, action string) (*model.Board, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	board, err := toBoard(row)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	return board, nil
}

func boardPlace(orgID uuid.UUID, teamID *uuid.UUID) string {
	if teamID == nil {
		return "organisation " + orgID.String()
	}

	return fmt.Sprintf("team %s of organisation %s", *teamID, orgID)
}

func (r *boardDAO) Create(ctx context.Context, s spec.NewBoardRecord, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateBoard(ctx, sqlcgen.CreateBoardParams{
		OrgID:       s.OrgID,
		TeamID:      s.TeamID,
		Title:       s.Title,
		ShareSlug:   s.ShareSlug,
		LinkAccess:  string(s.LinkAccess),
		CreatedBy:   s.CreatedBy,
		Context:     s.Context,
		TemplateKey: s.TemplateKey,
		Anonymity:   string(s.Anonymity),
		CardsHidden: s.CardsHidden,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create board in %s: %w: %w", boardPlace(s.OrgID, s.TeamID), ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create board in %s: %w: %w", boardPlace(s.OrgID, s.TeamID), ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create board in %s: %w", boardPlace(s.OrgID, s.TeamID), err)
	}

	return toBoard(row)
}

func (r *boardDAO) GetHeader(ctx context.Context, id uuid.UUID, tx ...*sql.Tx) (*model.BoardHeader, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardHeader(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get board header %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get board header %s: %w", id, err)
	}

	return toBoardHeader(row.Board, organisationState{deletedAt: row.OrgDeletedAt, allowGuests: row.OrgAllowGuests}, sql.NullString{})
}

func (r *boardDAO) GetHeaderBySlug(ctx context.Context, slug string, tx ...*sql.Tx) (*model.BoardHeader, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardHeaderBySlug(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get the board header for a share link: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get the board header for a share link: %w", err)
	}

	return toBoardHeader(row.Board, organisationState{deletedAt: row.OrgDeletedAt, allowGuests: row.OrgAllowGuests}, sql.NullString{})
}

func (r *boardDAO) GetByID(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.BoardHeader, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoard(ctx, sqlcgen.GetBoardParams{OrgID: s.OrgID, ID: s.BoardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get board %s of organisation %s: %w", s.BoardID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get board %s of organisation %s: %w", s.BoardID, s.OrgID, err)
	}

	return toBoardHeader(row.Board, organisationState{deletedAt: row.OrgDeletedAt, allowGuests: row.OrgAllowGuests}, row.CreatorName)
}

func (r *boardDAO) ListForTeam(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) ([]model.BoardHeader, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListBoardsForTeam(ctx, sqlcgen.ListBoardsForTeamParams{OrgID: s.OrgID, TeamID: s.TeamID})
	if err != nil {
		return nil, fmt.Errorf("list boards of team %s in organisation %s: %w", s.TeamID, s.OrgID, err)
	}

	boards := make([]model.BoardHeader, 0, len(rows))
	for _, row := range rows {
		header, err := toBoardHeader(row.Board, organisationState{}, row.CreatorName)
		if err != nil {
			return nil, fmt.Errorf("list boards of team %s in organisation %s: %w", s.TeamID, s.OrgID, err)
		}

		boards = append(boards, *header)
	}

	return boards, nil
}

func (r *boardDAO) ListForOrganisation(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.BoardHeader, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListBoardsForOrganisation(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list boards of organisation %s: %w", orgID, err)
	}

	boards := make([]model.BoardHeader, 0, len(rows))
	for _, row := range rows {
		header, err := toBoardHeader(row.Board, organisationState{}, row.CreatorName)
		if err != nil {
			return nil, fmt.Errorf("list boards of organisation %s: %w", orgID, err)
		}

		boards = append(boards, *header)
	}

	return boards, nil
}

func (r *boardDAO) ListForGuestDevice(ctx context.Context, deviceID uuid.UUID, tx ...*sql.Tx) ([]model.GuestBoard, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListBoardsForGuestDevice(ctx, new(deviceID))
	if err != nil {
		return nil, fmt.Errorf("list the boards of guest device %s: %w", deviceID, err)
	}

	joined := make([]model.GuestBoard, 0, len(rows))
	for _, row := range rows {
		board, err := toBoard(row.Board)
		if err != nil {
			return nil, fmt.Errorf("list the boards of guest device %s: %w", deviceID, err)
		}

		joined = append(joined, model.GuestBoard{Board: *board, Participant: *toBoardParticipant(row.BoardParticipant)})
	}

	return joined, nil
}

func (r *boardDAO) SetLinkAccess(ctx context.Context, s spec.BoardLinkAccessChange, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardLinkAccess(ctx, sqlcgen.SetBoardLinkAccessParams{LinkAccess: string(s.LinkAccess), OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("set the link access of board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) SetPassword(ctx context.Context, s spec.BoardPasswordChange, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardPassword(ctx, sqlcgen.SetBoardPasswordParams{PasswordHash: toNullString(s.PasswordHash), OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("set the password of board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) RotateLink(ctx context.Context, s spec.BoardLinkRotation, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RotateBoardLink(ctx, sqlcgen.RotateBoardLinkParams{ShareSlug: s.ShareSlug, OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("rotate the share link of board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) SetSettings(ctx context.Context, s spec.BoardSettingsChange, tx ...*sql.Tx) (*model.Board, error) {
	patch := s.Patch

	params := sqlcgen.SetBoardSettingsParams{
		VotingOpen:       toNullBool(patch.VotingOpen),
		VoteCountsHidden: toNullBool(patch.VoteCountsHidden),
		MaxVotes:         toNullInt16(patch.MaxVotes),
		MaxVotesPerCard:  toNullInt16(patch.MaxVotesPerCard),
		CommentsEnabled:  toNullBool(patch.CommentsEnabled),
		MergingEnabled:   toNullBool(patch.MergingEnabled),
		OrgID:            s.OrgID,
		ID:               s.BoardID,
	}
	if patch.SortMode != nil {
		params.SortMode = sql.NullString{String: string(*patch.SortMode), Valid: true}
	}

	params.CardsLocked = toNullBool(patch.CardsLocked)
	params.MovesLocked = toNullBool(patch.MovesLocked)
	params.BoardLocked = toNullBool(patch.BoardLocked)
	params.TimerPlaySound = toNullBool(patch.TimerPlaySound)
	params.TimerAutoLock = toNullBool(patch.TimerAutoLock)
	params.SortEnforced = toNullBool(patch.SortEnforced)
	params.AnyoneCanEdit = toNullBool(patch.AnyoneCanEdit)
	params.ShowCardDates = toNullBool(patch.ShowCardDates)

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardSettings(ctx, params)

	return boardRow(row, err, fmt.Sprintf("change the settings of board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) SetCardsHidden(ctx context.Context, s spec.BoardCardsHiddenChange, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardCardsHidden(ctx, sqlcgen.SetBoardCardsHiddenParams{CardsHidden: s.Hidden, OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("set whether the cards of board %s of organisation %s are hidden", s.BoardID, s.OrgID))
}

func (r *boardDAO) SetStage(ctx context.Context, s spec.BoardStageChange, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardStage(ctx, sqlcgen.SetBoardStageParams{Stage: string(s.To), OrgID: s.OrgID, ID: s.BoardID, FromStage: string(s.From)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("move board %s of organisation %s from stage %s to %s: %w", s.BoardID, s.OrgID, s.From, s.To, ErrStageMoved)
	}
	if err != nil {
		return nil, fmt.Errorf("move board %s of organisation %s from stage %s to %s: %w", s.BoardID, s.OrgID, s.From, s.To, err)
	}

	return toBoard(row)
}

func (r *boardDAO) Close(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CloseBoard(ctx, sqlcgen.CloseBoardParams{OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("close board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) SetTimer(ctx context.Context, s spec.BoardTimerChange, tx ...*sql.Tx) (*model.Board, error) {
	params := sqlcgen.SetBoardTimerParams{
		TimerDurationMs:        toNullInt32(s.Timer.DurationMS),
		TimerPausedRemainingMs: toNullInt32(s.Timer.PausedRemainingMS),
		OrgID:                  s.OrgID,
		ID:                     s.BoardID,
	}
	if s.Timer.EndsAt != nil {
		params.TimerEndsAt = sql.NullTime{Time: *s.Timer.EndsAt, Valid: true}
	}

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardTimer(ctx, params)

	return boardRow(row, err, fmt.Sprintf("set the timer of board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) ListIdle(ctx context.Context, s spec.IdleBoardsQuery, tx ...*sql.Tx) ([]model.BoardRef, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListIdleBoards(ctx, sqlcgen.ListIdleBoardsParams{IdleSince: s.IdleSince, RowLimit: clampInt32(s.Limit)})
	if err != nil {
		return nil, fmt.Errorf("list the open boards idle since %s: %w", s.IdleSince.Format(time.RFC3339), err)
	}

	refs := make([]model.BoardRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, model.BoardRef{ID: row.ID, OrgID: row.OrgID})
	}

	return refs, nil
}

func (r *boardDAO) SetFacilitation(ctx context.Context, s spec.BoardFacilitationChange, tx ...*sql.Tx) (*model.Board, error) {
	action := fmt.Sprintf("set the facilitation state of board %s of organisation %s", s.BoardID, s.OrgID)

	revealed, err := encodeColumnIDs(s.RevealedColumnIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	hidden, err := encodeColumnIDs(s.HiddenColumnIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardFacilitationState(ctx, sqlcgen.SetBoardFacilitationStateParams{
		RevealedColumnIds: revealed,
		HiddenColumnIds:   hidden,
		PresentingCardID:  s.Spotlight.PresentingCardID,
		HighlightedCardID: s.Spotlight.HighlightedCardID,
		FocusedColumnID:   s.Spotlight.FocusedColumnID,
		OrgID:             s.OrgID,
		ID:                s.BoardID,
	})
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}

	return boardRow(row, err, action)
}

func (r *boardDAO) LockAtTimerEnd(ctx context.Context, s spec.BoardTimerLock, tx ...*sql.Tx) (*model.Board, error) {
	action := fmt.Sprintf("lock board %s of organisation %s at the end of its timer at %s", s.BoardID, s.OrgID, s.EndsAt.Format(time.RFC3339Nano))

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).LockBoardAtTimerEnd(ctx, sqlcgen.LockBoardAtTimerEndParams{EndsAt: s.EndsAt, OrgID: s.OrgID, ID: s.BoardID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", action, ErrTimerMoved)
	}

	return boardRow(row, err, action)
}

func (r *boardDAO) ListAutoLockTimers(ctx context.Context, tx ...*sql.Tx) ([]model.TimerDue, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListAutoLockTimers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the running timers that lock their board: %w", err)
	}

	timers := make([]model.TimerDue, 0, len(rows))
	for _, row := range rows {
		timers = append(timers, model.TimerDue{BoardID: row.ID, OrgID: row.OrgID, EndsAt: row.EndsAt})
	}

	return timers, nil
}

func (r *boardDAO) SetContext(ctx context.Context, s spec.BoardContextChange, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardContext(ctx, sqlcgen.SetBoardContextParams{OrgID: s.OrgID, ID: s.BoardID, Context: s.Context})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("set the context of board %s of organisation %s: %w", s.BoardID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("set the context of board %s of organisation %s: %w", s.BoardID, s.OrgID, err)
	}

	return toBoard(row)
}

func (r *boardDAO) BumpVersion(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (int64, error) {
	bumped, err := sqlcgen.New(db.TxOrDB(r.db, tx)).BumpBoardVersion(ctx, sqlcgen.BumpBoardVersionParams{
		OrgID:           s.OrgID,
		ID:              s.BoardID,
		ExpectedVersion: s.Version,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("bump the version of board %s of organisation %s: %w", s.BoardID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("bump the version of board %s of organisation %s: %w", s.BoardID, s.OrgID, err)
	}

	if !bumped.Valid {
		return 0, fmt.Errorf("bump board %s from version %d: %w", s.BoardID, s.Version, ErrVersionConflict)
	}

	return bumped.Int64, nil
}

func (r *boardDAO) RecordOp(ctx context.Context, s spec.NewBoardOp, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).RecordBoardOp(ctx, sqlcgen.RecordBoardOpParams{BoardID: s.BoardID, OpID: s.OpID, Version: s.Version})
	if isUniqueViolation(err) {
		return fmt.Errorf("record op %s on board %s: %w: %w", s.OpID, s.BoardID, ErrDuplicateOp, err)
	}
	if isForeignKeyViolation(err) {
		return fmt.Errorf("record op %s on board %s: %w: %w", s.OpID, s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("record op %s on board %s: %w", s.OpID, s.BoardID, err)
	}

	return nil
}

func (r *boardDAO) GetOp(ctx context.Context, s spec.BoardOpKey, tx ...*sql.Tx) (*model.BoardOp, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardOp(ctx, sqlcgen.GetBoardOpParams{BoardID: s.BoardID, OpID: s.OpID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get op %s on board %s: %w", s.OpID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get op %s on board %s: %w", s.OpID, s.BoardID, err)
	}

	return &model.BoardOp{BoardID: row.BoardID, OpID: row.OpID, Version: row.Version, CreatedAt: row.CreatedAt}, nil
}

func (r *boardDAO) SetBackground(ctx context.Context, s spec.BoardBackgroundChange, tx ...*sql.Tx) (*model.Board, error) {
	action := fmt.Sprintf("set the background of board %s of organisation %s", s.BoardID, s.OrgID)

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardBackground(ctx, sqlcgen.SetBoardBackgroundParams{BackgroundMediaID: s.MediaID, OrgID: s.OrgID, ID: s.BoardID})
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}

	return boardRow(row, err, action)
}

func (r *boardDAO) SetArchived(ctx context.Context, s spec.BoardArchiveChange, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardArchived(ctx, sqlcgen.SetBoardArchivedParams{Archived: s.Archived, OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("set whether board %s of organisation %s is archived", s.BoardID, s.OrgID))
}

func (r *boardDAO) SoftDelete(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SoftDeleteBoard(ctx, sqlcgen.SoftDeleteBoardParams{OrgID: s.OrgID, ID: s.BoardID})

	return boardRow(row, err, fmt.Sprintf("delete board %s of organisation %s", s.BoardID, s.OrgID))
}

func (r *boardDAO) Restore(ctx context.Context, s spec.BoardRestore, tx ...*sql.Tx) (*model.Board, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RestoreBoard(ctx, sqlcgen.RestoreBoardParams{OrgID: s.OrgID, ID: s.BoardID, DeletedSince: s.DeletedSince})

	return boardRow(row, err, fmt.Sprintf("restore board %s of organisation %s deleted since %s", s.BoardID, s.OrgID, s.DeletedSince.Format(time.RFC3339)))
}

func (r *boardDAO) PurgeDeletedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	purged, err := sqlcgen.New(db.TxOrDB(r.db, tx)).PurgeDeletedBoards(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge the boards deleted before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return purged, nil
}

func (r *boardDAO) ListDeletedForOrganisation(ctx context.Context, s spec.DeletedBoardsQuery, tx ...*sql.Tx) ([]model.BoardHeader, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListDeletedBoardsForOrganisation(ctx, sqlcgen.ListDeletedBoardsForOrganisationParams{OrgID: s.OrgID, DeletedSince: s.DeletedSince})
	if err != nil {
		return nil, fmt.Errorf("list the deleted boards of organisation %s: %w", s.OrgID, err)
	}

	boards := make([]model.BoardHeader, 0, len(rows))
	for _, row := range rows {
		header, err := toBoardHeader(row.Board, organisationState{}, row.CreatorName)
		if err != nil {
			return nil, fmt.Errorf("list the deleted boards of organisation %s: %w", s.OrgID, err)
		}

		boards = append(boards, *header)
	}

	return boards, nil
}

func (r *boardDAO) SetTeam(ctx context.Context, s spec.BoardTeamChange, tx ...*sql.Tx) (*model.Board, error) {
	action := fmt.Sprintf("move board %s to %s", s.BoardID, boardPlace(s.OrgID, s.TeamID))

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardTeam(ctx, sqlcgen.SetBoardTeamParams{TeamID: s.TeamID, OrgID: s.OrgID, ID: s.BoardID})
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
	}

	return boardRow(row, err, action)
}

func (r *boardDAO) Pin(ctx context.Context, s spec.BoardPinKey, tx ...*sql.Tx) error {
	err := sqlcgen.New(db.TxOrDB(r.db, tx)).PinBoard(ctx, sqlcgen.PinBoardParams{OrgID: s.OrgID, BoardID: s.BoardID, UserID: s.UserID})
	if isForeignKeyViolation(err) {
		return fmt.Errorf("pin board %s of organisation %s for user %s: %w: %w", s.BoardID, s.OrgID, s.UserID, ErrNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("pin board %s of organisation %s for user %s: %w", s.BoardID, s.OrgID, s.UserID, err)
	}

	return nil
}

func (r *boardDAO) Unpin(ctx context.Context, s spec.BoardPinKey, tx ...*sql.Tx) error {
	if err := sqlcgen.New(db.TxOrDB(r.db, tx)).UnpinBoard(ctx, sqlcgen.UnpinBoardParams{OrgID: s.OrgID, BoardID: s.BoardID, UserID: s.UserID}); err != nil {
		return fmt.Errorf("unpin board %s of organisation %s for user %s: %w", s.BoardID, s.OrgID, s.UserID, err)
	}

	return nil
}

func (r *boardDAO) ListPinned(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) ([]uuid.UUID, error) {
	pinned, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListPinnedBoards(ctx, sqlcgen.ListPinnedBoardsParams{OrgID: s.OrgID, UserID: s.UserID})
	if err != nil {
		return nil, fmt.Errorf("list the boards user %s pinned in organisation %s: %w", s.UserID, s.OrgID, err)
	}

	return pinned, nil
}

func (r *boardDAO) AddParticipant(ctx context.Context, s spec.NewBoardParticipant, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	joinedVia := s.JoinedVia
	if joinedVia == "" {
		joinedVia = model.JoinPathMembership
	}

	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddBoardParticipant(ctx, sqlcgen.AddBoardParticipantParams{
		BoardID:       s.BoardID,
		UserID:        s.UserID,
		GuestDeviceID: s.GuestDeviceID,
		DisplayName:   s.DisplayName,
		Role:          string(s.Role),
		AccessEpoch:   clampInt32(s.AccessEpoch),
		JoinedVia:     string(joinedVia),
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("add a participant to board %s: %w: %w", s.BoardID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("add a participant to board %s: %w: %w", s.BoardID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("add a participant to board %s: %w", s.BoardID, err)
	}

	return toBoardParticipant(row), nil
}

func (r *boardDAO) GetParticipantForUser(ctx context.Context, s spec.BoardParticipantUser, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardParticipantForUser(ctx, sqlcgen.GetBoardParticipantForUserParams{BoardID: s.BoardID, UserID: new(s.UserID)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get the participant of user %s on board %s: %w", s.UserID, s.BoardID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get the participant of user %s on board %s: %w", s.UserID, s.BoardID, err)
	}

	return toBoardParticipant(row), nil
}

func (r *boardDAO) GetParticipantForGuest(ctx context.Context, s spec.BoardParticipantGuest, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardParticipantForGuest(ctx, sqlcgen.GetBoardParticipantForGuestParams{BoardID: s.BoardID, GuestDeviceID: new(s.GuestDeviceID)})

	return boardParticipant(row, err, fmt.Sprintf("get the participant of guest device %s on board %s", s.GuestDeviceID, s.BoardID))
}

func (r *boardDAO) GetParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetBoardParticipant(ctx, sqlcgen.GetBoardParticipantParams{BoardID: s.BoardID, ID: s.ParticipantID})

	return boardParticipant(row, err, fmt.Sprintf("get participant %s of board %s", s.ParticipantID, s.BoardID))
}

func (r *boardDAO) SetParticipantRole(ctx context.Context, s spec.ParticipantRoleChange, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetBoardParticipantRole(ctx, sqlcgen.SetBoardParticipantRoleParams{Role: string(s.Role), BoardID: s.BoardID, ID: s.ParticipantID})

	return boardParticipant(row, err, fmt.Sprintf("make participant %s of board %s a %s", s.ParticipantID, s.BoardID, s.Role))
}

func (r *boardDAO) RemoveParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveBoardParticipant(ctx, sqlcgen.RemoveBoardParticipantParams{BoardID: s.BoardID, ID: s.ParticipantID})

	return boardParticipant(row, err, fmt.Sprintf("remove participant %s from board %s", s.ParticipantID, s.BoardID))
}

func (r *boardDAO) UpsertLinkParticipant(ctx context.Context, s spec.LinkParticipant, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	queries := sqlcgen.New(db.TxOrDB(r.db, tx))
	action := fmt.Sprintf("join a participant to board %s through its link", s.BoardID)

	if s.UserID != nil && s.GuestDeviceID == nil {
		row, err := queries.UpsertLinkParticipantForUser(ctx, sqlcgen.UpsertLinkParticipantForUserParams{
			BoardID:     s.BoardID,
			UserID:      *s.UserID,
			AccessEpoch: clampInt32(s.AccessEpoch),
		})
		if isForeignKeyViolation(err) {
			return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
		}

		return boardParticipant(row, err, action)
	}

	if s.GuestDeviceID != nil && s.UserID == nil {
		row, err := queries.UpsertLinkParticipantForGuest(ctx, sqlcgen.UpsertLinkParticipantForGuestParams{
			BoardID:       s.BoardID,
			GuestDeviceID: *s.GuestDeviceID,
			DisplayName:   s.DisplayName,
			AccessEpoch:   clampInt32(s.AccessEpoch),
		})
		if isForeignKeyViolation(err) {
			return nil, fmt.Errorf("%s: %w: %w", action, ErrNotFound, err)
		}

		return boardParticipant(row, err, action)
	}

	return nil, fmt.Errorf("%s: %w", action, ErrOnePrincipal)
}

func (r *boardDAO) CountLiveParticipants(ctx context.Context, s spec.BoardSeatCount, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountLiveBoardParticipants(ctx, sqlcgen.CountLiveBoardParticipantsParams{
		BoardID:     s.BoardID,
		AccessEpoch: clampInt32(s.AccessEpoch),
	})
	if err != nil {
		return 0, fmt.Errorf("count the participants of board %s: %w", s.BoardID, err)
	}

	return int(count), nil
}

func (r *boardDAO) ReassignParticipant(ctx context.Context, s spec.ParticipantReassignment, tx ...*sql.Tx) (*model.BoardParticipant, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ReassignBoardParticipantToUser(ctx, sqlcgen.ReassignBoardParticipantToUserParams{UserID: s.UserID, BoardID: s.BoardID, ID: s.ParticipantID})
	action := fmt.Sprintf("hand participant %s of board %s to user %s", s.ParticipantID, s.BoardID, s.UserID)
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("%s: %w: %w", action, ErrConflict, err)
	}

	return boardParticipant(row, err, action)
}

func (r *boardDAO) DeleteParticipant(ctx context.Context, s spec.BoardParticipantKey, tx ...*sql.Tx) error {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteBoardParticipant(ctx, sqlcgen.DeleteBoardParticipantParams{BoardID: s.BoardID, ID: s.ParticipantID})
	if err != nil {
		return fmt.Errorf("delete participant %s of board %s: %w", s.ParticipantID, s.BoardID, err)
	}

	if deleted == 0 {
		return fmt.Errorf("delete participant %s of board %s: %w", s.ParticipantID, s.BoardID, ErrNotFound)
	}

	return nil
}

func (r *boardDAO) ListParticipants(ctx context.Context, boardID uuid.UUID, tx ...*sql.Tx) ([]model.BoardParticipantProfile, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListBoardParticipants(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list participants of board %s: %w", boardID, err)
	}

	participants := make([]model.BoardParticipantProfile, 0, len(rows))
	for _, row := range rows {
		participants = append(participants, model.BoardParticipantProfile{
			BoardParticipant: *toBoardParticipant(row.BoardParticipant),
			AccountName:      row.AccountName,
		})
	}

	return participants, nil
}
