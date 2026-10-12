package actionitems

import (
	"strings"
	"testing"
	"time"

	"retrotide/internal/board"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	itemID        = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000001")
	olderBoardID  = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000006")
	aliceID       = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000002")
	outsiderID    = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000003")
	adminID       = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000004")
	guestDeviceID = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000005")
	due           = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
)

func inActions(access *model.BoardAccess) {
	access.Header.Stage = model.StageActions
}

func asGuest(access *model.BoardAccess) {
	access.Principal = model.Principal{Kind: model.PrincipalGuest, ID: guestDeviceID}
	access.OrgMember = false
	access.Capabilities = policy.Capabilities(access.Role, false)
}

func ended(access *model.BoardAccess) {
	access.Header.ClosedAt = new(stamp)
	access.Header.Stage = model.StageEnd
	access.Capabilities = []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityExport}
}

func boardItem(options ...func(*model.ActionItem)) *model.ActionItem {
	item := &model.ActionItem{
		ID:        itemID,
		OrgID:     orgID,
		TeamID:    new(teamID),
		BoardID:   new(boardID),
		Title:     "Fix the deploy pipeline",
		Status:    model.ActionItemOpen,
		CreatedAt: stamp,
		UpdatedAt: stamp,
		Mentions:  []model.ActionItemPerson{},
	}

	for _, apply := range options {
		apply(item)
	}

	return item
}

func addCommand(fields Fields) AddCommand {
	return AddCommand{Command: baseCommand(), Fields: fields, ItemID: itemID, SourceCardID: new(cardID)}
}

func cardRef() spec.CardRef {
	return spec.CardRef{BoardID: boardID, CardID: cardID}
}

func hidingCards(access *model.BoardAccess) {
	access.Header.CardsHidden = true
}

func (m *testMocks) people() {
	m.organisations.EXPECT().ListMembers(mock.Anything, orgID).Return([]model.OrganisationMemberProfile{
		{OrgID: orgID, UserID: aliceID, Role: model.OrgRoleMember, DisplayName: "Alice"},
		{OrgID: orgID, UserID: outsiderID, Role: model.OrgRoleMember, DisplayName: "Omar"},
		{OrgID: orgID, UserID: adminID, Role: model.OrgRoleAdmin, DisplayName: "bea"},
	}, nil).Once()
	m.teams.EXPECT().ListMembers(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return([]model.TeamMemberProfile{
		{OrgID: orgID, TeamID: teamID, UserID: aliceID, Role: model.TeamRoleMember, DisplayName: "Alice"},
	}, nil).Once()
}

func TestAddActionItem_WritesAndAnnouncesTheItemInTheActionsStage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	m.people()
	fields := Fields{Title: "  Fix the deploy pipeline  ", AssigneeUserID: new(aliceID), DueOn: new("2026-10-20"), Mentions: []uuid.UUID{adminID, adminID}}
	written := boardItem(func(item *model.ActionItem) {
		item.SourceCardID = new(cardID)
		item.AssigneeUserID = new(aliceID)
		item.AssigneeName = "Alice"
		item.DueOn = new(due)
		item.Mentions = []model.ActionItemPerson{{UserID: adminID, DisplayName: "bea"}}
	})
	m.actionItems.EXPECT().AddActionItem(mock.Anything, spec.ActionItemCreate{
		BoardWrite: target(),
		Title:      "Fix the deploy pipeline", AssigneeUserID: new(aliceID), DueOn: new(due), Mentions: []uuid.UUID{adminID},
		ItemID:       itemID,
		TeamID:       new(teamID),
		SourceCardID: new(cardID),
		CreatedBy:    new(userID),
	}).Return(&model.Versioned[model.ActionItem]{Value: *written, Version: 8}, nil).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(fields))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.ActionItem)
	assert.Equal(t, "Fix the deploy pipeline", got.ActionItem.Title)
	assert.Equal(t, new("2026-10-20"), got.ActionItem.DueOn)
	assert.Equal(t, new(cardID), got.ActionItem.SourceCardID)
	require.Len(t, m.events.published, 1)
	upserted, ok := m.events.published[0].event.Payload(projection.ClassOthers).(dto.ActionItemUpsertedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventActionItemUpserted, upserted.Type)
	assert.Equal(t, []teamAnnouncement{{teamID: teamID, exclude: []uuid.UUID{boardID}}}, m.events.teams)
}

