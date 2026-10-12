package controllers

import (
	"fmt"
	"net/http"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/board/policy"
	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	boardComment     = uuid.MustParse("5d6e7f80-91a2-4b3c-94d5-3b4c5d6e7f80")
	boardCommentBody = dto.CreateCommentRequest{BoardCommandRequest: boardOpBody, ID: boardComment, Body: "Agreed, the pipeline helped"}
)

func TestBoardVoteController_EveryRouteRunsThePrincipalFailureSuite(t *testing.T) {
	cardPath := boardPath("/cards/" + boardCard.String())
	commentPath := boardPath("/comments/" + boardComment.String())

	routes := []testutil.AuthRoute{
		{Method: http.MethodPost, Path: cardPath + "/merge", Body: dto.MergeCardRequest{BoardCommandRequest: boardOpBody, TargetCardID: boardAnchor}},
		{Method: http.MethodPost, Path: cardPath + "/unmerge", Body: boardOpBody},
		{Method: http.MethodPost, Path: cardPath + "/votes", Body: boardOpBody},
		{Method: http.MethodDelete, Path: cardPath + "/votes", Body: boardOpBody},
		{Method: http.MethodPost, Path: boardPath("/votes/reset"), Body: boardOpBody},
		{Method: http.MethodPut, Path: boardPath("/settings"), Body: dto.BoardSettingsRequest{BoardCommandRequest: boardOpBody}},
		{Method: http.MethodPost, Path: cardPath + "/comments", Body: boardCommentBody},
		{Method: http.MethodPut, Path: commentPath, Body: dto.EditCommentRequest{BoardCommandRequest: boardOpBody, Body: "Agreed"}},
		{Method: http.MethodDelete, Path: commentPath, Body: boardOpBody},
	}

	for _, route := range routes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			testutil.RunPrincipalFailureSuite(t, func(t *testing.T) *testutil.Harness {
				return newBoardHarness(t).Harness
			}, route)
		})
	}
}

