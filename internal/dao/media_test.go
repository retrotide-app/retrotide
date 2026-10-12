package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f boardFixture) upload(t *testing.T, kind model.MediaKind, data []byte) *model.Media {
	t.Helper()

	created, err := f.repos.Media.Create(t.Context(), spec.NewMedia{OrgID: f.orgID(), BoardID: f.boardID(), Kind: kind, Width: 800, Height: 600, Data: data})
	require.NoError(t, err)

	return created
}

func (f boardFixture) uploadedAgo(t *testing.T, mediaID uuid.UUID, ago time.Duration) {
	t.Helper()

	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE media SET created_at = $2 WHERE id = $1`, mediaID, time.Now().Add(-ago))
	require.NoError(t, err)
}

func TestMedia_CreateStoresTheImageAndGetLeavesTheBytesBehind(t *testing.T) {
	// given
	f := newBoardFixture(t)
	data := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")

	// when
	created := f.upload(t, model.MediaKindCard, data)
	got, getErr := f.repos.Media.Get(t.Context(), spec.MediaKey{BoardID: f.boardID(), MediaID: created.ID})
	opened, openErr := f.repos.Media.Open(t.Context(), spec.MediaKey{BoardID: f.boardID(), MediaID: created.ID})

	// then
	assert.Equal(t, model.MediaKindCard, created.Kind)
	assert.Equal(t, model.MediaContentType, created.ContentType)
	assert.Equal(t, len(data), created.ByteSize)
	assert.Equal(t, 800, created.Width)
	assert.Equal(t, 600, created.Height)
	require.NoError(t, getErr)
	assert.Equal(t, created, got)
	require.NoError(t, openErr)
	assert.Equal(t, *created, opened.Media)
	assert.Equal(t, data, opened.Data)
}

func TestMedia_AnUnknownImageIsNotFound(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	_, getErr := f.repos.Media.Get(t.Context(), spec.MediaKey{BoardID: f.boardID(), MediaID: uuid.New()})
	_, openErr := f.repos.Media.Open(t.Context(), spec.MediaKey{BoardID: f.boardID(), MediaID: uuid.New()})

	// then
	require.ErrorIs(t, getErr, dao.ErrNotFound)
	require.ErrorIs(t, openErr, dao.ErrNotFound)
}

func TestMedia_TheSchemaRefusesAnUnknownKindAndOversizedDimensions(t *testing.T) {
	tests := []struct {
		name       string
		media      func(f boardFixture) spec.NewMedia
		constraint string
	}{
		{
			name: "an unknown kind",
			media: func(f boardFixture) spec.NewMedia {
				return spec.NewMedia{OrgID: f.orgID(), BoardID: f.boardID(), Kind: "avatar", Width: 1, Height: 1, Data: []byte("x")}
			},
			constraint: "media_kind_check",
		},
		{
			name: "an edge beyond 4096",
			media: func(f boardFixture) spec.NewMedia {
				return spec.NewMedia{OrgID: f.orgID(), BoardID: f.boardID(), Kind: model.MediaKindCard, Width: 4097, Height: 1, Data: []byte("x")}
			},
			constraint: "media_dimensions_check",
		},
		{
			name: "no bytes at all",
			media: func(f boardFixture) spec.NewMedia {
				return spec.NewMedia{OrgID: f.orgID(), BoardID: f.boardID(), Kind: model.MediaKindCard, Width: 1, Height: 1, Data: []byte{}}
			},
			constraint: "media_byte_size_check",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newBoardFixture(t)

			// when
			_, err := f.repos.Media.Create(t.Context(), tc.media(f))

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func TestMedia_CardImagesAttachAndDeletingTheImageDetachesIt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	media := f.upload(t, model.MediaKindCard, []byte("webp"))

	// when
	attached, attachErr := f.repos.Card.SetImage(t.Context(), spec.CardImageChange{BoardID: f.boardID(), CardID: card.ID, MediaID: new(media.ID)})
	_, err := f.repos.DB().ExecContext(t.Context(), `DELETE FROM media WHERE id = $1`, media.ID)
	require.NoError(t, err)
	after, getErr := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})

	// then
	require.NoError(t, attachErr)
	assert.Equal(t, new(media.ID), attached.ImageMediaID)
	require.NoError(t, getErr)
	assert.Nil(t, after.ImageMediaID)
}

func TestMedia_PurgeUnattachedBeforeKeepsWhatACardOrTheBoardUses(t *testing.T) {
	// given four old images: one on a card, one as the background, two unused; and one fresh unused image
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	onCard := f.upload(t, model.MediaKindCard, []byte("card"))
	onBoard := f.upload(t, model.MediaKindBackground, []byte("background"))
	unused := f.upload(t, model.MediaKindCard, []byte("unused"))
	replaced := f.upload(t, model.MediaKindBackground, []byte("replaced"))
	fresh := f.upload(t, model.MediaKindCard, []byte("fresh"))
	for _, old := range []uuid.UUID{onCard.ID, onBoard.ID, unused.ID, replaced.ID} {
		f.uploadedAgo(t, old, model.MediaUnattachedLimit+time.Hour)
	}
	_, err := f.repos.Card.SetImage(t.Context(), spec.CardImageChange{BoardID: f.boardID(), CardID: card.ID, MediaID: new(onCard.ID)})
	require.NoError(t, err)
	_, err = f.repos.Board.SetBackground(t.Context(), spec.BoardBackgroundChange{OrgID: f.orgID(), BoardID: f.boardID(), MediaID: new(onBoard.ID)})
	require.NoError(t, err)

	// when
	purged, err := f.repos.Media.PurgeUnattachedBefore(t.Context(), time.Now().Add(-model.MediaUnattachedLimit))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), purged)
	for _, kept := range []uuid.UUID{onCard.ID, onBoard.ID, fresh.ID} {
		_, getErr := f.repos.Media.Get(t.Context(), spec.MediaKey{BoardID: f.boardID(), MediaID: kept})
		require.NoError(t, getErr)
	}
	for _, gone := range []uuid.UUID{unused.ID, replaced.ID} {
		_, getErr := f.repos.Media.Get(t.Context(), spec.MediaKey{BoardID: f.boardID(), MediaID: gone})
		require.ErrorIs(t, getErr, dao.ErrNotFound)
	}
}

func TestCard_SetPinnedKeepsTheFirstPinTimeAndUnpinClearsIt(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	pin := spec.CardPinChange{BoardID: f.boardID(), CardID: card.ID, Pinned: true}

	// when
	first, firstErr := f.repos.Card.SetPinned(t.Context(), pin)
	second, secondErr := f.repos.Card.SetPinned(t.Context(), pin)
	unpinned, unpinErr := f.repos.Card.SetPinned(t.Context(), spec.CardPinChange{BoardID: f.boardID(), CardID: card.ID})
	_, missingErr := f.repos.Card.SetPinned(t.Context(), spec.CardPinChange{BoardID: f.boardID(), CardID: uuid.New(), Pinned: true})

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, unpinErr)
	require.NotNil(t, first.PinnedAt)
	assert.Equal(t, first.PinnedAt, second.PinnedAt)
	assert.Nil(t, unpinned.PinnedAt)
	require.ErrorIs(t, missingErr, dao.ErrNotFound)
}
