package board

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
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
		boards        *repository.MockBoardRepository
		columns       *repository.MockBoardColumnRepository
		cards         *repository.MockCardRepository
		votes         *repository.MockCardVoteRepository
		comments      *repository.MockCardCommentRepository
		teams         *repository.MockTeamRepository
		organisations *repository.MockOrganisationRepository
		live          *live.MockService
		orgAuthz      *orgauthz.MockService
		auditLog      *repository.MockAuditLogRepository
		templates     *repository.MockSavedTemplateRepository
		events        *recordedEvents

		acquired int
		released int
	}

	recordedEvents struct {
		published  []publishedEvent
		headers    []model.BoardHeader
		tallies    map[uuid.UUID]projection.HiddenTally
		me         []dto.BoardMeFrame
		everyoneMe []everyoneMe
		teams      []teamAnnouncement
	}

	teamAnnouncement struct {
		teamID  uuid.UUID
		exclude []uuid.UUID
	}

	everyoneMe struct {
		board   model.Board
		votes   []model.CardVote
		version int64
	}

	publishedEvent struct {
		access model.BoardAccess
		event  projection.Event
	}

	accessOption func(*model.BoardAccess)
)

const (
	secret = "SECRET-TEXT-9f2c"
)

var (
	orgID    = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000001")
	teamID   = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000002")
	boardID  = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000003")
	userID   = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000004")
	myID     = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000005")
	theirID  = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000006")
	opID     = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000007")
	columnID = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000008")
	cardID   = uuid.MustParse("1a2b3c4d-0000-4000-8000-000000000009")
	otherID  = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000a")
	v7ID     = uuid.MustParse("01920000-0000-7000-8000-000000000001")
	stamp    = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
	me     = model.Principal{Kind: model.PrincipalUser, ID: userID}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		boards:        repository.NewMockBoardRepository(t),
		columns:       repository.NewMockBoardColumnRepository(t),
		cards:         repository.NewMockCardRepository(t),
		votes:         repository.NewMockCardVoteRepository(t),
		comments:      repository.NewMockCardCommentRepository(t),
		teams:         repository.NewMockTeamRepository(t),
		organisations: repository.NewMockOrganisationRepository(t),
		live:          live.NewMockService(t),
		orgAuthz:      orgauthz.NewMockService(t),
		auditLog:      repository.NewMockAuditLogRepository(t),
		templates:     repository.NewMockSavedTemplateRepository(t),
		events:        &recordedEvents{tallies: map[uuid.UUID]projection.HiddenTally{}},
	}

	t.Cleanup(func() {
		assert.Equal(t, m.acquired, m.released, "every gate taken is released")
	})

	svc := NewService(Deps{
		Boards:         m.boards,
		Columns:        m.columns,
		Cards:          m.cards,
		Votes:          m.votes,
		Comments:       m.comments,
		Teams:          m.teams,
		Organisations:  m.organisations,
		Live:           m.live,
		Events:         m.events,
		OrgAuthz:       m.orgAuthz,
		AuditLog:       m.auditLog,
		SavedTemplates: m.templates,
	}).(*service)

	return svc, m
}

func (r *recordedEvents) Publish(access model.BoardAccess, events ...projection.Event) {
	r.headers = append(r.headers, access.Header)

	for _, event := range events {
		r.published = append(r.published, publishedEvent{access: access, event: event})
	}
}

func (r *recordedEvents) PublishMe(_ model.BoardAccess, me dto.BoardMeFrame) {
	r.me = append(r.me, me)
}

func (r *recordedEvents) PublishMeToEveryone(access model.BoardAccess, votes []model.CardVote, version int64) {
	r.everyoneMe = append(r.everyoneMe, everyoneMe{board: access.Header.Board, votes: votes, version: version})
}

func (r *recordedEvents) PublishedTally(boardID uuid.UUID) (projection.HiddenTally, bool) {
	tally, ok := r.tallies[boardID]

	return tally, ok
}

func (r *recordedEvents) AnnounceTeamActionItems(teamID uuid.UUID, exclude []uuid.UUID) {
	r.teams = append(r.teams, teamAnnouncement{teamID: teamID, exclude: exclude})
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

func hiddenCards(access *model.BoardAccess) {
	access.Header.CardsHidden = true
}

func anonymous(access *model.BoardAccess) {
	access.Header.Anonymity = model.AnonymityAnonymous
}

func notJoined(access *model.BoardAccess) {
	access.ParticipantID = nil
}

func movesLocked(access *model.BoardAccess) {
	access.Header.MovesLocked = true
}

func atStage(stage model.Stage) accessOption {
	return func(access *model.BoardAccess) {
		access.Header.Stage = stage
	}
}

func baseCommand() Command {
	return Command{Principal: me, BoardID: boardID, OpID: opID}
}

func target() spec.BoardWrite {
	return spec.BoardWrite{OrgID: orgID, BoardID: boardID, Version: 7, OpID: opID}
}

func (m *testMocks) gate() {
	m.acquired++
	m.live.EXPECT().Acquire(mock.Anything, boardID).Return(func() {
		m.released++
	}, nil).Once()
}

func (m *testMocks) resolved(access *model.BoardAccess) {
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil).Once()
}

func (m *testMocks) entered(access *model.BoardAccess) {
	m.resolved(access)
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errNotFound).Once()
}

