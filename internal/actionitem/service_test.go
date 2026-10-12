package actionitem

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		actionItems   *repository.MockActionItemRepository
		cards         *repository.MockCardRepository
		comments      *repository.MockCardCommentRepository
		organisations *repository.MockOrganisationRepository
		orgAuthz      *orgauthz.MockService
	}
)

var (
	orgID      = uuid.MustParse("5e6f7081-0000-4000-8000-000000000001")
	userID     = uuid.MustParse("5e6f7081-0000-4000-8000-000000000002")
	teamID     = uuid.MustParse("5e6f7081-0000-4000-8000-000000000003")
	otherTeam  = uuid.MustParse("5e6f7081-0000-4000-8000-000000000004")
	boardID    = uuid.MustParse("5e6f7081-0000-4000-8000-000000000005")
	itemID     = uuid.MustParse("5e6f7081-0000-4000-8000-000000000006")
	otherOrgID = uuid.MustParse("5e6f7081-0000-4000-8000-000000000007")
	viewer     = spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}

	errDatabase = errors.New("the database is unreachable")
)

func newTestService(t *testing.T) (Service, *testMocks) {
	t.Helper()

	m := &testMocks{
		actionItems:   repository.NewMockActionItemRepository(t),
		cards:         repository.NewMockCardRepository(t),
		comments:      repository.NewMockCardCommentRepository(t),
		organisations: repository.NewMockOrganisationRepository(t),
		orgAuthz:      orgauthz.NewMockService(t),
	}

	svc := NewService(Deps{ActionItems: m.actionItems, Cards: m.cards, Comments: m.comments, Organisations: m.organisations, OrgAuthz: m.orgAuthz})

	return svc, m
}

func (m *testMocks) mentioned(items, cards, comments []model.Mention) {
	query := spec.UserMentionsQuery{UserID: userID, Limit: mentionsRead}
	m.actionItems.EXPECT().ListMentionsForUser(mock.Anything, query).Return(items, nil).Once()
	m.cards.EXPECT().ListMentionsForUser(mock.Anything, query).Return(cards, nil).Once()
	m.comments.EXPECT().ListMentionsForUser(mock.Anything, query).Return(comments, nil).Once()
}

func (m *testMocks) member(role model.OrgRole) {
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme"}, nil).Once()
	m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: role}, nil).Once()
}

func listedItem(status model.ActionItemStatus) model.ActionItemListing {
	return model.ActionItemListing{
		ID: itemID, OrgID: orgID, TeamID: new(teamID), BoardID: new(boardID), Title: "Fix the pipeline", Status: status, Mentions: []model.ActionItemPerson{},
		BoardTitle: "Sprint 42",
		TeamName:   "Payments",
	}
}

func TestList_APlainMemberSeesOnlyTheirOwnTeamsAndTheRate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.member(model.OrgRoleMember)
	filter := spec.ActionItemFilter{OrgID: orgID, ViewerID: userID, Limit: DefaultLimit}
	page := &model.ActionItemPage{Items: []model.ActionItemListing{listedItem(model.ActionItemOpen)}, Counts: model.ActionItemCounts{Open: 1, Done: 3}}
	m.actionItems.EXPECT().ListPage(mock.Anything, filter).Return(page, nil).Once()

	// when
	got, err := svc.List(t.Context(), ListQuery{Viewer: viewer})

	// then the filter asks the database for the viewer's own teams only
	require.NoError(t, err)
	require.Len(t, got.ActionItems, 1)
	assert.Equal(t, 4, got.Total)
	assert.Equal(t, DefaultLimit, got.Limit)
	require.NotNil(t, got.CompletionRate)
	assert.InDelta(t, 0.75, *got.CompletionRate, 0.0001)
}

func TestList_AnAdminSeesEveryTeamAndTheFiltersReachTheQuery(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.member(model.OrgRoleAdmin)
	status := model.ActionItemDone
	filter := spec.ActionItemFilter{
		OrgID:          orgID,
		ViewerID:       userID,
		EveryTeam:      true,
		Status:         &status,
		AssigneeUserID: new(userID),
		TeamID:         new(otherTeam),
		Limit:          MaxLimit,
		Offset:         0,
	}
	page := &model.ActionItemPage{Items: []model.ActionItemListing{}, Counts: model.ActionItemCounts{Open: 2, Done: 5, Dropped: 1}}
	m.actionItems.EXPECT().ListPage(mock.Anything, filter).Return(page, nil).Once()

	// when
	got, err := svc.List(t.Context(), ListQuery{Viewer: viewer, Status: &status, AssigneeUserID: new(userID), TeamID: new(otherTeam), Limit: 1000, Offset: -3})

	// then the total counts only the status asked for
	require.NoError(t, err)
	assert.Equal(t, 5, got.Total)
	assert.Equal(t, MaxLimit, got.Limit)
	assert.Equal(t, 0, got.Offset)
}

