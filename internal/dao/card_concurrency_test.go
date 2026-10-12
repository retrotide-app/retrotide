package dao_test

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"retrotide/internal/board"
	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/fracindex"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/orgauthz"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	concurrentInserts = 20
)

func gatedBoardService(t *testing.T, f boardFixture) board.Service {
	t.Helper()

	orgAuthz := orgauthz.NewMockService(t)
	orgAuthz.EXPECT().CanInTeam(mock.Anything, mock.Anything).Return(orgauthz.Decision{Member: true, Allowed: true}, nil)

	boardLive := live.New(live.Deps{Boards: f.repos.Board, Cards: f.repos.Card, OrgAuthz: orgAuthz})

	return board.NewService(board.Deps{
		Boards:        f.repos.Board,
		Columns:       f.repos.BoardColumn,
		Cards:         f.repos.Card,
		Votes:         f.repos.CardVote,
		Comments:      f.repos.CardComment,
		Teams:         f.repos.Team,
		Organisations: f.repos.Organisation,
		Live:          boardLive,
		Events:        boardLive,
		OrgAuthz:      orgAuthz,
	})
}

func TestCard_TwentyConcurrentInsertsAfterOneAnchorGetDistinctOrderedPositions(t *testing.T) {
	// given an anchor and a tail in the second column, and twenty cards waiting in the first
	f := newBoardFixture(t)
	anchor := f.addCard(t, f.column(1), "V")
	tail := f.addCard(t, f.column(1), "k")

	movers := make([]model.Card, 0, concurrentInserts)
	for _, position := range fracindex.Spread(concurrentInserts) {
		movers = append(movers, f.addCard(t, f.column(0), position))
	}

	boards := gatedBoardService(t, f)
	principal := model.Principal{Kind: model.PrincipalUser, ID: f.workspace.Owner.UserID}
	before := f.version(t)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		responses []dto.BoardCommandResponse
		failures  []error
	)

	// when every card is moved to just after the anchor at once
	for _, mover := range movers {
		wg.Go(func() {
			moved, err := boards.MoveCard(t.Context(), board.MoveCardCommand{
				Principal: principal, BoardID: f.boardID(), OpID: uuid.New(),
				CardID:      mover.ID,
				ColumnID:    f.column(1),
				AfterCardID: new(anchor.ID),
			})

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failures = append(failures, err)
				return
			}

			responses = append(responses, *moved)
		})
	}
	wg.Wait()

	// then the gate serialises them, so each lands between the anchor and the one before it
	require.Empty(t, failures)
	require.Len(t, responses, concurrentInserts)
	assert.Equal(t, before+concurrentInserts, f.version(t))

	slices.SortFunc(responses, func(a dto.BoardCommandResponse, b dto.BoardCommandResponse) int {
		return int(a.Version - b.Version)
	})
	for i, response := range responses {
		assert.Equal(t, before+int64(i)+1, response.Version)
		require.NotNil(t, response.Card)
		assert.Greater(t, response.Card.Position, anchor.Position)
		assert.Less(t, response.Card.Position, tail.Position)
		if i > 0 {
			assert.Less(t, response.Card.Position, responses[i-1].Card.Position, "a later insert after the anchor sorts before the earlier ones")
		}
	}

	listed, err := f.repos.Card.ListInColumn(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(1)})
	require.NoError(t, err)
	require.Len(t, listed, concurrentInserts+2)

	positions := make([]string, 0, len(listed))
	for _, card := range listed {
		positions = append(positions, card.Position)
	}
	assert.True(t, slices.IsSortedFunc(positions, strings.Compare))
	assert.Len(t, slices.Compact(slices.Clone(positions)), concurrentInserts+2, "every position is distinct")
	assert.Equal(t, anchor.ID, listed[0].ID)
	assert.Equal(t, tail.ID, listed[len(listed)-1].ID)
	for i, response := range responses {
		assert.Equal(t, response.Card.ID, listed[concurrentInserts-i].ID)
	}
}