func (m *testMocks) applied(access *model.BoardAccess, version int64) {
	m.resolved(access)
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(&model.BoardOp{BoardID: boardID, OpID: opID, Version: version}, nil).Once()
}

func (m *testMocks) team(team *model.Team) {
	m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(team, nil).Once()
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme"}, nil).Once()
}

func (m *testMocks) creatableTeam(team *model.Team) {
	m.team(team)
	m.mayCreate(orgauthz.Decision{Member: true, Allowed: true})
}

func (m *testMocks) mayCreate(decision orgauthz.Decision) {
	check := spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamCreateBoard}
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, check).Return(decision, nil).Once()
}

func (m *testMocks) mayCreateInOrganisation(decision orgauthz.Decision) {
	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme"}, nil).Once()
	check := spec.OrgPermissionCheck{OrgID: orgID, UserID: userID, Permission: orgauthz.PermOrgCreateBoard}
	m.orgAuthz.EXPECT().CanInOrg(mock.Anything, check).Return(decision, nil).Once()
}

func liveTeam() *model.Team {
	return &model.Team{ID: teamID, OrgID: orgID, Name: "Platform", CreatedAt: stamp}
}

func listedBoard(board model.Board, anonymity model.Anonymity) model.BoardHeader {
	board.Anonymity = anonymity

	return model.BoardHeader{Board: board}
}

func versionResponse(version int64) *dto.BoardCommandResponse {
	return &dto.BoardCommandResponse{Version: version}
}

func replayResponse(version int64, id *uuid.UUID) *dto.BoardCommandResponse {
	return &dto.BoardCommandResponse{Version: version, Replayed: true, ID: id}
}

func TestCommand_RefusesACommandWithoutAnOpBeforeTakingTheGate(t *testing.T) {
	// given
	svc, _ := newTestService(t)
	command := CardCommand{Principal: me, BoardID: boardID, CardID: cardID}

	// when
	got, err := svc.DeleteCard(t.Context(), command)

	// then the strict mocks prove the gate was never asked for
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrOpRequired)
}

func TestCommand_ABusyGateIsReturnedAsItIs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Acquire(mock.Anything, boardID).Return(nil, live.ErrBusy)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, live.ErrBusy)
}

func TestCommand_AFailedResolveReleasesTheGate(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(nil, fmt.Errorf("%w: removed", live.ErrRemoved))

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then the cleanup proves the gate was released
	assert.Nil(t, got)
	assert.ErrorIs(t, err, live.ErrRemoved)
}

func TestCommand_AFailedOpLookupIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.resolved(boardAccess(host))
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errDatabase)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestCommand_AnAppliedOpReplaysItsVersionWithoutWriting(t *testing.T) {
	// given a retried delete whose card is already gone
	svc, m := newTestService(t)
	m.applied(boardAccess(writer), 5)

	// when
	got, err := svc.DeleteCard(t.Context(), CardCommand{Command: baseCommand(), CardID: cardID})

	// then the strict mocks prove no card was read or written
	require.NoError(t, err)
	assert.Equal(t, replayResponse(5, nil), got)
}

func TestCommand_ADuplicateOpFromTheWriteReplaysTheRecordedVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(nil, fmt.Errorf("record op: %w", dao.ErrDuplicateOp))
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(&model.BoardOp{BoardID: boardID, OpID: opID, Version: 9}, nil).Once()

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(9, nil), got)
}

func TestCommand_ADuplicateOpThatIsNotRecordedIsAnError(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(nil, fmt.Errorf("record op: %w", dao.ErrDuplicateOp))
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errNotFound).Once()

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, dao.ErrDuplicateOp)
}

func TestCommand_ADuplicateOpWhoseLookupFailsIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(nil, fmt.Errorf("record op: %w", dao.ErrDuplicateOp))
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errDatabase).Once()

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestCommand_AVersionConflictIsReportedWithItsCause(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.columns.EXPECT().DeleteColumn(mock.Anything, spec.ColumnRemoval{BoardWrite: target(), ColumnID: columnID}).Return(nil, fmt.Errorf("bump: %w", dao.ErrVersionConflict))

	// when
	got, err := svc.DeleteColumn(t.Context(), ColumnCommand{Command: baseCommand(), ColumnID: columnID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, dao.ErrVersionConflict)
}

func TestGetSnapshot_ProjectsTheBoardForTheViewer(t *testing.T) {
	// given a board in the writing phase with one card of mine and one of someone else's
	svc, m := newTestService(t)
	access := boardAccess(host, hiddenCards)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(access, nil)
	state := &model.BoardState{
		Header:  access.Header,
		Columns: []model.BoardColumn{{ID: columnID, BoardID: boardID, Title: "Went well", Colour: model.ColourGreen, Position: "V"}},
		Cards: []model.Card{
			{ID: cardID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(myID), Body: "mine", Position: "V", CreatedAt: stamp},
			{ID: otherID, BoardID: boardID, ColumnID: columnID, AuthorParticipantID: new(theirID), Body: secret, Position: "W", CreatedAt: stamp},
		},
		Participants: []model.BoardParticipantProfile{
			{ID: myID, BoardID: boardID, UserID: new(userID), Role: host, AccountName: "Sam"},
			{ID: theirID, BoardID: boardID, DisplayName: "Ali", Role: writer},
		},
	}
	m.boards.EXPECT().GetSnapshot(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(state, nil)

	// when
	got, err := svc.GetSnapshot(t.Context(), spec.BoardSnapshotQuery{Principal: me, BoardID: boardID, KnownVersion: new(int64(6))})

	// then even the facilitator sees only their own card and a count for the other
	require.NoError(t, err)
	assert.Equal(t, boardID, got.BoardID)
	assert.Equal(t, int64(7), got.Version)
	require.Len(t, got.Cards, 1)
	assert.Equal(t, cardID, got.Cards[0].ID)
	assert.Equal(t, map[uuid.UUID]int{columnID: 1}, got.HiddenCounts)
	assert.Equal(t, &myID, got.Viewer.ParticipantID)
	assert.NotContains(t, fmt.Sprintf("%+v", *got), secret)
}

func TestGetSnapshot_AnswersNotModifiedForTheVersionTheClientHolds(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(writer), nil)

	// when
	got, err := svc.GetSnapshot(t.Context(), spec.BoardSnapshotQuery{Principal: me, BoardID: boardID, KnownVersion: new(int64(7))})

	// then the strict mocks prove no snapshot was read
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrNotModified)
}

