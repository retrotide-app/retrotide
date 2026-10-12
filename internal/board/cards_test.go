package board

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	firstCardID  = uuid.MustParse("3c4d5e6f-0000-4000-8000-000000000001")
	secondCardID = uuid.MustParse("3c4d5e6f-0000-4000-8000-000000000002")
	thirdCardID  = uuid.MustParse("3c4d5e6f-0000-4000-8000-000000000003")
)

func card(author uuid.UUID) model.Card {
	return model.Card{ID: cardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(author), Body: secret, Position: "V", CreatedAt: stamp}
}

func columnCards(positions ...string) []model.Card {
	ids := []uuid.UUID{firstCardID, secondCardID, thirdCardID}
	cards := make([]model.Card, 0, len(positions))
	for i, position := range positions {
		cards = append(cards, model.Card{ID: ids[i], BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(theirID), Body: secret, Position: position, CreatedAt: stamp})
	}

	return cards
}

func profiles() []model.BoardParticipantProfile {
	return []model.BoardParticipantProfile{
		{ID: theirID, BoardID: boardID, DisplayName: "Ali", Role: writer},
		{ID: myID, BoardID: boardID, UserID: new(userID), Role: writer, AccountName: "Sam"},
	}
}

func addCardCommand() AddCardCommand {
	return AddCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID, Body: "  Deploys were smooth \n"}
}

func (m *testMocks) column() {
	key := spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}

	m.columns.EXPECT().GetByID(mock.Anything, key).Return(&model.BoardColumn{ID: columnID, BoardID: boardID}, nil).Once()
}

func (m *testMocks) stored(stored model.Card) {
	key := spec.CardRef{BoardID: boardID, CardID: stored.ID}

	m.cards.EXPECT().GetByID(mock.Anything, key).Return(&stored, nil).Once()
}

func (m *testMocks) participants() {
	listed := profiles()

	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return(listed, nil).Once()
}

func TestAddCard_AddsTheCardAtTheEndOfItsColumn(t *testing.T) {
	// given a named board in the writing phase, whose column holds someone else's hidden cards
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hiddenCards))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(columnCards("V", "k"), nil)
	added := model.Card{ID: cardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(myID), Body: "Deploys were smooth", Colour: new(model.ColourGreen), Position: "V", CreatedAt: stamp}
	m.cards.EXPECT().AddCard(mock.Anything, spec.CardCreate{
		BoardWrite:          target(),
		CardID:              cardID,
		ColumnID:            columnID,
		AuthorParticipantID: new(myID),
		Body:                "Deploys were smooth",
		Colour:              new(model.ColourGreen),
		Position:            "V",
	}).Return(&model.Versioned[model.Card]{Value: added, Version: 8}, nil)
	m.participants()
	command := addCardCommand()
	command.Colour = new(model.ColourGreen)

	// when
	got, err := svc.AddCard(t.Context(), command)

	// then the author gets their own hidden card back in full
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Card)
	assert.Equal(t, cardID, got.Card.ID)
	assert.Equal(t, "Deploys were smooth", got.Card.Body)
	assert.True(t, got.Card.IsMine)
	assert.True(t, got.Card.Hidden)
	assert.Equal(t, &dto.AuthorView{ParticipantID: myID, DisplayName: "Sam"}, got.Card.Author)
}

func TestAddCard_AppendsAfterTheLastCardOnceCardsAreShown(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anonymous))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(columnCards("V", "k"), nil)
	added := model.Card{ID: cardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(myID), Body: "Deploys were smooth", Position: "l", CreatedAt: stamp}
	m.cards.EXPECT().AddCard(mock.Anything, mock.MatchedBy(func(s spec.CardCreate) bool {
		return s.Position == "l"
	})).Return(&model.Versioned[model.Card]{Value: added, Version: 8}, nil)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Card)
	assert.Equal(t, "l", got.Card.Position)
}

