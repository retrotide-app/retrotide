package dao_test

import (
	"testing"
	"time"

	"retrotide/internal/dao"
	"retrotide/internal/dao/daotest"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f boardFixture) mentionCard(t *testing.T, author uuid.UUID, body string, mentions ...uuid.UUID) model.Card {
	t.Helper()

	added, err := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		BoardWrite:          f.write(t),
		CardID:              uuid.New(),
		ColumnID:            f.column(0),
		AuthorParticipantID: new(author),
		Body:                body,
		Position:            "a" + uuid.NewString()[:4],
		Mentions:            mentions,
	})
	require.NoError(t, err)

	return added.Value
}

func (f boardFixture) mentionComment(t *testing.T, cardID, author uuid.UUID, body string, mentions ...uuid.UUID) model.CardComment {
	t.Helper()

	added, err := f.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{
		BoardWrite:          f.write(t),
		CommentID:           uuid.New(),
		CardID:              cardID,
		AuthorParticipantID: new(author),
		Body:                body,
		Mentions:            mentions,
	})
	require.NoError(t, err)

	return added.Value
}

func (f boardFixture) seat(t *testing.T, user *model.User) uuid.UUID {
	t.Helper()

	return daotest.CreateParticipant(t, f.repos, f.boardID(), user.ID).ID
}

func (f boardFixture) mentionedAt(t *testing.T, statement string, id, userID uuid.UUID) time.Time {
	t.Helper()

	var at time.Time
	require.NoError(t, f.repos.DB().QueryRowContext(t.Context(), statement, id, userID).Scan(&at))

	return at
}

func TestCardMention_AddStoresTheMentionsAndEveryReadCarriesThemWithNames(t *testing.T) {
	// given
	f := newBoardFixture(t)
	ben := f.teamMember(t, "Ben")
	ana := f.teamMember(t, "Ana")
	before := f.version(t)

	// when
	added := f.mentionCard(t, f.facilitator(), "Thanks @Ben and @Ana", ben.ID, ana.ID)

	// then the mentions read back sorted by name, from the write, a single read and every list
	want := []model.ActionItemPerson{{UserID: ana.ID, DisplayName: "Ana"}, {UserID: ben.ID, DisplayName: "Ben"}}
	assert.Equal(t, want, added.Mentions)
	assert.Equal(t, before+1, f.version(t))
	assert.Equal(t, want, f.card(t, added.ID).Mentions)
	listed, err := f.repos.Card.List(t.Context(), f.boardID())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, want, listed[0].Mentions)
	inColumn, err := f.repos.Card.ListInColumn(t.Context(), spec.BoardColumnKey{BoardID: f.boardID(), ColumnID: f.column(0)})
	require.NoError(t, err)
	require.Len(t, inColumn, 1)
	assert.Equal(t, want, inColumn[0].Mentions)
}

func TestCardMention_ACardWithoutMentionsReadsBackWithNone(t *testing.T) {
	// given
	f := newBoardFixture(t)

	// when
	added := f.addCard(t, f.column(0), "a0")

	// then
	assert.Nil(t, added.Mentions)
	assert.Nil(t, f.card(t, added.ID).Mentions)
}