func TestGetSnapshot_RefusesAViewerWithoutAccess(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(nil, fmt.Errorf("%w: no path in", live.ErrNoAccess))

	// when
	got, err := svc.GetSnapshot(t.Context(), spec.BoardSnapshotQuery{Principal: me, BoardID: boardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, live.ErrNoAccess)
}

func TestGetSnapshot_ABoardGoneSinceTheResolveIsNoAccess(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(writer), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(nil, errNotFound)

	// when
	got, err := svc.GetSnapshot(t.Context(), spec.BoardSnapshotQuery{Principal: me, BoardID: boardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, live.ErrNoAccess)
	assert.ErrorIs(t, err, dao.ErrNotFound)
}

func TestGetSnapshot_AFailedReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(writer), nil)
	m.boards.EXPECT().GetSnapshot(mock.Anything, spec.BoardKey{OrgID: orgID, BoardID: boardID}).Return(nil, errDatabase)

	// when
	got, err := svc.GetSnapshot(t.Context(), spec.BoardSnapshotQuery{Principal: me, BoardID: boardID})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func TestList_ListsTheTeamsLiveBoards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.team(liveTeam())
	archivedAt := stamp.Add(time.Hour)
	m.boards.EXPECT().ListForTeam(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return([]model.BoardHeader{
		listedBoard(model.Board{ID: boardID, OrgID: orgID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: stamp}, model.AnonymityNamed),
		listedBoard(model.Board{ID: otherID, OrgID: orgID, TeamID: new(teamID), Title: "Sprint 11", CreatedAt: stamp.Add(-time.Hour), ArchivedAt: &archivedAt}, model.AnonymityNamed),
	}, nil)

	// when
	got, err := svc.List(t.Context(), teamID)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardListResponse{Boards: []dto.BoardListItem{
		{ID: boardID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: "2026-10-04T12:00:00.000Z"},
		{ID: otherID, TeamID: new(teamID), Title: "Sprint 11", CreatedAt: "2026-10-04T11:00:00.000Z", ArchivedAt: "2026-10-04T13:00:00.000Z"},
	}}, got)
}

func TestList_MarksAnEndedRetro(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.team(liveTeam())
	closedAt := stamp.Add(time.Hour)
	m.boards.EXPECT().ListForTeam(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return([]model.BoardHeader{
		listedBoard(model.Board{ID: boardID, OrgID: orgID, TeamID: new(teamID), Title: "Sprint 10", CreatedAt: stamp, ClosedAt: &closedAt}, model.AnonymityAnonymous),
	}, nil)

	// when
	got, err := svc.List(t.Context(), teamID)

	// then the list says it ended without saying when, so an anonymous board's timing stays private
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardListResponse{Boards: []dto.BoardListItem{{ID: boardID, TeamID: new(teamID), Title: "Sprint 10", Ended: true}}}, got)
}

func TestList_LeavesTheCreationTimeOffAnAnonymousBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.team(liveTeam())
	m.boards.EXPECT().ListForTeam(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return([]model.BoardHeader{
		listedBoard(model.Board{ID: boardID, OrgID: orgID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: stamp}, model.AnonymityAnonymous),
	}, nil)

	// when
	got, err := svc.List(t.Context(), teamID)

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardListResponse{Boards: []dto.BoardListItem{{ID: boardID, TeamID: new(teamID), Title: "Sprint 12"}}}, got)
}

func TestList_AnEmptyTeamListsNoBoards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.team(liveTeam())
	m.boards.EXPECT().ListForTeam(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return([]model.BoardHeader{}, nil)

	// when
	got, err := svc.List(t.Context(), teamID)

	// then
	require.NoError(t, err)
	assert.NotNil(t, got.Boards)
	assert.Empty(t, got.Boards)
}

