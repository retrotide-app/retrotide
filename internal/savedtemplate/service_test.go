package savedtemplate

import (
	"errors"
	"strings"
	"testing"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
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
		savedTemplates *repository.MockSavedTemplateRepository
		columns        *repository.MockBoardColumnRepository
		teams          *repository.MockTeamRepository
		organisations  *repository.MockOrganisationRepository
		live           *live.MockService
		orgAuthz       *orgauthz.MockService
		entitlements   *entitlements.MockService
	}
)

var (
	testCreated = time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		savedTemplates: repository.NewMockSavedTemplateRepository(t),
		columns:        repository.NewMockBoardColumnRepository(t),
		teams:          repository.NewMockTeamRepository(t),
		organisations:  repository.NewMockOrganisationRepository(t),
		live:           live.NewMockService(t),
		orgAuthz:       orgauthz.NewMockService(t),
		entitlements:   entitlements.NewMockService(t),
	}

	svc := NewService(Deps{
		SavedTemplates: m.savedTemplates,
		Columns:        m.columns,
		Teams:          m.teams,
		Organisations:  m.organisations,
		Live:           m.live,
		OrgAuthz:       m.orgAuthz,
		Entitlements:   m.entitlements,
	}).(*service)

	return svc, m
}

func freePlan() *entitlements.Plan {
	return &entitlements.Plan{Code: model.PlanFree, Limits: map[model.Limit]int{model.LimitSavedTemplates: 3}}
}

func user(id uuid.UUID) model.Principal {
	return model.Principal{Kind: model.PrincipalUser, ID: id}
}

func boardAccess(principal model.Principal, orgID uuid.UUID, teamID *uuid.UUID, capabilities ...model.BoardCapability) *model.BoardAccess {
	return &model.BoardAccess{
		Principal:    principal,
		Header:       model.BoardHeader{Board: model.Board{ID: uuid.New(), OrgID: orgID, TeamID: teamID, Title: "Sprint 12"}},
		Role:         model.BoardRoleFacilitator,
		OrgMember:    true,
		Capabilities: capabilities,
	}
}

func boardColumns(boardID uuid.UUID) []model.BoardColumn {
	return []model.BoardColumn{
		{ID: uuid.New(), BoardID: boardID, Title: "Went well", Description: "What went well?", Colour: model.ColourGreen, Position: "a0"},
		{ID: uuid.New(), BoardID: boardID, Title: "To improve", Description: "What could be better?", Colour: model.ColourAmber, Position: "a1"},
	}
}

func templateColumns() []model.SavedTemplateColumn {
	return []model.SavedTemplateColumn{
		{Title: "Went well", Description: "What went well?", Colour: model.ColourGreen},
		{Title: "To improve", Description: "What could be better?", Colour: model.ColourAmber},
	}
}

func saveCommand(principal model.Principal, boardID uuid.UUID, scope model.SavedTemplateScope) SaveCommand {
	return SaveCommand{Principal: principal, BoardID: boardID, Name: "  Our retro  ", Description: "  The format we like  ", Scope: scope}
}

func (m *testMocks) resolves(access *model.BoardAccess) {
	query := spec.BoardAccessQuery{Principal: access.Principal, BoardID: access.Header.ID}

	m.live.EXPECT().Resolve(mock.Anything, query).Return(access, nil)
}

func (m *testMocks) withinTheFreePlan(orgID uuid.UUID, current int) {
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(), nil)
	m.savedTemplates.EXPECT().Count(mock.Anything, orgID).Return(current, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, spec.LimitCheck{OrgID: orgID, Limit: model.LimitSavedTemplates, Current: current}).Return(nil)
}

func (m *testMocks) member(orgID uuid.UUID, userID uuid.UUID, role model.OrgRole, teams []model.Team, joined []model.MemberTeam) {
	key := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}

	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme Retros"}, nil)
	m.organisations.EXPECT().GetMember(mock.Anything, key).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: role, JoinedAt: testCreated}, nil)
	m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return(teams, nil)
	m.teams.EXPECT().ListForMember(mock.Anything, key).Return(joined, nil)
}

