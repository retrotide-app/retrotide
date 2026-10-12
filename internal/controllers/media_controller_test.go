package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"retrotide/internal/board/images"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
	"retrotide/internal/media"
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
	mediaHarness struct {
		*testutil.Harness
		media *media.MockService
	}

	mediaCrossTenantHarness struct {
		*crossTenantHarness
		mediaRepo *repository.MockMediaRepository
	}
)

var (
	boardMedia     = uuid.MustParse("5d6e7f80-91a2-43b4-85c6-3b4c5d6e7f81")
	mediaPNG       = []byte("\x89PNG\r\n\x1a\n-not-really-decoded-here")
	mediaWebP      = []byte("RIFF-encoded-webp")
	mediaUploaded  = &dto.MediaView{ID: boardMedia, Path: "/uploads/" + boardID.String() + "/" + boardMedia.String() + ".webp", Width: 640, Height: 480}
	mediaUploadURL = boardPath("/media?kind=card")
)

func newMediaHarness(t *testing.T) *mediaHarness {
	t.Helper()

	h := testutil.NewHarness(t)
	h.UseBaseURL(testutil.TestBaseURL)

	mh := &mediaHarness{Harness: h, media: media.NewMockService(t)}

	s := &Service{MediaService: mh.media, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllMediaRoutes())
	RegisterRoutes(h.Engine, s.getAllUploadRoutes())

	return mh
}

func (mh *mediaHarness) upload(path string, contentType string, body []byte) *httptest.ResponseRecorder {
	cookie := mh.SignIn(boardUser)
	mh.Authz.EXPECT().IsLocked(mock.Anything, boardUser).Return(false, nil)

	return mh.NewRequest(http.MethodPost, path).WithCookie(cookie).WithRawBody(string(body), contentType).Record()
}

func (mh *mediaHarness) read(path string) *httptest.ResponseRecorder {
	cookie := mh.SignIn(boardUser)

	return mh.NewRequest(http.MethodGet, path).WithCookie(cookie).Record()
}

func uploadPath(board uuid.UUID, file string) string {
	return "/uploads/" + board.String() + "/" + file
}

func newMediaCrossTenantHarness(t *testing.T) *mediaCrossTenantHarness {
	t.Helper()

	x := &mediaCrossTenantHarness{crossTenantHarness: newCrossTenantHarness(t), mediaRepo: repository.NewMockMediaRepository(t)}

	boardLive := live.New(live.Deps{Boards: x.boardRepo, Cards: x.cardRepo, OrgAuthz: orgauthz.NewService(x.orgRepo, x.teamRepo)})

	s := &Service{
		MediaService: media.NewService(media.Deps{Media: x.mediaRepo, Live: boardLive, Entitlements: entitlements.NewMockService(t), Encoder: media.NewMockEncoder(t)}),
		BoardImages:  images.NewService(images.Deps{Boards: x.boardRepo, Cards: x.cardRepo, Media: x.mediaRepo, Live: boardLive, Events: boardLive}),
		Guards:       x.Guards,
	}
	RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllMediaRoutes())
	RegisterRoutes(x.Engine.Group("/api/v1"), s.getAllBoardImageRoutes())
	RegisterRoutes(x.Engine, s.getAllUploadRoutes())

	return x
}

func (x *mediaCrossTenantHarness) boardBelongsToAnotherOrganisation() {
	x.boardRepo.EXPECT().GetHeader(mock.Anything, tenantBoardB).Return(tenantBoardBOpen, nil)
	x.teamRepo.EXPECT().GetHeader(mock.Anything, tenantTeamB).Return(&model.Team{ID: tenantTeamB, OrgID: tenantOrgB, Name: teamName}, nil)
	x.orgRepo.EXPECT().GetMember(mock.Anything, tenantAliceInB).Return(nil, errTenantNoMember)
	x.boardRepo.EXPECT().GetParticipantForUser(mock.Anything, spec.BoardParticipantUser{BoardID: tenantBoardB, UserID: tenantAlice}).Return(nil, fmt.Errorf("get participant: %w", dao.ErrNotFound))
}

