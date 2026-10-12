package dao_test

import (
	"strings"
	"testing"

	"retrotide/internal/dao/daotest"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	boardInsert           = `INSERT INTO boards (org_id, team_id, title, share_slug, link_access) VALUES ($1, $2, $3, $4, $5)`
	participantInsert     = `INSERT INTO board_participants (board_id, user_id, display_name, role) VALUES ($1, $2::uuid, $3, $4)`
	participantPathInsert = `INSERT INTO board_participants (board_id, joined_via) VALUES ($1, $2)`
	boardUpdate           = `UPDATE boards SET anonymity = $2, sort_mode = $3, max_votes = $4, max_votes_per_card = $5 WHERE id = $1`
	timerUpdate           = `UPDATE boards SET timer_duration_ms = $2::int, timer_ends_at = $3::timestamptz, timer_paused_remaining_ms = $4::int WHERE id = $1`
	columnInsert          = `INSERT INTO board_columns (id, board_id, title, colour, position) VALUES ($1, $2, $3, $4, 'a0')`
	cardInsert            = `INSERT INTO cards (id, board_id, column_id, parent_card_id, body, colour, position) VALUES ($1, $2, $3, $4::uuid, $5, $6::text, 'a0')`
)

func TestBoard_SchemaConstraintsRefuseBadRows(t *testing.T) {
	tests := []struct {
		name       string
		statement  string
		args       func(f boardFixture) []any
		constraint string
	}{
		{
			name:      "a board in a team its organisation does not have",
			statement: boardInsert,
			args: func(f boardFixture) []any {
				return []any{f.orgID(), uuid.New(), "Planted", uuid.NewString(), "off"}
			},
			constraint: "boards_team_fkey",
		},
		{
			name:       "an unknown link access",
			statement:  boardInsert,
			args:       func(f boardFixture) []any { return f.boardArgs("Retro", "everyone") },
			constraint: "boards_link_access_check",
		},
		{
			name:       "an empty board title",
			statement:  boardInsert,
			args:       func(f boardFixture) []any { return f.boardArgs("", "off") },
			constraint: "boards_title_length",
		},
		{
			name:       "a board title over 200 characters",
			statement:  boardInsert,
			args:       func(f boardFixture) []any { return f.boardArgs(strings.Repeat("t", 201), "off") },
			constraint: "boards_title_length",
		},
		{
			name:      "a share slug another board holds",
			statement: boardInsert,
			args: func(f boardFixture) []any {
				return []any{f.orgID(), f.workspace.Team.ID, "Copy", f.state.Header.ShareSlug, "off"}
			},
			constraint: "boards_share_slug_key",
		},
		{
			name:       "a second participant row for one user",
			statement:  participantInsert,
			args:       func(f boardFixture) []any { return []any{f.boardID(), f.workspace.Owner.UserID, "", "participant"} },
			constraint: "board_participants_board_user_key",
		},
		{
			name:       "an unknown board role",
			statement:  participantInsert,
			args:       func(f boardFixture) []any { return []any{f.boardID(), nil, "", "owner"} },
			constraint: "board_participants_role_check",
		},
		{
			name:       "a display name over 40 characters",
			statement:  participantInsert,
			args:       func(f boardFixture) []any { return []any{f.boardID(), nil, strings.Repeat("n", 41), "participant"} },
			constraint: "board_participants_display_name_length",
		},
		{
			name:       "an unknown way in",
			statement:  participantPathInsert,
			args:       func(f boardFixture) []any { return []any{f.boardID(), "invite"} },
			constraint: "board_participants_joined_via_check",
		},
		{
			name:       "an unknown anonymity",
			statement:  boardUpdate,
			args:       func(f boardFixture) []any { return []any{f.boardID(), "secret", "manual", 6, 0} },
			constraint: "boards_anonymity_check",
		},
		{
			name:       "an unknown sort mode",
			statement:  boardUpdate,
			args:       func(f boardFixture) []any { return []any{f.boardID(), "named", "random", 6, 0} },
			constraint: "boards_sort_mode_check",
		},
		{
			name:       "a hundred votes per board",
			statement:  boardUpdate,
			args:       func(f boardFixture) []any { return []any{f.boardID(), "named", "manual", 100, 0} },
			constraint: "boards_max_votes_check",
		},
		{
			name:       "a negative cap per card",
			statement:  boardUpdate,
			args:       func(f boardFixture) []any { return []any{f.boardID(), "named", "manual", 6, -1} },
			constraint: "boards_max_votes_per_card_check",
		},
		{
			name:       "a timer both running and paused",
			statement:  timerUpdate,
			args:       func(f boardFixture) []any { return []any{f.boardID(), 60000, "2026-10-04T10:00:00Z", 30000} },
			constraint: "boards_timer_state",
		},
		{
			name:       "a timer shorter than a second",
			statement:  timerUpdate,
			args:       func(f boardFixture) []any { return []any{f.boardID(), 999, nil, nil} },
			constraint: "boards_timer_duration",
		},
		{
			name:       "an unknown column colour",
			statement:  columnInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.boardID(), "Ideas", "orange"} },
			constraint: "board_columns_colour_check",
		},
		{
			name:       "an empty column title",
			statement:  columnInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.boardID(), "", "grey"} },
			constraint: "board_columns_title_length",
		},
		{
			name:       "a column title over 80 characters",
			statement:  columnInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.boardID(), strings.Repeat("c", 81), "grey"} },
			constraint: "board_columns_title_length",
		},
		{
			name:       "a card in a column of another board",
			statement:  cardInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.boardID(), uuid.New(), nil, "Lost", nil} },
			constraint: "cards_column_fkey",
		},
		{
			name:      "a card that is its own parent",
			statement: cardInsert,
			args: func(f boardFixture) []any {
				id := uuid.New()
				return []any{id, f.boardID(), f.column(0), id, "Loop", nil}
			},
			constraint: "cards_not_own_parent",
		},
		{
			name:       "an empty card",
			statement:  cardInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.boardID(), f.column(0), nil, "", nil} },
			constraint: "cards_body_length",
		},
		{
			name:      "a card over 2000 characters",
			statement: cardInsert,
			args: func(f boardFixture) []any {
				return []any{uuid.New(), f.boardID(), f.column(0), nil, strings.Repeat("b", 2001), nil}
			},
			constraint: "cards_body_length",
		},
		{
			name:       "an unknown card colour",
			statement:  cardInsert,
			args:       func(f boardFixture) []any { return []any{uuid.New(), f.boardID(), f.column(0), nil, "Body", "orange"} },
			constraint: "cards_colour_check",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			f := newBoardFixture(t)

			// when
			_, err := f.repos.DB().ExecContext(t.Context(), tc.statement, tc.args(f)...)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.constraint)
		})
	}
}

