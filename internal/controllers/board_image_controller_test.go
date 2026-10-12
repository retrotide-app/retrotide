package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/images"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	boardImageHarness struct {
		*testutil.Harness
		images *images.MockService
	}
)

var (
	boardCardImagePath = boardPath("/cards/" + boardCard.String() + "/image")
	boardBackgroundURL = boardPath("/background")
)

func newBoardImageHarness(t *testing.T) *boardImageHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	ih := &boardImageHarness{Harness: h, images: images.NewMockService(t)}

	s := &Service{BoardImages: ih.images, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardImageRoutes())

	return ih
}

func (ih *boardImageHarness) record(method string, path string, body any) (int, string) {
	cookie := ih.SignIn(boardUser)
	ih.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

	rec := ih.NewRequest(method, path).WithCookie(cookie).WithJSONBody(body).Record()

	return rec.Code, rec.Body.String()
}

func TestBoardImageController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{BoardImages: images.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllBoardImageRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"PUT /api/v1/boards/:boardID/cards/:cardID/image",
		"PUT /api/v1/boards/:boardID/background",
	}, registeredRoutes(h.Engine))
}

func TestBoardImageController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPut, Path: boardCardImagePath, Body: dto.CardImageRequest{BoardCommandRequest: boardOpBody, ImageID: new(boardMedia)}},
		{Method: http.MethodPut, Path: boardBackgroundURL, Body: dto.BoardBackgroundRequest{BoardCommandRequest: boardOpBody, ImageID: new(boardMedia)}},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newBoardImageHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardImageController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   any
		expect func(ih *boardImageHarness)
	}{
		{
			name: "attach a card image",
			path: boardCardImagePath,
			body: dto.CardImageRequest{BoardCommandRequest: boardOpBody, ImageID: new(boardMedia)},
			expect: func(ih *boardImageHarness) {
				ih.images.EXPECT().SetCardImage(mock.Anything, images.CardImageCommand{Command: boardCommandOf, CardID: boardCard, MediaID: new(boardMedia)}).Return(boardVersioned, nil).Once()
			},
		},
		{
			name: "remove a card image",
			path: boardCardImagePath,
			body: dto.CardImageRequest{BoardCommandRequest: boardOpBody},
			expect: func(ih *boardImageHarness) {
				ih.images.EXPECT().SetCardImage(mock.Anything, images.CardImageCommand{Command: boardCommandOf, CardID: boardCard}).Return(boardVersioned, nil).Once()
			},
		},
		{
			name: "set the background",
			path: boardBackgroundURL,
			body: dto.BoardBackgroundRequest{BoardCommandRequest: boardOpBody, ImageID: new(boardMedia)},
			expect: func(ih *boardImageHarness) {
				ih.images.EXPECT().SetBackground(mock.Anything, images.BackgroundCommand{Command: boardCommandOf, MediaID: new(boardMedia)}).Return(boardVersioned, nil).Once()
			},
		},
		{
			name: "remove the background",
			path: boardBackgroundURL,
			body: dto.BoardBackgroundRequest{BoardCommandRequest: boardOpBody},
			expect: func(ih *boardImageHarness) {
				ih.images.EXPECT().SetBackground(mock.Anything, images.BackgroundCommand{Command: boardCommandOf}).Return(boardVersioned, nil).Once()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ih := newBoardImageHarness(t)
			tc.expect(ih)

			// when
			status, body := ih.record(http.MethodPut, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardImageController_ABodyOrPathThatIsNotTheRequestIsRefused(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want string
	}{
		{name: "an image ID that is not a UUID", path: boardCardImagePath, body: `{"op_id":"` + boardOp.String() + `","image_id":"https://example.com/cat.png"}`, want: `{"error":"invalid request body"}`},
		{name: "a background that is not JSON", path: boardBackgroundURL, body: `{"op_id":`, want: `{"error":"invalid request body"}`},
		{name: "a card ID that is not a UUID", path: boardPath("/cards/card-1/image"), body: `{"op_id":"` + boardOp.String() + `"}`, want: `{"error":"invalid cardID"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ih := newBoardImageHarness(t)
			cookie := ih.SignIn(boardUser)
			ih.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

			// when
			rec := ih.NewRequest(http.MethodPut, tc.path).WithCookie(cookie).WithRawBody(tc.body, "application/json").Record()

			// then the strict images mock proves the service is never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, tc.want, rec.Body.String())
		})
	}
}

func TestBoardImageController_MapsEveryImageRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "media not found", err: images.ErrMediaNotFound, status: http.StatusNotFound, body: `{"error":"image not found"}`},
		{name: "wrong kind", err: images.ErrWrongKind, status: http.StatusConflict, body: `{"error":"that image was uploaded for something else, so it cannot be used here","code":"wrong_media_kind"}`},
		{name: "members only", err: policy.ErrMembersOnly, status: http.StatusForbidden, body: `{"error":"only members of the board's organisation can do that","code":"members_only"}`},
		{name: "card not found", err: policy.ErrCardNotFound, status: http.StatusNotFound, body: `{"error":"card not found"}`},
		{name: "not allowed", err: policy.ErrNotAllowed, status: http.StatusForbidden, body: `{"error":"your role on this board does not allow that"}`},
		{name: "wrong stage", err: policy.ErrCardsOutOfStage, status: http.StatusConflict, body: `{"error":"cards can be added, edited, moved and deleted only in the Write stage","code":"wrong_stage"}`},
		{name: "op required", err: board.ErrOpRequired, status: http.StatusBadRequest, body: `{"error":"every board command needs an op_id","code":"op_required"}`},
		{name: "no access", err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ih := newBoardImageHarness(t)
			ih.images.EXPECT().SetCardImage(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("set the image: %w", tc.err)).Once()

			// when
			status, body := ih.record(http.MethodPut, boardCardImagePath, dto.CardImageRequest{BoardCommandRequest: boardOpBody, ImageID: new(boardMedia)})

			// then
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardImageController_TheBackgroundAnswersItsOwnRefusals(t *testing.T) {
	// given
	ih := newBoardImageHarness(t)
	ih.images.EXPECT().SetBackground(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("set the background: %w", images.ErrWrongKind)).Once()

	// when
	status, body := ih.record(http.MethodPut, boardBackgroundURL, dto.BoardBackgroundRequest{BoardCommandRequest: boardOpBody, ImageID: new(boardMedia)})

	// then
	assert.Equal(t, http.StatusConflict, status)
	assert.JSONEq(t, `{"error":"that image was uploaded for something else, so it cannot be used here","code":"wrong_media_kind"}`, body)
}

func TestBoardImageController_ImagesOfAnotherOrganisationsBoardAreNotFound(t *testing.T) {
	cardB := tenantBoardBPath + "/cards/" + tenantCardB.String()

	routes := []crossTenantRoute{
		{method: http.MethodPut, path: cardB + "/image", body: dto.CardImageRequest{BoardCommandRequest: tenantOpBody, ImageID: new(boardMedia)}},
		{method: http.MethodPut, path: cardB + "/image", body: dto.CardImageRequest{BoardCommandRequest: tenantOpBody}},
		{method: http.MethodPut, path: tenantBoardBPath + "/background", body: dto.BoardBackgroundRequest{BoardCommandRequest: tenantOpBody, ImageID: new(boardMedia)}},
		{method: http.MethodPut, path: tenantBoardBPath + "/background", body: dto.BoardBackgroundRequest{BoardCommandRequest: tenantOpBody}},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat
			x := newMediaCrossTenantHarness(t)
			x.boardBelongsToAnotherOrganisation()

			// when
			status, body := x.recordAsAlice(route)

			// then the strict card, board and media mocks prove nothing past the board is read or written
			assert.Equal(t, http.StatusNotFound, status)
			assert.JSONEq(t, `{"error":"board not found"}`, body)
		})
	}
}
