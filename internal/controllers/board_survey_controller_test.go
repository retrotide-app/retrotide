package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/board/surveys"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	surveyHarness struct {
		*testutil.Harness
		surveys *surveys.MockService
	}
)

var (
	boardSurveyID    = uuid.MustParse("6a7b8c9d-0e1f-4a2b-9c3d-4e5f6a7b8c9d")
	boardQuestionID  = uuid.MustParse("7b8c9d0e-1f2a-4b3c-8d4e-5f6a7b8c9d0e")
	boardSurveyRoute = "/surveys/" + boardSurveyID.String()
	boardSurveyBody  = dto.CreateSurveyRequest{
		BoardCommandRequest: boardOpBody,
		ID:                  boardSurveyID,
		Title:               "Sprint health",
		Questions: []dto.SurveyQuestionRequest{
			{ID: boardQuestionID, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", ScaleMin: new(1), ScaleMax: new(5)},
		},
	}
	boardSurveyAnswers = dto.SurveyResponseRequest{
		BoardCommandRequest: boardOpBody,
		Answers:             []dto.SurveyAnswerRequest{{QuestionID: boardQuestionID, Answer: []int{4}}},
	}
	boardSurveyAnswered = &dto.BoardCommandResponse{Version: 7, Survey: &dto.SurveyView{
		ID:          boardSurveyID,
		Title:       "Sprint health",
		Status:      model.SurveyOpen,
		Questions:   []dto.SurveyQuestionView{{ID: boardQuestionID, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", Options: []string{}, ScaleMin: new(1), ScaleMax: new(5)}},
		Respondents: 1,
		MyAnswers:   map[uuid.UUID][]int{boardQuestionID: {4}},
	}}

	tenantSurveyB = uuid.MustParse("5abbccdd-eeff-4a01-b2c3-d4e5f6071829")
)

func newSurveyHarness(t *testing.T) *surveyHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	sh := &surveyHarness{Harness: h, surveys: surveys.NewMockService(t)}

	s := &Service{BoardSurveys: sh.surveys, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), append(s.getAllBoardSurveyRoutes(), s.getAllSurveyTemplateRoutes()...))

	return sh
}

func (sh *surveyHarness) record(method string, path string, body any) (int, string) {
	cookie := sh.SignIn(boardUser)
	if method != http.MethodGet && method != http.MethodHead {
		sh.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)
	}

	request := sh.NewRequest(method, path).WithCookie(cookie)
	if body != nil {
		request = request.WithJSONBody(body)
	}

	rec := request.Record()

	return rec.Code, rec.Body.String()
}

func TestBoardSurveyController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardSurveys: surveys.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), append(s.getAllBoardSurveyRoutes(), s.getAllSurveyTemplateRoutes()...))

	// then
	assert.ElementsMatch(t, []string{
		"POST /api/v1/boards/:boardID/surveys",
		"POST /api/v1/boards/:boardID/surveys/:surveyID/close",
		"DELETE /api/v1/boards/:boardID/surveys/:surveyID",
		"PUT /api/v1/boards/:boardID/surveys/:surveyID/responses",
		"GET /api/v1/survey-templates",
		"HEAD /api/v1/survey-templates",
	}, registeredRoutes(h.Engine))
}