func TestBoardVoteController_EveryCommandBuildsItsCommandFromThePathAndTheBody(t *testing.T) {
	cardCommandOf := board.CardCommand{Command: boardCommandOf, CardID: boardCard}
	commentCommandOf := board.CommentCommand{Command: boardCommandOf, CommentID: boardComment}
	cardPath := boardPath("/cards/" + boardCard.String())
	commentPath := boardPath("/comments/" + boardComment.String())
	votes := 3
	sortByVotes := model.SortModeVotes
	everySetting := dto.BoardSettingsRequest{
		BoardCommandRequest: boardOpBody,
		VotingOpen:          new(false),
		VoteCountsHidden:    new(false),
		MaxVotes:            &votes,
		MaxVotesPerCard:     new(1),
		CommentsEnabled:     new(false),
		MergingEnabled:      new(false),
		SortMode:            &sortByVotes,
		TimerAutoLock:       new(true),
		SortEnforced:        new(true),
		AnyoneCanEdit:       new(true),
	}
	everyPatch := spec.BoardSettingsPatch{
		VotingOpen:       new(false),
		VoteCountsHidden: new(false),
		MaxVotes:         &votes,
		MaxVotesPerCard:  new(1),
		CommentsEnabled:  new(false),
		MergingEnabled:   new(false),
		SortMode:         &sortByVotes,
		TimerAutoLock:    new(true),
		SortEnforced:     new(true),
		AnyoneCanEdit:    new(true),
	}

	tests := []boardCommandCase{
		{
			name:   "merge a card into another",
			method: http.MethodPost,
			path:   cardPath + "/merge",
			body:   dto.MergeCardRequest{BoardCommandRequest: boardOpBody, TargetCardID: boardAnchor},
			expect: func(bh *boardHarness) {
				merge := board.MergeCardCommand{Command: boardCommandOf, CardID: boardCard, TargetCardID: boardAnchor}
				bh.boards.EXPECT().MergeCard(mock.Anything, merge).Return(boardVersioned, nil)
			},
		},
		{
			name:   "unmerge a card",
			method: http.MethodPost,
			path:   cardPath + "/unmerge",
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().UnmergeCard(mock.Anything, cardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "vote for a card",
			method: http.MethodPost,
			path:   cardPath + "/votes",
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().AddVote(mock.Anything, cardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "take back a vote",
			method: http.MethodDelete,
			path:   cardPath + "/votes",
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().RemoveVote(mock.Anything, cardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "reset every vote",
			method: http.MethodPost,
			path:   boardPath("/votes/reset"),
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ResetVotes(mock.Anything, boardCommandOf).Return(boardVersioned, nil)
			},
		},
		{
			name:   "change some settings",
			method: http.MethodPut,
			path:   boardPath("/settings"),
			body:   dto.BoardSettingsRequest{BoardCommandRequest: boardOpBody, VoteCountsHidden: new(true), MaxVotes: &votes, SortMode: &sortByVotes},
			expect: func(bh *boardHarness) {
				settings := spec.BoardSettingsPatch{VoteCountsHidden: new(true), MaxVotes: &votes, SortMode: &sortByVotes}
				bh.boards.EXPECT().ChangeSettings(mock.Anything, board.SettingsCommand{Command: boardCommandOf, Settings: settings}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "change every setting",
			method: http.MethodPut,
			path:   boardPath("/settings"),
			body:   everySetting,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ChangeSettings(mock.Anything, board.SettingsCommand{Command: boardCommandOf, Settings: everyPatch}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "turn off auto-lock, sort enforcement and anyone can edit",
			method: http.MethodPut,
			path:   boardPath("/settings"),
			body:   dto.BoardSettingsRequest{BoardCommandRequest: boardOpBody, TimerAutoLock: new(false), SortEnforced: new(false), AnyoneCanEdit: new(false)},
			expect: func(bh *boardHarness) {
				settings := spec.BoardSettingsPatch{TimerAutoLock: new(false), SortEnforced: new(false), AnyoneCanEdit: new(false)}
				bh.boards.EXPECT().ChangeSettings(mock.Anything, board.SettingsCommand{Command: boardCommandOf, Settings: settings}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "comment on a card",
			method: http.MethodPost,
			path:   cardPath + "/comments",
			body:   boardCommentBody,
			expect: func(bh *boardHarness) {
				add := board.AddCommentCommand{Command: boardCommandOf, CardID: boardCard, CommentID: boardComment, Body: boardCommentBody.Body}
				bh.boards.EXPECT().AddComment(mock.Anything, add).Return(boardVersioned, nil)
			},
		},
		{
			name:   "edit a comment",
			method: http.MethodPut,
			path:   commentPath,
			body:   dto.EditCommentRequest{BoardCommandRequest: boardOpBody, Body: "Agreed"},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().EditComment(mock.Anything, board.EditCommentCommand{Command: boardCommandOf, CommentID: boardComment, Body: "Agreed"}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "comment on a card, mentioning someone",
			method: http.MethodPost,
			path:   cardPath + "/comments",
			body:   dto.CreateCommentRequest{BoardCommandRequest: boardOpBody, ID: boardComment, Body: "Agreed, @Sam", Mentions: []uuid.UUID{boardUser}},
			expect: func(bh *boardHarness) {
				add := board.AddCommentCommand{Command: boardCommandOf, CardID: boardCard, CommentID: boardComment, Body: "Agreed, @Sam", Mentions: []uuid.UUID{boardUser}}
				bh.boards.EXPECT().AddComment(mock.Anything, add).Return(boardVersioned, nil)
			},
		},
		{
			name:   "edit a comment's mentions",
			method: http.MethodPut,
			path:   commentPath,
			body:   dto.EditCommentRequest{BoardCommandRequest: boardOpBody, Body: "Agreed, @Sam", Mentions: []uuid.UUID{boardUser}},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().EditComment(mock.Anything, board.EditCommentCommand{Command: boardCommandOf, CommentID: boardComment, Body: "Agreed, @Sam", Mentions: []uuid.UUID{boardUser}}).Return(boardVersioned, nil)
			},
		},
		{
			name:   "delete a comment",
			method: http.MethodDelete,
			path:   commentPath,
			body:   boardOpBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().DeleteComment(mock.Anything, commentCommandOf).Return(boardVersioned, nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusOK, status)
			assert.JSONEq(t, `{"version":7}`, body)
		})
	}
}

func TestBoardVoteController_AVoteAnswersTheVotersOwnTally(t *testing.T) {
	// given
	bh := newBoardHarness(t)
	voted := &dto.BoardCommandResponse{Version: 8, Me: &dto.MeView{VotesRemaining: 5, MyVotes: map[uuid.UUID]int{boardCard: 1}}}
	bh.boards.EXPECT().AddVote(mock.Anything, board.CardCommand{Command: boardCommandOf, CardID: boardCard}).Return(voted, nil)

	// when
	status, body := bh.record(http.MethodPost, boardPath("/cards/"+boardCard.String()+"/votes"), boardOpBody)

	// then
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"version":8,"me":{"votes_remaining":5,"my_votes":{"`+boardCard.String()+`":1}}}`, body)
}

func TestBoardVoteController_ACommentIDThatIsNotVersion4IsRefusedBeforeTheService(t *testing.T) {
	tests := []struct {
		name string
		body dto.CreateCommentRequest
	}{
		{name: "a version 7 ID", body: dto.CreateCommentRequest{BoardCommandRequest: boardOpBody, ID: boardV7CardID, Body: "Agreed"}},
		{name: "no ID", body: dto.CreateCommentRequest{BoardCommandRequest: boardOpBody, Body: "Agreed"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)

			// when
			status, body := bh.record(http.MethodPost, boardPath("/cards/"+boardCard.String()+"/comments"), tc.body)

			// then the strict board mock proves the service is never asked
			assert.Equal(t, http.StatusBadRequest, status)
			assert.JSONEq(t, `{"error":"a board content ID must be a version 4 UUID","code":"invalid_id"}`, body)
		})
	}
}

func TestBoardVoteController_ACommentPathIDThatIsNotAUUIDIsRefused(t *testing.T) {
	// given
	bh := newBoardHarness(t)

	// when
	status, body := bh.record(http.MethodDelete, boardPath("/comments/not-a-uuid"), boardOpBody)

	// then
	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"invalid commentID"}`, body)
}

func TestBoardVoteController_MapsEveryVoteCommentAndMergeErrorToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		body   string
	}{
		{err: board.ErrVoteLimitReached, status: http.StatusConflict, body: `{"error":"you have used every vote this board allows","code":"vote_limit_reached"}`},
		{err: board.ErrCardVoteLimitReached, status: http.StatusConflict, body: `{"error":"you already have as many votes on that card as this board allows","code":"card_vote_limit_reached"}`},
		{err: board.ErrNoVote, status: http.StatusConflict, body: `{"error":"you have no vote on that card to take back","code":"no_vote"}`},
		{err: board.ErrCommentBodyRequired, status: http.StatusBadRequest, body: `{"error":"a comment needs some text","code":"comment_body_required"}`},
		{err: board.ErrCommentBodyTooLong, status: http.StatusBadRequest, body: `{"error":"a comment can be at most 1000 characters","code":"comment_body_too_long"}`},
		{err: board.ErrInvalidMaxVotes, status: http.StatusBadRequest, body: `{"error":"a vote limit must be between 0 and 99","code":"invalid_max_votes"}`},
		{err: board.ErrInvalidSortMode, status: http.StatusBadRequest, body: `{"error":"the board's sort order must be manual or votes","code":"invalid_sort_mode"}`},
		{err: policy.ErrVotingClosed, status: http.StatusConflict, body: `{"error":"voting is closed on this board","code":"voting_closed"}`},
		{err: policy.ErrCommentsDisabled, status: http.StatusConflict, body: `{"error":"comments are switched off on this board","code":"comments_disabled"}`},
		{err: policy.ErrCommentNotFound, status: http.StatusNotFound, body: `{"error":"comment not found"}`},
		{err: policy.ErrMergingDisabled, status: http.StatusConflict, body: `{"error":"merging cards is switched off on this board","code":"merging_disabled"}`},
		{err: policy.ErrCardsHidden, status: http.StatusConflict, body: `{"error":"cards are hidden, so they cannot be merged or unmerged until they are revealed","code":"cards_hidden"}`},
		{err: policy.ErrMergeIntoSelf, status: http.StatusBadRequest, body: `{"error":"a card cannot be merged into itself","code":"merge_into_self"}`},
		{err: policy.ErrNotMerged, status: http.StatusConflict, body: `{"error":"the card is not part of a merged stack","code":"not_merged"}`},
		{err: policy.ErrCardNotFound, status: http.StatusNotFound, body: `{"error":"card not found"}`},
		{err: live.ErrBusy, status: http.StatusServiceUnavailable, body: `{"error":"the board is busy, so try again in a moment","code":"board_busy"}`},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			bh.boards.EXPECT().AddVote(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("vote for card %s: %w", boardCard, tc.err))

			// when
			status, body := bh.record(http.MethodPost, boardPath("/cards/"+boardCard.String()+"/votes"), boardOpBody)

			// then the body carries the sentinel's own words, never the wrapped detail
			assert.Equal(t, tc.status, status)
			assert.JSONEq(t, tc.body, body)
		})
	}
}

