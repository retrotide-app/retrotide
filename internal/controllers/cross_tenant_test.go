package controllers

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"retrotide/internal/actionitem"
	"retrotide/internal/board"
	"retrotide/internal/board/actionitems"
	"retrotide/internal/board/live"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/export"
	"retrotide/internal/middleware"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/org"
	"retrotide/internal/org/invite"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/savedtemplate"
	"retrotide/internal/team"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	crossTenantHarness struct {
		*testutil.Harness
		orgRepo    *repository.MockOrganisationRepository
		teamRepo   *repository.MockTeamRepository
		boardRepo  *repository.MockBoardRepository
		columnRepo *repository.MockBoardColumnRepository
		cardRepo   *repository.MockCardRepository
		voteRepo   *repository.MockCardVoteRepository
		noteRepo   *repository.MockCardCommentRepository
		itemRepo   *repository.MockActionItemRepository
		savedRepo  *repository.MockSavedTemplateRepository
		orgs       *org.MockService
		teams      *team.MockService
		invites    *invite.MockService
	}

	crossTenantRoute struct {
		method string
		path   string
		body   any
	}
)

var (
	tenantAlice   = uuid.MustParse("6a0f7c1e-3b2d-4e5f-8a9b-0c1d2e3f4a5b")
	tenantBob     = uuid.MustParse("7b1a8d2f-4c3e-4f60-9bac-1d2e3f4a5b6c")
	tenantOrgA    = uuid.MustParse("1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f")
	tenantOrgB    = uuid.MustParse("2d3e4f5a-6b7c-4d8e-9f0a-1b2c3d4e5f6a")
	tenantTeamA   = uuid.MustParse("3e4f5a6b-7c8d-4e9f-a0b1-2c3d4e5f6a7b")
	tenantTeamB   = uuid.MustParse("4f5a6b7c-8d9e-4fa0-b1c2-3d4e5f6a7b8c")
	tenantInviteB = uuid.MustParse("5a6b7c8d-9eaf-40b1-c2d3-4e5f6a7b8c9d")

	tenantOrgAPath  = "/api/v1/orgs/" + tenantOrgA.String()
	tenantOrgBPath  = "/api/v1/orgs/" + tenantOrgB.String()
	tenantTeamAPath = "/api/v1/teams/" + tenantTeamA.String()
	tenantTeamBPath = "/api/v1/teams/" + tenantTeamB.String()

	tenantBoardA       = uuid.MustParse("6b7c8d9e-af01-42c3-94e5-f60718293a4b")
	tenantBoardB       = uuid.MustParse("7c8d9eaf-0112-43d4-a5f6-0718293a4b5c")
	tenantColumnA      = uuid.MustParse("8d9eaf01-1223-44e5-b607-18293a4b5c6d")
	tenantColumnB      = uuid.MustParse("9eaf0112-2334-45f6-8718-293a4b5c6d7e")
	tenantCardA        = uuid.MustParse("af011223-3445-4607-9829-3a4b5c6d7e8f")
	tenantCardB        = uuid.MustParse("b0122334-4556-4718-a93a-4b5c6d7e8f90")
	tenantNewCard      = uuid.MustParse("c1233445-5667-4829-ba4b-5c6d7e8f90a1")
	tenantOp           = uuid.MustParse("d2344556-6778-493a-8b5c-6d7e8f90a1b2")
	tenantAliceSeatInA = uuid.MustParse("e3455667-7889-4a4b-9c6d-7e8f90a1b2c3")
	tenantCommentB     = uuid.MustParse("16788990-aabb-4c7d-8e9f-a0b1c2d3e4f5")
	tenantSeatB        = uuid.MustParse("27899aab-bbcc-4d8e-9fa0-b1c2d3e4f506")
	tenantItemB        = uuid.MustParse("389aabbc-ccdd-4e9f-a0b1-c2d3e4f50617")
	tenantItemFields   = dto.ActionItemFieldsRequest{OpID: tenantOp, Title: "Fix the pipeline"}
	tenantTemplateB    = uuid.MustParse("49abbccd-ddee-4fa0-b1c2-d3e4f5061728")

	tenantBoardAPath = "/api/v1/boards/" + tenantBoardA.String()
	tenantBoardBPath = "/api/v1/boards/" + tenantBoardB.String()
	tenantOpBody     = dto.BoardCommandRequest{OpID: tenantOp}
	tenantWriteToA   = spec.BoardWrite{OrgID: tenantOrgA, BoardID: tenantBoardA, Version: 4, OpID: tenantOp}
	tenantBoardBOpen = &model.BoardHeader{
		ID:          tenantBoardB,
		OrgID:       tenantOrgB,
		TeamID:      new(tenantTeamB),
		LinkAccess:  model.LinkAccessOff,
		Version:     9,
		Anonymity:   model.AnonymityNamed,
		Stage:       model.StageWrite,
		CardsHidden: true}
	tenantBoardAOpen = &model.BoardHeader{
		ID:          tenantBoardA,
		OrgID:       tenantOrgA,
		TeamID:      new(tenantTeamA),
		LinkAccess:  model.LinkAccessOff,
		Version:     4,
		Anonymity:   model.AnonymityNamed,
		Stage:       model.StageWrite,
		CardsHidden: true}
	tenantAliceSeat = &model.BoardParticipant{ID: tenantAliceSeatInA, BoardID: tenantBoardA, UserID: new(tenantAlice), Role: model.BoardRoleFacilitator}

	tenantAliceInA   = spec.OrganisationMemberKey{OrgID: tenantOrgA, UserID: tenantAlice}
	tenantAliceInB   = spec.OrganisationMemberKey{OrgID: tenantOrgB, UserID: tenantAlice}
	tenantAliceOwnsA = &model.OrganisationMember{OrgID: tenantOrgA, UserID: tenantAlice, Role: model.OrgRoleOwner}
)