func TestMediaController_RouteTable(t *testing.T) {
	// given
	h := testutil.NewHarness(t)
	s := &Service{MediaService: media.NewMockService(t), Guards: h.Guards}

	// when
	RegisterRoutes(h.Engine.Group("/api/v1"), s.getAllMediaRoutes())
	RegisterRoutes(h.Engine, s.getAllUploadRoutes())

	// then
	assert.ElementsMatch(t, []string{
		"POST /api/v1/boards/:boardID/media",
		"GET /uploads/:boardID/:file",
		"HEAD /uploads/:boardID/:file",
	}, registeredRoutes(h.Engine))
}

func TestMediaController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: mediaUploadURL},
		{Method: http.MethodGet, Path: uploadPath(boardID, boardMedia.String()+".webp")},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newMediaHarness(t).Harness
			}, route)
		})
	}
}

func TestMediaController_HandsTheRawBodyAndItsKindToTheService(t *testing.T) {
	tests := []struct {
		name        string
		kind        model.MediaKind
		contentType string
	}{
		{name: "a PNG card image", kind: model.MediaKindCard, contentType: "image/png"},
		{name: "a JPEG background", kind: model.MediaKindBackground, contentType: "image/jpeg"},
		{name: "a WebP card image with a parameter", kind: model.MediaKindCard, contentType: "image/webp; charset=binary"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newMediaHarness(t)
			var received []byte
			var command media.UploadCommand
			mh.media.EXPECT().Upload(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, upload media.UploadCommand) (*dto.MediaView, error) {
				command = upload
				data, err := io.ReadAll(upload.Body)
				require.NoError(t, err)
				received = data

				return mediaUploaded, nil
			}).Once()

			// when
			rec := mh.upload(boardPath("/media?kind="+string(tc.kind)), tc.contentType, mediaPNG)

			// then
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.JSONEq(t, `{"id":"`+boardMedia.String()+`","path":"/uploads/`+boardID.String()+`/`+boardMedia.String()+`.webp","width":640,"height":480}`, rec.Body.String())
			assert.Equal(t, boardPrincipal, command.Principal)
			assert.Equal(t, boardID, command.BoardID)
			assert.Equal(t, tc.kind, command.Kind)
			assert.Equal(t, int64(len(mediaPNG)), command.ContentLength)
			assert.Equal(t, mediaPNG, received)
		})
	}
}

