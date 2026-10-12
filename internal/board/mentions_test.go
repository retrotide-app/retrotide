package board

import (
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	aliceID       = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000002")
	outsiderID    = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000003")
	adminID       = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000004")
	guestDeviceID = uuid.MustParse("4d5e6f70-0000-4000-8000-000000000005")

	alice = model.ActionItemPerson{UserID: aliceID, DisplayName: "Alice"}
	omar  = model.ActionItemPerson{UserID: outsiderID, DisplayName: "Omar"}
)

func asGuest(access *model.BoardAccess) {
	access.Principal = model.Principal{Kind: model.PrincipalGuest, ID: guestDeviceID}
	access.OrgMember = false
	access.Capabilities = policy.Capabilities(access.Role, false)
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

func manyPeople(count int) []uuid.UUID {
	people := make([]uuid.UUID, 0, count)
	for range count {
		people = append(people, uuid.New())
	}

	return people
}

func TestPeople_ListsOnlyMembersWhoCanOpenTheTeamBoard(t *testing.T) {
	// given alice on the team, an organisation admin, and omar who is in the organisation but not on the team
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(writer), nil).Once()
	m.people()

	// when
	got, err := svc.People(t.Context(), spec.BoardAccessQuery{Principal: me, BoardID: boardID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardPeopleResponse{People: []dto.PersonView{
		{UserID: aliceID, DisplayName: "Alice"},
		{UserID: adminID, DisplayName: "bea"},
	}}, got)
}

func TestPeople_ListsEveryOrganisationMemberOnAnOrganisationBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(writer, func(access *model.BoardAccess) {
		access.Header.TeamID = nil
	})
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil).Once()
	m.organisations.EXPECT().ListMembers(mock.Anything, orgID).Return([]model.OrganisationMemberProfile{
		{OrgID: orgID, UserID: outsiderID, Role: model.OrgRoleMember, DisplayName: "Omar"},
		{OrgID: orgID, UserID: aliceID, Role: model.OrgRoleMember, DisplayName: "Alice"},
	}, nil).Once()

	// when
	got, err := svc.People(t.Context(), spec.BoardAccessQuery{Principal: me, BoardID: boardID})

	// then
	require.NoError(t, err)
	assert.Equal(t, []dto.PersonView{{UserID: aliceID, DisplayName: "Alice"}, {UserID: outsiderID, DisplayName: "Omar"}}, got.People)
}

func TestPeople_IsRefusedToAGuest(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(writer, asGuest), nil).Once()

	// when
	got, err := svc.People(t.Context(), spec.BoardAccessQuery{Principal: me, BoardID: boardID})

	// then the strict mocks prove no member list was read
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrMembersOnly)
}

func mentioning(people ...model.ActionItemPerson) func(*model.Card) {
	return func(card *model.Card) {
		card.Mentions = people
	}
}

func mentionedCard(author uuid.UUID, options ...func(*model.Card)) model.Card {
	stored := card(author)
	for _, apply := range options {
		apply(&stored)
	}

	return stored
}

func TestAddCard_StoresTheMembersItMentionsAndAnnouncesThemWithTheCard(t *testing.T) {
	// given a member writing a card that names Alice twice
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.people()
	m.cards.EXPECT().ListInColumn(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return([]model.Card{}, nil)
	added := mentionedCard(myID, mentioning(alice))
	m.cards.EXPECT().AddCard(mock.Anything, mock.MatchedBy(func(s spec.CardCreate) bool {
		return assert.ObjectsAreEqual([]uuid.UUID{aliceID}, s.Mentions)
	})).Return(&model.Versioned[model.Card]{Value: added, Version: 8}, nil).Once()
	m.participants()
	command := addCardCommand()
	command.Body = "Thanks @Alice and @Alice"
	command.Mentions = []uuid.UUID{aliceID, aliceID}

	// when
	got, err := svc.AddCard(t.Context(), command)

	// then the duplicate is folded, and the card names Alice by user ID and name only
	require.NoError(t, err)
	require.NotNil(t, got.Card)
	assert.Equal(t, []dto.PersonView{{UserID: aliceID, DisplayName: "Alice"}}, got.Card.Mentions)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	require.Equal(t, projection.ClassOthers, event.ClassOf(projection.Viewer{ParticipantID: new(theirID)}))
	assert.Equal(t, []dto.PersonView{{UserID: aliceID, DisplayName: "Alice"}}, event.Payload(projection.ClassOthers).(dto.CardUpsertedEvent).Card.Mentions)
}

func TestAddCard_RefusesAMentionOfSomeoneWhoCannotOpenTheBoard(t *testing.T) {
	// given omar, who is in the organisation but not on the board's team
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.column()
	m.people()
	command := addCardCommand()
	command.Mentions = []uuid.UUID{outsiderID}

	// when
	got, err := svc.AddCard(t.Context(), command)

	// then the strict mocks prove nothing was written
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrNotBoardMember)
	assert.Empty(t, m.events.published)
}

