package cardpolish

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		boards    *repository.MockBoardRepository
		cards     *repository.MockCardRepository
		reactions *repository.MockCardReactionRepository
		live      *live.MockService
		events    *recordedEvents

		acquired int
		released int
	}

	recordedEvents struct {
		published []projection.Event
	}

	accessOption func(*model.BoardAccess)
)

const (
	secret = "SECRET-TEXT-5c1e"
)

var (
	orgID         = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000001")
	boardID       = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000002")
	userID        = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000003")
	myID          = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000004")
	theirID       = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000005")
	opID          = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000006")
	columnID      = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000007")
	cardID        = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000008")
	guestDeviceID = uuid.MustParse("7c8d9e0f-0000-4000-8000-000000000009")
	stamp         = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	errNotFound = fmt.Errorf("lookup: %w", dao.ErrNotFound)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
	me     = model.Principal{Kind: model.PrincipalUser, ID: userID}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		boards:    repository.NewMockBoardRepository(t),
		cards:     repository.NewMockCardRepository(t),
		reactions: repository.NewMockCardReactionRepository(t),
		live:      live.NewMockService(t),
		events:    &recordedEvents{},
	}

	t.Cleanup(func() {
		assert.Equal(t, m.acquired, m.released, "every gate taken is released")
	})

	svc := NewService(Deps{
		Boards:    m.boards,
		Cards:     m.cards,
		Reactions: m.reactions,
		Live:      m.live,
		Events:    m.events,
	}).(*service)

	return svc, m
}

func (r *recordedEvents) Publish(_ model.BoardAccess, events ...projection.Event) {
	r.published = append(r.published, events...)
}

func (r *recordedEvents) PublishMe(_ model.BoardAccess, _ dto.BoardMeFrame) {
}

func (r *recordedEvents) PublishMeToEveryone(_ model.BoardAccess, _ []model.CardVote, _ int64) {
}

func (r *recordedEvents) PublishedTally(_ uuid.UUID) (projection.HiddenTally, bool) {
	return nil, false
}

func (r *recordedEvents) AnnounceTeamActionItems(_ uuid.UUID, _ []uuid.UUID) {
}

func boardAccess(role model.BoardRole, options ...accessOption) *model.BoardAccess {
	access := &model.BoardAccess{
		Principal: me,
		Header: model.BoardHeader{Board: model.Board{
			ID:        boardID,
			OrgID:     orgID,
			Title:     "Sprint 12",
			Version:   7,
			CreatedAt: stamp,
			Anonymity: model.AnonymityNamed,
			Stage:     model.StageDiscuss,
		}},
		ParticipantID: new(myID),
		Role:          role,
		OrgMember:     true,
		Capabilities:  policy.Capabilities(role, true),
	}

	for _, apply := range options {
		apply(access)
	}

	return access
}

func hidingCards(access *model.BoardAccess) {
	access.Header.CardsHidden = true
}

func asGuestWithoutSeat(access *model.BoardAccess) {
	access.Principal = model.Principal{Kind: model.PrincipalGuest, ID: guestDeviceID}
	access.OrgMember = false
	access.ParticipantID = nil
	access.Capabilities = policy.Capabilities(access.Role, false)
}

func baseCommand() board.Command {
	return board.Command{Principal: me, BoardID: boardID, OpID: opID}
}

func target() spec.BoardWrite {
	return spec.BoardWrite{OrgID: orgID, BoardID: boardID, Version: 7, OpID: opID}
}

func card(author uuid.UUID) model.Card {
	return model.Card{ID: cardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(author), Body: secret, Position: "V", CreatedAt: stamp}
}

func cardRef() spec.CardRef {
	return spec.CardRef{BoardID: boardID, CardID: cardID}
}

func reactCommand(reaction model.Reaction) ReactCommand {
	return ReactCommand{Command: baseCommand(), CardID: cardID, Reaction: reaction}
}

func (m *testMocks) gate() {
	m.acquired++
	m.live.EXPECT().Acquire(mock.Anything, boardID).Return(func() {
		m.released++
	}, nil).Once()
}

func (m *testMocks) entered(access *model.BoardAccess) {
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil).Once()
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errNotFound).Once()
}

func (m *testMocks) applied(access *model.BoardAccess, version int64) {
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil).Once()
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(&model.BoardOp{BoardID: boardID, OpID: opID, Version: version}, nil).Once()
}

