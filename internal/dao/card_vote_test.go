package dao_test

import (
	"errors"
	"sync"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	concurrentVoters = 40
	voteBudget       = 5
)

func (f boardFixture) vote(t *testing.T, cardID, participantID uuid.UUID, maxVotes, perCard int) (*model.Versioned[model.CardVoteResult], error) {
	t.Helper()

	write := f.write(t)

	return f.repos.CardVote.AddVote(t.Context(), spec.CardVote{BoardWrite: write, CardID: cardID, ParticipantID: participantID, MaxVotes: maxVotes, MaxVotesPerCard: perCard})
}

func (f boardFixture) unvote(t *testing.T, cardID, participantID uuid.UUID) (*model.Versioned[model.CardVoteResult], error) {
	t.Helper()

	write := f.write(t)

	return f.repos.CardVote.RemoveVote(t.Context(), spec.CardVote{BoardWrite: write, CardID: cardID, ParticipantID: participantID, MaxVotes: 6})
}

func (f boardFixture) budgetUsed(t *testing.T, participantID uuid.UUID) int {
	t.Helper()

	used := countWhere(t, f.repos, `SELECT COALESCE(SUM(used), 0) FROM board_vote_budgets WHERE board_id = $1 AND participant_id = $2`, f.boardID(), participantID)

	return used
}

func (f boardFixture) voteTotal(t *testing.T, cardID uuid.UUID) int {
	t.Helper()

	card, err := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: cardID})
	require.NoError(t, err)

	return card.VoteTotal
}

func (f boardFixture) secondParticipant(t *testing.T) uuid.UUID {
	t.Helper()

	member := daotest.CreateUser(t, f.repos)
	daotest.AddOrgMember(t, f.repos, f.orgID(), member.ID, model.OrgRoleMember)
	seat := daotest.CreateParticipant(t, f.repos, f.boardID(), member.ID)

	return seat.ID
}

func TestCardVote_AddVoteSpendsTheBudgetCountsTheVoteAndRaisesTheTotal(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	other := f.addCard(t, f.column(1), "a0")

	// when
	first, firstErr := f.vote(t, card.ID, f.facilitator(), 6, 0)
	second, secondErr := f.vote(t, card.ID, f.facilitator(), 6, 0)
	third, thirdErr := f.vote(t, other.ID, f.facilitator(), 6, 0)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, thirdErr)
	assert.Equal(t, 1, first.Value.Count)
	assert.Equal(t, 2, second.Value.Count)
	assert.Equal(t, 2, second.Value.Card.VoteTotal)
	assert.Equal(t, 1, third.Value.Card.VoteTotal)
	assert.Equal(t, int64(5), third.Version)
	assert.ElementsMatch(t, []model.CardVote{
		{CardID: card.ID, ParticipantID: f.facilitator(), Count: 2, CardAuthorParticipantID: new(f.facilitator()), CardColumnID: f.column(0)},
		{CardID: other.ID, ParticipantID: f.facilitator(), Count: 1, CardAuthorParticipantID: new(f.facilitator()), CardColumnID: f.column(1)},
	}, third.Value.Mine)
	assert.Equal(t, 3, f.budgetUsed(t, f.facilitator()))
	assert.Equal(t, 2, f.voteTotal(t, card.ID))
}

func TestCardVote_TheBoardCapRefusesAVoteAndBurnsNoVersion(t *testing.T) {
	// given a participant who has spent the only vote the board allows
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	_, err := f.vote(t, card.ID, f.facilitator(), 1, 0)
	require.NoError(t, err)
	before := f.version(t)

	// when
	_, err = f.vote(t, card.ID, f.facilitator(), 1, 0)

	// then
	require.ErrorIs(t, err, dao.ErrVoteBudgetSpent)
	assert.Equal(t, before, f.version(t))
	assert.Equal(t, 1, f.voteTotal(t, card.ID))
	assert.Equal(t, 1, f.budgetUsed(t, f.facilitator()))
}

