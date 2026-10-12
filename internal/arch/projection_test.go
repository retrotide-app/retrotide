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
	dtoImport     = "retrotide/internal/dto"
	projectionDir = "internal/board/projection"
)

var (
	boardDTOs = []string{
		"BoardSnapshot",
		"BoardView",
		"ViewerView",
		"ColumnView",
		"CardView",
		"AuthorView",
		"ParticipantView",
		"BoardCommandResponse",
		"BoardListItem",
		"BoardListResponse",
		"OrgBoardListItem",
		"OrgBoardListResponse",
		"BoardEventsFrame",
		"BoardEventHead",
		"BoardUpdatedEvent",
		"ParticipantUpsertedEvent",
		"ColumnUpsertedEvent",
		"ColumnMovedEvent",
		"ColumnRemovedEvent",
		"CardUpsertedEvent",
		"CardMovedEvent",
		"CardDeletedEvent",
		"HiddenCountsEvent",
		"SettingsView",
		"MeView",
		"VotesView",
		"CommentView",
		"SettingsChangedEvent",
		"VotesChangedEvent",
		"CommentUpsertedEvent",
		"CommentDeletedEvent",
		"CardsMergedEvent",
		"CardUnmergedEvent",
		"BoardMeFrame",
		"PositionView",
		"BoardPresenceFrame",
		"PresenceView",
		"JoinPreview",
		"LinkJoinResponse",
		"SharingView",
		"GuestClaimOffer",
		"GuestClaimBoard",
		"GuestClaimResult",
		"TimerView",
		"CardsRevealedEvent",
		"CardsHiddenEvent",
		"ParticipantRemovedEvent",
		"TimerChangedEvent",
		"PersonView",
		"ActionItemView",
		"PreviousActionView",
		"ActionItemUpsertedEvent",
		"ActionItemRemovedEvent",
		"PreviousActionUpsertedEvent",
		"BoardPeopleResponse",
		"ActionItemBoardView",
		"ActionItemListItem",
		"ActionItemCountsView",
		"ActionItemListResponse",
		"MentionView",
		"ActionItemMentionView",
		"TextMentionView",
		"MentionListResponse",
		"BoardExport",
		"ExportColumn",
		"ExportCard",
		"ExportComment",
		"ExportActionItem",
	}
)

func TestOnlyTheProjectionBuildsBoardDTOs(t *testing.T) {
	// given
	files, err := loadRepo()
	require.NoError(t, err)

	// when
	offenders := projectionOffenders(files)

	// then
	assert.Empty(t, offenders, "these build a board DTO outside internal/board/projection, the one place that decides what a viewer may see of hidden cards and anonymous authorship; call a projection function instead:\n%s", strings.Join(offenders, "\n"))
}

func TestProjectionOffenders_FlagsABoardDTOBuiltOutsideTheProjection(t *testing.T) {
	tests := []struct {
		name string
		path string
		src  string
		want int
	}{
		{name: "a card view in the board service", path: "internal/board/cards.go", src: "import \"retrotide/internal/dto\"\n\nvar view = dto.CardView{Body: body}\n", want: 1},
		{name: "a pointer to a response in a controller", path: "internal/controllers/board_controller.go", src: "import d \"retrotide/internal/dto\"\n\nvar response = &d.BoardCommandResponse{Version: 1}\n", want: 1},
		{name: "a slice of card views with elided element types", path: "internal/board/live/outbox.go", src: "import \"retrotide/internal/dto\"\n\nvar cards = []dto.CardView{{Body: body}}\n", want: 1},
		{name: "a map of column views", path: "internal/board/columns.go", src: "import \"retrotide/internal/dto\"\n\nvar columns = map[string]dto.ColumnView{}\n", want: 1},
		{name: "a snapshot in the projection", path: "internal/board/projection/projection.go", src: "import \"retrotide/internal/dto\"\n\nvar snapshot = dto.BoardSnapshot{Version: 1}\n", want: 0},
		{name: "a request DTO in a controller", path: "internal/controllers/board_controller.go", src: "import \"retrotide/internal/dto\"\n\nvar request = dto.CreateCardRequest{Body: body}\n", want: 0},
		{name: "a declared variable of a board DTO type", path: "internal/board/cards.go", src: "import \"retrotide/internal/dto\"\n\nvar response *dto.BoardCommandResponse\n", want: 0},
		{name: "a board DTO in a test", path: "internal/controllers/board_controller_test.go", src: "import \"retrotide/internal/dto\"\n\nvar response = dto.BoardCommandResponse{Version: 1}\n", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			file := parseFixture(t, tc.path, "package fixture\n\n"+tc.src)

			// when
			offenders := projectionOffenders([]sourceFile{file})

			// then
			assert.Len(t, offenders, tc.want, "%v", offenders)
		})
	}
}

func projectionOffenders(files []sourceFile) []string {
	var offenders []string
	for _, file := range files {
		if file.test || within(file.dir, projectionDir) {
			continue
		}

		alias, imported := file.localName(dtoImport)
		if !imported {
			continue
		}

		ast.Inspect(file.syntax, func(node ast.Node) bool {
			literal, isLiteral := node.(*ast.CompositeLit)
			if !isLiteral || literal.Type == nil {
				return true
			}

			if name, builds := boardDTOIn(literal.Type, alias); builds {
				offenders = append(offenders, file.offence(literal, "builds dto."+name))
			}

			return true
		})
	}

	return offenders
}

func boardDTOIn(expr ast.Expr, alias string) (string, bool) {
	var found string
	ast.Inspect(expr, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}

		pkg, isIdent := selector.X.(*ast.Ident)
		if isIdent && pkg.Name == alias && slices.Contains(boardDTOs, selector.Sel.Name) {
			found = selector.Sel.Name
		}

		return found == ""
	})

	return found, found != ""
}