func TestCardMention_EditKeepsAnUnchangedMentionsRowAndDropsTheRest(t *testing.T) {
	// given a card naming Ana and Ben
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	ben := f.teamMember(t, "Ben")
	cleo := f.teamMember(t, "Cleo")
	added := f.mentionCard(t, f.facilitator(), "@Ana and @Ben", ana.ID, ben.ID)
	const benRow = `SELECT created_at FROM card_mentions WHERE card_id = $1 AND user_id = $2`
	benFirstNamed := f.mentionedAt(t, benRow, added.ID, ben.ID)

	// when the edit keeps Ben, drops Ana and adds Cleo
	edited, err := f.repos.Card.EditCard(t.Context(), spec.CardBodyWrite{BoardWrite: f.write(t), CardID: added.ID, Body: "@Ben and @Cleo", Mentions: []uuid.UUID{ben.ID, cleo.ID}})

	// then
	require.NoError(t, err)
	assert.Equal(t, []model.ActionItemPerson{{UserID: ben.ID, DisplayName: "Ben"}, {UserID: cleo.ID, DisplayName: "Cleo"}}, edited.Value.Mentions)
	assert.Equal(t, benFirstNamed, f.mentionedAt(t, benRow, added.ID, ben.ID), "a kept mention keeps the time it was first made")

	// and an edit that sends none drops them all
	cleared, err := f.repos.Card.EditCard(t.Context(), spec.CardBodyWrite{BoardWrite: f.write(t), CardID: added.ID, Body: "Nobody"})
	require.NoError(t, err)
	assert.Nil(t, cleared.Value.Mentions)
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_mentions WHERE card_id = $1`, added.ID))
}

func TestCardMention_RecolouringKeepsTheMentions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	added := f.mentionCard(t, f.facilitator(), "@Ana", ana.ID)

	// when
	recoloured, err := f.repos.Card.RecolourCard(t.Context(), spec.CardColourWrite{BoardWrite: f.write(t), CardID: added.ID, Colour: new(model.ColourAmber)})

	// then
	require.NoError(t, err)
	assert.Equal(t, []model.ActionItemPerson{{UserID: ana.ID, DisplayName: "Ana"}}, recoloured.Value.Mentions)
}

func TestCardMention_OnlyAMemberOfTheBoardsOrganisationCanBeMentioned(t *testing.T) {
	// given someone outside the organisation
	f := newBoardFixture(t)
	stranger := daotest.CreateUser(t, f.repos)
	before := f.version(t)
	cardID := uuid.New()

	// when
	_, cardErr := f.repos.Card.AddCard(t.Context(), spec.CardCreate{
		BoardWrite: f.write(t), CardID: cardID, ColumnID: f.column(0), AuthorParticipantID: new(f.facilitator()), Body: "@Stranger", Position: "a0", Mentions: []uuid.UUID{stranger.ID},
	})

	// then the composite key refuses it, and the card and its version roll back with it
	require.ErrorIs(t, cardErr, dao.ErrNotFound)
	assert.Equal(t, before, f.version(t))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM cards WHERE id = $1`, cardID))
}

func TestCommentMention_AddAndEditFollowTheSameRules(t *testing.T) {
	// given
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	ben := f.teamMember(t, "Ben")
	card := f.addCard(t, f.column(0), "a0")

	// when
	added := f.mentionComment(t, card.ID, f.facilitator(), "Agreed, @Ana", ana.ID)
	edited, err := f.repos.CardComment.EditComment(t.Context(), spec.CommentBodyWrite{BoardWrite: f.write(t), CommentID: added.ID, Body: "Agreed, @Ben", Mentions: []uuid.UUID{ben.ID}})

	// then
	assert.Equal(t, []model.ActionItemPerson{{UserID: ana.ID, DisplayName: "Ana"}}, added.Mentions)
	require.NoError(t, err)
	assert.Equal(t, []model.ActionItemPerson{{UserID: ben.ID, DisplayName: "Ben"}}, edited.Value.Mentions)
	listed, err := f.repos.CardComment.List(t.Context(), f.boardID())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, edited.Value.Mentions, listed[0].Mentions)

	stranger := daotest.CreateUser(t, f.repos)
	_, strangerErr := f.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{BoardWrite: f.write(t), CommentID: uuid.New(), CardID: card.ID, AuthorParticipantID: new(f.facilitator()), Body: "@Stranger", Mentions: []uuid.UUID{stranger.ID}})
	require.ErrorIs(t, strangerErr, dao.ErrNotFound)
}

func TestCardMention_DeletingTheCardOrTheCommentDeletesItsMentions(t *testing.T) {
	// given a mentioning card with a mentioning comment, and a second mentioning comment on another card
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	doomed := f.mentionCard(t, f.facilitator(), "@Ana", ana.ID)
	f.mentionComment(t, doomed.ID, f.facilitator(), "Agreed, @Ana", ana.ID)
	kept := f.addCard(t, f.column(1), "a0")
	removed := f.mentionComment(t, kept.ID, f.facilitator(), "Thanks, @Ana", ana.ID)

	// when
	_, cardErr := f.repos.Card.DeleteCard(t.Context(), spec.CardRemoval{BoardWrite: f.write(t), CardID: doomed.ID})
	_, commentErr := f.repos.CardComment.DeleteComment(t.Context(), spec.CommentRemoval{BoardWrite: f.write(t), CommentID: removed.ID})

	// then
	require.NoError(t, cardErr)
	require.NoError(t, commentErr)
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_mentions WHERE board_id = $1`, f.boardID()))
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_comment_mentions WHERE board_id = $1`, f.boardID()))
}