func TestCardVote_ABoardWithNoVotesRefusesTheFirst(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")

	// when
	_, err := f.vote(t, card.ID, f.facilitator(), 0, 0)

	// then
	require.ErrorIs(t, err, dao.ErrVoteBudgetSpent)
	assert.Zero(t, f.voteTotal(t, card.ID))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM board_vote_budgets WHERE board_id = $1`, f.boardID()))
}

func TestCardVote_ThePerCardCapRefusesAVoteAndRollsTheBudgetBack(t *testing.T) {
	// given one vote on a card that allows one per person
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	other := f.addCard(t, f.column(1), "a0")
	_, err := f.vote(t, card.ID, f.facilitator(), 2, 1)
	require.NoError(t, err)
	before := f.version(t)

	// when
	_, capErr := f.vote(t, card.ID, f.facilitator(), 2, 1)
	_, otherErr := f.vote(t, other.ID, f.facilitator(), 2, 1)

	// then the refused vote spent nothing, so the second card still takes the last vote
	require.ErrorIs(t, capErr, dao.ErrCardVoteCapReached)
	require.NoError(t, otherErr)
	assert.Equal(t, before+1, f.version(t))
	assert.Equal(t, 1, f.voteTotal(t, card.ID))
	assert.Equal(t, 2, f.budgetUsed(t, f.facilitator()))
}

func TestCardVote_RemoveVoteLowersThenDropsAndRefundsEachTime(t *testing.T) {
	// given two votes on one card
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	for range 2 {
		_, err := f.vote(t, card.ID, f.facilitator(), 6, 0)
		require.NoError(t, err)
	}

	// when
	lowered, lowerErr := f.unvote(t, card.ID, f.facilitator())
	dropped, dropErr := f.unvote(t, card.ID, f.facilitator())
	_, goneErr := f.unvote(t, card.ID, f.facilitator())

	// then
	require.NoError(t, lowerErr)
	assert.Equal(t, 1, lowered.Value.Count)
	assert.Equal(t, 1, lowered.Value.Card.VoteTotal)
	require.NoError(t, dropErr)
	assert.Zero(t, dropped.Value.Count)
	assert.Zero(t, dropped.Value.Card.VoteTotal)
	assert.Empty(t, dropped.Value.Mine)
	require.ErrorIs(t, goneErr, dao.ErrNotFound)
	assert.Zero(t, f.budgetUsed(t, f.facilitator()))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_votes WHERE board_id = $1`, f.boardID()))
}