func team(orgID uuid.UUID, name string) model.Team {
	return model.Team{ID: uuid.New(), OrgID: orgID, Name: name, CreatedAt: testCreated}
}

func joinedAs(t model.Team, role model.TeamRole) model.MemberTeam {
	return model.MemberTeam{Team: t, Role: role, JoinedAt: testCreated}
}

func personal(orgID uuid.UUID, ownerID uuid.UUID) model.SavedTemplate {
	return model.SavedTemplate{ID: uuid.New(), OrgID: orgID, OwnerID: new(ownerID), CreatedBy: new(ownerID), Name: "Mine", Columns: templateColumns(), CreatedAt: testCreated}
}

func forTeam(orgID uuid.UUID, t model.Team, createdBy uuid.UUID) model.SavedTemplate {
	return model.SavedTemplate{ID: uuid.New(), OrgID: orgID, TeamID: new(t.ID), CreatedBy: new(createdBy), Name: "Team format", TeamName: t.Name, Columns: templateColumns(), CreatedAt: testCreated}
}

func TestCleanName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "trimmed", in: "  Our retro  ", want: "Our retro"},
		{name: "eighty characters", in: strings.Repeat("é", 80), want: strings.Repeat("é", 80)},
		{name: "empty", in: "", wantErr: ErrNameRequired},
		{name: "only spaces", in: " \t ", wantErr: ErrNameRequired},
		{name: "eighty-one characters", in: strings.Repeat("é", 81), wantErr: ErrNameTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, err := CleanName(tt.in)

			// then
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCleanDescription(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "empty is fine", in: "", want: ""},
		{name: "trimmed", in: "  The format we like  ", want: "The format we like"},
		{name: "three hundred characters", in: strings.Repeat("é", 300), want: strings.Repeat("é", 300)},
		{name: "three hundred and one characters", in: strings.Repeat("é", 301), wantErr: ErrDescriptionTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			got, err := CleanDescription(tt.in)

			// then
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCleanScope(t *testing.T) {
	for _, scope := range []model.SavedTemplateScope{model.SavedTemplatePersonal, model.SavedTemplateTeam} {
		// when
		got, err := CleanScope(scope)

		// then
		require.NoError(t, err)
		assert.Equal(t, scope, got)
	}

	for _, scope := range []model.SavedTemplateScope{"", "org", "Personal"} {
		// when
		got, err := CleanScope(scope)

		// then
		assert.ErrorIs(t, err, ErrInvalidScope, scope)
		assert.Empty(t, got)
	}
}

func TestSave_SavesAPersonalTemplateFromTheBoardColumnsInOrder(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	access := boardAccess(user(userID), orgID, nil, model.BoardCapabilityManage)
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.withinTheFreePlan(orgID, 2)
	created := &model.SavedTemplate{ID: uuid.New(), OrgID: orgID, OwnerID: new(userID), CreatedBy: new(userID), Name: "Our retro", Description: "The format we like", Columns: templateColumns(), CreatedAt: testCreated}
	m.savedTemplates.EXPECT().Create(mock.Anything, spec.NewSavedTemplate{
		OrgID:        orgID,
		OwnerID:      new(userID),
		CreatedBy:    userID,
		Name:         "Our retro",
		Description:  "The format we like",
		Columns:      templateColumns(),
		MaxTemplates: 3,
	}).Return(created, nil)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplatePersonal))

	// then only the column titles, descriptions and colours are copied
	require.NoError(t, err)
	assert.Equal(t, dto.NewSavedTemplateResponse(*created, true), *got)
	assert.Equal(t, "personal", got.Scope)
	assert.Nil(t, got.TeamID)
}

