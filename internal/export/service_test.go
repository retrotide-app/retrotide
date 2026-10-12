package export_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/export"
	"retrotide/internal/export/engine"
	"retrotide/internal/export/engines"
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
	serviceMocks struct {
		boards        *repository.MockBoardRepository
		teams         *repository.MockTeamRepository
		organisations *repository.MockOrganisationRepository
		users         *repository.MockUserRepository
		identities    *repository.MockUserIdentityRepository
		cards         *repository.MockCardRepository
		comments      *repository.MockCardCommentRepository
		actionItems   *repository.MockActionItemRepository
		live          *live.MockService
		orgAuthz      *orgauthz.MockService
	}
)

const (
	ownCardText    = "My own idea"
	secretCardText = "The secret complaint about Ali"
	secretItemText = "Fix the thing nobody may read about yet"
)

var (
	svcUser       = uuid.MustParse("5a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d")
	svcOrg        = uuid.MustParse("6b2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e")
	svcTeam       = uuid.MustParse("7c3d4e5f-6a7b-4c8d-ae9f-1a2b3c4d5e6f")
	svcBoard      = uuid.MustParse("8d4e5f6a-7b8c-4d9e-bf0a-2b3c4d5e6f7a")
	svcOtherBoard = uuid.MustParse("9e5f6a7b-8c9d-4eaf-80a1-3c4d5e6f7a8b")
	svcColumn     = uuid.MustParse("af6a7b8c-9d0e-4fb0-91b2-4d5e6f7a8b9c")
	svcOwnSeat    = uuid.MustParse("b07b8c9d-0e1f-40c1-a2c3-5e6f7a8b9c0d")
	svcOtherSeat  = uuid.MustParse("c18c9d0e-1f2a-41d2-b3d4-6f7a8b9c0d1e")
	svcOwnCard    = uuid.MustParse("d29d0e1f-2a3b-42e3-84e5-7a8b9c0d1e2f")
	svcSecretCard = uuid.MustParse("e3ae1f2a-3b4c-43f4-95f6-8b9c0d1e2f3a")
	svcSecretItem = uuid.MustParse("f4bf2a3b-4c5d-4405-a607-9c0d1e2f3a4b")

	svcPrincipal = model.Principal{Kind: model.PrincipalUser, ID: svcUser}
	svcViewer    = spec.OrganisationMemberKey{OrgID: svcOrg, UserID: svcUser}
	svcWritten   = time.Date(2026, 10, 8, 9, 30, 0, 0, time.UTC)

	errSvcStoreDown = errors.New("the database is unreachable")
)

func newServiceMocks(t *testing.T) (*serviceMocks, export.Service) {
	t.Helper()

	m := &serviceMocks{
		boards:        repository.NewMockBoardRepository(t),
		teams:         repository.NewMockTeamRepository(t),
		organisations: repository.NewMockOrganisationRepository(t),
		users:         repository.NewMockUserRepository(t),
		identities:    repository.NewMockUserIdentityRepository(t),
		cards:         repository.NewMockCardRepository(t),
		comments:      repository.NewMockCardCommentRepository(t),
		actionItems:   repository.NewMockActionItemRepository(t),
		live:          live.NewMockService(t),
		orgAuthz:      orgauthz.NewMockService(t),
	}

	svc := export.NewService(export.Deps{
		Engines:       export.NewEngineManager(),
		Boards:        m.boards,
		Teams:         m.teams,
		Organisations: m.organisations,
		Users:         m.users,
		Identities:    m.identities,
		Cards:         m.cards,
		Comments:      m.comments,
		ActionItems:   m.actionItems,
		Live:          m.live,
		OrgAuthz:      m.orgAuthz,
	})

	return m, svc
}