func TestList_AFailedListIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.team(liveTeam())
	m.boards.EXPECT().ListForTeam(mock.Anything, spec.TeamKey{OrgID: orgID, TeamID: teamID}).Return(nil, errDatabase)

	// when
	got, err := svc.List(t.Context(), teamID)

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func (m *testMocks) organisationBoards(role model.OrgRole, joined []model.MemberTeam, boards []model.BoardHeader, pinned ...uuid.UUID) {
	viewer := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}
	archivedAt := stamp.Add(-time.Hour)

	m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, Name: "Acme"}, nil).Once()
	m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: role}, nil).Once()
	m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return([]model.Team{
		{ID: teamID, OrgID: orgID, Name: "Platform", CreatedAt: stamp},
		{ID: otherID, OrgID: orgID, Name: "Payments", CreatedAt: stamp, ArchivedAt: &archivedAt},
	}, nil).Once()
	m.teams.EXPECT().ListForMember(mock.Anything, viewer).Return(joined, nil).Once()
	m.boards.EXPECT().ListForOrganisation(mock.Anything, orgID).Return(boards, nil).Once()
	m.boards.EXPECT().ListPinned(mock.Anything, viewer).Return(append([]uuid.UUID{}, pinned...), nil).Once()
}

func organisationBoardHeaders() []model.BoardHeader {
	named := listedBoard(model.Board{ID: boardID, OrgID: orgID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: stamp}, model.AnonymityNamed)
	named.Board.TemplateKey = "mad-sad-glad"

	anonymousBoard := listedBoard(model.Board{ID: cardID, OrgID: orgID, TeamID: new(otherID), Title: "Payments retro", CreatedAt: stamp.Add(-time.Hour)}, model.AnonymityAnonymous)
	anonymousBoard.Board.TemplateKey = "blank"

	return []model.BoardHeader{named, anonymousBoard}
}

func TestListForOrganisation_AnAdminSeesTheBoardsOfEveryTeamArchivedOnesIncluded(t *testing.T) {
	// given an admin on no team, and an anonymous board on an archived team
	svc, m := newTestService(t)
	m.organisationBoards(model.OrgRoleAdmin, []model.MemberTeam{}, organisationBoardHeaders())

	// when
	got, err := svc.ListForOrganisation(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then the anonymous board keeps its creation time to itself
	require.NoError(t, err)
	assert.Equal(t, &dto.OrgBoardListResponse{Boards: []dto.OrgBoardListItem{
		{ID: boardID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: "2026-10-04T12:00:00.000Z", TeamName: "Platform", TemplateKey: "mad-sad-glad"},
		{ID: cardID, TeamID: new(otherID), Title: "Payments retro", TeamName: "Payments", TemplateKey: "blank"},
	}}, got)
}

func TestListForOrganisation_AMemberSeesOnlyTheBoardsOfTheirOwnTeams(t *testing.T) {
	// given a plain member on the Platform team only
	svc, m := newTestService(t)
	joined := []model.MemberTeam{{ID: teamID, OrgID: orgID, Name: "Platform", Role: model.TeamRoleMember}}
	m.organisationBoards(model.OrgRoleMember, joined, organisationBoardHeaders())

	// when
	got, err := svc.ListForOrganisation(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.OrgBoardListResponse{Boards: []dto.OrgBoardListItem{
		{ID: boardID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: "2026-10-04T12:00:00.000Z", TeamName: "Platform", TemplateKey: "mad-sad-glad"},
	}}, got)
}

func TestListForOrganisation_MarksTheBoardsTheViewerPinned(t *testing.T) {
	// given an admin who pinned the anonymous board
	svc, m := newTestService(t)
	m.organisationBoards(model.OrgRoleAdmin, []model.MemberTeam{}, organisationBoardHeaders(), cardID)

	// when
	got, err := svc.ListForOrganisation(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then only the pinned board says so
	require.NoError(t, err)
	assert.Equal(t, &dto.OrgBoardListResponse{Boards: []dto.OrgBoardListItem{
		{ID: boardID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: "2026-10-04T12:00:00.000Z", TeamName: "Platform", TemplateKey: "mad-sad-glad"},
		{ID: cardID, TeamID: new(otherID), Title: "Payments retro", TeamName: "Payments", TemplateKey: "blank", Pinned: true},
	}}, got)
}

func TestListForOrganisation_AMemberOnNoTeamSeesNoBoards(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.organisationBoards(model.OrgRoleMember, []model.MemberTeam{}, organisationBoardHeaders())

	// when
	got, err := svc.ListForOrganisation(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then
	require.NoError(t, err)
	assert.NotNil(t, got.Boards)
	assert.Empty(t, got.Boards)
}

func TestListForOrganisation_RefusesAnOrganisationThatIsGone(t *testing.T) {
	deletedAt := stamp

	cases := []struct {
		name   string
		expect func(m *testMocks)
	}{
		{
			name: "unknown",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errNotFound)
			},
		},
		{
			name: "deleted",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, DeletedAt: &deletedAt}, nil)
			},
		},
		{
			name: "not a member",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}).Return(nil, errNotFound)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.expect(m)

			// when
			got, err := svc.ListForOrganisation(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

			// then no team or board is read
			assert.Nil(t, got)
			assert.ErrorIs(t, err, ErrOrganisationNotFound)
		})
	}
}

