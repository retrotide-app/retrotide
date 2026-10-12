package images

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
		boards *repository.MockBoardRepository
		cards  *repository.MockCardRepository
		media  *repository.MockMediaRepository
		live   *live.MockService
		events *recordedEvents

		acquired int
		released int
	}

	recordedEvents struct {
		published []publishedEvent
	}

	publishedEvent struct {
		access model.BoardAccess
		event  projection.Event
	}

	accessOption func(*model.BoardAccess)
)

const (
	secret = "SECRET-TEXT-5b1e"
)

var (
	orgID      = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000001")
	teamID     = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000002")
	boardID    = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000003")
	userID     = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000004")
	myID       = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000005")
	theirID    = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000006")
	opID       = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000007")
	columnID   = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000008")
	cardID     = uuid.MustParse("7a8b9c0d-0000-4000-8000-000000000009")
	mediaID    = uuid.MustParse("7a8b9c0d-0000-4000-8000-00000000000a")
	deviceID   = uuid.MustParse("7a8b9c0d-0000-4000-8000-00000000000b")
	stamp      = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	errMissing = fmt.Errorf("lookup: %w", dao.ErrNotFound)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
	me     = model.Principal{Kind: model.PrincipalUser, ID: userID}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		boards: repository.NewMockBoardRepository(t),
		cards:  repository.NewMockCardRepository(t),
		media:  repository.NewMockMediaRepository(t),
		live:   live.NewMockService(t),
		events: &recordedEvents{},
	}

	t.Cleanup(func() {
		assert.Equal(t, m.acquired, m.released, "every gate taken is released")
	})

	svc := NewService(Deps{
		Boards: m.boards,
		Cards:  m.cards,
		Media:  m.media,
		Live:   m.live,
		Events: m.events,
	}).(*service)

	return svc, m
}

func (r *recordedEvents) Publish(access model.BoardAccess, events ...projection.Event) {
	for _, event := range events {
		r.published = append(r.published, publishedEvent{access: access, event: event})
	}
}

func (r *recordedEvents) PublishMe(_ model.BoardAccess, _ dto.BoardMeFrame) {
}

func (r *recordedEvents) PublishMeToEveryone(_ model.BoardAccess, _ []model.CardVote, _ int64) {
}

func (r *recordedEvents) PublishedTally(_ uuid.UUID) (projection.HiddenTally, bool) {
	return projection.HiddenTally{}, false
}

func (r *recordedEvents) AnnounceTeamActionItems(_ uuid.UUID, _ []uuid.UUID) {
}

