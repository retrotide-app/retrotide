package management

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/board"
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
		boards   *repository.MockBoardRepository
		columns  *repository.MockBoardColumnRepository
		cards    *repository.MockCardRepository
		teams    *repository.MockTeamRepository
		live     *live.MockService
		orgAuthz *orgauthz.MockService
		auditLog *repository.MockAuditLogRepository
		events   *recordedEvents

		acquired int
		released int
	}

	recordedEvents struct {
		published []publishedEvent
		announced []uuid.UUID
	}

	publishedEvent struct {
		access model.BoardAccess
		event  projection.Event
	}

	accessOption func(*model.BoardAccess)
)

var (
	orgID      = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000001")
	teamID     = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000002")
	boardID    = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000003")
	userID     = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000004")
	myID       = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000005")
	opID       = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000006")
	otherOrgID = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000007")
	otherTeam  = uuid.MustParse("2b3c4d5e-0000-4000-8000-000000000008")
	stamp      = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	errDatabase = errors.New("the database is unreachable")
	errNotFound = fmt.Errorf("lookup: %w", dao.ErrNotFound)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
	me     = model.Principal{Kind: model.PrincipalUser, ID: userID}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		boards:   repository.NewMockBoardRepository(t),
		columns:  repository.NewMockBoardColumnRepository(t),
		cards:    repository.NewMockCardRepository(t),
		teams:    repository.NewMockTeamRepository(t),
		live:     live.NewMockService(t),
		orgAuthz: orgauthz.NewMockService(t),
		auditLog: repository.NewMockAuditLogRepository(t),
		events:   &recordedEvents{},
	}

	t.Cleanup(func() {
		assert.Equal(t, m.acquired, m.released, "every gate taken is released")
	})

	svc := NewService(Deps{
		Boards:   m.boards,
		Columns:  m.columns,
		Cards:    m.cards,
		Teams:    m.teams,
		Live:     m.live,
		Events:   m.events,
		OrgAuthz: m.orgAuthz,
		AuditLog: m.auditLog,
		Now: func() time.Time {
			return stamp
		},
	}).(*service)

	return svc, m
}

func (r *recordedEvents) Publish(access model.BoardAccess, events ...projection.Event) {
	for _, event := range events {
		r.published = append(r.published, publishedEvent{access: access, event: event})
	}
}

func (r *recordedEvents) PublishMe(model.BoardAccess, dto.BoardMeFrame) {}

func (r *recordedEvents) PublishMeToEveryone(model.BoardAccess, []model.CardVote, int64) {}

func (r *recordedEvents) PublishedTally(uuid.UUID) (projection.HiddenTally, bool) {
	return nil, false
}

func (r *recordedEvents) AnnounceTeamActionItems(teamID uuid.UUID, _ []uuid.UUID) {
	r.announced = append(r.announced, teamID)
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

func archived(access *model.BoardAccess) {
	access.Header.ArchivedAt = new(stamp)
	access.Capabilities = []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityManage, model.BoardCapabilityExport}
}

func anonymous(access *model.BoardAccess) {
	access.Header.Anonymity = model.AnonymityAnonymous
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

func versioned(board model.Board, version int64) *model.Versioned[model.Board] {
	return &model.Versioned[model.Board]{Value: board, Version: version}
}

func boardAudit(action audit.Action, details map[string]string) audit.NewEntry {
	return audit.NewEntry{
		OrgID:      orgID,
		ActorID:    userID,
		ActorKind:  audit.ActorUser,
		Action:     action,
		TargetType: audit.TargetBoard,
		TargetID:   boardID.String(),
		SubjectID:  userID,
		Details:    details,
	}
}

func resyncAt(version int64) dto.BoardEventHead {
	return dto.BoardEventHead{Type: projection.EventResync, Version: version}
}

func TestArchive_ArchivesTheBoardRevokesItsLiveViewersAndAuditsIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	stored := access.Header.Board
	stored.ArchivedAt = new(stamp)
	m.boards.EXPECT().ArchiveBoard(mock.Anything, spec.BoardArchiveWrite{BoardWrite: target(), Archived: true}).Return(versioned(stored, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, boardAudit(audit.ActionBoardArchived, map[string]string{"archived": "true"})).Return(nil).Once()

	// when
	got, err := svc.Archive(t.Context(), baseCommand())

	// then everyone refetches the read-only board
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, resyncAt(8), m.events.published[0].event.Payload(projection.ClassOthers))
	assert.True(t, m.events.published[0].access.Header.Archived())
}

func TestUnarchive_BringsAnArchivedBoardBackAndAuditsItAsNotArchived(t *testing.T) {
	// given a board archived earlier, which a manager still resolves
	svc, m := newTestService(t)
	access := boardAccess(host, archived)
	m.entered(access)
	stored := access.Header.Board
	stored.ArchivedAt = nil
	m.boards.EXPECT().ArchiveBoard(mock.Anything, spec.BoardArchiveWrite{BoardWrite: target(), Archived: false}).Return(versioned(stored, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, boardAudit(audit.ActionBoardArchived, map[string]string{"archived": "false"})).Return(nil).Once()

	// when
	got, err := svc.Unarchive(t.Context(), baseCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	assert.False(t, m.events.published[0].access.Header.Archived())
}

func TestArchive_IsAllowedOnAnEndedBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	access.Header.ClosedAt = new(stamp)
	m.entered(access)
	m.boards.EXPECT().ArchiveBoard(mock.Anything, mock.Anything).Return(versioned(access.Header.Board, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	// when
	_, err := svc.Archive(t.Context(), baseCommand())

	// then
	require.NoError(t, err)
}

func TestArchive_IsRefusedWithoutBoardManage(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.Archive(t.Context(), baseCommand())

	// then the strict mocks prove nothing was written or audited
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Nil(t, got)
	assert.Empty(t, m.events.published)
}

func TestArchive_AMissingAuditEntryStillAnswersTheArchive(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().ArchiveBoard(mock.Anything, mock.Anything).Return(versioned(access.Header.Board, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(nil)
	m.auditLog.EXPECT().Create(mock.Anything, mock.Anything).Return(errDatabase).Once()

	// when
	got, err := svc.Archive(t.Context(), baseCommand())

	// then
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.ErrorIs(t, err, ErrNotAudited)
	require.ErrorIs(t, err, errDatabase)
}

func TestArchive_AFailedReevaluationIsReported(t *testing.T) {
	// given
	svc, m := newTestService(t)
	access := boardAccess(host)
	m.entered(access)
	m.boards.EXPECT().ArchiveBoard(mock.Anything, mock.Anything).Return(versioned(access.Header.Board, 8), nil)
	m.live.EXPECT().ReevaluateHeld(mock.Anything, boardID).Return(errDatabase)

	// when
	got, err := svc.Archive(t.Context(), baseCommand())

	// then
	require.ErrorIs(t, err, errDatabase)
	assert.Nil(t, got)
}

func TestArchive_ReplaysAnOpItAlreadyApplied(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 5)

	// when
	got, err := svc.Archive(t.Context(), baseCommand())

	// then
	require.NoError(t, err)
	want := projection.ReplayResponse(5, nil)
	assert.Equal(t, &want, got)
}

func TestArchive_PassesOnABoardTheViewerCannotOpen(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(nil, fmt.Errorf("resolve: %w", live.ErrNoAccess)).Once()

	// when
	_, err := svc.Archive(t.Context(), baseCommand())

	// then
	require.ErrorIs(t, err, live.ErrNoAccess)
}