func TestMediaController_RefusesABodyThatDoesNotClaimToBeAnImage(t *testing.T) {
	tests := []string{"image/gif", "application/json", "image/svg+xml", ""}

	for _, contentType := range tests {
		t.Run(contentType, func(t *testing.T) {
			// given
			mh := newMediaHarness(t)

			// when
			rec := mh.upload(mediaUploadURL, contentType, mediaPNG)

			// then the strict media mock proves the service is never asked
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"error":"only PNG, JPEG and WebP images can be uploaded","code":"unsupported_image"}`, rec.Body.String())
		})
	}
}

func TestMediaController_MapsEveryUploadRefusalToItsStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "unsupported", err: media.ErrUnsupportedImage, status: http.StatusBadRequest, body: `{"error":"only PNG, JPEG and WebP images can be uploaded","code":"unsupported_image"}`},
		{name: "kind", err: media.ErrInvalidKind, status: http.StatusBadRequest, body: `{"error":"an image is either a card image or a board background","code":"invalid_media_kind"}`},
		{name: "dimensions", err: media.ErrImageDimensions, status: http.StatusBadRequest, body: `{"error":"the image is too large: it can have at most 40 megapixels","code":"image_dimensions"}`},
		{name: "over the plan", err: &entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitCardImageBytes, Allowed: 5242880, Current: 6000000}, status: http.StatusPaymentRequired, body: `{"error":"upgrade required","code":"upgrade_required","limit":"card_image_bytes","allowed":5242880,"current":6000000,"plan":"free"}`},
		{name: "members only", err: policy.ErrMembersOnly, status: http.StatusForbidden, body: `{"error":"only members of the board's organisation can do that","code":"members_only"}`},
		{name: "not seated", err: policy.ErrNotParticipant, status: http.StatusConflict, body: `{"error":"join the board before writing on it","code":"not_participant"}`},
		{name: "read only", err: policy.ErrReadOnly, status: http.StatusConflict, body: `{"error":"the board is read-only","code":"read_only"}`},
		{name: "no access", err: live.ErrNoAccess, status: http.StatusNotFound, body: `{"error":"board not found"}`},
		{name: "encoder missing", err: media.ErrEncoderUnavailable, status: http.StatusServiceUnavailable, body: `{"error":"images cannot be processed right now, so try again later","code":"images_unavailable"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newMediaHarness(t)
			mh.media.EXPECT().Upload(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("upload the image: %w", tc.err)).Once()

			// when
			rec := mh.upload(mediaUploadURL, "image/png", mediaPNG)

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestMediaController_ServesAnImageOfTheBoardAsWebP(t *testing.T) {
	// given
	mh := newMediaHarness(t)
	mh.media.EXPECT().Open(mock.Anything, media.OpenQuery{Principal: boardPrincipal, BoardID: boardID, MediaID: boardMedia}).
		Return(&model.MediaFile{Media: model.Media{ID: boardMedia, BoardID: boardID, Kind: model.MediaKindCard}, Data: mediaWebP}, nil).Once()

	// when
	rec := mh.read(uploadPath(boardID, boardMedia.String()+".webp"))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/webp", rec.Header().Get("Content-Type"))
	assert.Equal(t, mediaWebP, rec.Body.Bytes())
}

func TestMediaController_AnUploadPathThatNamesNoImageIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "another extension", path: uploadPath(boardID, boardMedia.String()+".png")},
		{name: "no extension", path: uploadPath(boardID, boardMedia.String())},
		{name: "not a media ID", path: uploadPath(boardID, "avatar.webp")},
		{name: "not a board ID", path: "/uploads/board-1/" + boardMedia.String() + ".webp"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newMediaHarness(t)

			// when
			rec := mh.read(tc.path)

			// then the strict media mock proves nothing is read
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"error":"image not found"}`, rec.Body.String())
		})
	}
}

func TestMediaController_AnImageTheViewerCannotReachIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		body string
	}{
		{name: "a board the principal has no access to", err: live.ErrNoAccess, body: `{"error":"board not found"}`},
		{name: "an image of another board", err: media.ErrMediaNotFound, body: `{"error":"image not found"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mh := newMediaHarness(t)
			mh.media.EXPECT().Open(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("open the image: %w", tc.err)).Once()

			// when
			rec := mh.read(uploadPath(boardID, boardMedia.String()+".webp"))

			// then
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestMediaController_UploadsAndImagesOfAnotherOrganisationsBoardAreNotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{name: "an upload", method: http.MethodPost, path: tenantBoardBPath + "/media?kind=card", body: mediaPNG},
		{name: "an image read", method: http.MethodGet, path: uploadPath(tenantBoardB, boardMedia.String()+".webp")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given board B belongs to a team of organisation B, where Alice holds nothing and has no seat
			x := newMediaCrossTenantHarness(t)
			x.boardBelongsToAnotherOrganisation()
			cookie := x.SignIn(tenantAlice)
			request := x.NewRequest(tc.method, tc.path).WithCookie(cookie).WithRemoteAddr(publicPeer)
			if tc.method != http.MethodGet {
				x.Authz.EXPECT().IsLocked(mock.Anything, tenantAlice).Return(false, nil)
				request = request.WithRawBody(string(tc.body), "image/png")
			}

			// when
			rec := request.Record()

			// then the strict media and entitlement mocks prove nothing is read, planned or stored
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"error":"board not found"}`, rec.Body.String())
		})
	}
}
