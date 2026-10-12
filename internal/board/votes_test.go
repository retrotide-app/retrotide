package board

import (
	"fmt"
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

func openVoting(access *model.BoardAccess) {
	access.Header.Stage = model.StageVote
	access.Header.VotingOpen = true
	access.Header.MaxVotes = 6
	access.Header.MaxVotesPerCard = 2
}

func hiddenTotals(access *model.BoardAccess) {
	access.Header.VoteCountsHidden = true
}

func voteCommand() CardCommand {
	return CardCommand{Command: baseCommand(), CardID: cardID}
}

func voteOn(voted model.Card, count int, mine []model.CardVote) *model.Versioned[model.CardVoteResult] {
	return &model.Versioned[model.CardVoteResult]{Value: model.CardVoteResult{Card: voted, Mine: mine, Count: count}, Version: 8}
}

func TestAddVote_SpendsAVoteWithTheBoardsLimitsAndTellsTheVoterTheirTally(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting))
	stored := card(theirID)
	m.stored(stored)
	m.column()
	voted := stored
	voted.VoteTotal = 3
	mine := []model.CardVote{{CardID: cardID, ParticipantID: myID, Count: 2, CardAuthorParticipantID: new(theirID)}}
	m.votes.EXPECT().AddVote(mock.Anything, spec.CardVote{BoardWrite: target(), CardID: cardID, ParticipantID: myID, MaxVotes: 6, MaxVotesPerCard: 2}).Return(voteOn(voted, 2, mine), nil)

	// when
	got, err := svc.AddVote(t.Context(), voteCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.BoardCommandResponse{Version: 8, Me: &dto.MeView{VotesRemaining: 4, MyVotes: map[uuid.UUID]int{cardID: 2}}}, got)
	assert.Equal(t, []dto.BoardMeFrame{{BoardID: boardID, Version: 8, VotesRemaining: 4, MyVotes: map[uuid.UUID]int{cardID: 2}}}, m.events.me)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	assert.Equal(t, int64(8), event.Version())
	assert.Equal(t, dto.VotesChangedEvent{Type: projection.EventVotesChanged, Version: 8, CardID: cardID, Total: new(3)}, event.Payload(projection.ClassOthers))
}

func TestAddVote_AVoteOnAStackGoesToItsTopCard(t *testing.T) {
	// given a card merged under another
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting))
	child := card(theirID)
	child.ID = otherID
	child.ParentCardID = new(cardID)
	m.stored(child)
	top := card(theirID)
	m.stored(top)
	m.column()
	m.votes.EXPECT().AddVote(mock.Anything, mock.MatchedBy(func(s spec.CardVote) bool {
		return s.CardID == cardID
	})).Return(voteOn(top, 1, nil), nil)

	// when
	_, err := svc.AddVote(t.Context(), CardCommand{Command: baseCommand(), CardID: otherID})

	// then
	require.NoError(t, err)
}

func TestAddVote_WithHiddenTotalsNobodyElseIsToldAndTheVoterSeesNoTotal(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting, hiddenTotals))
	stored := card(theirID)
	m.stored(stored)
	m.column()
	m.votes.EXPECT().AddVote(mock.Anything, mock.Anything).Return(voteOn(stored, 1, nil), nil)

	// when
	_, err := svc.AddVote(t.Context(), voteCommand())

	// then
	require.NoError(t, err)
	require.Len(t, m.events.published, 1)
	event := m.events.published[0].event
	assert.Nil(t, event.Payload(projection.ClassOthers))
	assert.Equal(t, dto.VotesChangedEvent{Type: projection.EventVotesChanged, Version: 8, CardID: cardID}, event.Payload(projection.ClassAuthor))
	assert.Equal(t, projection.ClassAuthor, event.ClassOf(projection.Viewer{ParticipantID: new(myID)}))
}

func TestAddVote_RefusesWhatThePolicyRefusesWithoutWriting(t *testing.T) {
	tests := []struct {
		name    string
		options []accessOption
		author  uuid.UUID
		want    error
	}{
		{name: "voting is closed", options: []accessOption{atStage(model.StageVote)}, author: theirID, want: policy.ErrVotingClosed},
		{name: "someone else's hidden card", options: []accessOption{openVoting, hiddenCards}, author: theirID, want: policy.ErrCardNotFound},
		{name: "not joined", options: []accessOption{openVoting, notJoined}, author: theirID, want: policy.ErrNotParticipant},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, tc.options...))
			m.stored(card(tc.author))

			// when
			got, err := svc.AddVote(t.Context(), voteCommand())

			// then the strict vote mock proves nothing was written
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
			assert.Empty(t, m.events.me)
		})
	}
}

func TestAddVote_IsRefusedOutsideTheVoteStageEvenToAFacilitator(t *testing.T) {
	for _, stage := range []model.Stage{model.StageWrite, model.StageDiscuss, model.StageActions} {
		for _, role := range []model.BoardRole{writer, host} {
			t.Run(fmt.Sprintf("%s/%s", stage, role), func(t *testing.T) {
				// given voting is open as a setting, but the board is not in its Vote stage
				svc, m := newTestService(t)
				m.entered(boardAccess(role, openVoting, atStage(stage)))
				m.stored(card(theirID))

				// when
				got, err := svc.AddVote(t.Context(), voteCommand())

				// then the strict vote mock proves nothing was written
				assert.Nil(t, got)
				require.ErrorIs(t, err, policy.ErrVotesOutOfStage)
				assert.Empty(t, m.events.published)
				assert.Empty(t, m.events.me)
			})
		}
	}
}