func TestBoardSurveyController_EveryBoardRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: boardPath("/surveys"), Body: boardSurveyBody},
		{Method: http.MethodPost, Path: boardPath(boardSurveyRoute + "/close"), Body: boardOpBody},
		{Method: http.MethodDelete, Path: boardPath(boardSurveyRoute), Body: boardOpBody},
		{Method: http.MethodPut, Path: boardPath(boardSurveyRoute + "/responses"), Body: boardSurveyAnswers},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newSurveyHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardSurveyController_TheTemplatesNeedNoSignIn(t *testing.T) {
	// given
	sh := newSurveyHarness(t)
	listing := &dto.SurveyTemplateListResponse{Templates: []dto.SurveyTemplateView{{
		Key:         "team_mood",
		Title:       "Team mood",
		Description: "A single pick of how everyone is feeling.",
		Category:    "survey",
		Questions:   []dto.SurveyTemplateQuestionView{{Kind: model.QuestionSingleChoice, Prompt: "How are you feeling?", Options: []string{"Good", "Bad"}}},
	}}}
	sh.surveys.EXPECT().Templates().Return(listing)

	// when
	rec := sh.NewRequest(http.MethodGet, "/api/v1/survey-templates").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, mustJSON(t, listing), rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"scale_min":null`)
}

func TestBoardSurveyController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(sh *surveyHarness)
	}{
		{
			name:   "create a custom survey",
			method: http.MethodPost,
			path:   boardPath("/surveys"),
			body:   boardSurveyBody,
			expect: func(sh *surveyHarness) {
				create := surveys.CreateCommand{
					Command:   boardCommandOf,
					SurveyID:  boardSurveyID,
					Title:     "Sprint health",
					Questions: []surveys.QuestionDraft{{ID: boardQuestionID, Kind: model.QuestionScale, Prompt: "How sustainable was the pace?", ScaleMin: new(1), ScaleMax: new(5)}},
				}
				sh.surveys.EXPECT().Create(mock.Anything, create).Return(boardVersioned, nil)
			},
		},
		{
			name:   "create a survey from a template",
			method: http.MethodPost,
			path:   boardPath("/surveys"),
			body:   dto.CreateSurveyRequest{BoardCommandRequest: boardOpBody, ID: boardSurveyID, Title: "Team mood", TemplateKey: "team_mood"},
			expect: func(sh *surveyHarness) {
				create := surveys.CreateCommand{Command: boardCommandOf, SurveyID: boardSurveyID, Title: "Team mood", TemplateKey: "team_mood", Questions: []surveys.QuestionDraft{}}
				sh.surveys.EXPECT().Create(mock.Anything, create).Return(boardVersioned, nil)
			},
		},
		{
			name:   "close a survey",
			method: http.MethodPost,
			path:   boardPath(boardSurveyRoute + "/close"),
			body:   boardOpBody,
			expect: func(sh *surveyHarness) {
				sh.surveys.EXPECT().Close(mock.Anything, surveys.SurveyCommand{Command: boardCommandOf, SurveyID: boardSurveyID}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "delete a survey",
			method: http.MethodDelete,
			path:   boardPath(boardSurveyRoute),
			body:   boardOpBody,
			expect: func(sh *surveyHarness) {
				sh.surveys.EXPECT().Delete(mock.Anything, surveys.SurveyCommand{Command: boardCommandOf, SurveyID: boardSurveyID}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "answer a survey",
			method: http.MethodPut,
			path:   boardPath(boardSurveyRoute + "/responses"),
			body:   boardSurveyAnswers,
			expect: func(sh *surveyHarness) {
				respond := surveys.RespondCommand{Command: boardCommandOf, SurveyID: boardSurveyID, Answers: []spec.SurveyAnswer{{QuestionID: boardQuestionID, Answer: []int{4}}}}
				sh.surveys.EXPECT().Respond(mock.Anything, respond).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sh := newSurveyHarness(t)
			tc.expect(sh)

			// when
			status, body := sh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardSurveyController_AnAnswerComesBackWithTheSurvey(t *testing.T) {
	// given
	sh := newSurveyHarness(t)
	sh.surveys.EXPECT().Respond(mock.Anything, mock.Anything).Return(boardSurveyAnswered, nil)

	// when
	status, body := sh.record(http.MethodPut, boardPath(boardSurveyRoute+"/responses"), boardSurveyAnswers)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, mustJSON(t, boardSurveyAnswered), body)
	assert.Contains(t, body, `"my_answers":{"`+boardQuestionID.String()+`":[4]}`)
	assert.Contains(t, body, `"results":null`)
}

func TestBoardSurveyController_AGuestAnswersAnOpenSurveyAsItself(t *testing.T) {
	// given
	sh := newSurveyHarness(t)
	cookie := sh.SignInAsGuest(linkGuest)
	respond := surveys.RespondCommand{
		Command:  board.Command{Principal: linkGuestPerson, BoardID: boardID, OpID: boardOp},
		SurveyID: boardSurveyID,
		Answers:  []spec.SurveyAnswer{{QuestionID: boardQuestionID, Answer: []int{4}}},
	}
	sh.surveys.EXPECT().Respond(mock.Anything, respond).Return(boardSurveyAnswered, nil)

	// when
	rec := sh.NewRequest(http.MethodPut, boardPath(boardSurveyRoute+"/responses")).WithCookie(cookie).WithJSONBody(boardSurveyAnswers).Record()

	// then a guest with a seat answers like anyone else, and the account lock is never asked about
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, mustJSON(t, boardSurveyAnswered), rec.Body.String())
}

func TestBoardSurveyController_CreateRefusesAnIDThatIsNotV4(t *testing.T) {
	tests := []struct {
		name string
		body dto.CreateSurveyRequest
	}{
		{name: "the survey's ID", body: dto.CreateSurveyRequest{BoardCommandRequest: boardOpBody, ID: boardV7CardID, Title: "Team mood", TemplateKey: "team_mood"}},
		{name: "a question's ID", body: dto.CreateSurveyRequest{BoardCommandRequest: boardOpBody, ID: boardSurveyID, Title: "Pace", Questions: []dto.SurveyQuestionRequest{{ID: boardV7CardID, Kind: model.QuestionScale, Prompt: "Pace?", ScaleMin: new(1), ScaleMax: new(5)}}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sh := newSurveyHarness(t)

			// when
			status, body := sh.record(http.MethodPost, boardPath("/surveys"), tc.body)

			// then the strict mock proves the service was never asked
			assert.Equal(t, http.StatusBadRequest, status)
			assert.JSONEq(t, `{"error":"a board content ID must be a version 4 UUID","code":"invalid_id"}`, body)
		})
	}
}

func TestBoardSurveyController_RefusesAMalformedSurveyID(t *testing.T) {
	// given
	sh := newSurveyHarness(t)

	// when
	status, _ := sh.record(http.MethodPost, boardPath("/surveys/not-a-uuid/close"), boardOpBody)

	// then the strict mock proves the service was never asked
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestBoardSurveyController_MapsEverySurveyRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: surveys.ErrTitleRequired, status: http.StatusBadRequest, body: `{"error":"a survey needs a title","code":"survey_title_required"}`},
		{err: surveys.ErrTitleTooLong, status: http.StatusBadRequest, body: `{"error":"a survey title can be at most 120 characters","code":"survey_title_too_long"}`},
		{err: surveys.ErrQuestionsRequired, status: http.StatusBadRequest, body: `{"error":"a survey needs at least one question","code":"survey_questions_required"}`},
		{err: surveys.ErrTooManyQuestions, status: http.StatusBadRequest, body: `{"error":"a survey can have at most 20 questions","code":"too_many_questions"}`},
		{err: surveys.ErrInvalidQuestionKind, status: http.StatusBadRequest, body: `{"error":"a question must be single choice, multiple choice or a scale","code":"invalid_question_kind"}`},
		{err: surveys.ErrPromptRequired, status: http.StatusBadRequest, body: `{"error":"every question needs a prompt","code":"question_prompt_required"}`},
		{err: surveys.ErrPromptTooLong, status: http.StatusBadRequest, body: `{"error":"a question prompt can be at most 300 characters","code":"question_prompt_too_long"}`},
		{err: surveys.ErrInvalidOptions, status: http.StatusBadRequest, body: `{"error":"a choice question needs 2 to 10 different options of at most 100 characters, and a scale takes none","code":"invalid_options"}`},
		{err: surveys.ErrInvalidScale, status: http.StatusBadRequest, body: `{"error":"a scale runs between two whole numbers from 0 to 10, lowest first, and a choice question takes none","code":"invalid_scale"}`},
		{err: surveys.ErrUnknownTemplate, status: http.StatusBadRequest, body: `{"error":"there is no survey template with that key","code":"unknown_survey_template"}`},
		{err: surveys.ErrTemplateWithQuestions, status: http.StatusBadRequest, body: `{"error":"a survey comes from a template or from its own questions, never both","code":"template_with_questions"}`},
		{err: surveys.ErrTooManySurveys, status: http.StatusConflict, body: `{"error":"a board can hold at most 20 surveys","code":"too_many_surveys"}`},
		{err: board.ErrIDConflict, status: http.StatusConflict, body: `{"error":"that ID is already taken by another command","code":"id_conflict"}`},
		{err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to create the survey"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			sh := newSurveyHarness(t)
			sh.surveys.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("create the survey: %w", tc.err))

			// when
			status, body := sh.record(http.MethodPost, boardPath("/surveys"), boardSurveyBody)

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardSurveyController_MapsEveryAnswerRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: surveys.ErrAnswersRequired, status: http.StatusBadRequest, body: `{"error":"answer at least one question","code":"answers_required"}`},
		{err: surveys.ErrInvalidAnswer, status: http.StatusBadRequest, body: `{"error":"that answer does not fit its question","code":"invalid_answer"}`},
		{err: surveys.ErrUnknownQuestion, status: http.StatusBadRequest, body: `{"error":"each answer must be to a different question of this survey","code":"unknown_question"}`},
		{err: policy.ErrSurveyNotFound, status: http.StatusNotFound, body: `{"error":"survey not found"}`},
		{err: policy.ErrSurveyClosed, status: http.StatusConflict, body: `{"error":"the survey is closed, so it takes no more answers","code":"survey_closed"}`},
		{err: policy.ErrNotParticipant, status: http.StatusConflict, body: `{"error":"join the board before writing on it","code":"not_participant"}`},
		{err: errServiceDown, status: http.StatusInternalServerError, body: `{"error":"failed to answer the survey"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			sh := newSurveyHarness(t)
			sh.surveys.EXPECT().Respond(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("answer the survey: %w", tc.err))

			// when
			status, body := sh.record(http.MethodPut, boardPath(boardSurveyRoute+"/responses"), boardSurveyAnswers)

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardSurveyController_CloseAndDeleteMapAMissingSurveyToNotFound(t *testing.T) {
	// given
	sh := newSurveyHarness(t)
	sh.surveys.EXPECT().Close(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("close: %w", policy.ErrSurveyNotFound))
	sh.surveys.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("delete: %w", policy.ErrSurveyNotFound))

	// when
	closeStatus, closeBody := sh.record(http.MethodPost, boardPath(boardSurveyRoute+"/close"), boardOpBody)
	deleteStatus, deleteBody := sh.record(http.MethodDelete, boardPath(boardSurveyRoute), boardOpBody)

	// then
	assert.Equal(t, http.StatusNotFound, closeStatus)
	assert.JSONEq(t, `{"error":"survey not found"}`, closeBody)
	assert.Equal(t, http.StatusNotFound, deleteStatus)
	assert.JSONEq(t, `{"error":"survey not found"}`, deleteBody)
}

func TestBoardSurveyController_EveryRouteOfAnotherOrganisationsBoardAnswersNotFound(t *testing.T) {
	routes := []crossTenantRoute{
		{method: http.MethodPost, path: tenantBoardBPath + "/surveys", body: dto.CreateSurveyRequest{BoardCommandRequest: tenantOpBody, ID: tenantSurveyB, Title: "Planted", TemplateKey: "team_mood"}},
		{method: http.MethodPost, path: tenantBoardBPath + "/surveys/" + tenantSurveyB.String() + "/close", body: tenantOpBody},
		{method: http.MethodDelete, path: tenantBoardBPath + "/surveys/" + tenantSurveyB.String(), body: tenantOpBody},
		{method: http.MethodPut, path: tenantBoardBPath + "/surveys/" + tenantSurveyB.String() + "/responses", body: dto.SurveyResponseRequest{BoardCommandRequest: tenantOpBody, Answers: []dto.SurveyAnswerRequest{{QuestionID: tenantSurveyB, Answer: []int{1}}}}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat on the board
			x := newCrossTenantHarness(t)
			orgAuthz := orgauthz.NewService(x.orgRepo, x.teamRepo)
			boardLive := live.New(live.Deps{Boards: x.boardRepo, Cards: x.cardRepo, OrgAuthz: orgAuthz})
			surveyRepo := repository.NewMockSurveyRepository(t)
			boardSurveys := surveys.NewService(surveys.Deps{Boards: x.boardRepo, Cards: x.cardRepo, Surveys: surveyRepo, Live: boardLive, Events: boardLive})
			s := &Service{BoardSurveys: boardSurveys, BoardLive: boardLive, Guards: x.Guards}
			RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardSurveyRoutes())
			x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
			x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
			x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
			x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))

			// when
			status, body := x.recordAsAlice(route)

			// then board B is not even shown to exist, and the strict survey mock proves none of its surveys is read or written
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}