func newCrossTenantHarness(t *testing.T) *crossTenantHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	x := &crossTenantHarness{
		Harness:    h,
		orgRepo:    repository.NewMockOrganisationRepository(t),
		teamRepo:   repository.NewMockTeamRepository(t),
		boardRepo:  repository.NewMockBoardRepository(t),
		columnRepo: repository.NewMockBoardColumnRepository(t),
		cardRepo:   repository.NewMockCardRepository(t),
		voteRepo:   repository.NewMockCardVoteRepository(t),
		noteRepo:   repository.NewMockCardCommentRepository(t),
		itemRepo:   repository.NewMockActionItemRepository(t),
		savedRepo:  repository.NewMockSavedTemplateRepository(t),
		orgs:       org.NewMockService(t),
		teams:      team.NewMockService(t),
		invites:    invite.NewMockService(t),
	}

	orgAuthz := orgauthz.NewService(x.orgRepo, x.teamRepo)

	guards := middleware.NewGuards(middleware.GuardDeps{
		Settings: h.SettingsService,
		Sessions: h.Sessions,
		Guests:   h.Guests,
		Authz:    h.Authz,
		OrgAuthz: orgAuthz,
	})

	boardLive := live.New(live.Deps{Boards: x.boardRepo, Cards: x.cardRepo, OrgAuthz: orgAuthz})

	boards := board.NewService(board.Deps{
		Boards:         x.boardRepo,
		Columns:        x.columnRepo,
		Cards:          x.cardRepo,
		Votes:          x.voteRepo,
		Comments:       x.noteRepo,
		Teams:          x.teamRepo,
		Organisations:  x.orgRepo,
		Live:           boardLive,
		Events:         boardLive,
		OrgAuthz:       orgAuthz,
		SavedTemplates: x.savedRepo,
	})

	savedTemplates := savedtemplate.NewService(savedtemplate.Deps{
		SavedTemplates: x.savedRepo,
		Columns:        x.columnRepo,
		Teams:          x.teamRepo,
		Organisations:  x.orgRepo,
		Live:           boardLive,
		OrgAuthz:       orgAuthz,
		Entitlements:   entitlements.NewMockService(t),
	})

	boardActionItems := actionitems.NewService(actionitems.Deps{
		Boards:        x.boardRepo,
		Cards:         x.cardRepo,
		ActionItems:   x.itemRepo,
		Teams:         x.teamRepo,
		Organisations: x.orgRepo,
		Live:          boardLive,
		Events:        boardLive,
	})

	s := &Service{
		OrgService:           x.orgs,
		TeamService:          x.teams,
		InviteService:        x.invites,
		EntitlementsService:  entitlements.NewMockService(t),
		BoardLive:            boardLive,
		BoardService:         boards,
		BoardActionItems:     boardActionItems,
		ActionItemService:    actionitem.NewService(actionitem.Deps{ActionItems: x.itemRepo, Cards: x.cardRepo, Comments: x.noteRepo, Organisations: x.orgRepo, OrgAuthz: orgAuthz}),
		ExportService:        export.NewService(export.Deps{Engines: export.NewEngineManager(), Boards: x.boardRepo, Teams: x.teamRepo, Organisations: x.orgRepo, Cards: x.cardRepo, Comments: x.noteRepo, ActionItems: x.itemRepo, Live: boardLive, OrgAuthz: orgAuthz}),
		SavedTemplateService: savedTemplates,
		Guests:               h.Guests,
		Guards:               guards,
	}
	RegisterRoutes(h.Engine.Group("/api/v1"), slices.Concat(s.getAllOrganisationRoutes(), s.getAllTeamRoutes(), s.getAllInviteRoutes(), boardRouteLists(s), s.getAllBoardLinkRoutes(), s.getAllActionItemRoutes(), s.getAllExportRoutes(), s.getAllSavedTemplateRoutes()))

	return x
}

func (x *crossTenantHarness) recordAsAlice(route crossTenantRoute) (int, string) {
	cookie := x.SignIn(tenantAlice)
	if route.method != http.MethodGet {
		x.Authz.EXPECT().IsLocked(mock.Anything, tenantAlice).Return(false, nil)
	}

	request := x.NewRequest(route.method, route.path).WithCookie(cookie).WithRemoteAddr(publicPeer)
	if route.body != nil {
		request = request.WithJSONBody(route.body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func TestCrossTenant_EveryRouteOfAnotherOrganisationAnswersNotFound(t *testing.T) {
	routes := []crossTenantRoute{
		{method: http.MethodGet, path: tenantOrgBPath},
		{method: http.MethodPut, path: tenantOrgBPath, body: dto.UpdateOrganisationRequest{Name: orgName, Slug: orgSlug}},
		{method: http.MethodDelete, path: tenantOrgBPath},
		{method: http.MethodPost, path: tenantOrgBPath + "/restore"},
		{method: http.MethodPost, path: tenantOrgBPath + "/transfer", body: dto.TransferOwnershipRequest{UserID: tenantAlice}},
		{method: http.MethodPost, path: tenantOrgBPath + "/leave"},
		{method: http.MethodGet, path: tenantOrgBPath + "/members"},
		{method: http.MethodPut, path: tenantOrgBPath + "/members/" + tenantBob.String() + "/role", body: dto.ChangeMemberRoleRequest{Role: "admin"}},
		{method: http.MethodDelete, path: tenantOrgBPath + "/members/" + tenantBob.String()},
		{method: http.MethodGet, path: tenantOrgBPath + "/entitlements"},
		{method: http.MethodGet, path: tenantOrgBPath + "/teams"},
		{method: http.MethodGet, path: tenantOrgBPath + "/boards"},
		{method: http.MethodPost, path: tenantOrgBPath + "/boards", body: dto.CreateOrgBoardRequest{CreateBoardRequest: dto.CreateBoardRequest{Title: "Planted", TemplateKey: "mad-sad-glad"}}},
		{method: http.MethodPost, path: tenantOrgBPath + "/teams", body: dto.CreateTeamRequest{Name: teamName}},
		{method: http.MethodPost, path: tenantOrgBPath + "/teams/" + tenantTeamB.String() + "/archive"},
		{method: http.MethodDelete, path: tenantOrgBPath + "/teams/" + tenantTeamB.String()},
		{method: http.MethodGet, path: tenantOrgBPath + "/invites"},
		{method: http.MethodPost, path: tenantOrgBPath + "/invites", body: dto.CreateInviteRequest{Email: memberEmail}},
		{method: http.MethodPost, path: tenantOrgBPath + "/invites/" + tenantInviteB.String() + "/resend"},
		{method: http.MethodDelete, path: tenantOrgBPath + "/invites/" + tenantInviteB.String()},
		{method: http.MethodGet, path: tenantOrgBPath + "/action-items"},
		{method: http.MethodGet, path: tenantOrgBPath + "/action-items?status=open&assignee=me"},
		{method: http.MethodGet, path: tenantOrgBPath + "/export"},
		{method: http.MethodGet, path: tenantOrgBPath + "/saved-templates"},
		{method: http.MethodDelete, path: tenantOrgBPath + "/saved-templates/" + tenantTemplateB.String()},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given Alice holds nothing in organisation B
			x := newCrossTenantHarness(t)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)

			// when
			status, body := x.recordAsAlice(route)

			// then the guard reads the membership of the path's organisation, B is not even shown to exist, and no service is asked
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"organisation not found"}`, body)
		})
	}
}

func TestCrossTenant_EveryRouteOfAnotherOrganisationsTeamAnswersNotFound(t *testing.T) {
	routes := []crossTenantRoute{
		{method: http.MethodGet, path: tenantTeamBPath},
		{method: http.MethodPut, path: tenantTeamBPath, body: dto.RenameTeamRequest{Name: teamName}},
		{method: http.MethodGet, path: tenantTeamBPath + "/members"},
		{method: http.MethodPost, path: tenantTeamBPath + "/members", body: dto.AddTeamMemberRequest{UserID: tenantAlice, Role: "lead"}},
		{method: http.MethodPut, path: tenantTeamBPath + "/members/" + tenantBob.String() + "/role", body: dto.ChangeTeamMemberRoleRequest{Role: "lead"}},
		{method: http.MethodDelete, path: tenantTeamBPath + "/members/" + tenantBob.String()},
		{method: http.MethodGet, path: tenantTeamBPath + "/boards"},
		{method: http.MethodPost, path: tenantTeamBPath + "/boards", body: dto.CreateBoardRequest{Title: "Sprint 12", TemplateKey: "mad-sad-glad"}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given team B belongs to organisation B, where Alice holds nothing
			x := newCrossTenantHarness(t)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)

			// when
			status, body := x.recordAsAlice(route)

			// then the team is resolved to its own organisation, never to one of Alice's
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"team not found"}`, body)
		})
	}
}