func TestSave_SavesATeamTemplateWithTheTeamName(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	platform := team(orgID, "Platform")
	access := boardAccess(user(userID), orgID, new(platform.ID), model.BoardCapabilityManage)
	m.resolves(access)
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: platform.ID, UserID: userID, Permission: orgauthz.PermTeamCreateBoard}).Return(orgauthz.Decision{Member: true, Allowed: true}, nil)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.withinTheFreePlan(orgID, 0)
	created := &model.SavedTemplate{ID: uuid.New(), OrgID: orgID, TeamID: new(platform.ID), CreatedBy: new(userID), Name: "Our retro", Description: "The format we like", Columns: templateColumns(), CreatedAt: testCreated}
	m.savedTemplates.EXPECT().Create(mock.Anything, spec.NewSavedTemplate{
		OrgID:        orgID,
		TeamID:       new(platform.ID),
		CreatedBy:    userID,
		Name:         "Our retro",
		Description:  "The format we like",
		Columns:      templateColumns(),
		MaxTemplates: 3,
	}).Return(created, nil)
	m.teams.EXPECT().GetByID(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: platform.ID}).Return(&platform, nil)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplateTeam))

	// then
	require.NoError(t, err)
	assert.Equal(t, "team", got.Scope)
	assert.Equal(t, new(platform.ID), got.TeamID)
	assert.Equal(t, "Platform", got.TeamName)
	assert.True(t, got.CanDelete)
}

func TestSave_AGuestCannotSave(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	guest := model.Principal{Kind: model.PrincipalGuest, ID: uuid.New()}

	// when
	got, err := svc.Save(t.Context(), saveCommand(guest, uuid.New(), model.SavedTemplatePersonal))

	// then the strict mocks prove the board was never read
	assert.ErrorIs(t, err, ErrNotAllowed)
	assert.Nil(t, got)
}

func TestSave_TheFieldsAreCheckedBeforeTheBoardIsRead(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(command *SaveCommand)
		wantErr error
	}{
		{name: "blank name", mutate: func(command *SaveCommand) { command.Name = "  " }, wantErr: ErrNameRequired},
		{name: "long name", mutate: func(command *SaveCommand) { command.Name = strings.Repeat("a", 81) }, wantErr: ErrNameTooLong},
		{name: "long description", mutate: func(command *SaveCommand) { command.Description = strings.Repeat("a", 301) }, wantErr: ErrDescriptionTooLong},
		{name: "unknown scope", mutate: func(command *SaveCommand) { command.Scope = "org" }, wantErr: ErrInvalidScope},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)
			command := saveCommand(user(uuid.New()), uuid.New(), model.SavedTemplatePersonal)
			tt.mutate(&command)

			// when
			got, err := svc.Save(t.Context(), command)

			// then
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, got)
		})
	}
}

func TestSave_AnUnreachableBoardPassesTheLiveErrorUp(t *testing.T) {
	// given
	svc, m := newTestService(t)
	principal := user(uuid.New())
	boardID := uuid.New()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: principal, BoardID: boardID}).Return(nil, live.ErrNoAccess)

	// when
	got, err := svc.Save(t.Context(), saveCommand(principal, boardID, model.SavedTemplatePersonal))

	// then
	assert.ErrorIs(t, err, live.ErrNoAccess)
	assert.Nil(t, got)
}

func TestSave_AVisitorWhoIsNotAMemberCannotSave(t *testing.T) {
	// given
	svc, m := newTestService(t)
	principal := user(uuid.New())
	access := boardAccess(principal, uuid.New(), nil, model.BoardCapabilityManage)
	access.OrgMember = false
	m.resolves(access)

	// when
	got, err := svc.Save(t.Context(), saveCommand(principal, access.Header.ID, model.SavedTemplatePersonal))

	// then
	assert.ErrorIs(t, err, ErrNotAllowed)
	assert.Nil(t, got)
}

func TestSave_AParticipantWithoutTheManageCapabilityCannotSave(t *testing.T) {
	// given
	svc, m := newTestService(t)
	principal := user(uuid.New())
	access := boardAccess(principal, uuid.New(), nil)
	access.Role = model.BoardRoleParticipant
	m.resolves(access)

	// when
	got, err := svc.Save(t.Context(), saveCommand(principal, access.Header.ID, model.SavedTemplatePersonal))

	// then
	assert.ErrorIs(t, err, ErrNotAllowed)
	assert.Nil(t, got)
}