func TestAddCard_AGuestOrALinkOnlyAccountNeverMentionsAnyone(t *testing.T) {
	tests := []struct {
		name   string
		option accessOption
	}{
		{name: "a guest", option: asGuest},
		{name: "an account in through the link alone", option: func(access *model.BoardAccess) { access.OrgMember = false }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, tc.option))
			m.column()
			command := addCardCommand()
			command.Mentions = []uuid.UUID{aliceID}

			// when
			got, err := svc.AddCard(t.Context(), command)

			// then the strict mocks prove the member list was never read
			assert.Nil(t, got)
			require.ErrorIs(t, err, policy.ErrMembersOnly)
		})
	}
}

func TestAddCard_RefusesBadMentionsBeforeTheGate(t *testing.T) {
	tests := []struct {
		name     string
		mentions []uuid.UUID
		want     error
	}{
		{name: "too many people", mentions: manyPeople(21), want: ErrTooManyMentions},
		{name: "an empty user ID", mentions: []uuid.UUID{uuid.Nil}, want: ErrNotBoardMember},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)
			command := addCardCommand()
			command.Mentions = tc.mentions

			// when
			got, err := svc.AddCard(t.Context(), command)

			// then the strict mocks prove the gate was never asked for
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAddCard_OutsideTheWriteStageIsRefusedBeforeAnyoneIsLookedUp(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, atStage(model.StageVote)))
	command := addCardCommand()
	command.Mentions = []uuid.UUID{aliceID}

	// when
	got, err := svc.AddCard(t.Context(), command)

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCardsOutOfStage)
}

func TestEditCard_ChecksOnlyTheNewlyNamedAndSendsTheWholeList(t *testing.T) {
	// given a card that already names omar, who has since left the team, and an edit that adds Alice
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))
	m.stored(mentionedCard(myID, mentioning(omar)))
	m.people()
	edited := mentionedCard(myID, mentioning(alice, omar))
	write := spec.CardBodyWrite{BoardWrite: target(), CardID: cardID, Body: "Thanks @Omar and @Alice", Mentions: []uuid.UUID{outsiderID, aliceID}}
	m.cards.EXPECT().EditCard(mock.Anything, write).Return(&model.Versioned[model.Card]{Value: edited, Version: 8}, nil).Once()
	m.participants()

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Thanks @Omar and @Alice", Mentions: []uuid.UUID{outsiderID, aliceID}})

	// then omar is kept without being checked again, and the repository diffs the rest
	require.NoError(t, err)
	require.NotNil(t, got.Card)
	assert.Len(t, got.Card.Mentions, 2)
}

func TestEditCard_KeepingOrDroppingMentionsReadsNoMemberList(t *testing.T) {
	tests := []struct {
		name     string
		mentions []uuid.UUID
	}{
		{name: "every mention kept", mentions: []uuid.UUID{outsiderID}},
		{name: "every mention dropped", mentions: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given a card that names omar
			svc, m := newTestService(t)
			m.entered(boardAccess(writer))
			m.stored(mentionedCard(myID, mentioning(omar)))
			write := spec.CardBodyWrite{BoardWrite: target(), CardID: cardID, Body: "Deploys were smooth", Mentions: tc.mentions}
			m.cards.EXPECT().EditCard(mock.Anything, write).Return(&model.Versioned[model.Card]{Value: card(myID), Version: 8}, nil).Once()
			m.participants()

			// when
			_, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Deploys were smooth", Mentions: tc.mentions})

			// then the strict mocks prove nobody was looked up
			require.NoError(t, err)
		})
	}
}