func TestAddActionItem_TellsOnlyItsCreatorThatItIsTheirs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	written := boardItem(func(item *model.ActionItem) { item.CreatedBy = new(userID) })
	m.actionItems.EXPECT().AddActionItem(mock.Anything, mock.Anything).Return(&model.Versioned[model.ActionItem]{Value: *written, Version: 8}, nil).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

	// then the response and the creator's own event say it is theirs, and everyone else's payload does not
	require.NoError(t, err)
	require.NotNil(t, got.ActionItem)
	assert.True(t, got.ActionItem.IsMine)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	shared, ok := event.Payload(projection.ClassOthers).(dto.ActionItemUpsertedEvent)
	require.True(t, ok)
	assert.False(t, shared.ActionItem.IsMine)
	personal, isPersonal := event.PayloadFor(projection.ViewerFor(*boardAccess(writer, inActions)))
	require.True(t, isPersonal)
	assert.True(t, personal.(dto.ActionItemUpsertedEvent).ActionItem.IsMine)
	_, otherPersonal := event.PayloadFor(projection.Viewer{Kind: projection.ViewerMember, UserID: new(aliceID), ParticipantID: new(theirID)})
	assert.False(t, otherPersonal)
}

func TestAddActionItem_OnAnAnonymousBoardStoresNoCreator(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions, anonymous))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	m.actionItems.EXPECT().AddActionItem(mock.Anything, spec.ActionItemCreate{
		BoardWrite:   target(),
		Title:        "Fix it",
		ItemID:       itemID,
		TeamID:       new(teamID),
		SourceCardID: new(cardID),
	}).Return(&model.Versioned[model.ActionItem]{Value: *boardItem(), Version: 8}, nil).Once()

	// when
	_, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

	// then the strict mock proves CreatedBy was left empty
	require.NoError(t, err)
}

func TestAddActionItem_MakesAnotherItemFromACardThatAlreadyHasOne(t *testing.T) {
	// given a card that already carries an action item
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	m.actionItems.EXPECT().AddActionItem(mock.Anything, mock.MatchedBy(func(s spec.ActionItemCreate) bool {
		return s.SourceCardID != nil && *s.SourceCardID == cardID
	})).Return(&model.Versioned[model.ActionItem]{Value: *boardItem(func(item *model.ActionItem) { item.SourceCardID = new(cardID) }), Version: 8}, nil).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Follow up on the deploy"}))

	// then
	require.NoError(t, err)
	assert.Equal(t, new(cardID), got.ActionItem.SourceCardID)
}

func TestAddActionItem_AnOwnCardMayBeUsedWhileTheBoardHidesCards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions, hidingCards))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(myID)), nil).Once()
	m.actionItems.EXPECT().AddActionItem(mock.Anything, mock.Anything).Return(&model.Versioned[model.ActionItem]{Value: *boardItem(func(item *model.ActionItem) { item.SourceCardID = new(cardID) }), Version: 8}, nil).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
}

func TestAddActionItem_RefusesACardTheWriterCannotUse(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		card   *model.Card
		err    error
	}{
		{name: "a card that is not on the board", access: boardAccess(writer, inActions), err: errNotFound},
		{name: "someone else's card while the board hides cards", access: boardAccess(writer, inActions, hidingCards), card: new(card(theirID))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)
			m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(tc.card, tc.err).Once()

			// when
			got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

			// then the strict mocks prove nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrCardNotFound)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestAddActionItem_RefusesAnIDAlreadyTaken(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	m.actionItems.EXPECT().AddActionItem(mock.Anything, mock.Anything).Return(nil, dao.ErrConflict).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, board.ErrIDConflict)
	assert.Empty(t, m.events.published)
}

func TestAddActionItem_IsRefusedOutsideTheActionsStageFacilitatorsIncluded(t *testing.T) {
	for _, stage := range []model.Stage{model.StageWrite, model.StageVote, model.StageDiscuss} {
		t.Run(string(stage), func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(host, atStage(stage)))

			// when
			got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrActionItemsOutOfStage)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestAddActionItem_RefusesAPersonWhoCannotOpenTheBoard(t *testing.T) {
	// given omar, who is in the organisation but not on the board's team
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()
	m.people()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it", Mentions: []uuid.UUID{outsiderID}}))

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, board.ErrNotBoardMember)
}