func TestListForOrganisation_AFailedReadIsReported(t *testing.T) {
	viewer := spec.OrganisationMemberKey{OrgID: orgID, UserID: userID}

	cases := []struct {
		name   string
		expect func(m *testMocks)
	}{
		{
			name: "organisation",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errDatabase)
			},
		},
		{
			name: "membership",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(nil, errDatabase)
			},
		},
		{
			name: "teams",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: model.OrgRoleOwner}, nil)
				m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return(nil, errDatabase)
			},
		},
		{
			name: "memberships",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: model.OrgRoleOwner}, nil)
				m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return([]model.Team{}, nil)
				m.teams.EXPECT().ListForMember(mock.Anything, viewer).Return(nil, errDatabase)
			},
		},
		{
			name: "boards",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: model.OrgRoleOwner}, nil)
				m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return([]model.Team{}, nil)
				m.teams.EXPECT().ListForMember(mock.Anything, viewer).Return([]model.MemberTeam{}, nil)
				m.boards.EXPECT().ListForOrganisation(mock.Anything, orgID).Return(nil, errDatabase)
			},
		},
		{
			name: "pins",
			expect: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.organisations.EXPECT().GetMember(mock.Anything, viewer).Return(&model.OrganisationMember{OrgID: orgID, UserID: userID, Role: model.OrgRoleOwner}, nil)
				m.teams.EXPECT().ListForOrganisation(mock.Anything, orgID).Return([]model.Team{}, nil)
				m.teams.EXPECT().ListForMember(mock.Anything, viewer).Return([]model.MemberTeam{}, nil)
				m.boards.EXPECT().ListForOrganisation(mock.Anything, orgID).Return([]model.BoardHeader{}, nil)
				m.boards.EXPECT().ListPinned(mock.Anything, viewer).Return(nil, errDatabase)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.expect(m)

			// when
			got, err := svc.ListForOrganisation(t.Context(), viewer)

			// then
			assert.Nil(t, got)
			assert.ErrorIs(t, err, errDatabase)
		})
	}
}

func TestLiveTeam_RefusesATeamThatIsGone(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks)
		want  error
	}{
		{
			name: "an unknown team",
			given: func(m *testMocks) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(nil, errNotFound)
			},
			want: ErrTeamNotFound,
		},
		{
			name: "a team that cannot be read",
			given: func(m *testMocks) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(nil, errDatabase)
			},
			want: errDatabase,
		},
		{
			name: "an organisation that is gone",
			given: func(m *testMocks) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(liveTeam(), nil)
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errNotFound)
			},
			want: ErrTeamNotFound,
		},
		{
			name: "an organisation that cannot be read",
			given: func(m *testMocks) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(liveTeam(), nil)
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errDatabase)
			},
			want: errDatabase,
		},
		{
			name: "a soft-deleted organisation",
			given: func(m *testMocks) {
				m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(liveTeam(), nil)
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, DeletedAt: new(stamp)}, nil)
			},
			want: ErrTeamNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.given(m)

			// when
			got, err := svc.List(t.Context(), teamID)

			// then
			assert.Nil(t, got)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func createCommand() CreateCommand {
	return CreateCommand{Principal: me, TeamID: new(teamID), Title: "  Sprint 12  ", TemplateKey: "mad-sad-glad", Context: "  Release week  "}
}

func createdState(s spec.NewBoard) *model.BoardState {
	columns := make([]model.BoardColumn, 0, len(s.Columns))
	for _, seed := range s.Columns {
		columns = append(columns, model.BoardColumn{ID: seed.ID, BoardID: boardID, Title: seed.Title, Description: seed.Description, Colour: seed.Colour, Position: seed.Position, CreatedAt: stamp})
	}

	return &model.BoardState{
		Header: model.BoardHeader{Board: model.Board{
			ID:          boardID,
			OrgID:       s.OrgID,
			TeamID:      s.TeamID,
			Title:       s.Title,
			ShareSlug:   s.ShareSlug,
			CreatedBy:   new(s.CreatedBy),
			CreatedAt:   stamp,
			Context:     s.Context,
			TemplateKey: s.TemplateKey,
			Anonymity:   s.Anonymity,
			CardsHidden: s.CardsHidden,
		}},
		Columns: columns,
		Cards:   []model.Card{},
		Participants: []model.BoardParticipantProfile{
			{ID: theirID, BoardID: boardID, UserID: new(otherID), Role: writer},
			{ID: myID, BoardID: boardID, UserID: new(userID), Role: host, AccountName: "Sam"},
		},
	}
}

func TestCreate_CopiesTheTemplateAndStartsInTheWritingPhase(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.creatableTeam(liveTeam())
	var written spec.NewBoard
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		written = s
		return createdState(s), nil
	})

	// when
	got, err := svc.Create(t.Context(), createCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, orgID, written.OrgID)
	assert.Equal(t, new(teamID), written.TeamID)
	assert.Equal(t, "Sprint 12", written.Title)
	assert.Equal(t, "Release week", written.Context)
	assert.Equal(t, userID, written.CreatedBy)
	assert.Equal(t, "mad-sad-glad", written.TemplateKey)
	assert.Equal(t, model.AnonymityNamed, written.Anonymity)
	assert.True(t, written.CardsHidden)
	assert.GreaterOrEqual(t, len(written.ShareSlug), 26)

	require.Len(t, written.Columns, 3)
	titles := []string{}
	for i, column := range written.Columns {
		titles = append(titles, column.Title)
		assert.Equal(t, uuid.Version(4), column.ID.Version())
		if i > 0 {
			assert.Less(t, written.Columns[i-1].Position, column.Position)
		}
	}
	assert.Equal(t, []string{"Mad", "Sad", "Glad"}, titles)
	assert.Equal(t, model.ColourRed, written.Columns[0].Colour)
	assert.Equal(t, "What frustrated you?", written.Columns[0].Description)

	assert.Equal(t, boardID, got.BoardID)
	assert.Len(t, got.Columns, 3)
	assert.True(t, got.Board.CardsHidden)
	assert.Equal(t, &myID, got.Viewer.ParticipantID)
	assert.Equal(t, host, got.Viewer.Role)
	assert.Equal(t, policy.Capabilities(host, true), got.Viewer.Capabilities)
}