func TestCrossTenant_YourOrganisationsBoardListNeverShowsAnotherOrganisationsBoard(t *testing.T) {
	// given Alice owns organisation A, and a board of team B is planted among A's rows
	x := newCrossTenantHarness(t)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil).Twice()
	x.orgRepo.EXPECT().GetByID(mock.Anything, tenantOrgA).Return(&model.Organisation{ID: tenantOrgA, Name: orgName}, nil)
	x.teamRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.Team{{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}}, nil)
	x.teamRepo.EXPECT().ListForMember(mock.Anything, tenantAliceInA).Return([]model.MemberTeam{}, nil)
	own := model.BoardHeader{ID: tenantBoardA, OrgID: tenantOrgA, TeamID: new(tenantTeamA), Title: "Sprint 12", CreatedAt: time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC), TemplateKey: "mad-sad-glad", Anonymity: model.AnonymityNamed}
	planted := model.BoardHeader{ID: tenantBoardB, OrgID: tenantOrgB, TeamID: new(tenantTeamB), Title: "Planted", TemplateKey: "blank", Anonymity: model.AnonymityNamed}
	x.boardRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.BoardHeader{own, planted}, nil)
	x.boardRepo.EXPECT().ListPinned(mock.Anything, tenantAliceInA).Return([]uuid.UUID{tenantBoardB}, nil)

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodGet, path: tenantOrgAPath + "/boards"})

	// then every read is scoped to the path's organisation, and only a board of one of its own teams is listed, even with a planted pin on B
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"boards":[{"id":"`+tenantBoardA.String()+`","team_id":"`+tenantTeamA.String()+`","team_name":"`+teamName+`","title":"Sprint 12","template_key":"mad-sad-glad","created_at":"2026-10-04T09:00:00.000Z"}]}`, body)
	assert.NotContains(t, body, "Planted")
	assert.NotContains(t, body, tenantBoardB.String())
}

func TestCrossTenant_YourOrganisationsExportNeverCarriesAnotherOrganisationsBoard(t *testing.T) {
	// given Alice owns organisation A, and a board of team B is planted among A's rows
	x := newCrossTenantHarness(t)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil).Twice()
	x.orgRepo.EXPECT().GetByID(mock.Anything, tenantOrgA).Return(&model.Organisation{ID: tenantOrgA, Name: orgName, Slug: orgSlug}, nil)
	x.teamRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.Team{{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}}, nil)
	planted := model.BoardHeader{ID: tenantBoardB, OrgID: tenantOrgB, TeamID: new(tenantTeamB), Title: "Planted", TemplateKey: "blank", Anonymity: model.AnonymityNamed}
	x.boardRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.BoardHeader{planted}, nil)
	cookie := x.SignIn(tenantAlice)

	// when
	rec := x.NewRequest(http.MethodGet, tenantOrgAPath+"/export").WithCookie(cookie).WithRemoteAddr(publicPeer).RecordStream()

	// then the planted board is never resolved, read or written
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"organisation":{"name":"`+orgName+`","slug":"`+orgSlug+`"},"boards":[]}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Planted")
	assert.NotContains(t, rec.Body.String(), tenantBoardB.String())
}

func TestCrossTenant_YourOrganisationsBoardListNeverShowsAnotherOrganisationsOrganisationBoard(t *testing.T) {
	// given Alice owns organisation A, and an organisation board of B, which names no team, is planted among A's rows
	x := newCrossTenantHarness(t)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil).Twice()
	x.orgRepo.EXPECT().GetByID(mock.Anything, tenantOrgA).Return(&model.Organisation{ID: tenantOrgA, Name: orgName}, nil)
	x.teamRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.Team{}, nil)
	x.teamRepo.EXPECT().ListForMember(mock.Anything, tenantAliceInA).Return([]model.MemberTeam{}, nil)
	own := model.BoardHeader{ID: tenantBoardA, OrgID: tenantOrgA, Title: "All hands", CreatedAt: time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC), TemplateKey: "blank", Anonymity: model.AnonymityNamed}
	planted := model.BoardHeader{ID: tenantBoardB, OrgID: tenantOrgB, Title: "Planted", TemplateKey: "blank", Anonymity: model.AnonymityNamed}
	x.boardRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.BoardHeader{own, planted}, nil)
	x.boardRepo.EXPECT().ListPinned(mock.Anything, tenantAliceInA).Return([]uuid.UUID{}, nil)

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodGet, path: tenantOrgAPath + "/boards"})

	// then only A's own organisation board is listed, with no team
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"boards":[{"id":"`+tenantBoardA.String()+`","title":"All hands","template_key":"blank","created_at":"2026-10-04T09:00:00.000Z"}]}`, body)
	assert.NotContains(t, body, "Planted")
	assert.NotContains(t, body, tenantBoardB.String())
}

func TestCrossTenant_ABoardCannotBeCreatedInYourOrganisationOnAnotherOrganisationsTeam(t *testing.T) {
	// given Alice owns organisation A and names team B, which belongs to organisation B
	x := newCrossTenantHarness(t)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil).Once()
	x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
	x.orgRepo.EXPECT().GetByID(mock.Anything, tenantOrgB).Return(&model.Organisation{ID: tenantOrgB, Name: orgName}, nil)
	body := dto.CreateOrgBoardRequest{Title: "Planted", TemplateKey: "mad-sad-glad", TeamID: new(tenantTeamB)}

	// when
	status, answer := x.recordAsAlice(crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/boards", body: body})

	// then team B is not shown to exist, and no board is written
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"team not found"}`, answer)
	x.boardRepo.AssertNotCalled(t, "CreateBoard", mock.Anything, mock.Anything)
}

func TestCrossTenant_AnUnknownTeamLooksLikeAnotherOrganisationsTeam(t *testing.T) {
	// given
	x := newCrossTenantHarness(t)
	x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(nil, fmt.Errorf("get team header: %w", dao.ErrNotFound))

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodGet, path: tenantTeamBPath})

	// then
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"team not found"}`, body)
}