func TestAddCard_TheKeyOfYourOwnCardSaysNothingAboutHiddenCards(t *testing.T) {
	mine := model.Card{ID: thirdCardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(myID), Body: "Mine", Position: "V", CreatedAt: stamp}

	for hiddenAfterMine := range 3 {
		t.Run(fmt.Sprintf("%d hidden cards since my last one", hiddenAfterMine), func(t *testing.T) {
			// given my card, then as many of someone else's hidden cards
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, hiddenCards, anonymous))
			m.column()
			listed := append([]model.Card{mine}, columnCards([]string{"W", "X"}[:hiddenAfterMine]...)...)
			m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(listed, nil)

			var placedAt string
			m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.CardCreate, _ ...*sql.Tx) (*model.Versioned[model.Card], error) {
				placedAt = s.Position
				return &model.Versioned[model.Card]{Value: model.Card{ID: s.CardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(myID), Body: s.Body, Position: s.Position}, Version: 8}, nil
			})

			// when
			got, err := svc.AddCard(t.Context(), addCardCommand())

			// then my next key follows my own last key, however many hidden cards came between
			require.NoError(t, err)
			assert.Equal(t, "W", placedAt)
			require.NotNil(t, got.Card)
			assert.Equal(t, "W", got.Card.Position)
		})
	}
}

func TestAddCard_AnAnonymousBoardNeverLooksUpTheAuthor(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anonymous))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return([]model.Card{}, nil)
	added := model.Card{ID: cardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(myID), Body: "Deploys were smooth", Position: "V", CreatedAt: stamp}
	m.cards.EXPECT().AddCard(mock.Anything, mock.MatchedBy(func(s spec.CardCreate) bool {
		return s.Position == "V" && s.Colour == nil
	})).Return(&model.Versioned[model.Card]{Value: added, Version: 8}, nil)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then the strict mocks prove no participant list was read
	require.NoError(t, err)
	require.NotNil(t, got.Card)
	assert.Nil(t, got.Card.Author)
}

func TestAddCard_RefusesACardIDThatIsNotV4(t *testing.T) {
	for _, id := range []uuid.UUID{uuid.Nil, v7ID} {
		// given
		svc, _ := newTestService(t)
		command := addCardCommand()
		command.CardID = id

		// when
		got, err := svc.AddCard(t.Context(), command)

		// then
		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrInvalidID)
	}
}

func TestCardFields_AreCheckedBeforeTheGate(t *testing.T) {
	cases := []struct {
		name   string
		change func(*AddCardCommand)
		want   error
	}{
		{name: "a blank body", change: func(c *AddCardCommand) { c.Body = " \n\t " }, want: ErrCardBodyRequired},
		{name: "a body over 2000 characters", change: func(c *AddCardCommand) { c.Body = strings.Repeat("é", 2001) }, want: ErrCardBodyTooLong},
		{name: "a colour outside the palette", change: func(c *AddCardCommand) { c.Colour = new(model.Colour("orange")) }, want: ErrInvalidColour},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)
			command := addCardCommand()
			tc.change(&command)

			// when
			got, err := svc.AddCard(t.Context(), command)

			// then
			assert.Nil(t, got)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAddCard_AMemberWhoHasNotJoinedCannotWrite(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, notJoined))

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotParticipant)
}

func TestAddCard_AnUnknownColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errNotFound)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}

func TestAddCard_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestAddCard_AStoredKeyItCannotOrderIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return(columnCards("not-a-key"), nil)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, fracindex.ErrInvalidKey)
}

func TestAddCard_AReusedIDIsAConflict(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(nil, dao.ErrConflict)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrIDConflict)
}

func TestAddCard_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrIDConflict)
}

func TestAddCard_AFailedAuthorLookupIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	m.cards.EXPECT().AddCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: card(myID), Version: 8}, nil)
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return(nil, errDatabase)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestAddCard_AReplayByTheAuthorReturnsTheirCard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer, hiddenCards), 5)
	m.stored(card(myID))
	m.participants()

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	require.NoError(t, err)
	assert.True(t, got.Replayed)
	assert.Equal(t, &cardID, got.ID)
	assert.Equal(t, int64(5), got.Version)
	require.NotNil(t, got.Card)
	assert.Equal(t, secret, got.Card.Body)
}