func TestSave_ATeamTemplateNeedsATeamBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	principal := user(uuid.New())
	access := boardAccess(principal, uuid.New(), nil, model.BoardCapabilityManage)
	m.resolves(access)

	// when
	got, err := svc.Save(t.Context(), saveCommand(principal, access.Header.ID, model.SavedTemplateTeam))

	// then
	assert.ErrorIs(t, err, ErrNotTeamBoard)
	assert.Nil(t, got)
}

func TestSave_ATeamTemplateNeedsThePermissionToCreateTeamBoards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	principal := user(uuid.New())
	teamID := uuid.New()
	access := boardAccess(principal, uuid.New(), new(teamID), model.BoardCapabilityManage)
	m.resolves(access)
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: teamID, UserID: principal.ID, Permission: orgauthz.PermTeamCreateBoard}).Return(orgauthz.Decision{Member: true}, nil)

	// when
	got, err := svc.Save(t.Context(), saveCommand(principal, access.Header.ID, model.SavedTemplateTeam))

	// then
	assert.ErrorIs(t, err, ErrNotAllowed)
	assert.Nil(t, got)
}

func TestSave_ABoardWithoutColumnsCannotBeSaved(t *testing.T) {
	// given
	svc, m := newTestService(t)
	principal := user(uuid.New())
	access := boardAccess(principal, uuid.New(), nil, model.BoardCapabilityManage)
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return([]model.BoardColumn{}, nil)

	// when
	got, err := svc.Save(t.Context(), saveCommand(principal, access.Header.ID, model.SavedTemplatePersonal))

	// then the strict mocks prove the plan was never read
	assert.ErrorIs(t, err, ErrNoColumns)
	assert.Nil(t, got)
}

func TestSave_AFourthTemplateOnFreeNeedsAnUpgrade(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	access := boardAccess(user(userID), orgID, nil, model.BoardCapabilityManage)
	denied := &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitSavedTemplates, Allowed: 3, Current: 3}
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan(), nil)
	m.savedTemplates.EXPECT().Count(mock.Anything, orgID).Return(3, nil)
	m.entitlements.EXPECT().CheckWithin(mock.Anything, spec.LimitCheck{OrgID: orgID, Limit: model.LimitSavedTemplates, Current: 3}).Return(denied)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplatePersonal))

	// then the strict mocks prove nothing was written
	assert.Nil(t, got)
	assert.ErrorIs(t, err, entitlements.ErrUpgradeRequired)
	found, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok)
	assert.Equal(t, denied, found)
}

func TestSave_TheConditionalInsertRefusesATemplateThatRacedPastThePlan(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	access := boardAccess(user(userID), orgID, nil, model.BoardCapabilityManage)
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.withinTheFreePlan(orgID, 2)
	m.savedTemplates.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errQuotaUsed)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplatePersonal))

	// then
	assert.Nil(t, got)
	found, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok)
	assert.Equal(t, &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitSavedTemplates, Allowed: 3, Current: 3}, found)
}

func TestSave_AMembershipThatWentAwayMidSaveIsNoAccess(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	access := boardAccess(user(userID), orgID, nil, model.BoardCapabilityManage)
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.withinTheFreePlan(orgID, 0)
	m.savedTemplates.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errRowMissing)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplatePersonal))

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, live.ErrNoAccess)
}

func TestSave_ARefusedRowIsAnInternalError(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	access := boardAccess(user(userID), orgID, nil, model.BoardCapabilityManage)
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.withinTheFreePlan(orgID, 0)
	m.savedTemplates.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errRowConflicted)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplatePersonal))

	// then no sentinel claims it, so the controller answers 500
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errRowConflicted)
	assert.NotErrorIs(t, err, ErrNotAllowed)
}

func TestSave_APlanThatDoesNotNameTheLimitIsAnError(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	access := boardAccess(user(userID), orgID, nil, model.BoardCapabilityManage)
	m.resolves(access)
	m.columns.EXPECT().List(mock.Anything, access.Header.ID).Return(boardColumns(access.Header.ID), nil)
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(&entitlements.Plan{Code: "team", Limits: map[model.Limit]int{}}, nil)

	// when
	got, err := svc.Save(t.Context(), saveCommand(user(userID), access.Header.ID, model.SavedTemplatePersonal))

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, entitlements.ErrUnknownLimit)
}