func hiddenBoardState(boardID uuid.UUID, anonymity model.Anonymity) model.BoardState {
	header := model.BoardHeader{
		ID:          boardID,
		OrgID:       svcOrg,
		TeamID:      new(svcTeam),
		Title:       "Sprint 42",
		TemplateKey: "blank",
		Anonymity:   anonymity,
		Stage:       model.StageWrite,
		CardsHidden: true,
		CreatedAt:   svcWritten,
	}

	return model.BoardState{
		Header:  header,
		Columns: []model.BoardColumn{{ID: svcColumn, BoardID: boardID, Title: "Went well", Position: "a0"}},
		Cards: []model.Card{
			{ID: svcOwnCard, BoardID: boardID, ColumnID: svcColumn, AuthorParticipantID: new(svcOwnSeat), Body: ownCardText, Position: "a0", CreatedAt: svcWritten},
			{ID: svcSecretCard, BoardID: boardID, ColumnID: svcColumn, AuthorParticipantID: new(svcOtherSeat), Body: secretCardText, Position: "a1", CreatedAt: svcWritten},
		},
		Participants: []model.BoardParticipantProfile{
			{ID: svcOwnSeat, BoardID: boardID, UserID: new(svcUser), DisplayName: "Sam", Role: model.BoardRoleParticipant},
			{ID: svcOtherSeat, BoardID: boardID, DisplayName: "Ali", Role: model.BoardRoleParticipant},
		},
		ActionItems: []model.ActionItem{
			{ID: svcSecretItem, OrgID: svcOrg, TeamID: new(svcTeam), BoardID: new(boardID), SourceCardID: new(svcSecretCard), Title: secretItemText, Status: model.ActionItemOpen},
		},
	}
}

func boardAccess(state model.BoardState, capabilities ...model.BoardCapability) *model.BoardAccess {
	return &model.BoardAccess{
		Principal:     svcPrincipal,
		Header:        state.Header,
		ParticipantID: new(svcOwnSeat),
		Role:          model.BoardRoleParticipant,
		OrgMember:     true,
		Capabilities:  capabilities,
	}
}

func boardKey(boardID uuid.UUID) spec.BoardKey {
	return spec.BoardKey{OrgID: svcOrg, BoardID: boardID}
}

func boardQuery(format engine.Format) export.BoardQuery {
	return export.BoardQuery{Access: spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcBoard}, Format: format}
}

func render(t *testing.T, download *export.Download) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, download.Write(&out))

	return out.String()
}

func TestService_BoardRefusesARequesterWithoutBoardExport(t *testing.T) {
	// given a link-only participant, who may view and take part but not export
	m, svc := newServiceMocks(t)
	state := hiddenBoardState(svcBoard, model.AnonymityNamed)
	m.live.EXPECT().Resolve(mock.Anything, boardQuery(engine.FormatCSV).Access).Return(boardAccess(state, model.BoardCapabilityView, model.BoardCapabilityParticipate), nil)

	// when
	download, err := svc.Board(t.Context(), boardQuery(engine.FormatCSV))

	// then the refusal comes before the board is read
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Nil(t, download)
}

func TestService_BoardPassesOnTheLiveBoardsRefusal(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	m.live.EXPECT().Resolve(mock.Anything, boardQuery(engine.FormatJSON).Access).Return(nil, fmt.Errorf("resolve: %w", live.ErrNoAccess))

	// when
	_, err := svc.Board(t.Context(), boardQuery(engine.FormatJSON))

	// then
	require.ErrorIs(t, err, live.ErrNoAccess)
}

func TestService_BoardRefusesAnUnknownFormatBeforeResolvingAccess(t *testing.T) {
	// given
	_, svc := newServiceMocks(t)

	// when
	_, err := svc.Board(t.Context(), boardQuery(engine.Format("pdf")))

	// then
	require.ErrorIs(t, err, export.ErrUnknownFormat)
}