func TestCardMention_RemovingAMemberDeletesTheirMentions(t *testing.T) {
	// given
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	card := f.mentionCard(t, f.facilitator(), "@Ana", ana.ID)
	f.mentionComment(t, card.ID, f.facilitator(), "Agreed, @Ana", ana.ID)

	// when
	_, err := f.repos.Organisation.RemoveMember(t.Context(), spec.OrganisationMemberKey{OrgID: f.orgID(), UserID: ana.ID})

	// then
	require.NoError(t, err)
	assert.Nil(t, f.card(t, card.ID).Mentions)
	assert.Zero(t, countWhere(t, f.repos, `SELECT COUNT(*) FROM card_comment_mentions WHERE user_id = $1`, ana.ID))
}

func TestCardMention_ForAUserNewestFirstWithWhetherTheCardIsStillHiddenFromThem(t *testing.T) {
	// given a hidden board with Ana's own card and the facilitator's card, both naming Ana, and a comment naming her
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	anaSeat := f.seat(t, ana)
	theirs := f.mentionCard(t, f.facilitator(), "Thanks @Ana", ana.ID)
	own := f.mentionCard(t, anaSeat, "Note to self, @Ana", ana.ID)
	comment := f.mentionComment(t, theirs.ID, f.facilitator(), "Agreed, @Ana", ana.ID)
	f.mentionCard(t, f.facilitator(), "Nobody")
	query := spec.UserMentionsQuery{UserID: ana.ID, Limit: 10}

	// when
	cards, cardErr := f.repos.Card.ListMentionsForUser(t.Context(), query)
	comments, commentErr := f.repos.CardComment.ListMentionsForUser(t.Context(), query)

	// then the rows say the board hides its cards, and which card is hers, so the projection can drop the rest
	require.NoError(t, cardErr)
	require.Len(t, cards, 2)
	assert.Equal(t, own.ID, cards[0].CardID)
	assert.Equal(t, model.MentionCard, cards[0].Kind)
	assert.True(t, cards[0].CardsHidden)
	assert.True(t, cards[0].OwnCard)
	assert.Equal(t, theirs.ID, cards[1].CardID)
	assert.True(t, cards[1].CardsHidden)
	assert.False(t, cards[1].OwnCard)
	assert.Equal(t, "Thanks @Ana", cards[1].Body)
	assert.Equal(t, new(f.boardID()), cards[1].BoardID)
	assert.Equal(t, f.state.Header.Title, cards[1].BoardTitle)
	assert.Equal(t, new(f.workspace.Team.ID), cards[1].TeamID)
	assert.Equal(t, f.workspace.Organisation.Slug, cards[1].OrgSlug)
	assert.True(t, cards[0].MentionedAt.After(cards[1].MentionedAt))

	require.NoError(t, commentErr)
	require.Len(t, comments, 1)
	assert.Equal(t, model.MentionComment, comments[0].Kind)
	assert.Equal(t, comment.ID, comments[0].CommentID)
	assert.Equal(t, theirs.ID, comments[0].CardID)
	assert.True(t, comments[0].CardsHidden)
	assert.False(t, comments[0].OwnCard, "a comment follows its card, not its writer")

	// and once the cards are revealed, nothing is hidden from her
	_, err := f.repos.Board.RevealCards(t.Context(), spec.CardsRevealWrite{BoardWrite: f.write(t)})
	require.NoError(t, err)
	revealed, err := f.repos.Card.ListMentionsForUser(t.Context(), query)
	require.NoError(t, err)
	for _, mention := range revealed {
		assert.False(t, mention.CardsHidden)
	}
}

