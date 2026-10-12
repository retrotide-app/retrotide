package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/entitlements"
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
		media        *repository.MockMediaRepository
		live         *live.MockService
		entitlements *entitlements.MockService
		encoder      *MockEncoder
	}

	accessOption func(*model.BoardAccess)
)

const (
	freeAllowance = 5242880
)

var (
	orgID    = uuid.MustParse("6e7f8091-0000-4000-8000-000000000001")
	boardID  = uuid.MustParse("6e7f8091-0000-4000-8000-000000000002")
	userID   = uuid.MustParse("6e7f8091-0000-4000-8000-000000000003")
	seatID   = uuid.MustParse("6e7f8091-0000-4000-8000-000000000004")
	mediaID  = uuid.MustParse("6e7f8091-0000-4000-8000-000000000005")
	deviceID = uuid.MustParse("6e7f8091-0000-4000-8000-000000000006")
	stamp    = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

	me       = model.Principal{Kind: model.PrincipalUser, ID: userID}
	freePlan = &entitlements.Plan{Code: model.PlanFree, Limits: map[model.Limit]int{model.LimitCardImageBytes: freeAllowance}}
	encoded  = []byte("RIFF-encoded-webp")
)

func newTestService(t *testing.T) (*service, *testMocks) {
	t.Helper()

	m := &testMocks{
		media:        repository.NewMockMediaRepository(t),
		live:         live.NewMockService(t),
		entitlements: entitlements.NewMockService(t),
		encoder:      NewMockEncoder(t),
	}

	svc := NewService(Deps{
		Media:        m.media,
		Live:         m.live,
		Entitlements: m.entitlements,
		Encoder:      m.encoder,
		Now: func() time.Time {
			return stamp
		},
	}).(*service)

	return svc, m
}

func boardAccess(role model.BoardRole, options ...accessOption) *model.BoardAccess {
	access := &model.BoardAccess{
		Principal: me,
		Header: model.BoardHeader{Board: model.Board{
			ID:        boardID,
			OrgID:     orgID,
			Title:     "Sprint 12",
			Version:   7,
			Anonymity: model.AnonymityNamed,
			Stage:     model.StageWrite,
		}},
		ParticipantID: new(seatID),
		Role:          role,
		OrgMember:     true,
		Capabilities:  policy.Capabilities(role, true),
	}

	for _, apply := range options {
		apply(access)
	}

	return access
}

func asGuest(access *model.BoardAccess) {
	access.Principal = model.Principal{Kind: model.PrincipalGuest, ID: deviceID}
	access.OrgMember = false
	access.Capabilities = policy.Capabilities(access.Role, false)
}

func outsider(access *model.BoardAccess) {
	access.OrgMember = false
	access.Capabilities = policy.Capabilities(access.Role, false)
}

func notJoined(access *model.BoardAccess) {
	access.ParticipantID = nil
}

func pngImage(t *testing.T, width, height int) []byte {
	t.Helper()

	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))))

	return buffer.Bytes()
}

func uploadOf(kind model.MediaKind, data []byte) UploadCommand {
	return UploadCommand{Principal: me, BoardID: boardID, Kind: kind, ContentLength: int64(len(data)), Body: bytes.NewReader(data)}
}

func TestUpload_ReEncodesAPNGAndStoresItForTheBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()
	upload := pngImage(t, 10, 8)
	m.encoder.EXPECT().Encode(mock.Anything, upload, model.CardImageMaxEdge).Return(&Encoded{Data: encoded, Width: 10, Height: 8}, nil).Once()
	m.media.EXPECT().Create(mock.Anything, spec.NewMedia{OrgID: orgID, BoardID: boardID, Kind: model.MediaKindCard, Width: 10, Height: 8, Data: encoded}).
		Return(&model.Media{ID: mediaID, OrgID: orgID, BoardID: boardID, Kind: model.MediaKindCard, ContentType: model.MediaContentType, Width: 10, Height: 8, ByteSize: len(encoded), CreatedAt: stamp}, nil).Once()

	// when
	got, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, upload))

	// then the view carries only the server-built path, never anything the client sent
	require.NoError(t, err)
	assert.Equal(t, &dto.MediaView{ID: mediaID, Path: "/uploads/" + boardID.String() + "/" + mediaID.String() + ".webp", Width: 10, Height: 8}, got)
}