func TestBoardVoteController_AnUnexpectedFailureAnswersTheRoutesOwnMessage(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		expect func(bh *boardHarness)
		want   string
	}{
		{
			name:   "settings",
			method: http.MethodPut,
			path:   boardPath("/settings"),
			body:   dto.BoardSettingsRequest{BoardCommandRequest: boardOpBody},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().ChangeSettings(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to change the board's settings"}`,
		},
		{
			name:   "comment",
			method: http.MethodPost,
			path:   boardPath("/cards/" + boardCard.String() + "/comments"),
			body:   boardCommentBody,
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().AddComment(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to add the comment"}`,
		},
		{
			name:   "merge",
			method: http.MethodPost,
			path:   boardPath("/cards/" + boardCard.String() + "/merge"),
			body:   dto.MergeCardRequest{BoardCommandRequest: boardOpBody, TargetCardID: boardAnchor},
			expect: func(bh *boardHarness) {
				bh.boards.EXPECT().MergeCard(mock.Anything, mock.Anything).Return(nil, errServiceDown)
			},
			want: `{"error":"failed to merge the card"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			bh := newBoardHarness(t)
			tc.expect(bh)

			// when
			status, body := bh.record(tc.method, tc.path, tc.body)

			// then
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.JSONEq(t, tc.want, body)
		})
	}
}