func TestAddActionItem_AGuestMayAddButNeverNameAMember(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions, asGuest))
	m.cards.EXPECT().GetByID(mock.Anything, cardRef()).Return(new(card(theirID)), nil).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it", AssigneeUserID: new(aliceID)}))

	// then the strict mocks prove the member list was never read
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrMembersOnly)
}

func TestAddActionItem_ValidatesItsFieldsBeforeTheGate(t *testing.T) {
	tests := []struct {
		name    string
		command AddCommand
		want    error
	}{
		{name: "a blank title", command: addCommand(Fields{Title: "   "}), want: ErrTitleRequired},
		{name: "a long title", command: addCommand(Fields{Title: strings.Repeat("a", 501)}), want: ErrTitleTooLong},
		{name: "a malformed date", command: addCommand(Fields{Title: "Fix it", DueOn: new("20/10/2026")}), want: ErrInvalidDueDate},
		{name: "too many mentions", command: addCommand(Fields{Title: "Fix it", Mentions: manyPeople(21)}), want: board.ErrTooManyMentions},
		{name: "a v7 ID", command: AddCommand{Command: baseCommand(), Fields: Fields{Title: "Fix it"}, ItemID: v7ID, SourceCardID: new(cardID)}, want: board.ErrInvalidID},
		{name: "no card", command: AddCommand{Command: baseCommand(), Fields: Fields{Title: "Fix it"}, ItemID: itemID}, want: ErrCardRequired},
		{name: "an empty card", command: AddCommand{Command: baseCommand(), Fields: Fields{Title: "Fix it"}, ItemID: itemID, SourceCardID: new(uuid.Nil)}, want: ErrCardRequired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.Add(t.Context(), tc.command)

			// then the strict mocks prove the gate was never asked for
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAddActionItem_AReplayAnswersTheStoredItem(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer, inActions), 6)
	m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(boardItem(), nil).Once()

	// when
	got, err := svc.Add(t.Context(), addCommand(Fields{Title: "Fix it"}))

	// then
	require.NoError(t, err)
	assert.True(t, got.Replayed)
	assert.Equal(t, new(itemID), got.ID)
	require.NotNil(t, got.ActionItem)
}

func TestEditActionItem_KeepsAnExistingAssigneeWithoutAskingWhoCanOpenTheBoard(t *testing.T) {
	// given an item already assigned to someone who has since left the team
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	current := boardItem(func(item *model.ActionItem) { item.AssigneeUserID = new(outsiderID) })
	m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(current, nil).Once()
	edit := spec.ActionItemEditWrite{BoardWrite: target(), Title: "Fix it today", AssigneeUserID: new(outsiderID), ItemID: itemID}
	m.actionItems.EXPECT().EditActionItem(mock.Anything, edit).Return(&model.Versioned[model.ActionItem]{Value: *current, Version: 8}, nil).Once()

	// when
	got, err := svc.Edit(t.Context(), EditCommand{Command: baseCommand(), Title: "Fix it today", AssigneeUserID: new(outsiderID), ItemID: itemID})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.Len(t, m.events.published, 1)
}

func TestEditActionItem_AnItemOfAnotherBoardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(boardItem(func(item *model.ActionItem) { item.BoardID = new(olderBoardID) }), nil).Once()

	// when
	got, err := svc.Edit(t.Context(), EditCommand{Command: baseCommand(), Title: "Fix it", ItemID: itemID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrActionItemNotFound)
}

func TestSetActionItemStatus_MarksTheItemInTheActionsStage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(boardItem(), nil).Once()
	done := boardItem(func(item *model.ActionItem) { item.Status = model.ActionItemDone })
	m.actionItems.EXPECT().SetActionItemStatus(mock.Anything, spec.ActionItemStatusWrite{BoardWrite: target(), ItemID: itemID, Status: model.ActionItemDone}).Return(&model.Versioned[model.ActionItem]{Value: *done, Version: 8}, nil).Once()

	// when
	got, err := svc.SetStatus(t.Context(), StatusCommand{Command: baseCommand(), ItemID: itemID, Status: model.ActionItemDone})

	// then
	require.NoError(t, err)
	assert.Equal(t, model.ActionItemDone, got.ActionItem.Status)
	require.Len(t, m.events.published, 1)
}

