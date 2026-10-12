package images

import (
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func backgroundCommand(media *uuid.UUID) BackgroundCommand {
	return BackgroundCommand{Command: baseCommand(), MediaID: media}
}

func decorated(media *uuid.UUID) model.Board {
	board := boardAccess(host).Header.Board
	board.Version = 8
	board.BackgroundMediaID = media

	return board
}

func TestSetBackground_SetsTheBoardBackgroundForEveryone(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(uploaded(model.MediaKindBackground), nil).Once()
	m.boards.EXPECT().SetBoardBackground(mock.Anything, spec.BoardBackgroundWrite{BoardWrite: target(), MediaID: new(mediaID)}).
		Return(&model.Versioned[model.Board]{Value: decorated(new(mediaID)), Version: 8}, nil).Once()

	// when
	got, err := svc.SetBackground(t.Context(), backgroundCommand(new(mediaID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8}, got)
	require.Len(t, m.events.published, 1)
	published := m.events.published[0]
	assert.Equal(t, new(mediaID), published.access.Header.BackgroundMediaID)
	assert.Equal(t, projection.ClassOthers, published.event.ClassOf(projection.Viewer{ParticipantID: new(theirID), Role: writer}))
	assert.JSONEq(t, `{"type":"board_background_changed","version":8,"background_path":"`+imagePath()+`"}`, payloadJSON(t, published.event.Payload(projection.ClassOthers)))
}

func TestSetBackground_RemovesTheBackground(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.boards.EXPECT().SetBoardBackground(mock.Anything, spec.BoardBackgroundWrite{BoardWrite: target()}).
		Return(&model.Versioned[model.Board]{Value: decorated(nil), Version: 8}, nil).Once()

	// when
	_, err := svc.SetBackground(t.Context(), backgroundCommand(nil))

	// then
	require.NoError(t, err)
	require.Len(t, m.events.published, 1)
	assert.JSONEq(t, `{"type":"board_background_changed","version":8,"background_path":null}`, payloadJSON(t, m.events.published[0].event.Payload(projection.ClassOthers)))
}

func TestSetBackground_RefusesACardImageUsedAsABackground(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(uploaded(model.MediaKindCard), nil).Once()

	// when
	_, err := svc.SetBackground(t.Context(), backgroundCommand(new(mediaID)))

	// then the strict board mock proves nothing is written
	assert.ErrorIs(t, err, ErrWrongKind)
	assert.Empty(t, m.events.published)
}

func TestSetBackground_RefusesAnImageOfAnotherBoard(t *testing.T) {
	// given an image ID that exists only on another board
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.media.EXPECT().Get(mock.Anything, spec.MediaKey{BoardID: boardID, MediaID: mediaID}).Return(nil, errMissing).Once()

	// when
	_, err := svc.SetBackground(t.Context(), backgroundCommand(new(mediaID)))

	// then
	assert.ErrorIs(t, err, ErrMediaNotFound)
	assert.Empty(t, m.events.published)
}

func TestSetBackground_RefusesEveryoneWhoCannotManageTheBoard(t *testing.T) {
	tests := []struct {
		name   string
		access *model.BoardAccess
		want   error
	}{
		{name: "a participant", access: boardAccess(writer), want: policy.ErrNotAllowed},
		{name: "a guest", access: boardAccess(writer, asGuest), want: policy.ErrNotAllowed},
		{name: "a facilitator of an ended board", access: boardAccess(host, func(access *model.BoardAccess) {
			access.Header.ClosedAt = new(stamp)
			access.Header.Stage = model.StageEnd
		}), want: policy.ErrReadOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(tc.access)

			// when
			_, err := svc.SetBackground(t.Context(), backgroundCommand(new(mediaID)))

			// then no media is looked up and nothing is written
			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestSetBackground_ARepeatedOpAnswersWithItsVersionAndWritesNothing(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(host), 8)

	// when
	got, err := svc.SetBackground(t.Context(), backgroundCommand(new(mediaID)))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.Empty(t, m.events.published)
}