func TestCrossTenant_AnotherOrganisationsRecordReachedThroughYourOwnAnswersNotFound(t *testing.T) {
	aliceInA := org.Command{Actor: orgActorFor(tenantAlice), OrgID: tenantOrgA}

	tests := []struct {
		name   string
		route  crossTenantRoute
		expect func(x *crossTenantHarness)
		body   string
	}{
		{
			name:  "another organisation's member, re-roled through A",
			route: crossTenantRoute{method: http.MethodPut, path: tenantOrgAPath + "/members/" + tenantBob.String() + "/role", body: dto.ChangeMemberRoleRequest{Role: "admin"}},
			expect: func(x *crossTenantHarness) {
				change := org.RoleChange{Command: aliceInA, UserID: tenantBob, Role: model.OrgRoleAdmin}
				x.orgs.EXPECT().ChangeMemberRole(mock.Anything, change).Return(org.ErrMemberNotFound)
			},
			body: `{"error":"that person is not a member of the organisation"}`,
		},
		{
			name:  "another organisation's member, removed through A",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantOrgAPath + "/members/" + tenantBob.String()},
			expect: func(x *crossTenantHarness) {
				x.orgs.EXPECT().RemoveMember(mock.Anything, org.MemberCommand{Command: aliceInA, UserID: tenantBob}).Return(org.ErrMemberNotFound)
			},
			body: `{"error":"that person is not a member of the organisation"}`,
		},
		{
			name:  "another organisation's member, handed A's ownership",
			route: crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/transfer", body: dto.TransferOwnershipRequest{UserID: tenantBob}},
			expect: func(x *crossTenantHarness) {
				x.orgs.EXPECT().TransferOwnership(mock.Anything, org.MemberCommand{Command: aliceInA, UserID: tenantBob}).Return(org.ErrMemberNotFound)
			},
			body: `{"error":"that person is not a member of the organisation"}`,
		},
		{
			name:  "another organisation's team, archived through A",
			route: crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/teams/" + tenantTeamB.String() + "/archive"},
			expect: func(x *crossTenantHarness) {
				x.teams.EXPECT().Archive(mock.Anything, team.OrgTeamCommand{Actor: teamActorFor(tenantAlice), OrgID: tenantOrgA, TeamID: tenantTeamB}).Return(team.ErrTeamNotFound)
			},
			body: `{"error":"team not found"}`,
		},
		{
			name:  "another organisation's team, deleted through A",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantOrgAPath + "/teams/" + tenantTeamB.String()},
			expect: func(x *crossTenantHarness) {
				x.teams.EXPECT().Delete(mock.Anything, team.OrgTeamCommand{Actor: teamActorFor(tenantAlice), OrgID: tenantOrgA, TeamID: tenantTeamB}).Return(team.ErrTeamNotFound)
			},
			body: `{"error":"team not found"}`,
		},
		{
			name:  "another organisation's team, named in an invite from A",
			route: crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/invites", body: dto.CreateInviteRequest{TeamID: new(tenantTeamB)}},
			expect: func(x *crossTenantHarness) {
				x.invites.EXPECT().Create(mock.Anything, spec.InviteCreation{OrgID: tenantOrgA, ActorID: tenantAlice, TeamID: new(tenantTeamB), IP: tenantPeerIP}).Return(nil, invite.ErrTeamNotFound)
			},
			body: `{"error":"team not found"}`,
		},
		{
			name:  "another organisation's invite, resent through A",
			route: crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/invites/" + tenantInviteB.String() + "/resend"},
			expect: func(x *crossTenantHarness) {
				x.invites.EXPECT().Resend(mock.Anything, spec.InviteAction{OrgID: tenantOrgA, InviteID: tenantInviteB, ActorID: tenantAlice, IP: tenantPeerIP}).Return(nil, invite.ErrInviteNotFound)
			},
			body: `{"error":"invite not found"}`,
		},
		{
			name:  "another organisation's invite, revoked through A",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantOrgAPath + "/invites/" + tenantInviteB.String()},
			expect: func(x *crossTenantHarness) {
				x.invites.EXPECT().Revoke(mock.Anything, spec.InviteAction{OrgID: tenantOrgA, InviteID: tenantInviteB, ActorID: tenantAlice, IP: tenantPeerIP}).Return(invite.ErrInviteNotFound)
			},
			body: `{"error":"invite not found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given Alice owns organisation A, so the guard lets her through, and the record belongs to organisation B
			x := newCrossTenantHarness(t)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
			tc.expect(x)

			// when
			status, body := x.recordAsAlice(tc.route)

			// then the service is handed the path's organisation, scopes the record to it, and finds nothing
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestCrossTenant_AnotherOrganisationsMemberIsNotFoundOnYourTeam(t *testing.T) {
	bobOnTeamA := team.MemberCommand{Actor: teamActorFor(tenantAlice), TeamID: tenantTeamA, UserID: tenantBob}

	tests := []struct {
		name   string
		route  crossTenantRoute
		expect func(x *crossTenantHarness)
		body   string
	}{
		{
			name:  "added",
			route: crossTenantRoute{method: http.MethodPost, path: tenantTeamAPath + "/members", body: dto.AddTeamMemberRequest{UserID: tenantBob, Role: "member"}},
			expect: func(x *crossTenantHarness) {
				x.teams.EXPECT().AddMember(mock.Anything, team.MemberRole{MemberCommand: bobOnTeamA, Role: model.TeamRoleMember}).Return(team.ErrMemberNotFound)
			},
			body: `{"error":"that person is not a member of the organisation"}`,
		},
		{
			name:  "re-roled",
			route: crossTenantRoute{method: http.MethodPut, path: tenantTeamAPath + "/members/" + tenantBob.String() + "/role", body: dto.ChangeTeamMemberRoleRequest{Role: "lead"}},
			expect: func(x *crossTenantHarness) {
				x.teams.EXPECT().SetMemberRole(mock.Anything, team.MemberRole{MemberCommand: bobOnTeamA, Role: model.TeamRoleLead}).Return(team.ErrTeamMemberNotFound)
			},
			body: `{"error":"that person is not a member of the team"}`,
		},
		{
			name:  "removed",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantTeamAPath + "/members/" + tenantBob.String()},
			expect: func(x *crossTenantHarness) {
				x.teams.EXPECT().RemoveMember(mock.Anything, bobOnTeamA).Return(team.ErrTeamMemberNotFound)
			},
			body: `{"error":"that person is not a member of the team"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given Alice owns organisation A, so she leads its team without a membership row, and Bob is only in organisation B
			x := newCrossTenantHarness(t)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamA).Return(&model.Team{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
			tc.expect(x)

			// when
			status, body := x.recordAsAlice(tc.route)

			// then
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestCrossTenant_EveryRouteOfAnotherOrganisationsBoardAnswersNotFound(t *testing.T) {
	columnB := tenantBoardBPath + "/columns/" + tenantColumnB.String()
	cardB := tenantBoardBPath + "/cards/" + tenantCardB.String()
	fields := dto.ColumnFieldsRequest{BoardCommandRequest: tenantOpBody, Title: "Went well"}

	routes := []crossTenantRoute{
		{method: http.MethodGet, path: tenantBoardBPath},
		{method: http.MethodGet, path: tenantBoardBPath + "?known_version=9"},
		{method: http.MethodPost, path: tenantBoardBPath + "/participants/me"},
		{method: http.MethodPut, path: tenantBoardBPath + "/context", body: dto.DescribeBoardRequest{BoardCommandRequest: tenantOpBody, Context: "Two weeks of deploys"}},
		{method: http.MethodGet, path: tenantBoardBPath + "/sharing"},
		{method: http.MethodPut, path: tenantBoardBPath + "/sharing/link-access", body: dto.SetLinkAccessRequest{BoardCommandRequest: tenantOpBody, LinkAccess: model.LinkAccessGuests}},
		{method: http.MethodPut, path: tenantBoardBPath + "/sharing/password", body: dto.SetBoardPasswordRequest{BoardCommandRequest: tenantOpBody}},
		{method: http.MethodPost, path: tenantBoardBPath + "/sharing/rotate", body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/columns", body: dto.CreateColumnRequest{ColumnFieldsRequest: fields, ID: tenantNewCard}},
		{method: http.MethodPut, path: columnB, body: fields},
		{method: http.MethodPost, path: columnB + "/move", body: dto.MoveColumnRequest{BoardCommandRequest: tenantOpBody}},
		{method: http.MethodPost, path: columnB + "/clear", body: tenantOpBody},
		{method: http.MethodDelete, path: columnB, body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/cards", body: dto.CreateCardRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, ColumnID: tenantColumnB, Body: "Deploys were smooth"}},
		{method: http.MethodPost, path: tenantBoardBPath + "/cards", body: dto.CreateCardRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, ColumnID: tenantColumnB, Body: "Thanks @Bob", Mentions: []uuid.UUID{tenantBob}}},
		{method: http.MethodPut, path: cardB, body: dto.EditCardRequest{BoardCommandRequest: tenantOpBody, Body: "Deploys were smooth"}},
		{method: http.MethodPut, path: cardB, body: dto.EditCardRequest{BoardCommandRequest: tenantOpBody, Body: "Thanks @Bob", Mentions: []uuid.UUID{tenantBob}}},
		{method: http.MethodPut, path: cardB + "/colour", body: dto.RecolourCardRequest{BoardCommandRequest: tenantOpBody}},
		{method: http.MethodPost, path: cardB + "/move", body: dto.MoveCardRequest{BoardCommandRequest: tenantOpBody, ColumnID: tenantColumnB}},
		{method: http.MethodDelete, path: cardB, body: tenantOpBody},
		{method: http.MethodPost, path: cardB + "/merge", body: dto.MergeCardRequest{BoardCommandRequest: tenantOpBody, TargetCardID: tenantNewCard}},
		{method: http.MethodPost, path: cardB + "/unmerge", body: tenantOpBody},
		{method: http.MethodPost, path: cardB + "/votes", body: tenantOpBody},
		{method: http.MethodDelete, path: cardB + "/votes", body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/votes/reset", body: tenantOpBody},
		{method: http.MethodPut, path: tenantBoardBPath + "/settings", body: dto.BoardSettingsRequest{BoardCommandRequest: tenantOpBody, VotingOpen: new(false)}},
		{method: http.MethodPost, path: cardB + "/comments", body: dto.CreateCommentRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, Body: "Agreed"}},
		{method: http.MethodPost, path: cardB + "/comments", body: dto.CreateCommentRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, Body: "Agreed, @Bob", Mentions: []uuid.UUID{tenantBob}}},
		{method: http.MethodPut, path: tenantBoardBPath + "/comments/" + tenantCommentB.String(), body: dto.EditCommentRequest{BoardCommandRequest: tenantOpBody, Body: "Agreed"}},
		{method: http.MethodPut, path: tenantBoardBPath + "/comments/" + tenantCommentB.String(), body: dto.EditCommentRequest{BoardCommandRequest: tenantOpBody, Body: "Agreed, @Bob", Mentions: []uuid.UUID{tenantBob}}},
		{method: http.MethodDelete, path: tenantBoardBPath + "/comments/" + tenantCommentB.String(), body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/reveal", body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/hide", body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/timer", body: dto.TimerRequest{BoardCommandRequest: tenantOpBody, Action: model.TimerActionStart, DurationMS: 60000}},
		{method: http.MethodPost, path: tenantBoardBPath + "/stage", body: dto.StageRequest{BoardCommandRequest: tenantOpBody, Stage: model.StageVote}},
		{method: http.MethodPost, path: tenantBoardBPath + "/close", body: tenantOpBody},
		{method: http.MethodPut, path: tenantBoardBPath + "/participants/" + tenantSeatB.String() + "/role", body: dto.ParticipantRoleRequest{BoardCommandRequest: tenantOpBody, Role: model.BoardRoleFacilitator}},
		{method: http.MethodDelete, path: tenantBoardBPath + "/participants/" + tenantSeatB.String(), body: tenantOpBody},
		{method: http.MethodGet, path: tenantBoardBPath + "/people"},
		{method: http.MethodPost, path: tenantBoardBPath + "/action-items", body: dto.CreateActionItemRequest{ActionItemFieldsRequest: tenantItemFields, ID: tenantNewCard, SourceCardID: new(tenantCardB)}},
		{method: http.MethodPut, path: tenantBoardBPath + "/action-items/" + tenantItemB.String(), body: tenantItemFields},
		{method: http.MethodPost, path: tenantBoardBPath + "/action-items/" + tenantItemB.String() + "/status", body: dto.ActionItemStatusRequest{BoardCommandRequest: tenantOpBody, Status: model.ActionItemDone}},
		{method: http.MethodDelete, path: tenantBoardBPath + "/action-items/" + tenantItemB.String(), body: tenantOpBody},
		{method: http.MethodPost, path: tenantBoardBPath + "/previous-actions/" + tenantItemB.String() + "/review", body: dto.ReviewActionItemRequest{BoardCommandRequest: tenantOpBody, Outcome: model.ReviewDone}},
		{method: http.MethodGet, path: tenantBoardBPath + "/export?format=csv"},
		{method: http.MethodGet, path: tenantBoardBPath + "/export?format=markdown"},
		{method: http.MethodGet, path: tenantBoardBPath + "/export?format=json"},
		{method: http.MethodPost, path: tenantBoardBPath + "/saved-templates", body: dto.SaveTemplateRequest{Name: "Planted", Scope: "personal"}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat on the board
			x := newCrossTenantHarness(t)
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))

			// when
			status, body := x.recordAsAlice(route)

			// then the live board service resolves the board to its own organisation, B is not even shown to exist, and nothing is read or written past that
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}

func TestCrossTenant_ARemovedMembersOldBoardAnswersNotFound(t *testing.T) {
	oldSeat := &model.BoardParticipant{ID: tenantAliceSeatInA, BoardID: tenantBoardA, UserID: new(tenantAlice), Role: model.BoardRoleParticipant, JoinedVia: model.JoinPathMembership}
	routes := []crossTenantRoute{
		{method: http.MethodGet, path: tenantBoardAPath},
		{method: http.MethodPost, path: tenantBoardAPath + "/cards", body: dto.CreateCardRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, ColumnID: tenantColumnA, Body: "Deploys were smooth"}},
	}
	removals := []struct {
		name   string
		expect func(x *crossTenantHarness)
	}{
		{name: "removed from the organisation", expect: func(x *crossTenantHarness) {
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(nil, errTenantNoMember)
		}},
		{name: "removed from the team", expect: func(x *crossTenantHarness) {
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(&model.OrganisationMember{OrgID: tenantOrgA, UserID: tenantAlice, Role: model.OrgRoleMember}, nil)
			x.teamRepo.EXPECT().GetMember(mock.Anything, spec.TeamMemberKey{OrgID: tenantOrgA, TeamID: tenantTeamA, UserID: tenantAlice}).Return(nil, fmt.Errorf("get team member: %w", dao.ErrNotFound))
		}},
	}

	for _, removal := range removals {
		for _, link := range []model.LinkAccess{model.LinkAccessOff, model.LinkAccessAccounts, model.LinkAccessGuests} {
			for _, route := range routes {
				t.Run(removal.name+"/link "+string(link)+"/"+route.method+" "+route.path, func(t *testing.T) {
					// given Alice joined board A as a member, kept her seat, and has since been removed
					x := newCrossTenantHarness(t)
					header := *tenantBoardAOpen
					header.LinkAccess = link
					x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardA).Return(&header, nil)
					x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamA).Return(&model.Team{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}, nil)
					removal.expect(x)
					x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardA, UserID: tenantAlice}).Return(oldSeat, nil)

					// when
					status, body := x.recordAsAlice(route)

					// then a seat from a member join is no link credential, so the board is not even shown to exist
					assert.Equal(t, http.StatusNotFound, status)
					assert.JSONEq(t, `{"error":"board not found"}`, body)
				})
			}
		}
	}
}