func TestCreate_TheCreatorMayStartWithCardsShownOnAnAnonymousBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.creatableTeam(liveTeam())
	command := createCommand()
	command.CardsShown = true
	command.Anonymity = model.AnonymityAnonymous
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.MatchedBy(func(s spec.NewBoard) bool {
		return !s.CardsHidden && s.Anonymity == model.AnonymityAnonymous
	})).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		return createdState(s), nil
	})

	// when
	got, err := svc.Create(t.Context(), command)

	// then
	require.NoError(t, err)
	assert.False(t, got.Board.CardsHidden)
	assert.Equal(t, model.AnonymityAnonymous, got.Board.Anonymity)
}

func TestCreate_EachBoardGetsItsOwnShareSlug(t *testing.T) {
	// given
	svc, m := newTestService(t)
	slugs := map[string]struct{}{}
	for range 2 {
		m.creatableTeam(liveTeam())
	}
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		slugs[s.ShareSlug] = struct{}{}
		return createdState(s), nil
	}).Times(2)

	// when
	for range 2 {
		_, err := svc.Create(t.Context(), createCommand())
		require.NoError(t, err)
	}

	// then
	assert.Len(t, slugs, 2)
}

func TestCreate_RefusesBeforeReadingTheTeam(t *testing.T) {
	cases := []struct {
		name   string
		change func(*CreateCommand)
		want   error
	}{
		{name: "a guest", change: func(c *CreateCommand) { c.Principal = model.Principal{Kind: "guest", ID: userID} }, want: policy.ErrNotAllowed},
		{name: "a blank title", change: func(c *CreateCommand) { c.Title = "   " }, want: ErrTitleRequired},
		{name: "a title over 200 characters", change: func(c *CreateCommand) { c.Title = strings.Repeat("é", 201) }, want: ErrTitleTooLong},
		{name: "a description over 500 characters", change: func(c *CreateCommand) { c.Context = strings.Repeat("a", 501) }, want: ErrContextTooLong},
		{name: "an unknown anonymity", change: func(c *CreateCommand) { c.Anonymity = "secret" }, want: ErrInvalidAnonymity},
		{name: "an unknown template", change: func(c *CreateCommand) { c.TemplateKey = "no-such-template" }, want: ErrUnknownTemplate},
		{name: "no template", change: func(c *CreateCommand) { c.TemplateKey = "" }, want: ErrUnknownTemplate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)
			command := createCommand()
			tc.change(&command)

			// when
			got, err := svc.Create(t.Context(), command)

			// then
			assert.Nil(t, got)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCreate_RefusesAnArchivedTeam(t *testing.T) {
	// given
	svc, m := newTestService(t)
	archived := liveTeam()
	archived.ArchivedAt = new(stamp)
	m.creatableTeam(archived)

	// when
	got, err := svc.Create(t.Context(), createCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrTeamArchived)
}

func TestCreate_RefusesATeamThatIsGone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.teams.EXPECT().GetHeader(mock.Anything, teamID).Return(nil, errNotFound)

	// when
	got, err := svc.Create(t.Context(), createCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestCreate_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.creatableTeam(liveTeam())
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).Return(nil, errDatabase)

	// when
	got, err := svc.Create(t.Context(), createCommand())

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}

func savedTemplateFor(ownerID *uuid.UUID, teamID *uuid.UUID) *model.SavedTemplate {
	return &model.SavedTemplate{
		ID:        otherID,
		OrgID:     orgID,
		TeamID:    teamID,
		OwnerID:   ownerID,
		CreatedBy: new(theirID),
		Name:      "Our retro",
		Columns: []model.SavedTemplateColumn{
			{Title: "Wins", Description: "What went well?", Colour: model.ColourGreen},
			{Title: "Snags", Description: "What got in the way?", Colour: model.ColourRed},
		},
		CreatedAt: stamp,
	}
}

func TestCreate_CopiesAPersonalSavedTemplateAndStoresNoTemplateKey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.creatableTeam(liveTeam())
	m.templates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: otherID}).Return(savedTemplateFor(new(userID), nil), nil)
	var written spec.NewBoard
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		written = s
		return createdState(s), nil
	})
	command := createCommand()
	command.SavedTemplateID = new(otherID)

	// when
	got, err := svc.Create(t.Context(), command)

	// then the built-in key is ignored and template_key records built-in templates only
	require.NoError(t, err)
	assert.Empty(t, written.TemplateKey)
	require.Len(t, written.Columns, 2)
	assert.Equal(t, "Wins", written.Columns[0].Title)
	assert.Equal(t, "What went well?", written.Columns[0].Description)
	assert.Equal(t, model.ColourGreen, written.Columns[0].Colour)
	assert.Equal(t, "Snags", written.Columns[1].Title)
	assert.Equal(t, uuid.Version(4), written.Columns[1].ID.Version())
	assert.Less(t, written.Columns[0].Position, written.Columns[1].Position)
	assert.Len(t, got.Columns, 2)
}