func TestUpload_CapsABackgroundAtItsOwnLongerEdge(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleFacilitator), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()
	upload := extendedWebP(5000, 2500)
	m.encoder.EXPECT().Encode(mock.Anything, upload, model.BackgroundMaxEdge).Return(&Encoded{Data: encoded, Width: 2560, Height: 1280}, nil).Once()
	m.media.EXPECT().Create(mock.Anything, spec.NewMedia{OrgID: orgID, BoardID: boardID, Kind: model.MediaKindBackground, Width: 2560, Height: 1280, Data: encoded}).
		Return(&model.Media{ID: mediaID, OrgID: orgID, BoardID: boardID, Kind: model.MediaKindBackground, Width: 2560, Height: 1280}, nil).Once()

	// when
	got, err := svc.Upload(t.Context(), uploadOf(model.MediaKindBackground, upload))

	// then
	require.NoError(t, err)
	assert.Equal(t, 2560, got.Width)
}

func TestUpload_RefusesAKindThatIsNeitherACardNorABackground(t *testing.T) {
	// given
	svc, _ := newTestService(t)

	// when
	_, err := svc.Upload(t.Context(), uploadOf("avatar", pngImage(t, 4, 4)))

	// then nothing is resolved, read or stored
	assert.ErrorIs(t, err, ErrInvalidKind)
}

func TestUpload_RefusesAnUploadDeclaredOverThePlanBeforeReadingIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()
	upload := UploadCommand{Principal: me, BoardID: boardID, Kind: model.MediaKindCard, ContentLength: freeAllowance + 1, Body: strings.NewReader("never read")}

	// when
	_, err := svc.Upload(t.Context(), upload)

	// then
	denied, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, entitlements.DeniedError{Plan: model.PlanFree, Limit: model.LimitCardImageBytes, Allowed: freeAllowance, Current: freeAllowance + 1}, *denied)
	assert.ErrorIs(t, err, entitlements.ErrUpgradeRequired)
}

func TestUpload_RefusesABodyOfUnknownLengthOnceItRunsPastThePlan(t *testing.T) {
	// given a chunked upload that never declared its length
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(&entitlements.Plan{Code: model.PlanFree, Limits: map[model.Limit]int{model.LimitCardImageBytes: 64}}, nil).Once()
	upload := UploadCommand{Principal: me, BoardID: boardID, Kind: model.MediaKindCard, ContentLength: -1, Body: io.MultiReader(strings.NewReader(strings.Repeat("x", 1000)))}

	// when
	_, err := svc.Upload(t.Context(), upload)

	// then no more than the allowance and one byte is ever read
	denied, ok := errors.AsType[*entitlements.DeniedError](err)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, 64, denied.Allowed)
	assert.Equal(t, 65, denied.Current)
}

func TestUpload_AMissingLimitIsAnErrorRatherThanADefault(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(&entitlements.Plan{Code: model.PlanFree, Limits: map[model.Limit]int{}}, nil).Once()

	// when
	_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, pngImage(t, 4, 4)))

	// then
	assert.ErrorIs(t, err, entitlements.ErrUnknownLimit)
}