func TestCrossTenant_AnUnknownBoardLooksLikeAnotherOrganisationsBoard(t *testing.T) {
	// given
	x := newCrossTenantHarness(t)
	x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(nil, fmt.Errorf("get board header: %w", dao.ErrNotFound))

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodGet, path: tenantBoardBPath})

	// then
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"board not found"}`, body)
}

func TestCrossTenant_AnotherBoardsRecordReachedThroughYourOwnBoardAnswersNotFound(t *testing.T) {
	columnBThroughA := tenantBoardAPath + "/columns/" + tenantColumnB.String()
	cardBThroughA := tenantBoardAPath + "/cards/" + tenantCardB.String()
	cardBOnA := spec.CardRef{BoardID: tenantBoardA, CardID: tenantCardB}
	columnBOnA := spec.BoardColumnKey{BoardID: tenantBoardA, ColumnID: tenantColumnB}
	errNoCard := fmt.Errorf("get card: %w", dao.ErrNotFound)
	errNoColumn := fmt.Errorf("get column: %w", dao.ErrNotFound)

	tests := []struct {
		name   string
		route  crossTenantRoute
		expect func(x *crossTenantHarness)
		body   string
	}{
		{
			name:  "another board's column, given a new card",
			route: crossTenantRoute{method: http.MethodPost, path: tenantBoardAPath + "/cards", body: dto.CreateCardRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, ColumnID: tenantColumnB, Body: "Deploys were smooth"}},
			expect: func(x *crossTenantHarness) {
				x.columnRepo.EXPECT().GetByID(mock.Anything, columnBOnA).Return(nil, errNoColumn)
			},
			body: `{"error":"column not found"}`,
		},
		{
			name:  "another board's column, edited",
			route: crossTenantRoute{method: http.MethodPut, path: columnBThroughA, body: dto.ColumnFieldsRequest{BoardCommandRequest: tenantOpBody, Title: "Went well"}},
			expect: func(x *crossTenantHarness) {
				edit := spec.ColumnEdit{BoardWrite: tenantWriteToA, ColumnID: tenantColumnB, Title: "Went well", Colour: model.ColourGrey}
				x.columnRepo.EXPECT().EditColumn(mock.Anything, edit).Return(nil, errNoColumn)
			},
			body: `{"error":"column not found"}`,
		},
		{
			name:  "another board's column, moved",
			route: crossTenantRoute{method: http.MethodPost, path: columnBThroughA + "/move", body: dto.MoveColumnRequest{BoardCommandRequest: tenantOpBody}},
			expect: func(x *crossTenantHarness) {
				x.columnRepo.EXPECT().List(mock.Anything, tenantBoardA).Return([]model.BoardColumn{{ID: tenantColumnA, BoardID: tenantBoardA, Title: "Went well", Position: "a0"}}, nil)
			},
			body: `{"error":"column not found"}`,
		},
		{
			name:  "another board's column, cleared",
			route: crossTenantRoute{method: http.MethodPost, path: columnBThroughA + "/clear", body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.columnRepo.EXPECT().GetByID(mock.Anything, columnBOnA).Return(nil, errNoColumn)
			},
			body: `{"error":"column not found"}`,
		},
		{
			name:  "another board's column, deleted",
			route: crossTenantRoute{method: http.MethodDelete, path: columnBThroughA, body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.columnRepo.EXPECT().DeleteColumn(mock.Anything, spec.ColumnRemoval{BoardWrite: tenantWriteToA, ColumnID: tenantColumnB}).Return(nil, errNoColumn)
			},
			body: `{"error":"column not found"}`,
		},
		{
			name:  "another board's card, edited",
			route: crossTenantRoute{method: http.MethodPut, path: cardBThroughA, body: dto.EditCardRequest{BoardCommandRequest: tenantOpBody, Body: "Deploys were smooth"}},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's card, recoloured",
			route: crossTenantRoute{method: http.MethodPut, path: cardBThroughA + "/colour", body: dto.RecolourCardRequest{BoardCommandRequest: tenantOpBody}},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's card, moved",
			route: crossTenantRoute{method: http.MethodPost, path: cardBThroughA + "/move", body: dto.MoveCardRequest{BoardCommandRequest: tenantOpBody, ColumnID: tenantColumnA}},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's card, deleted",
			route: crossTenantRoute{method: http.MethodDelete, path: cardBThroughA, body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's card, voted for",
			route: crossTenantRoute{method: http.MethodPost, path: cardBThroughA + "/votes", body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's card, vote taken back",
			route: crossTenantRoute{method: http.MethodDelete, path: cardBThroughA + "/votes", body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's card, commented on",
			route: crossTenantRoute{method: http.MethodPost, path: cardBThroughA + "/comments", body: dto.CreateCommentRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, Body: "Agreed"}},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's comment, edited",
			route: crossTenantRoute{method: http.MethodPut, path: tenantBoardAPath + "/comments/" + tenantCommentB.String(), body: dto.EditCommentRequest{BoardCommandRequest: tenantOpBody, Body: "Agreed"}},
			expect: func(x *crossTenantHarness) {
				x.noteRepo.EXPECT().GetByID(mock.Anything, spec.CardCommentRef{BoardID: tenantBoardA, CommentID: tenantCommentB}).Return(nil, fmt.Errorf("get comment: %w", dao.ErrNotFound))
			},
			body: `{"error":"comment not found"}`,
		},
		{
			name:  "another board's comment, deleted",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantBoardAPath + "/comments/" + tenantCommentB.String(), body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.noteRepo.EXPECT().GetByID(mock.Anything, spec.CardCommentRef{BoardID: tenantBoardA, CommentID: tenantCommentB}).Return(nil, fmt.Errorf("get comment: %w", dao.ErrNotFound))
			},
			body: `{"error":"comment not found"}`,
		},
		{
			name:  "another board's card, unmerged",
			route: crossTenantRoute{method: http.MethodPost, path: cardBThroughA + "/unmerge", body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another board's participant, promoted",
			route: crossTenantRoute{method: http.MethodPut, path: tenantBoardAPath + "/participants/" + tenantSeatB.String() + "/role", body: dto.ParticipantRoleRequest{BoardCommandRequest: tenantOpBody, Role: model.BoardRoleFacilitator}},
			expect: func(x *crossTenantHarness) {
				x.boardRepo.EXPECT().GetParticipant(mock.Anything, spec.BoardParticipantKey{BoardID: tenantBoardA, ParticipantID: tenantSeatB}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))
			},
			body: `{"error":"participant not found"}`,
		},
		{
			name:  "another board's participant, removed",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantBoardAPath + "/participants/" + tenantSeatB.String(), body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.boardRepo.EXPECT().GetParticipant(mock.Anything, spec.BoardParticipantKey{BoardID: tenantBoardA, ParticipantID: tenantSeatB}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))
			},
			body: `{"error":"participant not found"}`,
		},
		{
			name:  "your own card, merged into another board's card",
			route: crossTenantRoute{method: http.MethodPost, path: tenantBoardAPath + "/cards/" + tenantCardA.String() + "/merge", body: dto.MergeCardRequest{BoardCommandRequest: tenantOpBody, TargetCardID: tenantCardB}},
			expect: func(x *crossTenantHarness) {
				ownCard := &model.Card{ID: tenantCardA, BoardID: tenantBoardA, ColumnID: tenantColumnA, AuthorParticipantID: new(tenantAliceSeatInA), Body: "Deploys were smooth", Position: "a0"}
				x.cardRepo.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: tenantBoardA, CardID: tenantCardA}).Return(ownCard, nil)
				x.cardRepo.EXPECT().GetByID(mock.Anything, cardBOnA).Return(nil, errNoCard)
			},
			body: `{"error":"card not found"}`,
		},
		{
			name:  "another organisation's action item, deleted",
			route: crossTenantRoute{method: http.MethodDelete, path: tenantBoardAPath + "/action-items/" + tenantItemB.String(), body: tenantOpBody},
			expect: func(x *crossTenantHarness) {
				x.itemRepo.EXPECT().GetByID(mock.Anything, spec.ActionItemKey{OrgID: tenantOrgA, ItemID: tenantItemB}).Return(nil, fmt.Errorf("get action item: %w", dao.ErrNotFound))
			},
			body: `{"error":"action item not found"}`,
		},
		{
			name:  "another organisation's action item, reviewed as a previous action",
			route: crossTenantRoute{method: http.MethodPost, path: tenantBoardAPath + "/previous-actions/" + tenantItemB.String() + "/review", body: dto.ReviewActionItemRequest{BoardCommandRequest: tenantOpBody, Outcome: model.ReviewDone}},
			expect: func(x *crossTenantHarness) {
				query := spec.PreviousActionsQuery{OrgID: tenantOrgA, TeamID: tenantTeamA, BoardID: tenantBoardA, Before: tenantBoardAOpen.CreatedAt}
				x.itemRepo.EXPECT().ListPrevious(mock.Anything, query).Return([]model.PreviousAction{}, nil)
			},
			body: `{"error":"action item not found"}`,
		},
		{
			name:  "your own card, moved into another board's column",
			route: crossTenantRoute{method: http.MethodPost, path: tenantBoardAPath + "/cards/" + tenantCardA.String() + "/move", body: dto.MoveCardRequest{BoardCommandRequest: tenantOpBody, ColumnID: tenantColumnB}},
			expect: func(x *crossTenantHarness) {
				ownCard := &model.Card{ID: tenantCardA, BoardID: tenantBoardA, ColumnID: tenantColumnA, AuthorParticipantID: new(tenantAliceSeatInA), Body: "Deploys were smooth", Position: "a0"}
				x.cardRepo.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: tenantBoardA, CardID: tenantCardA}).Return(ownCard, nil)
				x.columnRepo.EXPECT().GetByID(mock.Anything, columnBOnA).Return(nil, errNoColumn)
			},
			body: `{"error":"column not found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given Alice owns organisation A and hosts board A, and the record belongs to board B
			x := newCrossTenantHarness(t)
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardA).Return(tenantBoardAOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamA).Return(&model.Team{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardA, UserID: tenantAlice}).Return(tenantAliceSeat, nil)
			x.boardRepo.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: tenantBoardA, OpID: tenantOp}).Return(nil, fmt.Errorf("get op: %w", dao.ErrNotFound))
			tc.expect(x)

			// when
			status, body := x.recordAsAlice(tc.route)

			// then every read and write is keyed on the board Alice reached, so board B's record is simply not there, and no version is burned
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestCrossTenant_AnotherOrganisationsMemberIsNeverMentionedOnYourBoard(t *testing.T) {
	discussing := *tenantBoardAOpen
	discussing.Stage = model.StageDiscuss
	discussing.CardsHidden = false
	discussing.CommentsEnabled = true
	ownCard := &model.Card{ID: tenantCardA, BoardID: tenantBoardA, ColumnID: tenantColumnA, AuthorParticipantID: new(tenantAliceSeatInA), Body: "Deploys were smooth", Position: "a0"}

	tests := []struct {
		name   string
		header *model.BoardHeader
		route  crossTenantRoute
		expect func(x *crossTenantHarness)
	}{
		{
			name:   "on a new card",
			header: tenantBoardAOpen,
			route:  crossTenantRoute{method: http.MethodPost, path: tenantBoardAPath + "/cards", body: dto.CreateCardRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, ColumnID: tenantColumnA, Body: "Thanks @Bob", Mentions: []uuid.UUID{tenantBob}}},
			expect: func(x *crossTenantHarness) {
				column := &model.BoardColumn{ID: tenantColumnA, BoardID: tenantBoardA, Title: "Went well", Position: "a0"}
				x.columnRepo.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: tenantBoardA, ColumnID: tenantColumnA}).Return(column, nil)
			},
		},
		{
			name:   "on a new comment",
			header: &discussing,
			route:  crossTenantRoute{method: http.MethodPost, path: tenantBoardAPath + "/cards/" + tenantCardA.String() + "/comments", body: dto.CreateCommentRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, Body: "Agreed, @Bob", Mentions: []uuid.UUID{tenantBob}}},
			expect: func(x *crossTenantHarness) {
				x.cardRepo.EXPECT().GetByID(mock.Anything, spec.CardRef{BoardID: tenantBoardA, CardID: tenantCardA}).Return(ownCard, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given Alice owns organisation A and hosts board A, and Bob belongs only to organisation B
			x := newCrossTenantHarness(t)
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardA).Return(tc.header, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamA).Return(&model.Team{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardA, UserID: tenantAlice}).Return(tenantAliceSeat, nil)
			x.boardRepo.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: tenantBoardA, OpID: tenantOp}).Return(nil, fmt.Errorf("get op: %w", dao.ErrNotFound))
			x.orgRepo.EXPECT().ListMembers(mock.Anything, tenantOrgA).Return([]model.OrganisationMemberProfile{{OrganisationMember: *tenantAliceOwnsA, DisplayName: "Alice"}}, nil)
			x.teamRepo.EXPECT().ListMembers(mock.Anything, spec.TeamKey{OrgID: tenantOrgA, TeamID: tenantTeamA}).Return([]model.TeamMemberProfile{}, nil)
			tc.expect(x)

			// when
			status, body := x.recordAsAlice(tc.route)

			// then the people who may be named are read from the board's own organisation, so Bob is refused and nothing is written
			assert.Equal(t, http.StatusBadRequest, status)
			assert.JSONEq(t, `{"error":"an assignee or a mention must be a member who can open the board","code":"not_board_member"}`, body)
			x.cardRepo.AssertNotCalled(t, "AddCard", mock.Anything, mock.Anything)
			x.noteRepo.AssertNotCalled(t, "AddComment", mock.Anything, mock.Anything)
		})
	}
}