func TestCreate_ATeamSavedTemplateNeedsTheCreatorToSeeTheTeam(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.mayCreateInOrganisation(orgauthz.Decision{Member: true, Allowed: true})
	m.templates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: otherID}).Return(savedTemplateFor(nil, new(teamID)), nil)
	m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamView}).Return(orgauthz.Decision{Member: true, Allowed: true}, nil)
	var written spec.NewBoard
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		written = s
		return createdState(s), nil
	})
	command := organisationCreateCommand()
	command.TemplateKey = ""
	command.SavedTemplateID = new(otherID)

	// when
	_, err := svc.Create(t.Context(), command)

	// then
	require.NoError(t, err)
	assert.Empty(t, written.TemplateKey)
	assert.Len(t, written.Columns, 2)
}

func TestCreate_AnUnusableSavedTemplateIsAnUnknownTemplate(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks)
	}{
		{
			name: "unknown or in another organisation",
			given: func(m *testMocks) {
				m.templates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: otherID}).Return(nil, errNotFound)
			},
		},
		{
			name: "another member's personal template",
			given: func(m *testMocks) {
				m.templates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: otherID}).Return(savedTemplateFor(new(theirID), nil), nil)
			},
		},
		{
			name: "a team template the creator cannot see",
			given: func(m *testMocks) {
				m.templates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: otherID}).Return(savedTemplateFor(nil, new(teamID)), nil)
				m.orgAuthz.EXPECT().CanInTeam(mock.Anything, spec.TeamPermissionCheck{TeamID: teamID, UserID: userID, Permission: orgauthz.PermTeamView}).Return(orgauthz.Decision{Member: true}, nil)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.mayCreateInOrganisation(orgauthz.Decision{Member: true, Allowed: true})
			tc.given(m)
			command := organisationCreateCommand()
			command.SavedTemplateID = new(otherID)

			// when
			got, err := svc.Create(t.Context(), command)

			// then its existence never leaks, and the strict mocks prove no board was written
			assert.Nil(t, got)
			assert.ErrorIs(t, err, ErrUnknownTemplate)
		})
	}
}

func TestCreate_AFailedSavedTemplateReadIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.mayCreateInOrganisation(orgauthz.Decision{Member: true, Allowed: true})
	m.templates.EXPECT().Get(mock.Anything, spec.SavedTemplateKey{OrgID: orgID, ID: otherID}).Return(nil, errDatabase)
	command := organisationCreateCommand()
	command.SavedTemplateID = new(otherID)

	// when
	got, err := svc.Create(t.Context(), command)

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
	assert.NotErrorIs(t, err, ErrUnknownTemplate)
}

func organisationCreateCommand() CreateCommand {
	command := createCommand()
	command.OrgID = orgID
	command.TeamID = nil

	return command
}

func TestCreate_AnOrganisationBoardBelongsToNoTeamAndItsCreatorHostsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.mayCreateInOrganisation(orgauthz.Decision{Member: true, Allowed: true})
	var written spec.NewBoard
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		written = s
		return createdState(s), nil
	})

	// when
	got, err := svc.Create(t.Context(), organisationCreateCommand())

	// then no team is read or written
	require.NoError(t, err)
	assert.Equal(t, orgID, written.OrgID)
	assert.Nil(t, written.TeamID)
	assert.Equal(t, userID, written.CreatedBy)
	assert.Nil(t, got.Board.TeamID)
	assert.Equal(t, &myID, got.Viewer.ParticipantID)
	assert.Equal(t, host, got.Viewer.Role)
	assert.Equal(t, policy.Capabilities(host, true), got.Viewer.Capabilities)
	m.teams.AssertNotCalled(t, "GetHeader", mock.Anything, mock.Anything)
}

func TestCreate_AnOrganisationBoardIsRefusedOutsideALiveOrganisation(t *testing.T) {
	cases := []struct {
		name  string
		given func(m *testMocks)
		want  error
	}{
		{
			name: "an unknown organisation",
			given: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errNotFound)
			},
			want: ErrOrganisationNotFound,
		},
		{
			name: "a soft-deleted organisation",
			given: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID, DeletedAt: new(stamp)}, nil)
			},
			want: ErrOrganisationNotFound,
		},
		{
			name: "someone who is not a member",
			given: func(m *testMocks) {
				m.mayCreateInOrganisation(orgauthz.Decision{})
			},
			want: ErrOrganisationNotFound,
		},
		{
			name: "a member the organisation does not let create boards",
			given: func(m *testMocks) {
				m.mayCreateInOrganisation(orgauthz.Decision{Member: true})
			},
			want: ErrCreateNotAllowed,
		},
		{
			name: "an organisation that cannot be read",
			given: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(nil, errDatabase)
			},
			want: errDatabase,
		},
		{
			name: "a decision that cannot be made",
			given: func(m *testMocks) {
				m.organisations.EXPECT().GetByID(mock.Anything, orgID).Return(&model.Organisation{ID: orgID}, nil)
				m.orgAuthz.EXPECT().CanInOrg(mock.Anything, mock.Anything).Return(orgauthz.Decision{}, errDatabase)
			},
			want: errDatabase,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			tc.given(m)

			// when
			got, err := svc.Create(t.Context(), organisationCreateCommand())

			// then nothing is written
			assert.Nil(t, got)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCreate_TheOrganisationRouteMayShareABoardWithOneOfItsTeams(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.creatableTeam(liveTeam())
	command := organisationCreateCommand()
	command.TeamID = new(teamID)
	var written spec.NewBoard
	m.boards.EXPECT().CreateBoard(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, s spec.NewBoard, _ ...*sql.Tx) (*model.BoardState, error) {
		written = s
		return createdState(s), nil
	})

	// when
	got, err := svc.Create(t.Context(), command)

	// then
	require.NoError(t, err)
	assert.Equal(t, orgID, written.OrgID)
	assert.Equal(t, new(teamID), written.TeamID)
	assert.Equal(t, new(teamID), got.Board.TeamID)
}

