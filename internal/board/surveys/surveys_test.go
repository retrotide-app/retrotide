package surveys

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
	"github.com/stretchr/testify/require"
)

type (
	testMocks struct {
		boards  *repository.MockBoardRepository
		cards   *repository.MockCardRepository
		surveys *repository.MockSurveyRepository
		live    *live.MockService
		events  *recordedEvents

		acquired int
		released int
	}

	recordedEvents struct {
		published []publishedEvent
		strays    int
	}

	publishedEvent struct {
		access model.BoardAccess
		event  projection.Event
	}

	accessOption func(*model.BoardAccess)
)

var (
	orgID       = uuid.MustParse("5e6f7081-0000-4000-8000-000000000001")
	teamID      = uuid.MustParse("5e6f7081-0000-4000-8000-000000000002")
	boardID     = uuid.MustParse("5e6f7081-0000-4000-8000-000000000003")
	userID      = uuid.MustParse("5e6f7081-0000-4000-8000-000000000004")
	myID        = uuid.MustParse("5e6f7081-0000-4000-8000-000000000005")
	theirID     = uuid.MustParse("5e6f7081-0000-4000-8000-000000000006")
	opID        = uuid.MustParse("5e6f7081-0000-4000-8000-000000000007")
	surveyID    = uuid.MustParse("5e6f7081-0000-4000-8000-000000000008")
	paceID      = uuid.MustParse("5e6f7081-0000-4000-8000-000000000009")
	moodID      = uuid.MustParse("5e6f7081-0000-4000-8000-00000000000a")
	focusID     = uuid.MustParse("5e6f7081-0000-4000-8000-00000000000b")
	guestDevice = uuid.MustParse("5e6f7081-0000-4000-8000-00000000000c")
	v7ID        = uuid.MustParse("01920000-0000-7000-8000-000000000001")
	stamp       = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	errNotFound = fmt.Errorf("lookup: %w", dao.ErrNotFound)

	host   = model.BoardRoleFacilitator
	writer = model.BoardRoleParticipant
	me     = model.Principal{Kind: model.PrincipalUser, ID: userID}
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		boards:  repository.NewMockBoardRepository(t),
		cards:   repository.NewMockCardRepository(t),
		surveys: repository.NewMockSurveyRepository(t),
		live:    live.NewMockService(t),
		events:  &recordedEvents{},
	}

	t.Cleanup(func() {
		assert.Equal(t, m.acquired, m.released, "every gate taken is released")
		assert.Zero(t, m.events.strays, "a survey command sends no me frame and announces no action items")
	})

	svc := NewService(Deps{
		Boards:  m.boards,
		Cards:   m.cards,
		Surveys: m.surveys,
		Live:    m.live,
		Events:  m.events,
	}).(*service)

	return svc, m
}

func (r *recordedEvents) Publish(access model.BoardAccess, events ...projection.Event) {
	for _, event := range events {
		r.published = append(r.published, publishedEvent{access: access, event: event})
	}
}

func (r *recordedEvents) PublishMe(_ model.BoardAccess, _ dto.BoardMeFrame) {
	r.strays++
}

func (r *recordedEvents) PublishMeToEveryone(_ model.BoardAccess, _ []model.CardVote, _ int64) {
	r.strays++
}

func (r *recordedEvents) PublishedTally(_ uuid.UUID) (projection.HiddenTally, bool) {
	return nil, false
}

