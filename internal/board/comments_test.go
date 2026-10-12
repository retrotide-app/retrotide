package board

import (
	"fmt"
	"strings"
	"testing"

	"retrotide/internal/board/policy"
	"retrotide/internal/board/projection"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	commentID = uuid.MustParse("1a2b3c4d-0000-4000-8000-00000000000b")
)

func commentsOn(access *model.BoardAccess) {
	access.Header.Stage = model.StageDiscuss
	access.Header.CommentsEnabled = true
}

func comment(author uuid.UUID) model.CardComment {
	return model.CardComment{ID: commentID, BoardID: boardID, CardID: cardID, AuthorParticipantID: new(author), Body: "Agreed", CreatedAt: stamp}
}

func addCommentCommand() AddCommentCommand {
	return AddCommentCommand{Command: baseCommand(), CardID: cardID, CommentID: commentID, Body: "  Agreed \n"}
}

func (m *testMocks) storedComment(stored model.CardComment) {
	key := spec.CardCommentRef{BoardID: boardID, CommentID: stored.ID}

	m.comments.EXPECT().GetByID(mock.Anything, key).Return(&stored, nil).Once()
}

func TestAddComment_AddsTheCommentAsItsWriterAndAnnouncesIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn))
	m.stored(card(theirID))
	added := comment(myID)
	m.comments.EXPECT().AddComment(mock.Anything, spec.CommentCreate{
		BoardWrite:          target(),
		CommentID:           commentID,
		CardID:              cardID,
		AuthorParticipantID: new(myID),
		Body:                "Agreed",
	}).Return(&model.Versioned[model.CardComment]{Value: added, Version: 8}, nil)
	m.participants()

	// when
	got, err := svc.AddComment(t.Context(), addCommentCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	require.NotNil(t, got.Comment)
	assert.Equal(t, "Agreed", got.Comment.Body)
	assert.True(t, got.Comment.IsMine)
	assert.Equal(t, &dto.AuthorView{ParticipantID: myID, DisplayName: "Sam"}, got.Comment.Author)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, projection.ClassOthers, m.events.published[0].event.ClassOf(projection.Viewer{ParticipantID: new(theirID)}))
}

func TestAddComment_OnAnAnonymousBoardNamesNobody(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn, anonymous))
	m.stored(card(theirID))
	m.comments.EXPECT().AddComment(mock.Anything, mock.Anything).Return(&model.Versioned[model.CardComment]{Value: comment(myID), Version: 8}, nil)

	// when
	got, err := svc.AddComment(t.Context(), addCommentCommand())

	// then the strict board mock proves no participant was looked up
	require.NoError(t, err)
	require.NotNil(t, got.Comment)
	assert.Nil(t, got.Comment.Author)
	assert.Empty(t, got.Comment.CreatedAt)
	assert.True(t, got.Comment.IsMine)
}

func TestAddComment_RefusesBadInputBeforeTakingTheGate(t *testing.T) {
	tests := []struct {
		name    string
		command AddCommentCommand
		want    error
	}{
		{name: "a blank comment", command: AddCommentCommand{Command: baseCommand(), CardID: cardID, CommentID: commentID, Body: " \n "}, want: ErrCommentBodyRequired},
		{name: "a comment over 1000 characters", command: AddCommentCommand{Command: baseCommand(), CardID: cardID, CommentID: commentID, Body: strings.Repeat("c", 1001)}, want: ErrCommentBodyTooLong},
		{name: "a version 7 ID", command: AddCommentCommand{Command: baseCommand(), CardID: cardID, CommentID: v7ID, Body: "Agreed"}, want: ErrInvalidID},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, _ := newTestService(t)

			// when
			got, err := svc.AddComment(t.Context(), tc.command)

			// then the strict mocks prove the gate was never asked for
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAddComment_RefusesWhatThePolicyRefuses(t *testing.T) {
	tests := []struct {
		name    string
		options []accessOption
		want    error
	}{
		{name: "comments are switched off", options: []accessOption{atStage(model.StageDiscuss)}, want: policy.ErrCommentsDisabled},
		{name: "someone else's hidden card", options: []accessOption{commentsOn, hiddenCards}, want: policy.ErrCardNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, tc.options...))
			m.stored(card(theirID))

			// when
			got, err := svc.AddComment(t.Context(), addCommentCommand())

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestAddComment_IsRefusedOutsideTheDiscussStageEvenToAFacilitator(t *testing.T) {
	for _, stage := range []model.Stage{model.StageWrite, model.StageVote, model.StageActions} {
		for _, role := range []model.BoardRole{writer, host} {
			t.Run(fmt.Sprintf("%s/%s", stage, role), func(t *testing.T) {
				// given comments are on as a setting, but the board is not in its Discuss stage
				svc, m := newTestService(t)
				m.entered(boardAccess(role, commentsOn, atStage(stage)))
				m.stored(card(theirID))

				// when
				got, err := svc.AddComment(t.Context(), addCommentCommand())

				// then the strict comment mock proves nothing was written
				assert.Nil(t, got)
				require.ErrorIs(t, err, policy.ErrCommentsOutOfStage)
				assert.Empty(t, m.events.published)
			})
		}
	}
}

func TestEditComment_IsRefusedOutsideTheDiscussStage(t *testing.T) {
	// given the writer's own comment, once the board has moved on to Actions
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn, atStage(model.StageActions)))
	m.storedComment(comment(myID))
	m.stored(card(theirID))

	// when
	got, err := svc.EditComment(t.Context(), EditCommentCommand{Command: baseCommand(), CommentID: commentID, Body: "Changed"})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCommentsOutOfStage)
	assert.Empty(t, m.events.published)
}