func TestService_BoardTreatsABoardGoneAfterResolvingAsNoAccess(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	state := hiddenBoardState(svcBoard, model.AnonymityNamed)
	m.live.EXPECT().Resolve(mock.Anything, boardQuery(engine.FormatCSV).Access).Return(boardAccess(state, model.BoardCapabilityExport), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(nil, fmt.Errorf("get snapshot: %w", dao.ErrNotFound))

	// when
	_, err := svc.Board(t.Context(), boardQuery(engine.FormatCSV))

	// then
	require.ErrorIs(t, err, live.ErrNoAccess)
}

func TestService_BoardNeverWritesAnotherParticipantsHiddenCard(t *testing.T) {
	for _, format := range []engine.Format{engine.FormatCSV, engine.FormatMarkdown, engine.FormatJSON} {
		t.Run(string(format), func(t *testing.T) {
			// given a named board still hiding cards, where Ali's card has an action item made from it
			m, svc := newServiceMocks(t)
			state := hiddenBoardState(svcBoard, model.AnonymityNamed)
			m.live.EXPECT().Resolve(mock.Anything, boardQuery(format).Access).Return(boardAccess(state, model.BoardCapabilityView, model.BoardCapabilityExport), nil)
			m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(&state, nil)

			// when
			download, err := svc.Board(t.Context(), boardQuery(format))

			// then the requester's own card is there in full, Ali's only as a count, and the item lands in Other actions
			require.NoError(t, err)
			assert.Equal(t, "Sprint 42", download.Title)
			assert.Equal(t, format, download.Engine.Format())

			out := render(t, download)
			assert.Contains(t, out, ownCardText)
			assert.NotContains(t, out, secretCardText)
			assert.NotContains(t, out, svcSecretCard.String())
			assert.NotContains(t, out, svcOtherSeat.String())
			assert.Contains(t, out, secretItemText)
		})
	}
}

func TestService_BoardCountsHiddenCardsFromThePublishedTallyOnAnAnonymousBoard(t *testing.T) {
	tests := []struct {
		name      string
		published projection.HiddenTally
		ok        bool
		want      int
	}{
		{name: "a published tally", published: projection.HiddenTally{svcColumn: {svcOtherSeat: 4}}, ok: true, want: 4},
		{name: "no published tally yet", ok: false, want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given an anonymous board hiding cards, whose counts go out on the board tick
			m, svc := newServiceMocks(t)
			state := hiddenBoardState(svcBoard, model.AnonymityAnonymous)
			m.live.EXPECT().Resolve(mock.Anything, boardQuery(engine.FormatJSON).Access).Return(boardAccess(state, model.BoardCapabilityExport), nil)
			m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(&state, nil)
			m.live.EXPECT().PublishedTally(svcBoard).Return(tc.published, tc.ok)

			// when
			download, err := svc.Board(t.Context(), boardQuery(engine.FormatJSON))

			// then the export cannot beat the tick
			require.NoError(t, err)

			var board struct {
				Columns []struct {
					HiddenCount int `json:"hidden_count"`
				} `json:"columns"`
			}
			require.NoError(t, json.Unmarshal([]byte(render(t, download)), &board))
			require.Len(t, board.Columns, 1)
			assert.Equal(t, tc.want, board.Columns[0].HiddenCount)
		})
	}
}

func TestService_OrganisationRefusesAnyoneButAnOwnerOrAnAdmin(t *testing.T) {
	tests := []struct {
		name     string
		decision orgauthz.Decision
		want     error
	}{
		{name: "a member", decision: orgauthz.Decision{Member: true, Allowed: false}, want: export.ErrOrganisationNotAllowed},
		{name: "not a member", decision: orgauthz.Decision{}, want: export.ErrOrganisationNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m, svc := newServiceMocks(t)
			check := spec.OrgPermissionCheck{OrgID: svcOrg, UserID: svcUser, Permission: orgauthz.PermOrgExport}
			m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(tc.decision, nil)

			// when
			download, err := svc.Organisation(t.Context(), svcViewer)

			// then nothing of the organisation is read
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, download)
		})
	}
}

func (m *serviceMocks) allowOrganisationExport() {
	check := spec.OrgPermissionCheck{OrgID: svcOrg, UserID: svcUser, Permission: orgauthz.PermOrgExport}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(orgauthz.Decision{Member: true, Allowed: true}, nil)
	m.organisations.EXPECT().GetByID(mock.Anything, svcOrg).Return(&model.Organisation{ID: svcOrg, Name: "Acme", Slug: "acme"}, nil)
	m.teams.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.Team{{ID: svcTeam, OrgID: svcOrg, Name: "Payments"}}, nil)
}