func TestAddCard_AReplayBySomeoneElseLearnsOnlyTheIDAndVersion(t *testing.T) {
	cases := []struct {
		name   string
		access *model.BoardAccess
		stored model.Card
	}{
		{name: "another participant", access: boardAccess(writer), stored: card(theirID)},
		{name: "a member who has not joined", access: boardAccess(host, notJoined), stored: card(theirID)},
		{name: "a card whose author link is severed", access: boardAccess(host), stored: model.Card{ID: cardID, BoardID: boardID, Body: secret}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.applied(tc.access, 5)
			m.stored(tc.stored)

			// when
			got, err := svc.AddCard(t.Context(), addCardCommand())

			// then
			require.NoError(t, err)
			assert.Equal(t, replayResponse(5, &cardID), got)
		})
	}
}

func TestAddCard_AReplayOfADeletedCardAnswersTheIDAndVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer), 5)
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(5, &cardID), got)
}

func TestAddCard_AReplayWhoseCardCannotBeReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer), 5)
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errDatabase)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestAddCard_AReplayWhoseAuthorCannotBeReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer), 5)
	m.stored(card(myID))
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return(nil, errDatabase)

	// when
	got, err := svc.AddCard(t.Context(), addCardCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestEditCard_TheAuthorEditsTheirCard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	edited := card(myID)
	edited.Body = "Deploys were mostly smooth"
	edited.UpdatedAt = new(stamp)
	m.cards.EXPECT().EditCard(mock.Anything, spec.CardBodyWrite{BoardWrite: target(), CardID: cardID, Body: "Deploys were mostly smooth"}).Return(&model.Versioned[model.Card]{Value: edited, Version: 8}, nil)
	m.participants()

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: " Deploys were mostly smooth "})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.Equal(t, "Deploys were mostly smooth", got.Card.Body)
}

func TestEditCard_AnAuthorWhoLeftShowsNoName(t *testing.T) {
	// given a named board whose card author is no longer in the participant list
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.cards.EXPECT().EditCard(mock.Anything, mock.Anything).Return(&model.Versioned[model.Card]{Value: card(myID), Version: 8}, nil)
	m.boards.EXPECT().ListParticipants(mock.Anything, boardID).Return([]model.BoardParticipantProfile{}, nil)

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Edited"})

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Card)
	if got.Card.Author != nil {
		assert.Empty(t, got.Card.Author.DisplayName)
	}
}

func TestEditCard_RefusesABlankBodyBeforeTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "   "})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrCardBodyRequired)
}

func TestEditCard_NobodyEditsSomeoneElsesCard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.stored(card(theirID))

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Edited"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestEditCard_SomeoneElsesHiddenCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))
	m.stored(card(theirID))

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Edited"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestEditCard_AnUnknownCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound)

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Edited"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestEditCard_AFailedCardReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errDatabase)

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Edited"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, policy.ErrCardNotFound)
}

func TestEditCard_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.cards.EXPECT().EditCard(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Edited"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestRecolourCard_SetsAndClearsTheColour(t *testing.T) {
	for _, colour := range []*model.Colour{new(model.ColourPurple), nil} {
		// given
		svc, m := newTestService(t)
		m.entered(boardAccess(writer, anonymous))
		m.stored(card(myID))
		recoloured := card(myID)
		recoloured.Colour = colour
		m.cards.EXPECT().RecolourCard(mock.Anything, spec.CardColourWrite{BoardWrite: target(), CardID: cardID, Colour: colour}).Return(&model.Versioned[model.Card]{Value: recoloured, Version: 8}, nil)

		// when
		got, err := svc.RecolourCard(t.Context(), RecolourCardCommand{Command: baseCommand(), CardID: cardID, Colour: colour})

		// then
		require.NoError(t, err)
		assert.Equal(t, colour, got.Card.Colour)
	}
}

func TestRecolourCard_RefusesAColourOutsideThePaletteBeforeTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.RecolourCard(t.Context(), RecolourCardCommand{Command: baseCommand(), CardID: cardID, Colour: new(model.Colour("orange"))})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrInvalidColour)
}