func TestCrossTenant_TheMentionsListNeverShowsAnotherOrganisationsBoard(t *testing.T) {
	// given Alice was mentioned on a card of team B and in a comment on an organisation board of B, and has since left organisation B
	x := newCrossTenantHarness(t)
	query := spec.UserMentionsQuery{UserID: tenantAlice, Limit: 200}
	x.itemRepo.EXPECT().ListMentionsForUser(mock.Anything, query).Return([]model.Mention{}, nil)
	x.cardRepo.EXPECT().ListMentionsForUser(mock.Anything, query).Return([]model.Mention{
		{Kind: model.MentionCard, CardID: tenantCardB, OrgID: tenantOrgB, OrgSlug: "planted", OrgName: "Planted", TeamID: new(tenantTeamB), BoardID: new(tenantBoardB), BoardTitle: "Planted board", Body: "SECRET-TEXT-9f2c"},
	}, nil)
	x.noteRepo.EXPECT().ListMentionsForUser(mock.Anything, query).Return([]model.Mention{
		{Kind: model.MentionComment, CardID: tenantCardB, CommentID: tenantCommentB, OrgID: tenantOrgB, OrgSlug: "planted", OrgName: "Planted", BoardID: new(tenantBoardB), BoardTitle: "Planted board", Body: "SECRET-COMMENT-3a9d"},
	}, nil)
	x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodGet, path: "/api/v1/auth/mentions"})

	// then each mention is checked against the access Alice holds now, so nothing of organisation B is shown
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"mentions":[]}`, body)
	assert.NotContains(t, body, "SECRET")
	assert.NotContains(t, body, "Planted")
}

func TestCrossTenant_AGuestAfterALinkRotationAnswersNotFound(t *testing.T) {
	device := uuid.MustParse("f4566778-899a-4b5c-8d7e-8f90a1b2c3d4")
	staleSeat := &model.BoardParticipant{ID: tenantAliceSeatInA, BoardID: tenantBoardA, GuestDeviceID: new(device), Role: model.BoardRoleParticipant, JoinedVia: model.JoinPathLink, AccessEpoch: 2}
	routes := []crossTenantRoute{
		{method: http.MethodGet, path: tenantBoardAPath},
		{method: http.MethodPost, path: tenantBoardAPath + "/participants/me"},
		{method: http.MethodPost, path: tenantBoardAPath + "/cards", body: dto.CreateCardRequest{BoardCommandRequest: tenantOpBody, ID: tenantNewCard, ColumnID: tenantColumnA, Body: "Deploys were smooth"}},
		{method: http.MethodGet, path: tenantBoardAPath + "/sharing"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given a guest who joined board A through its link before the facilitator rotated it, with no password set
			x := newCrossTenantHarness(t)
			header := *tenantBoardAOpen
			header.LinkAccess = model.LinkAccessGuests
			header.OrgAllowsGuests = true
			header.AccessEpoch = 3
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardA).Return(&header, nil)
			x.boardRepo.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: tenantBoardA, GuestDeviceID: device}).Return(staleSeat, nil)
			cookie := x.SignInAsGuest(device)
			request := x.NewRequest(route.method, route.path).WithCookie(cookie)
			if route.body != nil {
				request = request.WithJSONBody(route.body)
			}

			// when
			rec := request.Record()

			// then the old seat is no credential at the new epoch, and the board is not even shown to exist
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"error":"board not found"}`, rec.Body.String())
		})
	}
}