func TestService_OrganisationProjectsEveryBoardForTheRequesterAndSkipsOnesTheyCannotOpen(t *testing.T) {
	// given an admin removed from one board, and a board still hiding Ali's card
	m, svc := newServiceMocks(t)
	m.allowOrganisationExport()
	state := hiddenBoardState(svcBoard, model.AnonymityNamed)
	removed := hiddenBoardState(svcOtherBoard, model.AnonymityNamed)
	m.boards.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.BoardHeader{removed.Header, state.Header}, nil)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcOtherBoard}).Return(nil, fmt.Errorf("resolve: %w", live.ErrRemoved))
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcBoard}).Return(boardAccess(state, model.BoardCapabilityExport), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(&state, nil)

	// when
	download, err := svc.Organisation(t.Context(), svcViewer)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Acme", download.Title)
	assert.Equal(t, engines.JSON{}, download.Engine)

	out := render(t, download)
	assert.NotContains(t, out, secretCardText)
	assert.Contains(t, out, secretItemText)

	var document struct {
		Organisation struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"organisation"`
		Boards []struct {
			Team     string `json:"team"`
			Archived bool   `json:"archived"`
			Board    struct {
				Title   string `json:"title"`
				Columns []struct {
					HiddenCount int `json:"hidden_count"`
				} `json:"columns"`
			} `json:"board"`
		} `json:"boards"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &document))
	assert.Equal(t, "Acme", document.Organisation.Name)
	assert.Equal(t, "acme", document.Organisation.Slug)
	require.Len(t, document.Boards, 1)
	assert.Equal(t, "Payments", document.Boards[0].Team)
	assert.False(t, document.Boards[0].Archived)
	assert.Equal(t, "Sprint 42", document.Boards[0].Board.Title)
	assert.Equal(t, 1, document.Boards[0].Board.Columns[0].HiddenCount)
}

func TestService_OrganisationWithNoBoardsWritesAnEmptyList(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	m.allowOrganisationExport()
	m.boards.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.BoardHeader{}, nil)

	// when
	download, err := svc.Organisation(t.Context(), svcViewer)

	// then
	require.NoError(t, err)
	assert.JSONEq(t, `{"organisation":{"name":"Acme","slug":"acme"},"boards":[]}`, render(t, download))
}

func TestService_OrganisationLoadsTheFirstBoardBeforeAnythingIsWritten(t *testing.T) {
	// given a first board that cannot be read
	m, svc := newServiceMocks(t)
	m.allowOrganisationExport()
	state := hiddenBoardState(svcBoard, model.AnonymityNamed)
	m.boards.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.BoardHeader{state.Header}, nil)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcBoard}).Return(boardAccess(state, model.BoardCapabilityExport), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(nil, errSvcStoreDown)

	// when
	download, err := svc.Organisation(t.Context(), svcViewer)

	// then the failure comes back before a download exists, so the controller can still answer with JSON
	require.ErrorIs(t, err, errSvcStoreDown)
	assert.Nil(t, download)
}

func TestService_OrganisationStopsBetweenBoardsWhenTheRequestIsCancelled(t *testing.T) {
	// given two boards, and a client that disconnects once the first is loaded
	m, svc := newServiceMocks(t)
	m.allowOrganisationExport()
	state := hiddenBoardState(svcBoard, model.AnonymityNamed)
	second := hiddenBoardState(svcOtherBoard, model.AnonymityNamed)
	m.boards.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.BoardHeader{state.Header, second.Header}, nil)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcBoard}).Return(boardAccess(state, model.BoardCapabilityExport), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(&state, nil)
	ctx, cancel := context.WithCancel(t.Context())
	download, err := svc.Organisation(ctx, svcViewer)
	require.NoError(t, err)

	// when
	cancel()
	var out bytes.Buffer
	err = download.Write(&out)

	// then the second board is never resolved, and the document is left unfinished
	require.ErrorIs(t, err, context.Canceled)
	assert.Contains(t, out.String(), "Sprint 42")
	assert.False(t, json.Valid(out.Bytes()), "a stopped export must not look complete")
}