func TestCardVote_DeletingCardsRefundsEveryVoteOnThem(t *testing.T) {
	tests := []struct {
		name   string
		remove func(t *testing.T, f boardFixture, card model.Card)
	}{
		{name: "one card", remove: func(t *testing.T, f boardFixture, card model.Card) {
			_, err := f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: card.ID})
			require.NoError(t, err)
		}},
		{name: "a cleared column", remove: func(t *testing.T, f boardFixture, card model.Card) {
			_, err := f.repos.Card.ClearCards(t.Context(), spec.ColumnRemoval{BoardWrite: f.write(t), ColumnID: card.ColumnID})
			require.NoError(t, err)
		}},
		{name: "a deleted column", remove: func(t *testing.T, f boardFixture, card model.Card) {
			_, err := f.repos.BoardColumn.DeleteColumn(t.Context(), spec.ColumnRemoval{BoardWrite: f.write(t), ColumnID: card.ColumnID})
			require.NoError(t, err)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given two people's votes on a card in the first column, and one vote kept in the second
			f := newBoardFixture(t)
			voter := f.secondParticipant(t)
			card := f.addCard(t, f.column(0), "a0")
			kept := f.addCard(t, f.column(1), "a0")
			for _, participant := range []uuid.UUID{f.facilitator(), f.facilitator(), voter} {
				_, err := f.vote(t, card.ID, participant, 6, 0)
				require.NoError(t, err)
			}
			_, err := f.vote(t, kept.ID, voter, 6, 0)
			require.NoError(t, err)

			// when
			tc.remove(t, f, card)

			// then each voter gets back exactly what they spent on it
			assert.Zero(t, f.budgetUsed(t, f.facilitator()))
			assert.Equal(t, 1, f.budgetUsed(t, voter))
			assert.Equal(t, 1, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_votes WHERE board_id = $1`, f.boardID()))
		})
	}
}

func TestCardVote_ResetVotesClearsVotesBudgetsAndTotals(t *testing.T) {
	// given
	f := newBoardFixture(t)
	voter := f.secondParticipant(t)
	card := f.addCard(t, f.column(0), "a0")
	for _, participant := range []uuid.UUID{f.facilitator(), voter, voter} {
		_, err := f.vote(t, card.ID, participant, 6, 0)
		require.NoError(t, err)
	}

	// when
	reset, err := f.repos.CardVote.ResetVotes(t.Context(), f.write(t))

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(2), reset.Value)
	assert.Zero(t, f.voteTotal(t, card.ID))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_votes WHERE board_id = $1`, f.boardID()))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM board_vote_budgets WHERE board_id = $1`, f.boardID()))

	again, err := f.vote(t, card.ID, voter, 1, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, again.Value.Card.VoteTotal)
}

func TestCardVote_ListCarriesEachVotesCardAuthor(t *testing.T) {
	// given
	f := newBoardFixture(t)
	voter := f.secondParticipant(t)
	card := f.addCard(t, f.column(0), "a0")
	_, err := f.vote(t, card.ID, voter, 6, 0)
	require.NoError(t, err)

	// when
	listed, listErr := f.repos.CardVote.List(t.Context(), f.boardID())
	mine, mineErr := f.repos.CardVote.ListForParticipant(t.Context(), spec.BoardParticipantKey{BoardID: f.boardID(), ParticipantID: f.facilitator()})

	// then
	require.NoError(t, listErr)
	assert.Equal(t, []model.CardVote{{CardID: card.ID, ParticipantID: voter, Count: 1, CardAuthorParticipantID: new(f.facilitator()), CardColumnID: f.column(0)}}, listed)
	require.NoError(t, mineErr)
	assert.Empty(t, mine)
}

func TestCardVote_HandingOverAddsToTheAccountsOwnVotes(t *testing.T) {
	// given a guest and an account who both voted for one card, and the guest for another
	f := newBoardFixture(t)
	guest := f.secondParticipant(t)
	card := f.addCard(t, f.column(0), "a0")
	other := f.addCard(t, f.column(1), "a0")
	for _, vote := range []struct{ card, participant uuid.UUID }{{card.ID, guest}, {card.ID, f.facilitator()}, {other.ID, guest}} {
		_, err := f.vote(t, vote.card, vote.participant, 6, 0)
		require.NoError(t, err)
	}
	handover := spec.VoteHandover{BoardID: f.boardID(), FromParticipantID: guest, ToParticipantID: f.facilitator()}

	// when
	_, votesErr := f.repos.CardVote.HandOverVotes(t.Context(), handover)
	_, budgetErr := f.repos.CardVote.HandOverBudget(t.Context(), handover)

	// then
	require.NoError(t, votesErr)
	require.NoError(t, budgetErr)
	mine, err := f.repos.CardVote.ListForParticipant(t.Context(), spec.BoardParticipantKey{BoardID: f.boardID(), ParticipantID: f.facilitator()})
	require.NoError(t, err)
	assert.Equal(t, 3, model.VotesUsed(mine))
	assert.Equal(t, 3, f.budgetUsed(t, f.facilitator()))
	assert.Equal(t, 2, f.voteTotal(t, card.ID))
}

func TestCardVote_FortyConcurrentVotesWithFiveToSpendGiveExactlyFive(t *testing.T) {
	// given one participant and a board that allows five votes
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	_, err := f.repos.Board.SetSettings(t.Context(), spec.BoardSettingsChange{OrgID: f.orgID(), BoardID: f.boardID(), Patch: spec.BoardSettingsPatch{MaxVotes: new(voteBudget)}})
	require.NoError(t, err)
	_, err = f.repos.Board.AdvanceStage(t.Context(), spec.BoardStageWrite{BoardWrite: f.write(t), From: model.StageWrite, To: model.StageVote, Reveal: true})
	require.NoError(t, err)

	boards := gatedBoardService(t, f)
	principal := model.Principal{Kind: model.PrincipalUser, ID: f.workspace.Owner.UserID}
	before := f.version(t)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		accepted int
		refused  int
		failures []error
	)

	// when forty votes arrive at once
	for range concurrentVoters {
		wg.Go(func() {
			_, err := boards.AddVote(t.Context(), board.CardCommand{Principal: principal, BoardID: f.boardID(), OpID: uuid.New(), CardID: card.ID})

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				accepted++
			case errors.Is(err, board.ErrVoteLimitReached):
				refused++
			default:
				failures = append(failures, err)
			}
		})
	}
	wg.Wait()

	// then exactly five land, and every refusal burned no version
	require.Empty(t, failures)
	assert.Equal(t, voteBudget, accepted)
	assert.Equal(t, concurrentVoters-voteBudget, refused)
	assert.Equal(t, voteBudget, f.voteTotal(t, card.ID))
	assert.Equal(t, voteBudget, f.budgetUsed(t, f.facilitator()))
	assert.Equal(t, before+voteBudget, f.version(t))
}

func TestCardVote_FortyConcurrentBudgetSpendsHoldTheCapWithoutTheGate(t *testing.T) {
	// given
	f := newBoardFixture(t)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		spent   int
		refused int
		other   []error
	)

	// when forty spends race on the statement itself, with no gate in front of it
	for range concurrentVoters {
		wg.Go(func() {
			_, err := f.repos.CardVote.SpendBudget(t.Context(), spec.VoteBudgetSpend{BoardID: f.boardID(), ParticipantID: f.facilitator(), MaxVotes: voteBudget})

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				spent++
			case errors.Is(err, dao.ErrVoteBudgetSpent):
				refused++
			default:
				other = append(other, err)
			}
		})
	}
	wg.Wait()

	// then the conditional statement alone keeps the cap
	require.Empty(t, other)
	assert.Equal(t, voteBudget, spent)
	assert.Equal(t, concurrentVoters-voteBudget, refused)
	assert.Equal(t, voteBudget, f.budgetUsed(t, f.facilitator()))
}

func TestCardVote_FortyConcurrentIncrementsHoldThePerCardCapWithoutTheGate(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")
	const perCard = 3

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		counted int
		refused int
		other   []error
	)

	// when
	for range concurrentVoters {
		wg.Go(func() {
			_, err := f.repos.CardVote.IncrementVote(t.Context(), spec.CardVoteIncrement{BoardID: f.boardID(), CardID: card.ID, ParticipantID: f.facilitator(), MaxVotesPerCard: perCard})

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				counted++
			case errors.Is(err, dao.ErrCardVoteCapReached):
				refused++
			default:
				other = append(other, err)
			}
		})
	}
	wg.Wait()

	// then
	require.Empty(t, other)
	assert.Equal(t, perCard, counted)
	assert.Equal(t, concurrentVoters-perCard, refused)
	assert.Equal(t, perCard, countWhere(t, f.repos, `SELECT count FROM card_votes WHERE board_id = $1 AND card_id = $2`, f.boardID(), card.ID))
}

func TestCardVote_CrossTenantVotesAreOnlyReachedThroughTheirBoard(t *testing.T) {
	// given a vote on board A's card
	bt := newBoardTenants(t)
	voterA := bt.boardA.Participants[0].ID
	_, err := bt.repos.CardVote.AddVote(t.Context(), spec.CardVote{
		OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, Version: 1, OpID: uuid.New(),
		CardID:        bt.cardA.ID,
		ParticipantID: voterA,
		MaxVotes:      6,
	})
	require.NoError(t, err)
	boardB := bt.boardB.Header.ID
	voterB := bt.boardB.Participants[0].ID

	// when
	_, plantErr := bt.repos.CardVote.IncrementVote(t.Context(), spec.CardVoteIncrement{BoardID: boardB, CardID: bt.cardA.ID, ParticipantID: voterB})
	_, foreignVoterErr := bt.repos.CardVote.IncrementVote(t.Context(), spec.CardVoteIncrement{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, ParticipantID: voterB})
	_, spendErr := bt.repos.CardVote.SpendBudget(t.Context(), spec.VoteBudgetSpend{BoardID: boardB, ParticipantID: voterA, MaxVotes: 6})
	_, lowerErr := bt.repos.CardVote.LowerVote(t.Context(), spec.CardVoteKey{BoardID: boardB, CardID: bt.cardA.ID, ParticipantID: voterA})
	dropErr := bt.repos.CardVote.DropVote(t.Context(), spec.CardVoteKey{BoardID: boardB, CardID: bt.cardA.ID, ParticipantID: voterA})
	_, refundErr := bt.repos.CardVote.RefundBudget(t.Context(), spec.BoardParticipantKey{BoardID: boardB, ParticipantID: voterA})
	refunded, refundCardErr := bt.repos.CardVote.RefundCard(t.Context(), spec.CardRef{BoardID: boardB, CardID: bt.cardA.ID})
	listed, listErr := bt.repos.CardVote.List(t.Context(), boardB)
	mine, mineErr := bt.repos.CardVote.ListForParticipant(t.Context(), spec.BoardParticipantKey{BoardID: boardB, ParticipantID: voterA})
	deleted, deleteErr := bt.repos.CardVote.DeleteAll(t.Context(), boardB)
	_, totalErr := bt.repos.Card.AddVoteTotal(t.Context(), spec.CardVoteTotalChange{BoardID: boardB, CardID: bt.cardA.ID, Delta: 1})
	_, resetErr := bt.repos.CardVote.ResetVotes(t.Context(), bt.boardAThroughB())

	// then a vote on board A can never be planted, read, taken back or refunded through board B
	require.ErrorIs(t, plantErr, dao.ErrNotFound)
	require.ErrorIs(t, foreignVoterErr, dao.ErrNotFound)
	require.ErrorIs(t, spendErr, dao.ErrNotFound)
	require.ErrorIs(t, lowerErr, dao.ErrNotFound)
	require.ErrorIs(t, dropErr, dao.ErrNotFound)
	require.ErrorIs(t, refundErr, dao.ErrNotFound)
	require.ErrorIs(t, totalErr, dao.ErrNotFound)
	require.ErrorIs(t, resetErr, dao.ErrNotFound)
	require.NoError(t, refundCardErr)
	assert.Zero(t, refunded)
	require.NoError(t, listErr)
	assert.Empty(t, listed)
	require.NoError(t, mineErr)
	assert.Empty(t, mine)
	require.NoError(t, deleteErr)
	assert.Zero(t, deleted)

	votesA, err := bt.repos.CardVote.List(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	assert.Equal(t, []model.CardVote{{CardID: bt.cardA.ID, ParticipantID: voterA, Count: 1, CardAuthorParticipantID: new(voterA), CardColumnID: bt.cardA.ColumnID}}, votesA)
	cardA, err := bt.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID})
	require.NoError(t, err)
	assert.Equal(t, 1, cardA.VoteTotal)
}

func TestCardVote_SchemaConstraintsRefuseBadRows(t *testing.T) {
	tests := []struct {
		name       string
		statement  string
		args       func(f boardFixture, card model.Card) []any
		constraint string
	}{
		{
			name:       "a vote with no count",
			statement:  `INSERT INTO card_votes (board_id, card_id, participant_id, count) VALUES ($1, $2, $3, 0)`,
			args:       func(f boardFixture, card model.Card) []any { return []any{f.boardID(), card.ID, f.facilitator()} },
			constraint: "card_votes_count_check",
		},
		{
			name:       "a vote on a card of another board",
			statement:  `INSERT INTO card_votes (board_id, card_id, participant_id, count) VALUES ($1, $2, $3, 1)`,
			args:       func(f boardFixture, _ model.Card) []any { return []any{f.boardID(), uuid.New(), f.facilitator()} },
			constraint: "card_votes_card_fkey",
		},
		{
			name:       "a vote by a participant of another board",
			statement:  `INSERT INTO card_votes (board_id, card_id, participant_id, count) VALUES ($1, $2, $3, 1)`,
			args:       func(f boardFixture, card model.Card) []any { return []any{f.boardID(), card.ID, uuid.New()} },
			constraint: "card_votes_participant_fkey",
		},
		{
			name:       "a negative budget",
			statement:  `INSERT INTO board_vote_budgets (board_id, participant_id, used) VALUES ($1, $2, -1)`,
			args:       func(f boardFixture, _ model.Card) []any { return []any{f.boardID(), f.facilitator()} },
			constraint: "board_vote_budgets_used_check",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newBoardFixture(t)
			card := f.addCard(t, f.column(0), "a0")

			// when
			_, err := f.repos.DB().ExecContext(t.Context(), tc.statement, tc.args(f, card)...)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}