func TestRecolourCard_OnlyTheAuthorRecolours(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(theirID))

	// when
	got, err := svc.RecolourCard(t.Context(), RecolourCardCommand{Command: baseCommand(), CardID: cardID, Colour: new(model.ColourRed)})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestRecolourCard_AnUnknownCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound)

	// when
	got, err := svc.RecolourCard(t.Context(), RecolourCardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestRecolourCard_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.cards.EXPECT().RecolourCard(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.RecolourCard(t.Context(), RecolourCardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestMoveCard_PlacesTheCardAfterItsAnchorsCurrentPosition(t *testing.T) {
	cases := []struct {
		name     string
		after    *uuid.UUID
		position string
	}{
		{name: "to the top of the column", after: nil, position: "U"},
		{name: "between two cards", after: new(firstCardID), position: "c"},
		{name: "to the end", after: new(thirdCardID), position: "u"},
		{name: "to the end when the anchor is gone", after: new(otherID), position: "u"},
		{name: "to the end when the anchor is the card itself", after: new(cardID), position: "u"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, anonymous))
			m.stored(card(myID))
			m.column()
			m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(append(columnCards("V", "k", "t"), card(myID)), nil)
			moved := card(myID)
			moved.Position = tc.position
			m.cards.EXPECT().MoveCard(mock.Anything, spec.CardPlacement{BoardWrite: target(), CardID: cardID, ColumnID: columnID, Position: tc.position}).Return(&model.Versioned[model.Card]{Value: moved, Version: 8}, nil)

			// when
			got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID, AfterCardID: tc.after})

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.position, got.Card.Position)
		})
	}
}

func TestMoveCard_RewritesTheColumnWhenTheKeyWouldGrowTooLong(t *testing.T) {
	// given two neighbours whose gap has been split until no short key fits
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, anonymous))
	m.stored(card(myID))
	m.column()
	long := "V" + strings.Repeat("0", fracindex.MaxKeyLength) + "1"
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return(columnCards("V", long), nil)
	placing := spec.CardPlacement{
		BoardWrite: target(),
		CardID:     cardID,
		ColumnID:   columnID,
		Position:   "V",
		Rewrite:    []spec.PositionChange{{ID: firstCardID, Position: "F"}, {ID: secondCardID, Position: "k"}},
	}
	moved := card(myID)
	m.cards.EXPECT().MoveCard(mock.Anything, placing).Return(&model.Versioned[model.Card]{Value: moved, Version: 8}, nil)

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID, AfterCardID: new(firstCardID)})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
}

func TestMoveCard_LockedMovesStopAParticipant(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, movesLocked))
	m.stored(card(myID))

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrMovesLocked)
}

func TestMoveCard_SomeoneElsesHiddenCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))
	m.stored(card(theirID))

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestMoveCard_AnUnknownCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound)

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestMoveCard_AnUnknownTargetColumnIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(nil, errNotFound)

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}

func TestMoveCard_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestMoveCard_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.column()
	m.cards.EXPECT().ListInColumn(mock.Anything, mock.Anything).Return([]model.Card{}, nil)
	m.cards.EXPECT().MoveCard(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestDeleteCard_AFacilitatorRemovesAVisibleCard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.stored(card(theirID))
	m.cards.EXPECT().DeleteCard(mock.Anything, spec.CardRemoval{BoardWrite: target(), CardID: cardID}).Return(&model.Versioned[model.Card]{Value: card(theirID), Version: 8}, nil)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then the answer carries no trace of the deleted card
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestDeleteCard_AParticipantCannotRemoveSomeoneElsesCard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(theirID))

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestDeleteCard_AHiddenCardIsBeyondEvenTheFacilitator(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))
	m.stored(card(theirID))

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestDeleteCard_AnUnknownCardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.cards.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: boardID, CardID: cardID}).Return(nil, errNotFound)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestAddCard_IsRefusedOutsideTheWriteStageEvenToAFacilitator(t *testing.T) {
	for _, stage := range []model.Stage{model.StageVote, model.StageDiscuss, model.StageActions} {
		for _, role := range []model.BoardRole{writer, host} {
			t.Run(fmt.Sprintf("%s/%s", stage, role), func(t *testing.T) {
				// given
				svc, m := newTestService(t)
				m.entered(boardAccess(role, atStage(stage)))

				// when
				got, err := svc.AddCard(t.Context(), addCardCommand())

				// then the strict card mock proves nothing was written
				assert.Nil(t, got)
				require.ErrorIs(t, err, policy.ErrCardsOutOfStage)
				assert.Empty(t, m.events.published)
			})
		}
	}
}