func TestService_SelfListsTheAccountItsIdentitiesMembershipsAndWhatIsStillLinkedToIt(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	joined := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	linked := time.Date(2026, 9, 2, 10, 15, 0, 0, time.UTC)
	edited := time.Date(2026, 10, 8, 9, 45, 0, 0, time.UTC)
	m.users.EXPECT().GetByID(mock.Anything, svcUser).Return(&model.User{ID: svcUser, Email: "sam@example.com", DisplayName: "Sam Lee", EmailVerified: true, CreatedAt: joined}, nil)
	m.identities.EXPECT().ListForUser(mock.Anything, svcUser).Return([]model.UserIdentity{
		{Provider: model.ProviderGoogle, Subject: "109876543210", UserID: svcUser, Email: "sam.lee@gmail.com", CreatedAt: linked},
	}, nil)
	m.organisations.EXPECT().ListForUser(mock.Anything, svcUser).Return([]model.UserOrganisation{
		{ID: svcOrg, Name: "Acme", Slug: "acme", Role: model.OrgRoleAdmin, JoinedAt: joined},
	}, nil)
	m.teams.EXPECT().ListForMember(mock.Anything, svcViewer).Return([]model.MemberTeam{
		{ID: svcTeam, OrgID: svcOrg, Name: "Payments", Role: model.TeamRoleLead, JoinedAt: linked},
	}, nil)
	m.cards.EXPECT().ListWrittenByUser(mock.Anything, svcUser).Return([]model.WrittenCard{
		{OrgName: "Acme", BoardTitle: "Sprint 42", ColumnTitle: "Went well", Body: ownCardText, CreatedAt: svcWritten, UpdatedAt: new(edited)},
	}, nil)
	m.comments.EXPECT().ListWrittenByUser(mock.Anything, svcUser).Return([]model.WrittenComment{
		{OrgName: "Acme", BoardTitle: "Sprint 42", Body: "Agreed", CreatedAt: svcWritten},
	}, nil)
	m.actionItems.EXPECT().ListForUser(mock.Anything, svcUser).Return([]model.UserActionItem{
		{OrgName: "Acme", TeamName: "Payments", BoardTitle: "Sprint 42", Title: "Thank @Hana", Status: model.ActionItemOpen, DueOn: new(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)), RaisedByUser: true},
		{OrgName: "Acme", Title: "Book the next retro", Status: model.ActionItemDone, AssignedToUser: true},
	}, nil)

	// when
	download, err := svc.Self(t.Context(), svcUser)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Sam Lee", download.Title)
	assert.Equal(t, engines.JSON{}, download.Engine)
	assert.JSONEq(t, `{
		"account": {"display_name": "Sam Lee", "email": "sam@example.com", "email_verified": true, "created_at": "2026-09-01T08:00:00Z"},
		"identities": [{"provider": "google", "subject": "109876543210", "email": "sam.lee@gmail.com", "linked_at": "2026-09-02T10:15:00Z"}],
		"organisations": [{"organisation": "Acme", "role": "admin", "joined_at": "2026-09-01T08:00:00Z", "teams": [{"name": "Payments", "role": "lead", "joined_at": "2026-09-02T10:15:00Z"}]}],
		"cards": [{"organisation": "Acme", "board": "Sprint 42", "column": "Went well", "body": "My own idea", "created_at": "2026-10-08T09:30:00Z", "updated_at": "2026-10-08T09:45:00Z"}],
		"comments": [{"organisation": "Acme", "board": "Sprint 42", "body": "Agreed", "created_at": "2026-10-08T09:30:00Z"}],
		"action_items": [
			{"organisation": "Acme", "team": "Payments", "board": "Sprint 42", "title": "Thank @Hana", "status": "open", "due_on": "2026-10-20", "raised_by_you": true, "assigned_to_you": false},
			{"organisation": "Acme", "title": "Book the next retro", "status": "done", "due_on": null, "raised_by_you": false, "assigned_to_you": true}
		]
	}`, render(t, download))
}

