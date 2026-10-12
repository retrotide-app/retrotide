package repository

import (
	"context"
	"database/sql"

	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	BoardRepository interface {
		dao.BoardDAO

		CreateBoard(ctx context.Context, s spec.NewBoard, tx ...*sql.Tx) (*model.BoardState, error)
		GetSnapshot(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.BoardState, error)
		SetBoardContext(ctx context.Context, s spec.BoardContextWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		JoinParticipant(ctx context.Context, s spec.BoardParticipantJoin, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error)
		JoinLinkParticipant(ctx context.Context, s spec.LinkParticipantJoin, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error)
		SetBoardLinkAccess(ctx context.Context, s spec.BoardLinkAccessWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		SetBoardPassword(ctx context.Context, s spec.BoardPasswordWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		RotateBoardLink(ctx context.Context, s spec.BoardLinkRotationWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		ClaimGuestParticipant(ctx context.Context, s spec.GuestClaimWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error)
		SetBoardSettings(ctx context.Context, s spec.BoardSettingsWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		RevealCards(ctx context.Context, s spec.CardsRevealWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error)
		HideCards(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error)
		AdvanceStage(ctx context.Context, s spec.BoardStageWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error)
		CloseBoard(ctx context.Context, s spec.BoardCloseWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		SetBoardTimer(ctx context.Context, s spec.BoardTimerWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		ChangeParticipantRole(ctx context.Context, s spec.ParticipantRoleWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error)
		RemoveBoardParticipant(ctx context.Context, s spec.ParticipantRemovalWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error)
		SetFacilitationState(ctx context.Context, s spec.BoardFacilitationWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error)
		LockBoardAtTimerEnd(ctx context.Context, s spec.BoardTimerLockWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		SetBoardBackground(ctx context.Context, s spec.BoardBackgroundWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		ArchiveBoard(ctx context.Context, s spec.BoardArchiveWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		DeleteBoard(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		MoveBoard(ctx context.Context, s spec.BoardTeamWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error)
		ImportCards(ctx context.Context, s spec.CardImportWrite, tx ...*sql.Tx) (*model.Versioned[model.CardImport], error)
	}

	BoardRepoDeps struct {
		Columns     dao.BoardColumnDAO
		Cards       dao.CardDAO
		Votes       dao.CardVoteDAO
		Comments    dao.CardCommentDAO
		ActionItems dao.ActionItemDAO
		Reactions   dao.CardReactionDAO
		Surveys     dao.SurveyDAO
	}

	boardRepository struct {
		dao.BoardDAO

		db          *sql.DB
		columns     dao.BoardColumnDAO
		cards       dao.CardDAO
		votes       dao.CardVoteDAO
		comments    dao.CardCommentDAO
		actionItems dao.ActionItemDAO
		reactions   dao.CardReactionDAO
		surveys     dao.SurveyDAO
	}
)

func NewBoardRepo(database *sql.DB, boardDAO dao.BoardDAO, deps BoardRepoDeps) BoardRepository {
	return &boardRepository{
		BoardDAO:    boardDAO,
		db:          database,
		columns:     deps.Columns,
		cards:       deps.Cards,
		votes:       deps.Votes,
		comments:    deps.Comments,
		actionItems: deps.ActionItems,
		reactions:   deps.Reactions,
		surveys:     deps.Surveys,
	}
}

func beginBoardWrite(ctx context.Context, boards dao.BoardDAO, s spec.BoardWrite, tx *sql.Tx) (int64, error) {
	version, err := boards.BumpVersion(ctx, s, tx)
	if err != nil {
		return 0, err
	}

	if s.OpID == uuid.Nil {
		return version, nil
	}

	if err := boards.RecordOp(ctx, spec.NewBoardOp{BoardID: s.BoardID, OpID: s.OpID, Version: version}, tx); err != nil {
		return 0, err
	}

	return version, nil
}

func writeBoard[T any](ctx context.Context, database *sql.DB, boards dao.BoardDAO, s spec.BoardWrite, tx []*sql.Tx, write func(tx *sql.Tx) (*T, error)) (*model.Versioned[T], error) {
	var written *model.Versioned[T]

	err := db.WithTx(ctx, database, tx, func(tx *sql.Tx) error {
		version, err := beginBoardWrite(ctx, boards, s, tx)
		if err != nil {
			return err
		}

		value, err := write(tx)
		if err != nil {
			return err
		}

		written = &model.Versioned[T]{Value: *value, Version: version}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return written, nil
}

func (r *boardRepository) CreateBoard(ctx context.Context, s spec.NewBoard, tx ...*sql.Tx) (*model.BoardState, error) {
	var created *model.BoardState

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		board, err := r.BoardDAO.Create(ctx, spec.NewBoardRecord{
			OrgID:       s.OrgID,
			TeamID:      s.TeamID,
			Title:       s.Title,
			ShareSlug:   s.ShareSlug,
			LinkAccess:  model.LinkAccessOff,
			CreatedBy:   new(s.CreatedBy),
			Context:     s.Context,
			TemplateKey: s.TemplateKey,
			Anonymity:   s.Anonymity,
			CardsHidden: s.CardsHidden,
		}, tx)
		if err != nil {
			return err
		}

		columns := make([]model.BoardColumn, 0, len(s.Columns))
		for _, seed := range s.Columns {
			column, err := r.columns.Create(ctx, spec.NewBoardColumn{BoardID: board.ID, BoardColumnSeed: seed}, tx)
			if err != nil {
				return err
			}

			columns = append(columns, *column)
		}

		if _, err := r.BoardDAO.AddParticipant(ctx, spec.NewBoardParticipant{BoardID: board.ID, UserID: new(s.CreatedBy), Role: model.BoardRoleFacilitator}, tx); err != nil {
			return err
		}

		participants, err := r.BoardDAO.ListParticipants(ctx, board.ID, tx)
		if err != nil {
			return err
		}

		created = &model.BoardState{
			Header:       model.BoardHeader{Board: *board},
			Columns:      columns,
			Cards:        []model.Card{},
			Participants: participants,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *boardRepository) GetSnapshot(ctx context.Context, s spec.BoardKey, tx ...*sql.Tx) (*model.BoardState, error) {
	var snapshot *model.BoardState

	err := db.WithReadTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		header, err := r.BoardDAO.GetByID(ctx, s, tx)
		if err != nil {
			return err
		}

		columns, err := r.columns.List(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		cards, err := r.cards.List(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		participants, err := r.BoardDAO.ListParticipants(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		votes, err := r.votes.List(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		comments, err := r.comments.List(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		items, err := r.actionItems.ListForBoard(ctx, s, tx)
		if err != nil {
			return err
		}

		previous := []model.PreviousAction{}
		if header.TeamID != nil {
			previous, err = r.actionItems.ListPrevious(ctx, spec.PreviousActionsQuery{OrgID: s.OrgID, TeamID: *header.TeamID, BoardID: s.BoardID, Before: header.CreatedAt}, tx)
			if err != nil {
				return err
			}
		}

		reactions, err := r.reactions.List(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		surveys, err := r.surveys.List(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		responses, err := r.surveys.ListResponses(ctx, s.BoardID, tx)
		if err != nil {
			return err
		}

		snapshot = &model.BoardState{
			Header:          *header,
			Columns:         columns,
			Cards:           cards,
			Participants:    participants,
			Votes:           votes,
			Comments:        comments,
			ActionItems:     items,
			PreviousActions: previous,
			Reactions:       reactions,
			Surveys:         surveys,
			SurveyResponses: responses,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (r *boardRepository) SetBoardContext(ctx context.Context, s spec.BoardContextWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	describe := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetContext(ctx, spec.BoardContextChange{OrgID: s.OrgID, BoardID: s.BoardID, Context: s.Context}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, describe)
}

func (r *boardRepository) JoinParticipant(ctx context.Context, s spec.BoardParticipantJoin, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
	join := func(tx *sql.Tx) (*model.BoardParticipant, error) {
		return r.BoardDAO.AddParticipant(ctx, spec.NewBoardParticipant{BoardID: s.BoardID, UserID: new(s.UserID), DisplayName: s.DisplayName, Role: s.Role}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, join)
}

func (r *boardRepository) JoinLinkParticipant(ctx context.Context, s spec.LinkParticipantJoin, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
	join := func(tx *sql.Tx) (*model.BoardParticipant, error) {
		return r.BoardDAO.UpsertLinkParticipant(ctx, spec.LinkParticipant{
			BoardID:       s.BoardID,
			UserID:        s.UserID,
			GuestDeviceID: s.GuestDeviceID,
			DisplayName:   s.DisplayName,
			AccessEpoch:   s.AccessEpoch,
		}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, join)
}

func (r *boardRepository) SetBoardLinkAccess(ctx context.Context, s spec.BoardLinkAccessWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	share := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetLinkAccess(ctx, spec.BoardLinkAccessChange{OrgID: s.OrgID, BoardID: s.BoardID, LinkAccess: s.LinkAccess}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, share)
}

func (r *boardRepository) SetBoardPassword(ctx context.Context, s spec.BoardPasswordWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	protect := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetPassword(ctx, spec.BoardPasswordChange{OrgID: s.OrgID, BoardID: s.BoardID, PasswordHash: s.PasswordHash}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, protect)
}

func (r *boardRepository) RotateBoardLink(ctx context.Context, s spec.BoardLinkRotationWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	rotate := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.RotateLink(ctx, spec.BoardLinkRotation{OrgID: s.OrgID, BoardID: s.BoardID, ShareSlug: s.ShareSlug}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, rotate)
}

func (r *boardRepository) ClaimGuestParticipant(ctx context.Context, s spec.GuestClaimWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
	claim := func(tx *sql.Tx) (*model.BoardParticipant, error) {
		if s.UserParticipantID == nil {
			return r.BoardDAO.ReassignParticipant(ctx, spec.ParticipantReassignment{BoardID: s.BoardID, ParticipantID: s.GuestParticipantID, UserID: s.UserID}, tx)
		}

		reassignment := spec.CardAuthorReassignment{BoardID: s.BoardID, FromParticipantID: s.GuestParticipantID, ToParticipantID: *s.UserParticipantID}
		if _, err := r.cards.ReassignAuthor(ctx, reassignment, tx); err != nil {
			return nil, err
		}

		if _, err := r.comments.ReassignAuthor(ctx, reassignment, tx); err != nil {
			return nil, err
		}

		handover := spec.VoteHandover{BoardID: s.BoardID, FromParticipantID: s.GuestParticipantID, ToParticipantID: *s.UserParticipantID}
		if _, err := r.votes.HandOverVotes(ctx, handover, tx); err != nil {
			return nil, err
		}

		if _, err := r.votes.HandOverBudget(ctx, handover, tx); err != nil {
			return nil, err
		}

		seat := spec.ParticipantHandover{BoardID: s.BoardID, FromParticipantID: s.GuestParticipantID, ToParticipantID: *s.UserParticipantID}
		if _, err := r.reactions.HandOver(ctx, seat, tx); err != nil {
			return nil, err
		}

		if _, err := r.surveys.HandOverResponses(ctx, seat, tx); err != nil {
			return nil, err
		}

		if err := r.BoardDAO.DeleteParticipant(ctx, spec.BoardParticipantKey{BoardID: s.BoardID, ParticipantID: s.GuestParticipantID}, tx); err != nil {
			return nil, err
		}

		return r.BoardDAO.GetParticipantForUser(ctx, spec.BoardParticipantUser{BoardID: s.BoardID, UserID: s.UserID}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, claim)
}

func (r *boardRepository) SetBoardSettings(ctx context.Context, s spec.BoardSettingsWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	configure := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetSettings(ctx, spec.BoardSettingsChange{OrgID: s.OrgID, BoardID: s.BoardID, Patch: s.Patch}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, configure)
}

func (r *boardRepository) RevealCards(ctx context.Context, s spec.CardsRevealWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error) {
	reveal := func(tx *sql.Tx) (*model.BoardState, error) {
		board, err := r.BoardDAO.SetCardsHidden(ctx, spec.BoardCardsHiddenChange{OrgID: s.OrgID, BoardID: s.BoardID, Hidden: false}, tx)
		if err != nil {
			return nil, err
		}

		if err := r.reposition(ctx, s.BoardID, s.Positions, tx); err != nil {
			return nil, err
		}

		return r.revealedState(ctx, *board, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, reveal)
}

func (r *boardRepository) AdvanceStage(ctx context.Context, s spec.BoardStageWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error) {
	advance := func(tx *sql.Tx) (*model.BoardState, error) {
		board, err := r.BoardDAO.SetStage(ctx, spec.BoardStageChange{OrgID: s.OrgID, BoardID: s.BoardID, From: s.From, To: s.To}, tx)
		if err != nil {
			return nil, err
		}

		if !s.Reveal {
			return &model.BoardState{Header: model.BoardHeader{Board: *board}}, nil
		}

		if err := r.reposition(ctx, s.BoardID, s.Positions, tx); err != nil {
			return nil, err
		}

		return r.revealedState(ctx, *board, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, advance)
}

func (r *boardRepository) revealedState(ctx context.Context, board model.Board, tx *sql.Tx) (*model.BoardState, error) {
	cards, err := r.cards.List(ctx, board.ID, tx)
	if err != nil {
		return nil, err
	}

	comments, err := r.comments.List(ctx, board.ID, tx)
	if err != nil {
		return nil, err
	}

	participants, err := r.BoardDAO.ListParticipants(ctx, board.ID, tx)
	if err != nil {
		return nil, err
	}

	votes, err := r.votes.List(ctx, board.ID, tx)
	if err != nil {
		return nil, err
	}

	reactions, err := r.reactions.List(ctx, board.ID, tx)
	if err != nil {
		return nil, err
	}

	return &model.BoardState{
		Header:       model.BoardHeader{Board: board},
		Cards:        cards,
		Comments:     comments,
		Participants: participants,
		Votes:        votes,
		Reactions:    reactions,
	}, nil
}

func (r *boardRepository) HideCards(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error) {
	hide := func(tx *sql.Tx) (*model.BoardState, error) {
		board, err := r.BoardDAO.SetCardsHidden(ctx, spec.BoardCardsHiddenChange{OrgID: s.OrgID, BoardID: s.BoardID, Hidden: true}, tx)
		if err != nil {
			return nil, err
		}

		cards, err := r.cards.List(ctx, s.BoardID, tx)
		if err != nil {
			return nil, err
		}

		return &model.BoardState{Header: model.BoardHeader{Board: *board}, Cards: cards}, nil
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s, tx, hide)
}

func (r *boardRepository) CloseBoard(ctx context.Context, s spec.BoardCloseWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	end := func(tx *sql.Tx) (*model.Board, error) {
		if err := r.reposition(ctx, s.BoardID, s.Positions, tx); err != nil {
			return nil, err
		}

		board, err := r.BoardDAO.Close(ctx, spec.BoardKey{OrgID: s.OrgID, BoardID: s.BoardID}, tx)
		if err != nil {
			return nil, err
		}

		if _, err := r.surveys.CloseOpen(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if !s.Sever {
			return board, nil
		}

		if _, err := r.reactions.Sever(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if _, err := r.surveys.SeverResponses(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if _, err := r.cards.SeverAuthors(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if _, err := r.comments.SeverAuthors(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if _, err := r.votes.DeleteAll(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		if _, err := r.votes.DeleteBudgets(ctx, s.BoardID, tx); err != nil {
			return nil, err
		}

		return board, nil
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, end)
}

func (r *boardRepository) SetBoardTimer(ctx context.Context, s spec.BoardTimerWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	schedule := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetTimer(ctx, spec.BoardTimerChange{OrgID: s.OrgID, BoardID: s.BoardID, Timer: s.Timer}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, schedule)
}

func (r *boardRepository) ChangeParticipantRole(ctx context.Context, s spec.ParticipantRoleWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
	assign := func(tx *sql.Tx) (*model.BoardParticipant, error) {
		return r.BoardDAO.SetParticipantRole(ctx, spec.ParticipantRoleChange{BoardID: s.BoardID, ParticipantID: s.ParticipantID, Role: s.Role}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, assign)
}

func (r *boardRepository) RemoveBoardParticipant(ctx context.Context, s spec.ParticipantRemovalWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardParticipant], error) {
	remove := func(tx *sql.Tx) (*model.BoardParticipant, error) {
		return r.BoardDAO.RemoveParticipant(ctx, spec.BoardParticipantKey{BoardID: s.BoardID, ParticipantID: s.ParticipantID}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, remove)
}

func (r *boardRepository) SetFacilitationState(ctx context.Context, s spec.BoardFacilitationWrite, tx ...*sql.Tx) (*model.Versioned[model.BoardState], error) {
	facilitate := func(tx *sql.Tx) (*model.BoardState, error) {
		board, err := r.BoardDAO.SetFacilitation(ctx, spec.BoardFacilitationChange{
			OrgID:             s.OrgID,
			BoardID:           s.BoardID,
			RevealedColumnIDs: s.RevealedColumnIDs,
			HiddenColumnIDs:   s.HiddenColumnIDs,
			Spotlight:         s.Spotlight,
		}, tx)
		if err != nil {
			return nil, err
		}

		if err := r.reposition(ctx, s.BoardID, s.Positions, tx); err != nil {
			return nil, err
		}

		if s.RevealedColumnID == nil {
			return &model.BoardState{Header: model.BoardHeader{Board: *board}}, nil
		}

		return r.revealedState(ctx, *board, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, facilitate)
}

func (r *boardRepository) LockBoardAtTimerEnd(ctx context.Context, s spec.BoardTimerLockWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	lock := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.LockAtTimerEnd(ctx, spec.BoardTimerLock{OrgID: s.OrgID, BoardID: s.BoardID, EndsAt: s.EndsAt}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, lock)
}

func (r *boardRepository) SetBoardBackground(ctx context.Context, s spec.BoardBackgroundWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	decorate := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetBackground(ctx, spec.BoardBackgroundChange{OrgID: s.OrgID, BoardID: s.BoardID, MediaID: s.MediaID}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, decorate)
}

func (r *boardRepository) ArchiveBoard(ctx context.Context, s spec.BoardArchiveWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	archive := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SetArchived(ctx, spec.BoardArchiveChange{OrgID: s.OrgID, BoardID: s.BoardID, Archived: s.Archived}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, archive)
}

func (r *boardRepository) DeleteBoard(ctx context.Context, s spec.BoardWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	remove := func(tx *sql.Tx) (*model.Board, error) {
		return r.BoardDAO.SoftDelete(ctx, spec.BoardKey{OrgID: s.OrgID, BoardID: s.BoardID}, tx)
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s, tx, remove)
}

func (r *boardRepository) MoveBoard(ctx context.Context, s spec.BoardTeamWrite, tx ...*sql.Tx) (*model.Versioned[model.Board], error) {
	move := func(tx *sql.Tx) (*model.Board, error) {
		board, err := r.BoardDAO.SetTeam(ctx, spec.BoardTeamChange{OrgID: s.OrgID, BoardID: s.BoardID, TeamID: s.TeamID}, tx)
		if err != nil {
			return nil, err
		}

		if _, err := r.actionItems.SetTeamForBoard(ctx, spec.ActionItemTeamChange{OrgID: s.OrgID, BoardID: s.BoardID, TeamID: s.TeamID}, tx); err != nil {
			return nil, err
		}

		return board, nil
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, move)
}

func (r *boardRepository) ImportCards(ctx context.Context, s spec.CardImportWrite, tx ...*sql.Tx) (*model.Versioned[model.CardImport], error) {
	load := func(tx *sql.Tx) (*model.CardImport, error) {
		columns := make([]model.BoardColumn, 0, len(s.Columns))
		for _, seed := range s.Columns {
			column, err := r.columns.Create(ctx, spec.NewBoardColumn{BoardID: s.BoardID, BoardColumnSeed: seed}, tx)
			if err != nil {
				return nil, err
			}

			columns = append(columns, *column)
		}

		cards := make([]model.Card, 0, len(s.Cards))
		for _, seed := range s.Cards {
			card, err := r.cards.Create(ctx, spec.NewCard{
				ID:                  seed.CardID,
				BoardID:             s.BoardID,
				ColumnID:            seed.ColumnID,
				AuthorParticipantID: seed.AuthorParticipantID,
				Body:                seed.Body,
				Position:            seed.Position,
			}, tx)
			if err != nil {
				return nil, err
			}

			cards = append(cards, *card)
		}

		return &model.CardImport{Columns: columns, Cards: cards}, nil
	}

	return writeBoard(ctx, r.db, r.BoardDAO, s.BoardWrite, tx, load)
}

func (r *boardRepository) reposition(ctx context.Context, boardID uuid.UUID, positions []spec.PositionChange, tx *sql.Tx) error {
	for _, change := range positions {
		if err := r.cards.SetPosition(ctx, spec.CardPositionChange{BoardID: boardID, CardID: change.ID, Position: change.Position}, tx); err != nil {
			return err
		}
	}

	return nil
}