func TestList_ShowsOwnPersonalAndViewableTeamTemplatesWithWhoMayDelete(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID, colleagueID := uuid.New(), uuid.New(), uuid.New()
	platform, design, secret := team(orgID, "Platform"), team(orgID, "Design"), team(orgID, "Secret")
	m.member(orgID, userID, model.OrgRoleMember, []model.Team{platform, design, secret}, []model.MemberTeam{joinedAs(platform, model.TeamRoleMember), joinedAs(design, model.TeamRoleLead)})
	mine := personal(orgID, userID)
	colleagues := forTeam(orgID, platform, colleagueID)
	ownTeam := forTeam(orgID, platform, userID)
	led := forTeam(orgID, design, colleagueID)
	hidden := forTeam(orgID, secret, colleagueID)
	m.savedTemplates.EXPECT().List(mock.Anything, spec.SavedTemplateViewer{OrgID: orgID, UserID: userID}).Return([]model.SavedTemplate{mine, colleagues, ownTeam, led, hidden}, nil)

	// when
	got, err := svc.List(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then a team template outside the viewer's teams is dropped, and only its creator or a lead may delete one
	require.NoError(t, err)
	assert.Equal(t, []dto.SavedTemplateResponse{
		dto.NewSavedTemplateResponse(mine, true),
		dto.NewSavedTemplateResponse(colleagues, false),
		dto.NewSavedTemplateResponse(ownTeam, true),
		dto.NewSavedTemplateResponse(led, true),
	}, got.Templates)
}

func TestList_AnAdminSeesEveryTeamTemplateButNotAnotherMembersPersonalOne(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, adminID, colleagueID := uuid.New(), uuid.New(), uuid.New()
	platform := team(orgID, "Platform")
	m.member(orgID, adminID, model.OrgRoleAdmin, []model.Team{platform}, []model.MemberTeam{})
	theirs := personal(orgID, colleagueID)
	shared := forTeam(orgID, platform, colleagueID)
	m.savedTemplates.EXPECT().List(mock.Anything, spec.SavedTemplateViewer{OrgID: orgID, UserID: adminID}).Return([]model.SavedTemplate{theirs, shared}, nil)

	// when
	got, err := svc.List(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: adminID})

	// then
	require.NoError(t, err)
	assert.Equal(t, []dto.SavedTemplateResponse{dto.NewSavedTemplateResponse(shared, true)}, got.Templates)
}

func TestList_ANonMemberIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme Retros"}, nil)
	m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}).Return(nil, errRowMissing)

	// when
	got, err := svc.List(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	assert.ErrorIs(t, err, ErrOrganisationNotFound)
	assert.Nil(t, got)
}

func TestList_ADeletedOrganisationIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID := uuid.New()
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme Retros", DeletedAt: new(testCreated)}, nil)

	// when
	got, err := svc.List(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: uuid.New()})

	// then
	assert.ErrorIs(t, err, ErrOrganisationNotFound)
	assert.Nil(t, got)
}

func TestList_AFailedReadIsAnError(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	m.member(orgID, userID, model.OrgRoleMember, []model.Team{}, []model.MemberTeam{})
	m.savedTemplates.EXPECT().List(mock.Anything, mock.Anything).Return(nil, errDatabaseDown)

	// when
	got, err := svc.List(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	assert.ErrorIs(t, err, errDatabaseDown)
	assert.Nil(t, got)
}

func TestDelete_TheOwnerDeletesAPersonalTemplate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	m.member(orgID, userID, model.OrgRoleMember, []model.Team{}, []model.MemberTeam{})
	mine := personal(orgID, userID)
	key := spec.SavedTemplateKey{OrgID: orgID, ID: mine.ID}
	m.savedTemplates.EXPECT().Get(mock.Anything, key).Return(&mine, nil)
	m.savedTemplates.EXPECT().Delete(mock.Anything, key).Return(&mine, nil)

	// when
	err := svc.Delete(t.Context(), spec.SavedTemplateRemoval{OrgID: orgID, ID: mine.ID, UserID: userID})

	// then
	require.NoError(t, err)
}