func TestBoard_ANegativeVoteTotalIsRefused(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")

	// when
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE cards SET vote_total = -1 WHERE board_id = $1 AND id = $2`, f.boardID(), card.ID)

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cards_vote_total_check")
}

func TestBoard_DeletingAParentCardLeavesItsStack(t *testing.T) {
	// given a card merged under another
	f := newBoardFixture(t)
	parent := f.addCard(t, f.column(0), "a0")
	child := f.addCard(t, f.column(0), "a1")
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE cards SET parent_card_id = $3 WHERE board_id = $1 AND id = $2`, f.boardID(), child.ID, parent.ID)
	require.NoError(t, err)

	// when
	_, err = f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: parent.ID})

	// then the child stays on the board, unmerged
	require.NoError(t, err)
	kept, getErr := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: child.ID})
	require.NoError(t, getErr)
	assert.Nil(t, kept.ParentCardID)
}

func TestBoard_RemovingAnAuthorKeepsTheirCards(t *testing.T) {
	// given
	f := newBoardFixture(t)
	card := f.addCard(t, f.column(0), "a0")

	// when
	_, err := f.repos.DB().ExecContext(t.Context(), `DELETE FROM board_participants WHERE board_id = $1 AND id = $2`, f.boardID(), f.facilitator())

	// then
	require.NoError(t, err)
	kept, getErr := f.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: f.boardID(), CardID: card.ID})
	require.NoError(t, getErr)
	assert.Nil(t, kept.AuthorParticipantID)
	assert.Equal(t, card.Body, kept.Body)
}

func TestBoard_DeletingATeamCascadesToItsBoards(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.addCard(t, f.column(0), "a0")
	daotest.CreateOrganisationBoard(t, f.repos, f.orgID(), f.workspace.Owner.UserID)

	// when
	_, err := f.repos.Team.Delete(t.Context(), spec.TeamKey{OrgID: f.orgID(), TeamID: f.workspace.Team.ID})

	// then the team's board and everything under it are gone, and the organisation board stays
	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, f.repos, "boards"))
	assert.Zero(t, countRows(t, f.repos, "cards"))
}

func TestBoard_DeletingAnOrganisationCascadesToItsBoards(t *testing.T) {
	// given
	f := newBoardFixture(t)
	f.addCard(t, f.column(0), "a0")

	// when
	_, err := f.repos.DB().ExecContext(t.Context(), `DELETE FROM organisations WHERE id = $1`, f.orgID())

	// then
	require.NoError(t, err)
	for _, table := range []string{"boards", "board_participants", "board_ops", "board_columns", "cards"} {
		assert.Zero(t, countRows(t, f.repos, table), table)
	}
}

func (f boardFixture) boardArgs(title string, linkAccess string) []any {
	return []any{f.orgID(), f.workspace.Team.ID, title, uuid.NewString(), linkAccess}
}