func (r *recordedEvents) AnnounceTeamActionItems(_ uuid.UUID, _ []uuid.UUID) {
	r.strays++
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
			Stage:     model.StageDiscuss,
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

func notJoined(access *model.BoardAccess) {
	access.ParticipantID = nil
}

func asGuest(access *model.BoardAccess) {
	access.Principal = model.Principal{Kind: model.PrincipalGuest, ID: guestDevice}
	access.OrgMember = false
	access.Capabilities = policy.Capabilities(access.Role, false)
}

func ended(access *model.BoardAccess) {
	access.Header.ClosedAt = new(stamp)
	access.Header.Stage = model.StageEnd
	access.Capabilities = []model.BoardCapability{model.BoardCapabilityView, model.BoardCapabilityExport}
}

func baseCommand() board.Command {
	return board.Command{Principal: me, BoardID: boardID, OpID: opID}
}

func target() spec.BoardWrite {
	return spec.BoardWrite{OrgID: orgID, BoardID: boardID, Version: 7, OpID: opID}
}

func surveyKey() spec.SurveyKey {
	return spec.SurveyKey{BoardID: boardID, SurveyID: surveyID}
}

func openSurvey() *model.Survey {
	return &model.Survey{
		ID:        surveyID,
		OrgID:     orgID,
		BoardID:   boardID,
		Title:     "Sprint health",
		Status:    model.SurveyOpen,
		CreatedAt: stamp,
		Questions: []model.SurveyQuestion{
			{ID: paceID, SurveyID: surveyID, Position: 0, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", Options: []string{}, ScaleMin: new(1), ScaleMax: new(5)},
			{ID: moodID, SurveyID: surveyID, Position: 1, Kind: model.QuestionSingleChoice, Prompt: "How do you feel?", Options: []string{"Good", "Okay", "Bad"}},
			{ID: focusID, SurveyID: surveyID, Position: 2, Kind: model.QuestionMultipleChoice, Prompt: "What should we focus on?", Options: []string{"Quality", "Speed", "Process"}},
		},
	}
}

func closedSurvey() *model.Survey {
	survey := openSurvey()
	survey.Status = model.SurveyClosed
	survey.ClosedAt = new(stamp)

	return survey
}

func answersFrom(participantID uuid.UUID, pace, mood int) []model.SurveyResponse {
	return []model.SurveyResponse{
		{ID: uuid.New(), SurveyID: surveyID, QuestionID: paceID, ParticipantID: new(participantID), Answer: []int{pace}},
		{ID: uuid.New(), SurveyID: surveyID, QuestionID: moodID, ParticipantID: new(participantID), Answer: []int{mood}},
	}
}

func (m *testMocks) gate() {
	m.acquired++
	m.live.EXPECT().Acquire(mock.Anything, boardID).Return(func() {
		m.released++
	}, nil).Once()
}

func (m *testMocks) entered(access *model.BoardAccess) {
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: access.Principal, BoardID: boardID}).Return(access, nil).Once()
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(nil, errNotFound).Once()
}

func (m *testMocks) applied(access *model.BoardAccess, version int64) {
	m.gate()
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: access.Principal, BoardID: boardID}).Return(access, nil).Once()
	m.boards.EXPECT().GetOp(mock.Anything, spec.BoardOpKey{BoardID: boardID, OpID: opID}).Return(&model.BoardOp{BoardID: boardID, OpID: opID, Version: version}, nil).Once()
}

func upsertedFor(t *testing.T, event projection.Event) dto.SurveyUpsertedEvent {
	t.Helper()

	upserted, ok := event.Payload(projection.ClassOthers).(dto.SurveyUpsertedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventSurveyUpserted, upserted.Type)

	return upserted
}

func customCommand() CreateCommand {
	return CreateCommand{
		Command:  baseCommand(),
		SurveyID: surveyID,
		Title:    "  Sprint health  ",
		Questions: []QuestionDraft{
			{ID: paceID, Kind: model.QuestionScale, Prompt: " How sustainable was the pace? ", ScaleMin: new(1), ScaleMax: new(5)},
			{ID: moodID, Kind: model.QuestionSingleChoice, Prompt: "How do you feel?", Options: []string{" Good ", "Okay", "Bad"}},
		},
	}
}