func TestDeleteComment_AParticipantDeletesTheirOwnOnlyInTheDiscussStage(t *testing.T) {
	// given the writer's own comment, once the board has moved on to Actions
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, atStage(model.StageActions)))
	m.storedComment(comment(myID))
	m.stored(card(theirID))

	// when
	got, err := svc.DeleteComment(t.Context(), CommentCommand{Command: baseCommand(), CommentID: commentID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCommentsOutOfStage)
	assert.Empty(t, m.events.published)
}

func TestAddComment_ASecondCreateWithTheSameIDConflicts(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn))
	m.stored(card(theirID))
	m.comments.EXPECT().AddComment(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("create: %w", dao.ErrConflict))

	// when
	got, err := svc.AddComment(t.Context(), addCommentCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrIDConflict)
}

func TestAddComment_AReplayGivesTheCommentBackOnlyToItsWriter(t *testing.T) {
	tests := []struct {
		name   string
		author uuid.UUID
		want   bool
	}{
		{name: "the writer", author: myID, want: true},
		{name: "someone replaying another's ID", author: theirID, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.applied(boardAccess(writer, commentsOn), 6)
			m.storedComment(comment(tc.author))
			if tc.want {
				m.stored(card(theirID))
				m.participants()
			}

			// when
			got, err := svc.AddComment(t.Context(), addCommentCommand())

			// then
			require.NoError(t, err)
			assert.True(t, got.Replayed)
			assert.Equal(t, new(commentID), got.ID)
			assert.Equal(t, tc.want, got.Comment != nil)
		})
	}
}

func TestEditComment_OnlyItsWriterMayEditIt(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, commentsOn))
	m.storedComment(comment(theirID))
	m.stored(card(theirID))

	// when
	got, err := svc.EditComment(t.Context(), EditCommentCommand{Command: baseCommand(), CommentID: commentID, Body: "Changed"})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}

func TestEditComment_EditsAndAnnouncesTheComment(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, commentsOn))
	m.storedComment(comment(myID))
	m.stored(card(theirID))
	edited := comment(myID)
	edited.Body = "Changed"
	edited.UpdatedAt = new(stamp)
	m.comments.EXPECT().EditComment(mock.Anything, spec.CommentBodyWrite{BoardWrite: target(), CommentID: commentID, Body: "Changed"}).Return(&model.Versioned[model.CardComment]{Value: edited, Version: 8}, nil)
	m.participants()

	// when
	got, err := svc.EditComment(t.Context(), EditCommentCommand{Command: baseCommand(), CommentID: commentID, Body: " Changed "})

	// then
	require.NoError(t, err)
	require.NotNil(t, got.Comment)
	assert.Equal(t, "Changed", got.Comment.Body)
	assert.NotEmpty(t, got.Comment.UpdatedAt)
	require.Len(t, m.events.published, 1)
}

func TestDeleteComment_AFacilitatorMayRemoveAnyCommentTheyCanSee(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	stored := comment(theirID)
	m.storedComment(stored)
	m.stored(card(theirID))
	m.comments.EXPECT().DeleteComment(mock.Anything, spec.CommentRemoval{BoardWrite: target(), CommentID: commentID}).Return(&model.Versioned[model.CardComment]{Value: stored, Version: 8}, nil)

	// when
	got, err := svc.DeleteComment(t.Context(), CommentCommand{Command: baseCommand(), CommentID: commentID})

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.CommentDeletedEvent{Type: projection.EventCommentDeleted, Version: 8, CommentID: commentID, CardID: cardID}, m.events.published[0].event.Payload(projection.ClassOthers))
}

func TestDeleteComment_ACommentOnAHiddenCardIsNotFoundForAFacilitator(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host, hiddenCards))
	m.storedComment(comment(theirID))
	m.stored(card(theirID))

	// when
	got, err := svc.DeleteComment(t.Context(), CommentCommand{Command: baseCommand(), CommentID: commentID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCommentNotFound)
}

func TestDeleteComment_AnUnknownCommentIsNotFound(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.comments.EXPECT().GetByID(mock.Anything, spec.CardCommentRef{BoardID: boardID, CommentID: commentID}).Return(nil, errNotFound)

	// when
	got, err := svc.DeleteComment(t.Context(), CommentCommand{Command: baseCommand(), CommentID: commentID})

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrCommentNotFound)
}