func TestList_RefusesAnUnknownStatusBeforeReadingAnything(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	status := model.ActionItemStatus("finished")

	// when
	got, err := svc.List(t.Context(), ListQuery{Viewer: viewer, Status: &status})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrInvalidStatus)
}

func TestList_ADeletedOrganisationOrANonMemberIsNotFound(t *testing.T) {
	tests := []struct {
		name  string
		given func(m *testMocks)
	}{
		{
			name: "a deleted organisation",
			given: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, DeletedAt: new(time.Now())}, nil).Once()
			},
		},
		{
			name: "a non-member",
			given: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil).Once()
				m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(nil, fmt.Errorf("get member: %w", dao.ErrNotFound)).Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.given(m)

			// when
			got, err := svc.List(t.Context(), ListQuery{Viewer: viewer})

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, ErrOrganisationNotFound)
		})
	}
}

func TestList_AFailedReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.member(model.OrgRoleMember)
	m.actionItems.EXPECT().ListPage(mock.Anything, mock.Anything).Return(nil, errDatabase).Once()

	// when
	got, err := svc.List(t.Context(), ListQuery{Viewer: viewer})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, errDatabase)
}

func TestMentions_ListsOnlyMentionsOnBoardsTheUserCanStillOpen(t *testing.T) {
	// given a mention on their team's board, one on a team they left, one on an organisation board, and one in an organisation they left
	svc, m := newTestService(t)
	mentions := []model.Mention{
		{ActionItemID: uuid.New(), OrgID: orgID, TeamID: new(teamID), BoardID: new(boardID), Title: "On my team"},
		{ActionItemID: uuid.New(), OrgID: orgID, TeamID: new(otherTeam), BoardID: new(boardID), Title: "On a team I left"},
		{ActionItemID: uuid.New(), OrgID: orgID, BoardID: new(boardID), Title: "On an organisation board"},
		{ActionItemID: uuid.New(), OrgID: orgID, TeamID: new(teamID), Title: "Mine again, on the same team"},
		{ActionItemID: uuid.New(), OrgID: otherOrgID, Title: "In an organisation I left"},
	}
	m.mentioned(mentions, []model.Mention{}, []model.Mention{})
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamView}).Return(orgauthz.Decision{Member: true, Allowed: true}, nil).Once()
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: otherTeam, UserID: userID, Permission: orgauthz.PermTeamView}).Return(orgauthz.Decision{Member: true}, nil).Once()
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: orgauthz.PermOrgView}).Return(orgauthz.Decision{Member: true, Allowed: true}, nil).Once()
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, spec.OrgPermissionCheck{OrgID: otherOrgID, UserID: userID, Permission: orgauthz.PermOrgView}).Return(orgauthz.Decision{}, nil).Once()

	// when
	got, err := svc.Mentions(t.Context(), userID)

	// then each team or organisation is decided once, and nothing reveals a board the user cannot open
	require.NoError(t, err)
	var titles []string
	for _, mention := range got.Mentions {
		titles = append(titles, mention.Title)
	}
	assert.Equal(t, []string{"On my team", "On an organisation board", "Mine again, on the same team"}, titles)
}

func TestMentions_AFailedDecisionIsReportedNotTreatedAsNoAccess(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.mentioned([]model.Mention{{ActionItemID: itemID, OrgID: orgID, TeamID: new(teamID)}}, []model.Mention{}, []model.Mention{})
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabase).Once()

	// when
	got, err := svc.Mentions(t.Context(), userID)

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, errDatabase)
}

func TestMentions_IsAnEmptyListWithNoMentions(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.mentioned([]model.Mention{}, []model.Mention{}, []model.Mention{})

	// when
	got, err := svc.Mentions(t.Context(), userID)

	// then
	require.NoError(t, err)
	assert.NotNil(t, got.Mentions)
	assert.Empty(t, got.Mentions)
}