func TestCreate_AFacilitatorAddsACustomSurveyAndEveryoneHearsOfIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Count(mock.Anything, boardID).Return(3, nil).Once()
	written := openSurvey()
	written.Questions = written.Questions[:2]
	m.surveys.EXPECT().CreateSurvey(mock.Anything, spec.SurveyCreateWrite{
		BoardWrite: target(),
		Survey:     spec.NewSurvey{OrgID: orgID, BoardID: boardID, ID: surveyID, Title: "Sprint health"},
		Questions: []spec.NewSurveyQuestion{
			{OrgID: orgID, BoardID: boardID, SurveyID: surveyID, ID: paceID, Position: 0, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", Options: []string{}, ScaleMin: new(1), ScaleMax: new(5)},
			{OrgID: orgID, BoardID: boardID, SurveyID: surveyID, ID: moodID, Position: 1, Kind: model.QuestionSingleChoice, Prompt: "How do you feel?", Options: []string{"Good", "Okay", "Bad"}},
		},
	}).Return(&model.Versioned[model.Survey]{Value: *written, Version: 8}, nil).Once()

	// when
	got, err := svc.Create(t.Context(), customCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Survey)
	assert.Equal(t, surveyID, got.Survey.ID)
	assert.Equal(t, model.SurveyOpen, got.Survey.Status)
	assert.Len(t, got.Survey.Questions, 2)
	require.Len(t, m.events.published, 1)
	upserted := upsertedFor(t, m.events.published[0].event)
	assert.Equal(t, int64(8), upserted.Version)
	assert.Equal(t, surveyID, upserted.Survey.ID)
	assert.Empty(t, upserted.Survey.MyAnswers)
	assert.Nil(t, upserted.Survey.Questions[0].Results)
}