func TestCardCommands_OnYourOwnCardAreRefusedOutsideTheWriteStage(t *testing.T) {
	commands := []struct {
		name string
		when func(svc *service) error
	}{
		{name: "edit", when: func(svc *service) error {
			_, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Deploys were mostly smooth"})
			return err
		}},
		{name: "recolour", when: func(svc *service) error {
			_, err := svc.RecolourCard(t.Context(), RecolourCardCommand{Command: baseCommand(), CardID: cardID, Colour: new(model.ColourPurple)})
			return err
		}},
		{name: "move", when: func(svc *service) error {
			_, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})
			return err
		}},
		{name: "delete", when: func(svc *service) error {
			_, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})
			return err
		}},
	}

	for _, stage := range []model.Stage{model.StageVote, model.StageDiscuss, model.StageActions} {
		for _, command := range commands {
			t.Run(fmt.Sprintf("%s/%s", stage, command.name), func(t *testing.T) {
				// given the participant's own card, once the board has left its Write stage
				svc, m := newTestService(t)
				m.entered(boardAccess(writer, atStage(stage)))
				m.stored(card(myID))

				// when
				err := command.when(svc)

				// then the strict card mock proves nothing was written
				require.ErrorIs(t, err, policy.ErrCardsOutOfStage)
				assert.Empty(t, m.events.published)
			})
		}
	}
}

func TestMoveCard_AFacilitatorIsBoundByTheWriteStageToo(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageVote)))
	m.stored(card(theirID))

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCardsOutOfStage)
}

func TestMoveCard_AParticipantMovesOnlyTheirOwnCardsInTheWriteStage(t *testing.T) {
	// given someone else's shown card
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(theirID))

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: columnID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestDeleteCard_AFacilitatorStillRemovesAVisibleCardInALaterStage(t *testing.T) {
	// given a card that has to go while the team discusses
	svc, m := newTestService(t)
	m.entered(boardAccess(host, atStage(model.StageDiscuss)))
	m.stored(card(theirID))
	m.cards.EXPECT().DeleteCard(mock.Anything, spec.CardRemoval{BoardWrite: target(), CardID: cardID}).Return(&model.Versioned[model.Card]{Value: card(theirID), Version: 8}, nil)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then moderation is not content, so the stage does not stop it
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestDeleteCard_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(card(myID))
	m.cards.EXPECT().DeleteCard(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func secondColumnRevealed(access *model.BoardAccess) {
	access.Header.RevealedColumnIDs = []uuid.UUID{secondColumnID}
}

func TestAddCard_AHiddenColumnIsFrozenForAFacilitatorAndUnknownToAParticipant(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, columnHidden))
	m.column()
	m.entered(boardAccess(writer, columnHidden))

	// when
	facilitated, facilitatorErr := svc.AddCard(t.Context(), addCardCommand())
	written, participantErr := svc.AddCard(t.Context(), addCardCommand())

	// then the strict mocks prove no card was written
	assert.Nil(t, facilitated)
	assert.Nil(t, written)
	assert.ErrorIs(t, facilitatorErr, policy.ErrColumnHidden)
	assert.ErrorIs(t, participantErr, ErrColumnNotFound)
}

func TestMoveCard_IntoARevealedColumnFromOneWhoseCardsAreHiddenIsRefused(t *testing.T) {
	// given my hidden card in the first column, and the second column revealed
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hiddenCards, secondColumnRevealed))
	m.stored(card(myID))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: secondColumnID}).Return(&model.BoardColumn{ID: secondColumnID, BoardID: boardID}, nil).Once()

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: secondColumnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrMoveAcrossReveal)
}

func TestMoveCard_IntoAHiddenColumnIsUnknownToAParticipant(t *testing.T) {
	// given
	svc, m := newTestService(t)
	hiddenSecond := func(access *model.BoardAccess) {
		access.Header.HiddenColumnIDs = []uuid.UUID{secondColumnID}
	}
	m.entered(boardAccess(writer, hiddenSecond))
	m.stored(card(myID))

	// when
	got, err := svc.MoveCard(t.Context(), MoveCardCommand{Command: baseCommand(), CardID: cardID, ColumnID: secondColumnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrColumnNotFound)
}