func TestCreate_RefusesATeamOfAnotherOrganisation(t *testing.T) {
	// given the path names another organisation than the team's
	svc, m := newTestService(t)
	m.team(liveTeam())
	command := organisationCreateCommand()
	command.OrgID = otherID
	command.TeamID = new(teamID)

	// when
	got, err := svc.Create(t.Context(), command)

	// then the team is not shown to exist, and no permission is asked
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrTeamNotFound)
	m.orgAuthz.AssertNotCalled(t, "CanInTeam", mock.Anything, mock.Anything)
}

func TestCreate_RefusesWhoeverMayNotCreateOnTheTeam(t *testing.T) {
	cases := []struct {
		name     string
		decision orgauthz.Decision
		want     error
	}{
		{name: "someone outside the organisation", decision: orgauthz.Decision{}, want: ErrTeamNotFound},
		{name: "an organisation member off the team", decision: orgauthz.Decision{Member: true}, want: ErrCreateNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.team(liveTeam())
			m.mayCreate(tc.decision)

			// when
			got, err := svc.Create(t.Context(), createCommand())

			// then
			assert.Nil(t, got)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestListForOrganisation_EveryMemberSeesTheOrganisationsOwnBoards(t *testing.T) {
	// given a plain member on no team, an organisation board, a team board, and another organisation's board planted among the rows
	svc, m := newTestService(t)
	organisationBoard := listedBoard(model.Board{ID: otherID, OrgID: orgID, Title: "All hands", CreatedAt: stamp}, model.AnonymityNamed)
	organisationBoard.Board.TemplateKey = "blank"
	teamBoard := listedBoard(model.Board{ID: boardID, OrgID: orgID, TeamID: new(teamID), Title: "Sprint 12", CreatedAt: stamp}, model.AnonymityNamed)
	planted := listedBoard(model.Board{ID: cardID, OrgID: theirID, Title: "Planted", CreatedAt: stamp}, model.AnonymityNamed)
	m.organisationBoards(model.OrgRoleMember, []model.MemberTeam{}, []model.BoardHeader{organisationBoard, teamBoard, planted})

	// when
	got, err := svc.ListForOrganisation(t.Context(), spec.OrganisationMemberKey{OrgID: orgID, UserID: userID})

	// then the organisation board is listed with no team, and nothing else
	require.NoError(t, err)
	assert.Equal(t, &dto.OrgBoardListResponse{Boards: []dto.OrgBoardListItem{
		{ID: otherID, Title: "All hands", CreatedAt: "2026-10-04T12:00:00.000Z", TemplateKey: "blank"},
	}}, got)
}

func TestDescribe_SetsTheDescriptionLine(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().SetBoardContext(mock.Anything, spec.BoardContextWrite{BoardWrite: target(), Context: "Release week"}).Return(&model.Versioned[model.Board]{Value: model.Board{ID: boardID, Context: "Release week"}, Version: 8}, nil)

	// when
	got, err := svc.Describe(t.Context(), DescribeCommand{Command: baseCommand(), Context: "  Release week "})

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestDescribe_ClearsTheDescriptionLine(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().SetBoardContext(mock.Anything, spec.BoardContextWrite{BoardWrite: target(), Context: ""}).Return(&model.Versioned[model.Board]{Value: model.Board{ID: boardID}, Version: 8}, nil)

	// when
	got, err := svc.Describe(t.Context(), DescribeCommand{Command: baseCommand(), Context: "   "})

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
}

func TestDescribe_RefusesALineThatIsTooLong(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got, err := svc.Describe(t.Context(), DescribeCommand{Command: baseCommand(), Context: strings.Repeat("a", ContextMaxRunes+1)})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrContextTooLong)
}

func TestDescribe_AParticipantMayNot(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.Describe(t.Context(), DescribeCommand{Command: baseCommand(), Context: "Release week"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestDescribe_AFailedWriteIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().SetBoardContext(mock.Anything, spec.BoardContextWrite{BoardWrite: target(), Context: "Release week"}).Return(nil, errDatabase)

	// when
	got, err := svc.Describe(t.Context(), DescribeCommand{Command: baseCommand(), Context: "Release week"})

	// then
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errDatabase)
}