func TestCrossTenant_AGuestOfOneBoardNeverReachesAnother(t *testing.T) {
	// given a guest seated on board A, and board B open to guests through its own link
	x := newCrossTenantHarness(t)
	device := uuid.MustParse("05677889-9aab-4c6d-9e8f-90a1b2c3d4e5")
	header := *tenantBoardBOpen
	header.LinkAccess = model.LinkAccessGuests
	header.OrgAllowsGuests = true
	x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(&header, nil)
	x.boardRepo.EXPECT().GetParticipantForGuest(mock.Anything, spec.BoardParticipantGuest{BoardID: tenantBoardB, GuestDeviceID: device}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))
	cookie := x.SignInAsGuest(device)

	// when
	rec := x.NewRequest(http.MethodGet, tenantBoardBPath).WithCookie(cookie).Record()

	// then a guest holds no seat anywhere it did not join through that board's own link
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"board not found"}`, rec.Body.String())
}

func TestCrossTenant_ARotatedLinkNoLongerFindsTheBoard(t *testing.T) {
	// given the slug a guest was sent before the rotation
	x := newCrossTenantHarness(t)
	x.boardRepo.EXPECT().GetHeaderBySlug(mock.Anything, "OLDSLUGOLDSLUGOLDSLUG234").Return(nil, fmt.Errorf("get the board header for a share link: %w", dao.ErrNotFound))

	// when
	rec := x.NewRequest(http.MethodGet, "/api/v1/b/OLDSLUGOLDSLUGOLDSLUG234").Record()

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"board not found"}`, rec.Body.String())
}