func TestCreate_ATemplateCopiesItsQuestionsUnderFreshIDs(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Count(mock.Anything, boardID).Return(0, nil).Once()
	var write spec.SurveyCreateWrite
	captured := mock.MatchedBy(func(s spec.SurveyCreateWrite) bool {
		write = s

		return true
	})
	created := model.Survey{ID: surveyID, BoardID: boardID, Title: "Team mood", TemplateKey: "team_mood", Status: model.SurveyOpen, Questions: []model.SurveyQuestion{}}
	m.surveys.EXPECT().CreateSurvey(mock.Anything, captured).Return(&model.Versioned[model.Survey]{Value: created, Version: 8}, nil).Once()

	// when
	got, err := svc.Create(t.Context(), CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Team mood", TemplateKey: "team_mood"})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.Equal(t, spec.NewSurvey{OrgID: orgID, BoardID: boardID, ID: surveyID, Title: "Team mood", TemplateKey: "team_mood"}, write.Survey)
	require.Len(t, write.Questions, 1)
	question := write.Questions[0]
	assert.Equal(t, model.QuestionSingleChoice, question.Kind)
	assert.Equal(t, "How are you feeling about the last sprint?", question.Prompt)
	assert.Equal(t, []string{"Great", "Good", "Okay", "Not great", "Bad"}, question.Options)
	assert.Equal(t, 4, int(question.ID.Version()))
	assert.Equal(t, surveyID, question.SurveyID)
	assert.Equal(t, 0, question.Position)
}

func TestCreate_TheHealthCheckCopiesEightScaleQuestionsInOrder(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Count(mock.Anything, boardID).Return(0, nil).Once()
	var write spec.SurveyCreateWrite
	captured := mock.MatchedBy(func(s spec.SurveyCreateWrite) bool {
		write = s

		return true
	})
	created := model.Survey{ID: surveyID, BoardID: boardID, Title: "Team health check", TemplateKey: "health_check", Status: model.SurveyOpen, Questions: []model.SurveyQuestion{}}
	m.surveys.EXPECT().CreateSurvey(mock.Anything, captured).Return(&model.Versioned[model.Survey]{Value: created, Version: 8}, nil).Once()

	// when
	_, err := svc.Create(t.Context(), CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Team health check", TemplateKey: "health_check"})

	// then
	require.NoError(t, err)
	require.Len(t, write.Questions, 8)
	seen := map[uuid.UUID]struct{}{}
	for position, question := range write.Questions {
		assert.Equal(t, position, question.Position)
		assert.Equal(t, model.QuestionScale, question.Kind)
		assert.Equal(t, new(1), question.ScaleMin)
		assert.Equal(t, new(5), question.ScaleMax)
		seen[question.ID] = struct{}{}
	}
	assert.Len(t, seen, 8)
}

func TestCreate_RefusesABadRequestBeforeTouchingTheBoard(t *testing.T) {
	tests := []struct {
		name    string
		command CreateCommand
		want    error
	}{
		{name: "a survey ID that is not v4", command: CreateCommand{Command: baseCommand(), SurveyID: v7ID, Title: "Mood", TemplateKey: "team_mood"}, want: board.ErrInvalidID},
		{name: "a question ID that is not v4", command: CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Mood", Questions: []QuestionDraft{{ID: v7ID, Kind: model.QuestionScale, Prompt: "Pace?", ScaleMin: new(1), ScaleMax: new(5)}}}, want: board.ErrInvalidID},
		{name: "a template and questions at once", command: CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Mood", TemplateKey: "team_mood", Questions: customCommand().Questions}, want: ErrTemplateWithQuestions},
		{name: "an unknown template", command: CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Mood", TemplateKey: "weather"}, want: ErrUnknownTemplate},
		{name: "no title", command: CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "   ", TemplateKey: "team_mood"}, want: ErrTitleRequired},
		{name: "no questions and no template", command: CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Mood"}, want: ErrQuestionsRequired},
		{name: "a question of no known kind", command: CreateCommand{Command: baseCommand(), SurveyID: surveyID, Title: "Mood", Questions: []QuestionDraft{{ID: paceID, Kind: "essay", Prompt: "Tell us"}}}, want: ErrInvalidQuestionKind},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.Create(t.Context(), tc.command)

			// then the strict mocks prove the board was never opened
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestCreate_OnlyAFacilitatorAddsASurvey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.Create(t.Context(), customCommand())

	// then the strict survey mock proves nothing was counted or written
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Nil(t, got)
	assert.Empty(t, m.events.published)
}

func TestCreate_AnEndedBoardTakesNoNewSurvey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, ended))

	// when
	_, err := svc.Create(t.Context(), customCommand())

	// then
	require.ErrorIs(t, err, policy.ErrReadOnly)
	assert.Empty(t, m.events.published)
}

func TestCreate_RefusesATwentyFirstSurvey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Count(mock.Anything, boardID).Return(model.BoardMaxSurveys, nil).Once()

	// when
	_, err := svc.Create(t.Context(), customCommand())

	// then
	require.ErrorIs(t, err, ErrTooManySurveys)
	assert.Empty(t, m.events.published)
}

func TestCreate_ATakenIDIsAnIDConflict(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Count(mock.Anything, boardID).Return(0, nil).Once()
	m.surveys.EXPECT().CreateSurvey(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("create survey: %w", dao.ErrConflict)).Once()

	// when
	_, err := svc.Create(t.Context(), customCommand())

	// then
	require.ErrorIs(t, err, board.ErrIDConflict)
	assert.Empty(t, m.events.published)
}

func TestCreate_AReplayReadsTheSurveyBack(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 8)
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()
	m.surveys.EXPECT().ListResponsesForSurvey(mock.Anything, surveyKey()).Return(answersFrom(myID, 4, 0), nil).Once()

	// when
	got, err := svc.Create(t.Context(), customCommand())

	// then nothing is written or published a second time
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.True(t, got.Replayed)
	assert.Equal(t, new(surveyID), got.ID)
	require.NotNil(t, got.Survey)
	assert.Equal(t, map[uuid.UUID][]int{paceID: {4}, moodID: {0}}, got.Survey.MyAnswers)
	assert.Empty(t, m.events.published)
}

func TestCreate_AReplayOfASurveyDeletedSinceAnswersWithItsID(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 8)
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(nil, errNotFound).Once()

	// when
	got, err := svc.Create(t.Context(), customCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8, Replayed: true, ID: new(surveyID)}, got)
}