func TestEditCard_AHiddenCardOfSomeoneElseAnswersNotFoundBeforeAnyoneIsLookedUp(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, hiddenCards))
	m.stored(card(theirID))

	// when
	got, err := svc.EditCard(t.Context(), EditCardCommand{Command: baseCommand(), CardID: cardID, Body: "Planted", Mentions: []uuid.UUID{aliceID}})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCardNotFound)
}

func TestAddComment_StoresTheMembersItMentions(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn))
	m.stored(card(theirID))
	m.people()
	added := comment(myID)
	added.Mentions = []model.ActionItemPerson{alice}
	m.comments.EXPECT().AddComment(mock.Anything, spec.CommentCreate{
		BoardWrite:          target(),
		CommentID:           commentID,
		CardID:              cardID,
		AuthorParticipantID: new(myID),
		Body:                "Agreed, @Alice",
		Mentions:            []uuid.UUID{aliceID},
	}).Return(&model.Versioned[model.CardComment]{Value: added, Version: 8}, nil).Once()
	m.participants()
	command := addCommentCommand()
	command.Body = "Agreed, @Alice"
	command.Mentions = []uuid.UUID{aliceID}

	// when
	got, err := svc.AddComment(t.Context(), command)

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Comment)
	assert.Equal(t, []dto.PersonView{{UserID: aliceID, DisplayName: "Alice"}}, got.Comment.Mentions)
}

func TestAddComment_MentionsAreRefusedToAGuestAndOutsideDiscuss(t *testing.T) {
	tests := []struct {
		name    string
		options []accessOption
		want    error
	}{
		{name: "a guest in Discuss", options: []accessOption{commentsOn, asGuest}, want: policy.ErrMembersOnly},
		{name: "a member in Write", options: nil, want: policy.ErrCommentsOutOfStage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, tc.options...))
			m.stored(card(theirID))
			command := addCommentCommand()
			command.Mentions = []uuid.UUID{aliceID}

			// when
			got, err := svc.AddComment(t.Context(), command)

			// then the strict mocks prove the member list was never read and nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestEditComment_ChecksOnlyTheNewlyNamed(t *testing.T) {
	// given a comment that already names omar, and an edit that swaps him for Alice
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn))
	stored := comment(myID)
	stored.Mentions = []model.ActionItemPerson{omar}
	m.storedComment(stored)
	m.stored(card(theirID))
	m.people()
	edited := comment(myID)
	edited.Mentions = []model.ActionItemPerson{alice}
	write := spec.CommentBodyWrite{BoardWrite: target(), CommentID: commentID, Body: "Agreed, @Alice", Mentions: []uuid.UUID{aliceID}}
	m.comments.EXPECT().EditComment(mock.Anything, write).Return(&model.Versioned[model.CardComment]{Value: edited, Version: 8}, nil).Once()
	m.participants()

	// when
	got, err := svc.EditComment(t.Context(), EditCommentCommand{Command: baseCommand(), CommentID: commentID, Body: "Agreed, @Alice", Mentions: []uuid.UUID{aliceID}})

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Comment)
	assert.Equal(t, []dto.PersonView{{UserID: aliceID, DisplayName: "Alice"}}, got.Comment.Mentions)
}

func TestEditComment_RefusesANewMentionOfSomeoneWhoCannotOpenTheBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn))
	m.storedComment(comment(myID))
	m.stored(card(theirID))
	m.people()

	// when
	got, err := svc.EditComment(t.Context(), EditCommentCommand{Command: baseCommand(), CommentID: commentID, Body: "Agreed, @Omar", Mentions: []uuid.UUID{outsiderID}})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrNotBoardMember)
}