func (x *crossTenantHarness) aliceViewsOrganisationA() {
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil).Twice()
	x.orgRepo.EXPECT().GetByID(mock.Anything, tenantOrgA).Return(&model.Organisation{ID: tenantOrgA, Name: orgName}, nil)
	x.teamRepo.EXPECT().ListForOrganisation(mock.Anything, tenantOrgA).Return([]model.Team{{ID: tenantTeamA, OrgID: tenantOrgA, Name: teamName}}, nil)
	x.teamRepo.EXPECT().ListForMember(mock.Anything, tenantAliceInA).Return([]model.MemberTeam{}, nil)
}

func TestCrossTenant_YourOrganisationsSavedTemplatesNeverShowAnotherOrganisationsTemplate(t *testing.T) {
	// given Alice owns organisation A, and a team template of organisation B is planted among A's rows
	x := newCrossTenantHarness(t)
	x.aliceViewsOrganisationA()
	columns := []model.SavedTemplateColumn{{Title: "Went well", Description: "What went well?", Colour: model.ColourGreen}}
	own := model.SavedTemplate{ID: uuid.MustParse("5abbccdd-eeff-4a01-b2c3-e4f506172839"), OrgID: tenantOrgA, OwnerID: new(tenantAlice), CreatedBy: new(tenantAlice), Name: "Ours", Columns: columns, CreatedAt: time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)}
	planted := model.SavedTemplate{ID: tenantTemplateB, OrgID: tenantOrgB, TeamID: new(tenantTeamB), CreatedBy: new(tenantBob), Name: "Planted", TeamName: teamName, Columns: columns, CreatedAt: time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)}
	x.savedRepo.EXPECT().List(mock.Anything, spec.SavedTemplateViewer{OrgID: tenantOrgA, UserID: tenantAlice}).Return([]model.SavedTemplate{own, planted}, nil)

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodGet, path: tenantOrgAPath + "/saved-templates"})

	// then every read is scoped to the path's organisation, and only A's own template is listed
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "Ours")
	assert.NotContains(t, body, "Planted")
	assert.NotContains(t, body, tenantTemplateB.String())
}

func TestCrossTenant_AnotherOrganisationsSavedTemplateCannotBeDeletedThroughYourOwn(t *testing.T) {
	// given Alice owns organisation A and names a template of organisation B
	x := newCrossTenantHarness(t)
	x.aliceViewsOrganisationA()
	x.savedRepo.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: tenantOrgA, ID: tenantTemplateB}).Return(nil, fmt.Errorf("get saved template: %w", dao.ErrNotFound))

	// when
	status, body := x.recordAsAlice(crossTenantRoute{method: http.MethodDelete, path: tenantOrgAPath + "/saved-templates/" + tenantTemplateB.String()})

	// then the read is scoped to A, so B's template is not shown to exist and nothing is deleted
	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"template not found"}`, body)
	x.savedRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestCrossTenant_AnotherOrganisationsSavedTemplateCannotSeedABoardInYourOwn(t *testing.T) {
	// given Alice owns organisation A and names a template of organisation B
	x := newCrossTenantHarness(t)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInA).Return(tenantAliceOwnsA, nil).Twice()
	x.orgRepo.EXPECT().GetByID(mock.Anything, tenantOrgA).Return(&model.Organisation{ID: tenantOrgA, Name: orgName}, nil)
	x.savedRepo.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: tenantOrgA, ID: tenantTemplateB}).Return(nil, fmt.Errorf("get saved template: %w", dao.ErrNotFound))
	body := dto.CreateOrgBoardRequest{CreateBoardRequest: dto.CreateBoardRequest{Title: "Planted", SavedTemplateID: new(tenantTemplateB)}}

	// when
	status, answer := x.recordAsAlice(crossTenantRoute{method: http.MethodPost, path: tenantOrgAPath + "/boards", body: body})

	// then the template is looked up in A only, answers like an unknown key, and no board is written
	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"`+board.ErrUnknownTemplate.Error()+`","code":"unknown_template"}`, answer)
	x.boardRepo.AssertNotCalled(t, "CreateBoard", mock.Anything, mock.Anything)
}
