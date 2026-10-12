package arch

import (
	"go/ast"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	repositoryImport = "retrotide/internal/repository"
)

var (
	boardWriters = []string{"internal/board", "internal/repository", "internal/dao", "internal/store"}

	boardWriteMethods = []string{
		"CreateBoard",
		"SetBoardContext",
		"JoinParticipant",
		"AddColumn",
		"EditColumn",
		"MoveColumn",
		"DeleteColumn",
		"AddCard",
		"EditCard",
		"RecolourCard",
		"MoveCard",
		"DeleteCard",
		"ClearCards",
		"BumpVersion",
		"RecordOp",
		"AddParticipant",
		"JoinLinkParticipant",
		"UpsertLinkParticipant",
		"SetBoardLinkAccess",
		"SetBoardPassword",
		"RotateBoardLink",
		"ClaimGuestParticipant",
		"ReassignParticipant",
		"DeleteParticipant",
		"ReassignAuthor",
		"MergeCard",
		"Restack",
		"AddVote",
		"RemoveVote",
		"ResetVotes",
		"SpendBudget",
		"RefundBudget",
		"IncrementVote",
		"LowerVote",
		"DropVote",
		"RefundCard",
		"RefundColumn",
		"DeleteBudgets",
		"HandOverVotes",
		"HandOverBudget",
		"AddVoteTotal",
		"ResetVoteTotals",
		"AddComment",
		"EditComment",
		"DeleteComment",
		"SetBoardSettings",
		"SetSettings",
		"RevealCards",
		"HideCards",
		"SetCardsHidden",
		"CloseBoard",
		"SeverAuthors",
		"SetBoardTimer",
		"ChangeParticipantRole",
		"SetParticipantRole",
		"RemoveBoardParticipant",
		"RemoveParticipant",
		"AddActionItem",
		"EditActionItem",
		"SetActionItemStatus",
		"DeleteActionItem",
		"ReviewActionItem",
		"CreateItem",
		"EditItem",
		"SetItemStatus",
		"DeleteItem",
		"AddMention",
		"RemoveMention",
		"RecordReview",
	}
)

func TestOnlyTheBoardPackageWritesABoard(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := boardWriteOffenders(files)

	// then
	assert.Empty(t, offenders, "these call a repository method that writes a board outside internal/board, so the write skips the board's gate and its version bump; hand the write to the board service instead:\n%s", strings.Join(offenders, "\n"))
}

func TestBoardWriteOffenders_FlagsABoardWriteOutsideTheBoardPackage(t *testing.T) {
	tests := []struct {
		name string
		path string
		src  string
		want int
	}{
		{name: "a card write from the team service", path: "internal/team/actions.go", src: "import \"retrotide/internal/repository\"\n\nfunc plant(cards repository.CardRepository) {\n\tcards.AddCard(nil, spec)\n}\n", want: 1},
		{name: "a join from the organisation service", path: "internal/org/members.go", src: "import r \"retrotide/internal/repository\"\n\nfunc join(boards r.BoardRepository) {\n\tboards.JoinParticipant(nil, spec)\n\tboards.BumpVersion(nil, write)\n}\n", want: 2},
		{name: "a card write in the board service", path: "internal/board/cards.go", src: "import \"retrotide/internal/repository\"\n\nfunc add(cards repository.CardRepository) {\n\tcards.AddCard(nil, spec)\n}\n", want: 0},
		{name: "a join in the live board service", path: "internal/board/live/join.go", src: "import \"retrotide/internal/repository\"\n\nfunc join(boards repository.BoardRepository) {\n\tboards.JoinParticipant(nil, spec)\n}\n", want: 0},
		{name: "a composite calling its DAO", path: "internal/repository/board.go", src: "func (r *boardRepository) join() {\n\tr.BoardDAO.AddParticipant(nil, spec)\n}\n", want: 0},
		{name: "a controller calling the board service's method of the same name", path: "internal/controllers/board_card_controller.go", src: "import \"retrotide/internal/board\"\n\nfunc add(s board.Service) {\n\ts.AddCard(nil, command)\n}\n", want: 0},
		{name: "a test outside the services", path: "internal/dao/card_test.go", src: "import \"retrotide/internal/repository\"\n\nfunc add(cards repository.CardRepository) {\n\tcards.AddCard(nil, spec)\n}\n", want: 0},
		{name: "a read from another service", path: "internal/team/boards.go", src: "import \"retrotide/internal/repository\"\n\nfunc read(cards repository.CardRepository) {\n\tcards.ListInColumn(nil, key)\n}\n", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, tc.path, "package fixture\n\n"+tc.src)

			// when
			offenders := boardWriteOffenders([]sourceFile{file})

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func boardWriteOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.test || within(file.dir, boardWriters...) {
			continue
		}

		if _, imported := file.localName(repositoryImport); !imported {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}

			selector, isSelector := call.Fun.(*ast.SelectorExpr)
			if isSelector && slices.Contains(boardWriteMethods, selector.Sel.Name) {
				offenders = append(offenders, file.offence(call, "calls the board write "+selector.Sel.Name))
			}

			return true
		})
	}

	return offenders
}