func TestReact_AddsTheReactionAndAnnouncesTheCountsWithoutWhoReacted(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	stored := []model.CardReaction{
		{BoardID: boardID, CardID: cardID, ParticipantID: new(myID), Reaction: model.ReactionHeart},
		{BoardID: boardID, CardID: cardID, ParticipantID: new(theirID), Reaction: model.ReactionHeart},
		{BoardID: boardID, CardID: cardID, ParticipantID: new(theirID), Reaction: model.ReactionThumbsUp},
	}
	m.reactions.EXPECT().React(mock.Anything, spec.CardReactionWrite{
		BoardWrite:    target(),
		CardID:        cardID,
		ParticipantID: myID,
		Reaction:      model.ReactionHeart,
		Add:           true,
	}).Return(&model.Versioned[[]model.CardReaction]{Value: stored, Version: 8}, nil).Once()

	// when
	got, err := svc.React(t.Context(), reactCommand(model.ReactionHeart))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Reactions)
	assert.Equal(t, cardID, got.Reactions.CardID)
	assert.Equal(t, []dto.ReactionCountView{{Reaction: model.ReactionThumbsUp, Count: 1}, {Reaction: model.ReactionHeart, Count: 2}}, got.Reactions.Reactions)
	assert.Equal(t, []model.Reaction{model.ReactionHeart}, got.Reactions.MyReactions)
	require.Len(t, m.events.published, 1)
	shared, ok := m.events.published[0].Payload(projection.ClassOthers).(dto.CardReactionsChangedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventCardReactionsChanged, shared.Type)
	assert.Equal(t, int64(8), shared.Version)
	assert.Equal(t, got.Reactions.Reactions, shared.Reactions)
	assert.Empty(t, shared.MyReactions)
	encoded, err := json.Marshal(shared)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), myID.String())
	assert.NotContains(t, string(encoded), theirID.String())
	assert.NotContains(t, string(encoded), secret)
	personal, isPersonal := m.events.published[0].PayloadFor(projection.ViewerFor(*boardAccess(writer)))
	require.True(t, isPersonal)
	assert.Equal(t, []model.Reaction{model.ReactionHeart}, personal.(dto.CardReactionsChangedEvent).MyReactions)
}

func TestUnreact_RemovesTheViewersReaction(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	m.reactions.EXPECT().React(mock.Anything, spec.CardReactionWrite{
		BoardWrite:    target(),
		CardID:        cardID,
		ParticipantID: myID,
		Reaction:      model.ReactionRocket,
		Add:           false,
	}).Return(&model.Versioned[[]model.CardReaction]{Value: []model.CardReaction{}, Version: 8}, nil).Once()

	// when
	got, err := svc.Unreact(t.Context(), reactCommand(model.ReactionRocket))

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Reactions)
	assert.Empty(t, got.Reactions.Reactions)
	assert.Empty(t, got.Reactions.MyReactions)
	require.Len(t, m.events.published, 1)
}

func TestReact_RefusesAnUnknownReactionBeforeTouchingTheBoard(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.React(t.Context(), reactCommand("banana"))

	// then the strict mocks prove nothing was read or written
	assert.ErrorIs(t, err, ErrUnknownReaction)
}

func TestUnreact_RefusesAnUnknownReaction(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.Unreact(t.Context(), reactCommand(""))

	// then
	assert.ErrorIs(t, err, ErrUnknownReaction)
}

func TestReact_RefusesSomeoneElsesHiddenCardAsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hidingCards))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()

	// when
	_, err := svc.React(t.Context(), reactCommand(model.ReactionHeart))

	// then
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
	assert.Empty(t, m.events.published)
}

func TestReact_RefusesTheViewersOwnHiddenCardAsNotPublic(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hidingCards))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(myID)), nil).Once()

	// when
	_, err := svc.React(t.Context(), reactCommand(model.ReactionHeart))

	// then
	assert.ErrorIs(t, err, policy.ErrCardNotPublic)
	assert.Empty(t, m.events.published)
}

func TestReact_RefusesAGuestWithoutASeat(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, asGuestWithoutSeat))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()

	// when
	_, err := svc.React(t.Context(), reactCommand(model.ReactionHeart))

	// then
	assert.ErrorIs(t, err, policy.ErrNotParticipant)
	assert.Empty(t, m.events.published)
}

func TestReact_AMissingCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(nil, errNotFound).Once()

	// when
	_, err := svc.React(t.Context(), reactCommand(model.ReactionHeart))

	// then
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestReact_ReplaysAnAppliedOpWithoutWriting(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer), 9)

	// when
	got, err := svc.React(t.Context(), reactCommand(model.ReactionHeart))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 9, Replayed: true}, got)
	assert.Empty(t, m.events.published)
}

func TestReact_RequiresAnOpID(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	command := reactCommand(model.ReactionHeart)
	command.OpID = uuid.Nil

	// when
	_, err := svc.React(t.Context(), command)

	// then
	assert.ErrorIs(t, err, board.ErrOpRequired)
}