func TestUpload_RefusesEveryFileThatIsNotAPNGAJPEGOrAWebP(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "a GIF", data: []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")},
		{name: "an SVG", data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`)},
		{name: "a PDF", data: []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")},
		{name: "a PNG signature with a broken header", data: []byte("\x89PNG\r\n\x1a\nbroken")},
		{name: "a WebP claiming no width", data: lossyWebP(0, 240)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
			m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()

			// when
			_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, tc.data))

			// then the strict encoder mock proves nothing reached cwebp
			assert.ErrorIs(t, err, ErrUnsupportedImage)
		})
	}
}

func TestUpload_RefusesAnImageOverFortyMegapixelsBeforeEncodingIt(t *testing.T) {
	// given a small file whose header claims 8000 by 8000 pixels
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()

	// when
	_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, extendedWebP(8000, 8000)))

	// then
	assert.ErrorIs(t, err, ErrImageDimensions)
}

func TestUpload_AcceptsAnImageOfExactlyFortyMegapixels(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()
	upload := extendedWebP(8000, 5000)
	m.encoder.EXPECT().Encode(mock.Anything, upload, model.CardImageMaxEdge).Return(&Encoded{Data: encoded, Width: 1600, Height: 1000}, nil).Once()
	m.media.EXPECT().Create(mock.Anything, mock.Anything).Return(&model.Media{ID: mediaID, BoardID: boardID, Width: 1600, Height: 1000}, nil).Once()

	// when
	_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, upload))

	// then
	assert.NoError(t, err)
}

func TestUpload_RefusesAnEncodedImageTheDatabaseCouldNotHold(t *testing.T) {
	// given a sliver so thin that shrinking it leaves no width at all
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()
	upload := extendedWebP(2, 10000)
	m.encoder.EXPECT().Encode(mock.Anything, upload, model.CardImageMaxEdge).Return(&Encoded{Data: encoded, Width: 0, Height: 1600}, nil).Once()

	// when
	_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, upload))

	// then
	assert.ErrorIs(t, err, ErrImageDimensions)
}

func TestUpload_PassesAMissingEncoderOnWithItsCause(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.entitlements.EXPECT().For(mock.Anything, orgID).Return(freePlan, nil).Once()
	upload := pngImage(t, 4, 4)
	cause := errors.New(`exec: "cwebp": executable file not found in $PATH`)
	m.encoder.EXPECT().Encode(mock.Anything, upload, model.CardImageMaxEdge).Return(nil, fmt.Errorf("%w: find cwebp: %w", ErrEncoderUnavailable, cause)).Once()

	// when
	_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, upload))

	// then
	assert.ErrorIs(t, err, ErrEncoderUnavailable)
	assert.ErrorIs(t, err, cause)
}

func TestUpload_RefusesEveryoneWhoMayNotUpload(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "a guest", access: boardAccess(model.BoardRoleParticipant, asGuest), want: policy.ErrMembersOnly},
		{name: "an account from outside the organisation", access: boardAccess(model.BoardRoleParticipant, outsider), want: policy.ErrMembersOnly},
		{name: "a member who has not joined", access: boardAccess(model.BoardRoleParticipant, notJoined), want: policy.ErrNotParticipant},
		{name: "a viewer of an ended board", access: boardAccess(model.BoardRoleParticipant, func(access *model.BoardAccess) {
			access.Header.ClosedAt = new(stamp)
			access.Header.Stage = model.StageEnd
		}), want: policy.ErrReadOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(tc.access, nil).Once()

			// when
			_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, pngImage(t, 4, 4)))

			// then the plan is never read and nothing is stored
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestUpload_ABoardOfAnotherOrganisationIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(nil, fmt.Errorf("resolve: %w", live.ErrNoAccess)).Once()

	// when
	_, err := svc.Upload(t.Context(), uploadOf(model.MediaKindCard, pngImage(t, 4, 4)))

	// then
	assert.ErrorIs(t, err, live.ErrNoAccess)
}

func TestOpen_ReadsTheImageOfTheResolvedBoard(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant, asGuest), nil).Once()
	file := &model.MediaFile{Media: model.Media{ID: mediaID, BoardID: boardID}, Data: encoded}
	m.media.EXPECT().Open(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(file, nil).Once()

	// when
	got, err := svc.Open(t.Context(), OpenQuery{Principal: me, BoardID: boardID, MediaID: mediaID})

	// then a guest seated on the board can see its images
	require.NoError(t, err)
	assert.Equal(t, file, got)
}

func TestOpen_AnImageOfAnotherBoardIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(boardAccess(model.BoardRoleParticipant), nil).Once()
	m.media.EXPECT().Open(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(nil, fmt.Errorf("open: %w", dao.ErrNotFound)).Once()

	// when
	_, err := svc.Open(t.Context(), OpenQuery{Principal: me, BoardID: boardID, MediaID: mediaID})

	// then
	assert.ErrorIs(t, err, ErrMediaNotFound)
}

func TestOpen_ABoardTheViewerCannotReachIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.live.EXPECT().Resolve(mock.Anything, spec.BoardAccessQuery{Principal: me, BoardID: boardID}).Return(nil, fmt.Errorf("resolve: %w", live.ErrNoAccess)).Once()

	// when
	_, err := svc.Open(t.Context(), OpenQuery{Principal: me, BoardID: boardID, MediaID: mediaID})

	// then the strict media mock proves no image is read
	assert.ErrorIs(t, err, live.ErrNoAccess)
}

func TestPurgeUnattached_PurgesImagesUnusedForADay(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.media.EXPECT().PurgeUnattachedBefore(mock.Anything, stamp.Add(-24*time.Hour)).Return(3, nil).Once()

	// when
	purged, err := svc.PurgeUnattached(t.Context())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(3), purged)
}
