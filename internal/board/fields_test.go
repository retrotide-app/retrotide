package board

import (
	"testing"

	"retrotide/internal/boardtemplate"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplates_EveryColumnPassesTheColumnRules(t *testing.T) {
	for _, template := range boardtemplate.All() {
		for _, column := range template.Columns {
			// when
			cleaned, err := CleanColumn(ColumnFields{Title: column.Title, Description: column.Description, Colour: column.Colour})

			// then a board made from the template holds exactly the columns a facilitator could add by hand
			require.NoError(t, err, "%s: %s", template.Key, column.Title)
			assert.Equal(t, ColumnFields{Title: column.Title, Description: column.Description, Colour: column.Colour}, cleaned)
		}
	}
}

func TestPlace_SpreadsEveryKeyWhenTheNewOneWouldNotFit(t *testing.T) {
	// given three siblings tied on one key
	siblings := []placed{{id: firstCardID, position: "V"}, {id: secondCardID, position: "V"}, {id: thirdCardID, position: "V"}}

	// when
	got := place(siblings, &secondCardID)

	// then every sibling keeps its order and the new item lands after its anchor
	assert.Equal(t, "b", got.position)
	require.Len(t, got.rewrite, 3)
	assert.Equal(t, firstCardID, got.rewrite[0].ID)
	assert.Equal(t, secondCardID, got.rewrite[1].ID)
	assert.Equal(t, thirdCardID, got.rewrite[2].ID)
	assert.Less(t, got.rewrite[0].Position, got.rewrite[1].Position)
	assert.Less(t, got.rewrite[1].Position, got.position)
	assert.Less(t, got.position, got.rewrite[2].Position)
}

func TestPlace_LeavesTheOthersAloneWhenTheKeyFits(t *testing.T) {
	// given
	siblings := []placed{{id: firstCardID, position: "V"}, {id: secondCardID, position: "k"}}

	// when
	got := place(siblings, &firstCardID)

	// then
	assert.Equal(t, placement{position: "c"}, got)
}
