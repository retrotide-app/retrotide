package projection_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"retrotide/internal/board/projection"
	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaResponse_GoldenCarriesOnlyTheServerBuiltPathAndSize(t *testing.T) {
	// given
	media := model.Media{
		ID:          polishImageID,
		OrgID:       orgID,
		BoardID:     boardID,
		Kind:        model.MediaKindCard,
		ContentType: model.MediaContentType,
		Width:       1600,
		Height:      900,
		ByteSize:    48213,
		CreatedAt:   time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}

	// when
	payload := marshal(t, projection.MediaResponse(media))

	// then nothing about the organisation, the bytes or the upload time leaves
	assert.JSONEq(t, `{"id":"0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d","path":"`+polishImagePath+`","width":1600,"height":900}`, payload)
	assert.NotContains(t, payload, orgID.String())
}

func TestMediaPathFormat_BuildsThePathFromTheBoardAndMediaIDs(t *testing.T) {
	// when
	path := fmt.Sprintf(model.MediaPathFormat, boardID, polishImageID)

	// then
	assert.Equal(t, "/uploads/8f0c1a2b-3c4d-4e5f-8a6b-7c8d9e0f1a2b/0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d.webp", path)
}

func TestCardChanged_AHiddenCardsImagePathReachesOnlyItsAuthor(t *testing.T) {
	for _, anonymity := range []model.Anonymity{model.AnonymityNamed, model.AnonymityAnonymous} {
		for _, tc := range viewerCases() {
			t.Run(fmt.Sprintf("%s/%s", anonymity, tc.name), func(t *testing.T) {
				// given Sam's hidden card has an image
				state := polishedState(anonymity, true)
				event := projection.CardChanged(state.Header.Board, secretCard(state), samProfile(state), 1044)

				// when
				payload, seen := seenPayload(t, event, tc.viewer)

				// then
				if !slices.Contains(tc.owns, secretID) {
					assert.False(t, seen)
					assert.NotContains(t, payload, polishImageID.String())

					return
				}

				require.True(t, seen)
				card, ok := decode(t, payload)["card"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, polishImagePath, card["image_path"])
				assert.Equal(t, true, card["pinned"])
			})
		}
	}
}

func TestCardChanged_ARevealedCardsImagePathReachesEveryone(t *testing.T) {
	for _, tc := range viewerCases() {
		t.Run(tc.name, func(t *testing.T) {
			// given
			state := polishedState(model.AnonymityAnonymous, false)
			event := projection.CardChanged(state.Header.Board, secretCard(state), samProfile(state), 1044)

			// when
			payload, seen := seenPayload(t, event, tc.viewer)

			// then the image travels, and the anonymous author still does not
			require.True(t, seen)
			card, ok := decode(t, payload)["card"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, polishImagePath, card["image_path"])
			assert.NotContains(t, payload, samID.String())
		})
	}
}