func TestCardMention_ForAUserLeavesOutDeletedBoardsAndOrganisations(t *testing.T) {
	// given Ana named on a card and in a comment of the fixture's board, and on a card of a later board
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	card := f.mentionCard(t, f.facilitator(), "@Ana", ana.ID)
	f.mentionComment(t, card.ID, f.facilitator(), "Agreed, @Ana", ana.ID)
	later := boardFixture{repos: f.repos, workspace: f.workspace, state: f.laterBoard(t)}
	laterCard := later.mentionCard(t, later.facilitator(), "@Ana again", ana.ID)
	query := spec.UserMentionsQuery{UserID: ana.ID, Limit: 10}

	// when the first board is deleted
	f.deleteBoard(t, f.boardID())
	cards, cardErr := f.repos.Card.ListMentionsForUser(t.Context(), query)
	comments, commentErr := f.repos.CardComment.ListMentionsForUser(t.Context(), query)

	// then only the later board's mention is left
	require.NoError(t, cardErr)
	require.NoError(t, commentErr)
	require.Len(t, cards, 1)
	assert.Equal(t, laterCard.ID, cards[0].CardID)
	assert.Empty(t, comments)

	// and once the organisation is deleted, nothing is
	_, err := f.repos.Organisation.SoftDelete(t.Context(), f.orgID())
	require.NoError(t, err)
	gone, err := f.repos.Card.ListMentionsForUser(t.Context(), query)
	require.NoError(t, err)
	assert.Empty(t, gone)
}

func TestCardMention_AfterEndNothingLinksAMentionToItsWriter(t *testing.T) {
	// given an anonymous board where Ana named herself on her own card and in her own comment
	f := newBoardFixture(t)
	ana := f.teamMember(t, "Ana")
	anaSeat := f.seat(t, ana)
	_, err := f.repos.DB().ExecContext(t.Context(), `UPDATE boards SET anonymity = 'anonymous' WHERE id = $1`, f.boardID())
	require.NoError(t, err)
	card := f.mentionCard(t, anaSeat, "Note to self, @Ana", ana.ID)
	f.mentionComment(t, card.ID, anaSeat, "Still me, @Ana", ana.ID)
	query := spec.UserMentionsQuery{UserID: ana.ID, Limit: 10}

	// when the retro ends and severs authorship
	_, err = f.repos.Board.CloseBoard(t.Context(), spec.BoardCloseWrite{BoardWrite: f.write(t), Sever: true})

	// then the mentions stay, shown, and no longer say whose card it was
	require.NoError(t, err)
	cards, err := f.repos.Card.ListMentionsForUser(t.Context(), query)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.False(t, cards[0].CardsHidden)
	assert.False(t, cards[0].OwnCard, "nothing re-derives authorship once it is severed")
	comments, err := f.repos.CardComment.ListMentionsForUser(t.Context(), query)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.False(t, comments[0].OwnCard)
}

func TestCardMention_TheTablesStoreNoWriter(t *testing.T) {
	tests := []struct {
		table string
		want  []string
	}{
		{table: "card_mentions", want: []string{"board_id", "card_id", "created_at", "org_id", "user_id"}},
		{table: "card_comment_mentions", want: []string{"board_id", "comment_id", "created_at", "org_id", "user_id"}},
	}

	for _, tc := range tests {
		t.Run(tc.table, func(t *testing.T) {
			// given
			f := newBoardFixture(t)

			// when
			rows, err := f.repos.DB().QueryContext(t.Context(), `SELECT column_name FROM information_schema.columns WHERE table_name = $1 ORDER BY column_name`, tc.table)
			require.NoError(t, err)
			defer rows.Close()

			var columns []string
			for rows.Next() {
				var column string
				require.NoError(t, rows.Scan(&column))
				columns = append(columns, column)
			}
			require.NoError(t, rows.Err())

			// then a mention names only who was mentioned, so severing has nothing to miss
			assert.Equal(t, tc.want, columns)
		})
	}
}