func TestSetActionItemStatus_AMemberMayStillMarkAnItemAfterTheRetroEnds(t *testing.T) {
	// given an ended retro, opened from the Action Items page by a member
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, ended, notJoined))
	m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(boardItem(), nil).Once()
	m.actionItems.EXPECT().SetActionItemStatus(mock.Anything, mock.Anything).Return(&model.Versioned[model.ActionItem]{Value: *boardItem(), Version: 8}, nil).Once()

	// when
	got, err := svc.SetStatus(t.Context(), StatusCommand{Command: baseCommand(), ItemID: itemID, Status: model.ActionItemDropped})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
}

func TestSetActionItemStatus_IsRefusedToAGuestAfterTheEndAndToAnyoneBeforeActions(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "a guest on an ended retro", access: boardAccess(writer, ended, asGuest), want: policy.ErrReadOnly},
		{name: "a facilitator in Discuss", access: boardAccess(host, atStage(model.StageDiscuss)), want: policy.ErrActionItemsOutOfStage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)

			// when
			got, err := svc.SetStatus(t.Context(), StatusCommand{Command: baseCommand(), ItemID: itemID, Status: model.ActionItemDone})

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestSetActionItemStatus_TheStatusItAlreadyHasWritesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, inActions))
	m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(boardItem(), nil).Once()

	// when
	got, err := svc.SetStatus(t.Context(), StatusCommand{Command: baseCommand(), ItemID: itemID, Status: model.ActionItemOpen})

	// then the strict mock proves nothing was written
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.Version)
	assert.Empty(t, m.events.published)
}

func TestSetActionItemStatus_RefusesAnUnknownStatusBeforeTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.SetStatus(t.Context(), StatusCommand{Command: baseCommand(), ItemID: itemID, Status: "finished"})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrInvalidStatus)
}

func TestDeleteActionItem_FollowsWhoMayDelete(t *testing.T) {
	tests := []struct {
		name    string
		access  *model.BoardAccess
		creator *uuid.UUID
		want    error
	}{
		{name: "a facilitator, in Discuss after going back", access: boardAccess(host, atStage(model.StageDiscuss))},
		{name: "the creator", access: boardAccess(writer, inActions), creator: new(userID)},
		{name: "another participant", access: boardAccess(writer, inActions), creator: new(aliceID), want: policy.ErrNotAllowed},
		{name: "a participant on an anonymous board, which stores no creator", access: boardAccess(writer, inActions, anonymous), want: policy.ErrNotAllowed},
		{name: "a creator stored before the board turned anonymous", access: boardAccess(writer, inActions, anonymous), creator: new(userID), want: policy.ErrNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)
			item := boardItem(func(item *model.ActionItem) { item.CreatedBy = tc.creator })
			m.actionItems.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: orgID, ItemID: itemID}).Return(item, nil).Once()
			if tc.want == nil {
				m.actionItems.EXPECT().DeleteActionItem(mock.Anything, spec.ActionItemRemoval{BoardWrite: target(), ItemID: itemID}).Return(&model.Versioned[model.ActionItem]{Value: *item, Version: 8}, nil).Once()
			}

			// when
			got, err := svc.Delete(t.Context(), ItemCommand{Command: baseCommand(), ItemID: itemID})

			// then
			if tc.want != nil {
				assert.Nil(t, got)
				require.ErrorIs(t, err, tc.want)
				assert.Empty(t, m.events.published)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, versionResponse(8), got)
			require.Len(t, m.events.published, 1)
			assert.Equal(t, dto.ActionItemRemovedEvent{Type: projection.EventActionItemRemoved, Version: 8, ActionItemID: itemID}, m.events.published[0].event.Payload(projection.ClassOthers))
		})
	}
}

func previousOf(status model.ActionItemStatus, raisedOn *uuid.UUID) model.PreviousAction {
	return model.PreviousAction{ActionItem: *boardItem(func(item *model.ActionItem) {
		item.Status = status
		item.BoardID = raisedOn
	})}
}

func (m *testMocks) previous(actions ...model.PreviousAction) {
	query := spec.PreviousActionsQuery{OrgID: orgID, TeamID: teamID, BoardID: boardID, Before: stamp}
	m.actionItems.EXPECT().ListPrevious(mock.Anything, query).Return(actions, nil).Once()
}