func TestService_SelfWithNothingLinkedWritesEmptyLists(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	m.users.EXPECT().GetByID(mock.Anything, svcUser).Return(&model.User{ID: svcUser, Email: "sam@example.com", DisplayName: "Sam Lee", CreatedAt: svcWritten}, nil)
	m.identities.EXPECT().ListForUser(mock.Anything, svcUser).Return(nil, nil)
	m.organisations.EXPECT().ListForUser(mock.Anything, svcUser).Return(nil, nil)
	m.cards.EXPECT().ListWrittenByUser(mock.Anything, svcUser).Return(nil, nil)
	m.comments.EXPECT().ListWrittenByUser(mock.Anything, svcUser).Return(nil, nil)
	m.actionItems.EXPECT().ListForUser(mock.Anything, svcUser).Return(nil, nil)

	// when
	download, err := svc.Self(t.Context(), svcUser)

	// then every list is present and empty, never null
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"account": {"display_name": "Sam Lee", "email": "sam@example.com", "email_verified": false, "created_at": "2026-10-08T09:30:00Z"},
		"identities": [], "organisations": [], "cards": [], "comments": [], "action_items": []
	}`, render(t, download))
}

func TestService_SelfRefusesAnAccountThatIsGone(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	m.users.EXPECT().GetByID(mock.Anything, svcUser).Return(nil, fmt.Errorf("get user: %w", dao.ErrNotFound))

	// when
	download, err := svc.Self(t.Context(), svcUser)

	// then
	require.ErrorIs(t, err, export.ErrAccountNotFound)
	assert.Nil(t, download)
}

func (m *serviceMocks) allowBoardsExport() {
	check := spec.OrgPermissionCheck{OrgID: svcOrg, UserID: svcUser, Permission: orgauthz.PermOrgView}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(orgauthz.Decision{Member: true}, nil)
	m.organisations.EXPECT().GetByID(mock.Anything, svcOrg).Return(&model.Organisation{ID: svcOrg, Name: "Acme", Slug: "acme"}, nil)
	m.teams.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.Team{{ID: svcTeam, OrgID: svcOrg, Name: "Payments"}}, nil)
}

func TestService_BoardsRefusesSomeoneOutsideTheOrganisation(t *testing.T) {
	// given
	m, svc := newServiceMocks(t)
	check := spec.OrgPermissionCheck{OrgID: svcOrg, UserID: svcUser, Permission: orgauthz.PermOrgView}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(orgauthz.Decision{}, nil)

	// when
	download, err := svc.Boards(t.Context(), export.BoardsQuery{Viewer: svcViewer, BoardIDs: []uuid.UUID{svcBoard}})

	// then nothing of the organisation is read
	require.ErrorIs(t, err, export.ErrOrganisationNotFound)
	assert.Nil(t, download)
}

func TestService_BoardsExportsOnlyTheChosenBoardsOfTheOrganisationAndNeedsNoExportPermission(t *testing.T) {
	// given a plain member who chose one of the organisation's boards and one board ID from another organisation
	m, svc := newServiceMocks(t)
	m.allowBoardsExport()
	chosen := hiddenBoardState(svcBoard, model.AnonymityNamed)
	unchosen := hiddenBoardState(svcOtherBoard, model.AnonymityNamed)
	foreign := uuid.MustParse("05c03b4c-5d6e-4516-b718-0d1e2f3a4b5c")
	m.boards.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.BoardHeader{unchosen.Header, chosen.Header}, nil)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcBoard}).Return(boardAccess(chosen, model.BoardCapabilityExport), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, boardKey(svcBoard)).Return(&chosen, nil)

	// when
	download, err := svc.Boards(t.Context(), export.BoardsQuery{Viewer: svcViewer, BoardIDs: []uuid.UUID{svcBoard, foreign}})

	// then the strict mocks prove the unchosen and the foreign boards were never resolved
	require.NoError(t, err)
	assert.Equal(t, "Acme - boards", download.Title)
	assert.Equal(t, engines.JSON{}, download.Engine)

	out := render(t, download)
	assert.NotContains(t, out, secretCardText)

	var document struct {
		Boards []struct {
			Team  string `json:"team"`
			Board struct {
				Title string `json:"title"`
			} `json:"board"`
		} `json:"boards"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &document))
	require.Len(t, document.Boards, 1)
	assert.Equal(t, "Payments", document.Boards[0].Team)
	assert.Equal(t, "Sprint 42", document.Boards[0].Board.Title)
}

func TestService_BoardsLeavesOutABoardTheRequesterCannotExport(t *testing.T) {
	// given a board the member may open but not export
	m, svc := newServiceMocks(t)
	m.allowBoardsExport()
	chosen := hiddenBoardState(svcBoard, model.AnonymityNamed)
	m.boards.EXPECT().ListForOrganisation(mock.Anything, svcOrg).Return([]model.BoardHeader{chosen.Header}, nil)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: svcPrincipal, BoardID: svcBoard}).Return(boardAccess(chosen, model.BoardCapabilityView), nil)

	// when
	download, err := svc.Boards(t.Context(), export.BoardsQuery{Viewer: svcViewer, BoardIDs: []uuid.UUID{svcBoard}})

	// then
	require.NoError(t, err)
	assert.JSONEq(t, `{"organisation":{"name":"Acme","slug":"acme"},"boards":[]}`, render(t, download))
}