func TestRemoveVote_IsRefusedOutsideTheVoteStage(t *testing.T) {
	// given a vote cast during the Vote stage, and the board since moved on to Discuss
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting, atStage(model.StageDiscuss)))
	m.stored(card(theirID))

	// when
	got, err := svc.RemoveVote(t.Context(), voteCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrVotesOutOfStage)
	assert.Empty(t, m.events.published)
}

func TestAddVote_MapsTheSpentLimitsToTheirRefusals(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "the board's budget", err: dao.ErrVoteBudgetSpent, want: ErrVoteLimitReached},
		{name: "the card's cap", err: dao.ErrCardVoteCapReached, want: ErrCardVoteLimitReached},
		{name: "the column's budget", err: dao.ErrColumnVoteBudgetSpent, want: ErrColumnVoteLimitReached},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			svc, m := newTestService(t)
			m.entered(boardAccess(writer, openVoting))
			m.stored(card(theirID))
			m.column()
			m.votes.EXPECT().AddVote(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("spend: %w", tc.err))

			// when
			got, err := svc.AddVote(t.Context(), voteCommand())

			// then
			assert.Nil(t, got)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, m.events.published)
		})
	}
}

func TestAddVote_PassesTheColumnsVoteLimitToTheWrite(t *testing.T) {
	// given a column that allows each voter three votes
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting))
	stored := card(theirID)
	m.stored(stored)
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(&model.BoardColumn{ID: columnID, BoardID: boardID, MaxVotes: new(3)}, nil).Once()
	cast := spec.CardVote{BoardWrite: target(), CardID: cardID, ParticipantID: myID, MaxVotes: 6, MaxVotesPerCard: 2, ColumnID: columnID, ColumnMaxVotes: 3}
	m.votes.EXPECT().AddVote(mock.Anything, cast).Return(voteOn(stored, 1, nil), nil)

	// when
	got, err := svc.AddVote(t.Context(), voteCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
}

func TestAddVote_ASpentColumnBudgetIsRefusedWithoutAnnouncing(t *testing.T) {
	// given a column whose limit the voter has already reached
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting))
	m.stored(card(theirID))
	m.columns.EXPECT().GetByID(mock.Anything, spec.BoardColumnKey{BoardID: boardID, ColumnID: columnID}).Return(&model.BoardColumn{ID: columnID, BoardID: boardID, MaxVotes: new(1)}, nil).Once()
	m.votes.EXPECT().AddVote(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("check: %w", dao.ErrColumnVoteBudgetSpent))

	// when
	got, err := svc.AddVote(t.Context(), voteCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrColumnVoteLimitReached)
	assert.Empty(t, m.events.published)
	assert.Empty(t, m.events.me)
}

func TestAddVote_AHiddenColumnIsFrozenForAFacilitatorAndUnknownToAParticipant(t *testing.T) {
	// given a card in a column hidden from participants
	svc, m := newTestService(t)
	m.entered(boardAccess(host, openVoting, columnHidden))
	m.stored(card(theirID))
	m.entered(boardAccess(writer, openVoting, columnHidden))
	m.stored(card(theirID))

	// when
	facilitated, facilitatorErr := svc.AddVote(t.Context(), voteCommand())
	voted, participantErr := svc.AddVote(t.Context(), voteCommand())

	// then the strict mocks prove no vote was written
	assert.Nil(t, facilitated)
	assert.Nil(t, voted)
	require.ErrorIs(t, facilitatorErr, policy.ErrColumnHidden)
	require.ErrorIs(t, participantErr, policy.ErrCardNotFound)
	assert.Empty(t, m.events.published)
}

func TestAddVote_ARetriedVoteReplaysItsVersion(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.applied(boardAccess(writer, openVoting), 6)

	// when
	got, err := svc.AddVote(t.Context(), voteCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, replayResponse(6, nil), got)
}

func TestRemoveVote_TakesBackAVoteOnTheCardNamed(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting))
	stored := card(theirID)
	m.stored(stored)
	m.votes.EXPECT().RemoveVote(mock.Anything, spec.CardVote{BoardWrite: target(), CardID: cardID, ParticipantID: myID, MaxVotes: 6, MaxVotesPerCard: 2}).Return(voteOn(stored, 0, nil), nil)

	// when
	got, err := svc.RemoveVote(t.Context(), voteCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, &dto.MeView{VotesRemaining: 6, MyVotes: map[uuid.UUID]int{}}, got.Me)
	require.Len(t, m.events.published, 1)
	require.Len(t, m.events.me, 1)
}

func TestRemoveVote_WithNoVoteToTakeBackIsRefused(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer, openVoting))
	m.stored(card(theirID))
	m.votes.EXPECT().RemoveVote(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("drop: %w", dao.ErrNotFound))

	// when
	got, err := svc.RemoveVote(t.Context(), voteCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, ErrNoVote)
}

func TestResetVotes_IsTheFacilitatorsAndAnnouncesOneEvent(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(host))
	m.votes.EXPECT().ResetVotes(mock.Anything, target()).Return(&model.Versioned[int64]{Value: 12, Version: 8}, nil)

	// when
	got, err := svc.ResetVotes(t.Context(), baseCommand())

	// then
	require.NoError(t, err)
	assert.Equal(t, versionResponse(8), got)
	require.Len(t, m.events.published, 1)
	assert.Equal(t, dto.BoardEventHead{Type: projection.EventVotesReset, Version: 8}, m.events.published[0].event.Payload(projection.ClassOthers))
}

func TestResetVotes_IsRefusedToAParticipant(t *testing.T) {
	// given
	svc, m := newTestService(t)
	m.entered(boardAccess(writer))

	// when
	got, err := svc.ResetVotes(t.Context(), baseCommand())

	// then
	assert.Nil(t, got)
	require.ErrorIs(t, err, policy.ErrNotAllowed)
}