func TestMentions_ListsCardAndCommentMentionsBesideActionItemsNewestFirst(t *testing.T) {
	// given an action item, a card and a comment mention, read in an order that is not the order they were made
	svc, m := newTestService(t)
	base := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	item := model.Mention{Kind: model.MentionActionItem, ActionItemID: itemID, OrgID: orgID, TeamID: new(teamID), BoardID: new(boardID), Title: "Fix the pipeline", MentionedAt: base}
	card := model.Mention{Kind: model.MentionCard, CardID: uuid.New(), OrgID: orgID, TeamID: new(teamID), BoardID: new(boardID), Body: "Thanks @Ana", MentionedAt: base.Add(2 * time.Minute)}
	comment := model.Mention{Kind: model.MentionComment, CardID: uuid.New(), CommentID: uuid.New(), OrgID: orgID, TeamID: new(teamID), BoardID: new(boardID), Body: "Agreed, @Ana", MentionedAt: base.Add(time.Minute)}
	m.mentioned([]model.Mention{item}, []model.Mention{card}, []model.Mention{comment})
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamView}).Return(orgauthz.Decision{Member: true, Allowed: true}, nil).Once()

	// when
	got, err := svc.Mentions(t.Context(), userID)

	// then the three kinds are merged newest first, and their team is decided once
	require.NoError(t, err)
	require.Len(t, got.Mentions, 3)
	assert.Equal(t, model.MentionCard, got.Mentions[0].Kind)
	assert.Equal(t, "Thanks @Ana", got.Mentions[0].Body)
	assert.Equal(t, model.MentionComment, got.Mentions[1].Kind)
	assert.Equal(t, comment.CommentID, *got.Mentions[1].CommentID)
	assert.Equal(t, model.MentionActionItem, got.Mentions[2].Kind)
	assert.Equal(t, itemID, got.Mentions[2].ActionItemID)
}

func TestMentions_LeavesOutACardStillHiddenFromTheMentionedPersonWithoutDecidingItsAccess(t *testing.T) {
	// given a card and a comment on a board whose cards are still hidden from them, and their own hidden card
	svc, m := newTestService(t)
	hiddenTeam := uuid.New()
	hidden := model.Mention{Kind: model.MentionCard, CardID: uuid.New(), OrgID: orgID, TeamID: new(hiddenTeam), BoardID: new(boardID), Body: "SECRET-TEXT-9f2c", CardsHidden: true}
	hiddenComment := model.Mention{Kind: model.MentionComment, CardID: uuid.New(), CommentID: uuid.New(), OrgID: orgID, TeamID: new(hiddenTeam), BoardID: new(boardID), Body: "SECRET-COMMENT-3a9d", CardsHidden: true}
	own := model.Mention{Kind: model.MentionCard, CardID: uuid.New(), OrgID: orgID, TeamID: new(teamID), BoardID: new(boardID), Body: "Note to self, @Ana", CardsHidden: true, OwnCard: true}
	m.mentioned([]model.Mention{}, []model.Mention{hidden, own}, []model.Mention{hiddenComment})
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamView}).Return(orgauthz.Decision{Member: true, Allowed: true}, nil).Once()

	// when
	got, err := svc.Mentions(t.Context(), userID)

	// then only their own card is listed, and the hidden board's team is never even asked about
	require.NoError(t, err)
	require.Len(t, got.Mentions, 1)
	assert.Equal(t, "Note to self, @Ana", got.Mentions[0].Body)
	m.orgAuthz.AssertNotCalled(t, "CanInTeam", mock.Anything, spec.TeamPermissionCheck{TeamID: hiddenTeam, UserID: userID, Permission: orgauthz.PermTeamView})
}

func TestMentions_AFailedReadOfAnyKindIsReported(t *testing.T) {
	tests := []struct {
		name   string
		expect func(m *testMocks)
	}{
		{name: "card mentions", expect: func(m *testMocks) {
			m.actionItems.EXPECT().ListMentionsForUser(mock.Anything, mock.Anything).Return([]model.Mention{}, nil).Once()
			m.cards.EXPECT().ListMentionsForUser(mock.Anything, mock.Anything).Return(nil, errDatabase).Once()
		}},
		{name: "comment mentions", expect: func(m *testMocks) {
			m.actionItems.EXPECT().ListMentionsForUser(mock.Anything, mock.Anything).Return([]model.Mention{}, nil).Once()
			m.cards.EXPECT().ListMentionsForUser(mock.Anything, mock.Anything).Return([]model.Mention{}, nil).Once()
			m.comments.EXPECT().ListMentionsForUser(mock.Anything, mock.Anything).Return(nil, errDatabase).Once()
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.expect(m)

			// when
			got, err := svc.Mentions(t.Context(), userID)

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, errDatabase)
		})
	}
}