func TestCardMention_CrossTenantMentionsAreOnlyReachedThroughTheirOrganisationAndBoard(t *testing.T) {
	// given card A of organisation A, Bob a member of organisation B only, and Alice a member of A
	bt := newBoardTenants(t)
	bob := daotest.CreateUser(t, bt.repos)
	daotest.AddOrgMember(t, bt.repos, bt.b.Organisation.ID, bob.ID, model.OrgRoleMember)
	onA := spec.CardMentionKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, UserID: bt.alice.ID}
	require.NoError(t, bt.repos.Card.AddMention(t.Context(), onA))

	// when
	bobOnA := bt.repos.Card.AddMention(t.Context(), spec.CardMentionKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, UserID: bob.ID})
	throughB := bt.repos.Card.AddMention(t.Context(), spec.CardMentionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID, UserID: bob.ID})
	onBoardB := bt.repos.Card.AddMention(t.Context(), spec.CardMentionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, UserID: bob.ID})
	removeErr := bt.repos.Card.RemoveMention(t.Context(), spec.CardMentionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID, UserID: bt.alice.ID})

	// then a member of B is never named on A's card, A's card is never reached through B, and A's mention is untouched
	require.ErrorIs(t, bobOnA, dao.ErrNotFound)
	require.ErrorIs(t, throughB, dao.ErrNotFound)
	require.ErrorIs(t, onBoardB, dao.ErrNotFound)
	require.NoError(t, removeErr)
	kept, err := bt.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: bt.boardA.Header.ID, CardID: bt.cardA.ID})
	require.NoError(t, err)
	require.Len(t, kept.Mentions, 1)
	assert.Equal(t, bt.alice.ID, kept.Mentions[0].UserID)
	_, err = bt.repos.Card.GetByID(t.Context(), spec.CardRef{BoardID: bt.boardB.Header.ID, CardID: bt.cardA.ID})
	require.ErrorIs(t, err, dao.ErrNotFound)

	bobsMentions, err := bt.repos.Card.ListMentionsForUser(t.Context(), spec.UserMentionsQuery{UserID: bob.ID, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, bobsMentions)
}

func TestCommentMention_CrossTenantMentionsAreOnlyReachedThroughTheirOrganisationAndBoard(t *testing.T) {
	// given a comment on card A of organisation A, and Bob a member of organisation B only
	bt := newBoardTenants(t)
	bob := daotest.CreateUser(t, bt.repos)
	daotest.AddOrgMember(t, bt.repos, bt.b.Organisation.ID, bob.ID, model.OrgRoleMember)
	header, err := bt.repos.Board.GetHeader(t.Context(), bt.boardA.Header.ID)
	require.NoError(t, err)
	added, err := bt.repos.CardComment.AddComment(t.Context(), spec.CommentCreate{
		OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, Version: header.Version, OpID: uuid.New(),
		CommentID:           uuid.New(),
		CardID:              bt.cardA.ID,
		AuthorParticipantID: new(bt.boardA.Participants[0].ID),
		Body:                "SECRET-COMMENT-3a9d",
		Mentions:            []uuid.UUID{bt.alice.ID},
	})
	require.NoError(t, err)
	commentID := added.Value.ID

	// when
	bobOnA := bt.repos.CardComment.AddMention(t.Context(), spec.CommentMentionKey{OrgID: bt.a.Organisation.ID, BoardID: bt.boardA.Header.ID, CommentID: commentID, UserID: bob.ID})
	throughB := bt.repos.CardComment.AddMention(t.Context(), spec.CommentMentionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardA.Header.ID, CommentID: commentID, UserID: bob.ID})
	onBoardB := bt.repos.CardComment.AddMention(t.Context(), spec.CommentMentionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, CommentID: commentID, UserID: bob.ID})
	removeErr := bt.repos.CardComment.RemoveMention(t.Context(), spec.CommentMentionKey{OrgID: bt.b.Organisation.ID, BoardID: bt.boardB.Header.ID, CommentID: commentID, UserID: bt.alice.ID})

	// then
	require.ErrorIs(t, bobOnA, dao.ErrNotFound)
	require.ErrorIs(t, throughB, dao.ErrNotFound)
	require.ErrorIs(t, onBoardB, dao.ErrNotFound)
	require.NoError(t, removeErr)
	kept, err := bt.repos.CardComment.GetByID(t.Context(), spec.CardCommentRef{BoardID: bt.boardA.Header.ID, CommentID: commentID})
	require.NoError(t, err)
	require.Len(t, kept.Mentions, 1)
	assert.Equal(t, bt.alice.ID, kept.Mentions[0].UserID)
}
