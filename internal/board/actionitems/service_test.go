package actionitems

import (
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
)

type (
	testMocks struct {
		boards        *repository.MockBoardRepository
		cards         *repository.MockCardRepository
		actionItems   *repository.MockActionItemRepository
		teams         *repository.MockTeamRepository
		organisations *repository.MockOrganisationRepository
		live          *live.MockService
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
	v7ID     = uuid.MustParse("01920000-0000-7000-8000-000000000001")
	stamp    = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

	errNotFound = fmt.Errorf("lookup: %w", dao.ErrNotFound)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
	me     = model.Principal{Kind: model.PrincipalUser, ID: userID}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		boards:        repository.NewMockBoardRepository(t),
		cards:         repository.NewMockCardRepository(t),
		actionItems:   repository.NewMockActionItemRepository(t),
		teams:         repository.NewMockTeamRepository(t),
		organisations: repository.NewMockOrganisationRepository(t),
		live:          live.NewMockService(t),
		events:        &recordedEvents{tallies: map[uuid.UUID]projection.HiddenTally{}},
	}

	t.Cleanup(func() {
		assert.Equal(t, m.acquired, m.released, "every gate taken is released")
	})

	svc := NewService(Deps{
		Boards:        m.boards,
		Cards:         m.cards,
		ActionItems:   m.actionItems,
		Teams:         m.teams,
		Organisations: m.organisations,
		Live:          m.live,
		Events:        m.events,
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

func anonymous(access *model.BoardAccess) {
	access.Header.Anonymity = model.AnonymityAnonymous
}

func notJoined(access *model.BoardAccess) {
	access.ParticipantID = nil
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

func versionResponse(version int64) *dto.BoardCommandResponse {
	return &dto.BoardCommandResponse{Version: version}
}