func TestClose_ClosingASurveySendsItsResultsToEveryone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()
	responses := append(answersFrom(myID, 4, 0), answersFrom(theirID, 2, 0)...)
	m.surveys.EXPECT().CloseSurvey(mock.Anything, spec.SurveyWrite{BoardWrite: target(), SurveyID: surveyID}).Return(&model.Versioned[model.SurveyState]{Value: model.SurveyState{Survey: *closedSurvey(), Responses: responses}, Version: 8}, nil).Once()

	// when
	got, err := svc.Close(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Survey)
	assert.Equal(t, model.SurveyClosed, got.Survey.Status)
	require.Len(t, m.events.published, 1)
	upserted := upsertedFor(t, m.events.published[0].event)
	assert.Equal(t, 2, upserted.Survey.Respondents)
	assert.Empty(t, upserted.Survey.MyAnswers)
	require.NotNil(t, upserted.Survey.Questions[0].Results)
	assert.Equal(t, []int{0, 1, 0, 1, 0}, upserted.Survey.Questions[0].Results.Counts)
	assert.Equal(t, new(3.0), upserted.Survey.Questions[0].Results.Average)
	require.NotNil(t, upserted.Survey.Questions[1].Results)
	assert.Equal(t, []int{2, 0, 0}, upserted.Survey.Questions[1].Results.Counts)
}

func TestClose_AClosedSurveyIsNotWrittenAgain(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(closedSurvey(), nil).Once()
	m.surveys.EXPECT().ListResponsesForSurvey(mock.Anything, surveyKey()).Return(answersFrom(theirID, 5, 2), nil).Once()

	// when
	got, err := svc.Close(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.Version)
	require.NotNil(t, got.Survey)
	assert.Equal(t, model.SurveyClosed, got.Survey.Status)
	assert.Empty(t, m.events.published)
}

func TestClose_AMissingSurveyIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(nil, errNotFound).Once()

	// when
	_, err := svc.Close(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then
	require.ErrorIs(t, err, policy.ErrSurveyNotFound)
	assert.Empty(t, m.events.published)
}

func TestClose_OnlyAFacilitatorClosesASurvey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	_, err := svc.Close(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then the strict survey mock proves the survey was never read
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Empty(t, m.events.published)
}

func TestDelete_AFacilitatorRemovesASurveyFromEveryone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(openSurvey(), nil).Once()
	m.surveys.EXPECT().DeleteSurvey(mock.Anything, spec.SurveyWrite{BoardWrite: target(), SurveyID: surveyID}).Return(&model.Versioned[model.Survey]{Value: *openSurvey(), Version: 8}, nil).Once()

	// when
	got, err := svc.Delete(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	removed, ok := m.events.published[0].event.Payload(projection.ClassOthers).(dto.SurveyRemovedEvent)
	require.True(t, ok)
	assert.Equal(t, projection.EventSurveyRemoved, removed.Type)
	assert.Equal(t, surveyID, removed.SurveyID)
	assert.Equal(t, int64(8), removed.Version)
}

func TestDelete_OnlyAFacilitatorRemovesASurvey(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	_, err := svc.Delete(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then
	require.ErrorIs(t, err, policy.ErrNotAllowed)
	assert.Empty(t, m.events.published)
}

func TestDelete_AMissingSurveyIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.surveys.EXPECT().Get(mock.Anything, surveyKey()).Return(nil, errNotFound).Once()

	// when
	_, err := svc.Delete(t.Context(), SurveyCommand{Command: baseCommand(), SurveyID: surveyID})

	// then
	require.ErrorIs(t, err, policy.ErrSurveyNotFound)
}

func TestTemplates_ListsTheCatalogueWithOptionsNeverNull(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	got := svc.Templates()

	// then
	require.Len(t, got.Templates, 5)
	health := got.Templates[0]
	assert.Equal(t, "health_check", health.Key)
	assert.Equal(t, "Team health check", health.Title)
	assert.Equal(t, "health_check", health.Category)
	require.Len(t, health.Questions, 8)
	assert.Equal(t, []string{}, health.Questions[0].Options)
	assert.Equal(t, new(1), health.Questions[0].ScaleMin)
	mood := got.Templates[3]
	assert.Equal(t, "team_mood", mood.Key)
	assert.Equal(t, "survey", mood.Category)
	assert.Nil(t, mood.Questions[0].ScaleMin)
}