func boardAccess(role model.BoardRole, options ...accessOption) *model.BoardAccess {
	access := &model.BoardAccess{
		Principal: me,
		Header: model.BoardHeader{Board: model.Board{
			ID:        boardID,
			OrgID:     orgID,
			TeamID:    new(teamID),
			Title:     "Sprint 12",
			Version:   7,
			CreatedAt: stamp,
			Anonymity: model.AnonymityNamed,
			Stage:     model.StageWrite,
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

func asGuest(access *model.BoardAccess) {
	access.Principal = model.Principal{Kind: model.PrincipalGuest, ID: deviceID}
	access.OrgMember = false
	access.Capabilities = policy.Capabilities(access.Role, false)
}

func hidingCards(access *model.BoardAccess) {
	access.Header.CardsHidden = true
}

func atStage(stage model.Stage) accessOption {
	return func(access *model.BoardAccess) {
		access.Header.Stage = stage
	}
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

func uploaded(kind model.MediaKind) *model.Media {
	return &model.Media{ID: mediaID, OrgID: orgID, BoardID: boardID, Kind: kind, ContentType: model.MediaContentType, Width: 640, Height: 480, ByteSize: 2048, CreatedAt: stamp}
}

func imagePath() string {
	return "/uploads/" + boardID.String() + "/" + mediaID.String() + ".webp"
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
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errMissing).Once()
}

func (m *testMocks) applied(access *model.BoardAccess, version int64) {
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil).Once()
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(&model.BoardOp{BoardID: boardID, OpID: opID, Version: version}, nil).Once()
}

func payloadJSON(t *testing.T, payload any) string {
	t.Helper()

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	return string(data)
}

func cardImageCommand(media *uuid.UUID) CardImageCommand {
	return CardImageCommand{Command: baseCommand(), CardID: cardID, MediaID: media}
}

func TestSetCardImage_AttachesAnImageToTheAuthorsOwnCard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(new(card(myID)), nil).Once()
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(uploaded(model.MediaKindCard), nil).Once()
	illustrated := card(myID)
	illustrated.ImageMediaID = new(mediaID)
	m.cards.EXPECT().SetCardImage(mock.Anything, spec.CardImageWrite{BoardWrite: target(), CardID: cardID, MediaID: new(mediaID)}).
		Return(&model.Versioned[model.Card]{Value: illustrated, Version: 8}, nil).Once()

	// when
	got, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	changed, ok := m.events.published[0].event.Payload(projection.ClassOthers).(dto.CardImageChangedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventCardImageChanged, changed.Type)
	assert.Equal(t, cardID, changed.CardID)
	assert.Equal(t, new(imagePath()), changed.ImagePath)
}

func TestSetCardImage_RemovesTheImageWithoutLookingAnyMediaUp(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	illustrated := card(myID)
	illustrated.ImageMediaID = new(mediaID)
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(new(illustrated), nil).Once()
	m.cards.EXPECT().SetCardImage(mock.Anything, spec.CardImageWrite{BoardWrite: target(), CardID: cardID}).
		Return(&model.Versioned[model.Card]{Value: card(myID), Version: 8}, nil).Once()

	// when
	got, err := svc.SetCardImage(t.Context(), cardImageCommand(nil))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.Len(t, m.events.published, 1)
	assert.JSONEq(t, `{"type":"card_image_changed","version":8,"card_id":"`+cardID.String()+`","image_path":null}`, payloadJSON(t, m.events.published[0].event.Payload(projection.ClassOthers)))
}

func TestSetCardImage_RefusesAnImageThatIsNotOnTheBoard(t *testing.T) {
	// given an image ID uploaded to another board, which the lookup scoped to this board cannot find
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(new(card(myID)), nil).Once()
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(nil, errMissing).Once()

	// when
	_, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

	// then the strict card mock proves nothing is written
	assert.ErrorIs(t, err, ErrMediaNotFound)
	assert.Empty(t, m.events.published)
}

func TestSetCardImage_RefusesABackgroundUsedAsACardImage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(new(card(myID)), nil).Once()
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(uploaded(model.MediaKindBackground), nil).Once()

	// when
	_, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

	// then
	assert.ErrorIs(t, err, ErrWrongKind)
	assert.Empty(t, m.events.published)
}

func TestSetCardImage_RefusesEveryoneWhoMayNotIllustrateTheCard(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		card   model.Card
		want   error
	}{
		{name: "someone else's card", access: boardAccess(writer), card: card(theirID), want: policy.ErrNotAllowed},
		{name: "a guest's own card", access: boardAccess(writer, asGuest), card: card(myID), want: policy.ErrMembersOnly},
		{name: "outside the Write stage", access: boardAccess(writer, atStage(model.StageVote)), card: card(myID), want: policy.ErrCardsOutOfStage},
		{name: "a hidden card of someone else", access: boardAccess(host, hidingCards), card: card(theirID), want: policy.ErrCardNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)
			m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(new(tc.card), nil).Once()

			// when
			_, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

			// then no media is looked up and nothing is written
			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestSetCardImage_ACardOfAnotherBoardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errMissing).Once()

	// when
	_, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

	// then
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestSetCardImage_KeepsTheImageOfAHiddenCardFromEveryoneButItsAuthor(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hidingCards))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(new(card(myID)), nil).Once()
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(uploaded(model.MediaKindCard), nil).Once()
	illustrated := card(myID)
	illustrated.ImageMediaID = new(mediaID)
	m.cards.EXPECT().SetCardImage(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: illustrated, Version: 8}, nil).Once()

	// when
	_, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

	// then
	require.NoError(t, err)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	assert.Equal(t, projection.ClassNone, event.ClassOf(projection.Viewer{ParticipantID: new(theirID), Role: writer}))
	authorClass := event.ClassOf(projection.Viewer{ParticipantID: new(myID), Role: writer})
	require.Equal(t, projection.ClassAuthor, authorClass)
	author := payloadJSON(t, event.Payload(authorClass))
	assert.Contains(t, author, imagePath())
	assert.NotContains(t, author, secret)
}

func TestSetCardImage_ARepeatedOpAnswersWithItsVersionAndWritesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer), 8)

	// when
	got, err := svc.SetCardImage(t.Context(), cardImageCommand(new(mediaID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.Empty(t, m.events.published)
}

func TestSetCardImage_NeedsAnOpID(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	command := cardImageCommand(new(mediaID))
	command.OpID = uuid.Nil

	// when
	_, err := svc.SetCardImage(t.Context(), command)

	// then
	assert.ErrorIs(t, err, board.ErrOpRequired)
}