func TestReviewActionItem_MarkingDoneBumpsTheItemsOwnBoardToo(t *testing.T) {
	// given an open action from an older board of the team
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.previous(previousOf(model.ActionItemOpen, new(olderBoardID)))
	m.acquired++
	m.live.EXPECT().Acquire(mock.Anything, olderBoardID).Return(func() {
		m.released++
	}, nil).Once()
	older := &model.BoardHeader{ID: olderBoardID, OrgID: orgID, TeamID: new(teamID), Version: 30, Stage: model.StageEnd, ClosedAt: new(stamp)}
	m.boards.EXPECT().GetHeader(mock.Anything, olderBoardID).Return(older, nil).Once()
	write := spec.ActionItemReviewWrite{
		BoardWrite: target(),
		ItemID:     itemID,
		Outcome:    model.ReviewDone,
		Status:     new(model.ActionItemDone),
		ItemBoard:  &spec.BoardWrite{OrgID: orgID, BoardID: olderBoardID, Version: 30},
	}
	reviewed := model.ActionItemReviewed{
		Item:             model.PreviousAction{ActionItem: *boardItem(func(item *model.ActionItem) { item.Status = model.ActionItemDone }), Review: new(model.ReviewDone)},
		ItemBoardVersion: new(int64(31)),
	}
	m.actionItems.EXPECT().ReviewActionItem(mock.Anything, write).Return(&model.Versioned[model.ActionItemReviewed]{Value: reviewed, Version: 8}, nil).Once()

	// when
	got, err := svc.Review(t.Context(), ReviewCommand{Command: baseCommand(), ItemID: itemID, Outcome: model.ReviewDone})

	// then this board hears about the review and the older board about its item, each at its own version
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.PreviousAction)
	assert.Equal(t, new(model.ReviewDone), got.PreviousAction.Review)
	require.Len(t, m.events.published, 2)
	assert.Equal(t, boardID, m.events.published[0].access.Header.ID)
	assert.Equal(t, int64(8), m.events.published[0].event.Version())
	assert.Equal(t, olderBoardID, m.events.published[1].access.Header.ID)
	assert.Equal(t, int64(31), m.events.published[1].event.Version())
	assert.Equal(t, []teamAnnouncement{{teamID: teamID, exclude: []uuid.UUID{boardID, olderBoardID}}}, m.events.teams)
}

func TestReviewActionItem_CarryingAnOpenItemOverWritesOnlyTheReview(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.previous(previousOf(model.ActionItemOpen, new(olderBoardID)))
	write := spec.ActionItemReviewWrite{BoardWrite: target(), ItemID: itemID, Outcome: model.ReviewCarried}
	reviewed := model.ActionItemReviewed{Item: model.PreviousAction{ActionItem: *boardItem(), Review: new(model.ReviewCarried)}}
	m.actionItems.EXPECT().ReviewActionItem(mock.Anything, write).Return(&model.Versioned[model.ActionItemReviewed]{Value: reviewed, Version: 8}, nil).Once()

	// when
	got, err := svc.Review(t.Context(), ReviewCommand{Command: baseCommand(), ItemID: itemID, Outcome: model.ReviewCarried})

	// then the strict mocks prove no second gate was taken
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.Len(t, m.events.published, 1)
}

func TestReviewActionItem_AnItemNotUpForReviewIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.previous()

	// when
	got, err := svc.Review(t.Context(), ReviewCommand{Command: baseCommand(), ItemID: itemID, Outcome: model.ReviewDone})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrActionItemNotFound)
}

func TestReviewActionItem_IsRefusedToAGuestAndOnAnOrganisationBoard(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "a guest", access: boardAccess(writer, asGuest), want: policy.ErrMembersOnly},
		{name: "an organisation board", access: boardAccess(writer, func(access *model.BoardAccess) { access.Header.TeamID = nil }), want: policy.ErrActionItemNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)

			// when
			got, err := svc.Review(t.Context(), ReviewCommand{Command: baseCommand(), ItemID: itemID, Outcome: model.ReviewDone})

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestReviewActionItem_RefusesAnUnknownOutcomeBeforeTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.Review(t.Context(), ReviewCommand{Command: baseCommand(), ItemID: itemID, Outcome: "ignored"})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrInvalidReviewOutcome)
}

func manyPeople(count int) []uuid.UUID {
	people := make([]uuid.UUID, 0, count)
	for range count {
		people = append(people, uuid.New())
	}

	return people
}