func TestDelete_ATeamLeadDeletesAColleaguesTeamTemplate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, leadID := uuid.New(), uuid.New()
	platform := team(orgID, "Platform")
	m.member(orgID, leadID, model.OrgRoleMember, []model.Team{platform}, []model.MemberTeam{joinedAs(platform, model.TeamRoleLead)})
	shared := forTeam(orgID, platform, uuid.New())
	key := spec.SavedTemplateKey{OrgID: orgID, ID: shared.ID}
	m.savedTemplates.EXPECT().Get(mock.Anything, key).Return(&shared, nil)
	m.savedTemplates.EXPECT().Delete(mock.Anything, key).Return(&shared, nil)

	// when
	err := svc.Delete(t.Context(), spec.SavedTemplateRemoval{OrgID: orgID, ID: shared.ID, UserID: leadID})

	// then
	require.NoError(t, err)
}

func TestDelete_ATeamMemberCannotDeleteAColleaguesTeamTemplate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	platform := team(orgID, "Platform")
	m.member(orgID, userID, model.OrgRoleMember, []model.Team{platform}, []model.MemberTeam{joinedAs(platform, model.TeamRoleMember)})
	shared := forTeam(orgID, platform, uuid.New())
	m.savedTemplates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: shared.ID}).Return(&shared, nil)

	// when
	err := svc.Delete(t.Context(), spec.SavedTemplateRemoval{OrgID: orgID, ID: shared.ID, UserID: userID})

	// then the strict mocks prove nothing was deleted
	assert.ErrorIs(t, err, ErrDeleteNotAllowed)
}

func TestDelete_AnInvisibleTemplateIsNotFound(t *testing.T) {
	tests := []struct {
		name       string
		teamScoped bool
	}{
		{name: "another member's personal template"},
		{name: "a team the viewer cannot see", teamScoped: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			orgID, userID := uuid.New(), uuid.New()
			secret := team(orgID, "Secret")
			m.member(orgID, userID, model.OrgRoleMember, []model.Team{secret}, []model.MemberTeam{})
			template := personal(orgID, uuid.New())
			if tt.teamScoped {
				template = forTeam(orgID, secret, uuid.New())
			}
			m.savedTemplates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: template.ID}).Return(&template, nil)

			// when
			err := svc.Delete(t.Context(), spec.SavedTemplateRemoval{OrgID: orgID, ID: template.ID, UserID: userID})

			// then its existence never leaks
			assert.ErrorIs(t, err, ErrTemplateNotFound)
			assert.NotErrorIs(t, err, ErrDeleteNotAllowed)
		})
	}
}

func TestDelete_AnUnknownTemplateIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID, templateID := uuid.New(), uuid.New(), uuid.New()
	m.member(orgID, userID, model.OrgRoleMember, []model.Team{}, []model.MemberTeam{})
	m.savedTemplates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: templateID}).Return(nil, errRowMissing)

	// when
	err := svc.Delete(t.Context(), spec.SavedTemplateRemoval{OrgID: orgID, ID: templateID, UserID: userID})

	// then
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestDelete_ATemplateDeletedConcurrentlyIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	orgID, userID := uuid.New(), uuid.New()
	m.member(orgID, userID, model.OrgRoleMember, []model.Team{}, []model.MemberTeam{})
	mine := personal(orgID, userID)
	key := spec.SavedTemplateKey{OrgID: orgID, ID: mine.ID}
	m.savedTemplates.EXPECT().Get(mock.Anything, key).Return(&mine, nil)
	m.savedTemplates.EXPECT().Delete(mock.Anything, key).Return(nil, errRowMissing)

	// when
	err := svc.Delete(t.Context(), spec.SavedTemplateRemoval{OrgID: orgID, ID: mine.ID, UserID: userID})

	// then
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}
