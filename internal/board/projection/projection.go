package projection

import (
	"cmp"
	"fmt"
	"slices"

	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	ViewerKind string

	Viewer struct {
		Kind          ViewerKind
		UserID        *uuid.UUID
		ParticipantID *uuid.UUID
		Role          model.BoardRole
		Capabilities  []model.BoardCapability
	}
)

const (
	ViewerMember ViewerKind = "member"
	ViewerGuest  ViewerKind = "guest"
	ViewerExport ViewerKind = "export"
)

const (
	timeLayout = "2006-01-02T15:04:05.000Z07:00"

	AnonymousName = "Anonymous"
)

func ViewerFor(access model.BoardAccess) Viewer {
	kind := ViewerGuest
	var userID *uuid.UUID
	if access.Principal.Kind == model.PrincipalUser {
		kind = ViewerMember
		userID = new(access.Principal.ID)
	}

	return Viewer{
		Kind:          kind,
		UserID:        userID,
		ParticipantID: access.ParticipantID,
		Role:          access.Role,
		Capabilities:  append([]model.BoardCapability{}, access.Capabilities...),
	}
}

func Visible(board model.Board, card model.Card, viewer Viewer) bool {
	if slices.Contains(board.HiddenColumnIDs, card.ColumnID) && !slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate) {
		return false
	}

	if !board.CardsHiddenIn(card.ColumnID) {
		return true
	}

	return Owns(card, viewer)
}

func VisibleToAll(board model.Board, card model.Card) bool {
	return !board.CardsHiddenIn(card.ColumnID) && !slices.Contains(board.HiddenColumnIDs, card.ColumnID)
}

func ColumnShown(board model.Board, columnID uuid.UUID, viewer Viewer) bool {
	return !slices.Contains(board.HiddenColumnIDs, columnID) || slices.Contains(viewer.Capabilities, model.BoardCapabilityFacilitate)
}

func BoardList(boards []model.BoardHeader) dto.BoardListResponse {
	items := make([]dto.BoardListItem, 0, len(boards))
	for _, header := range boards {
		items = append(items, boardListItem(header))
	}

	return dto.BoardListResponse{Boards: items}
}

func OrgBoardList(boards []model.BoardHeader, teamNames map[uuid.UUID]string, pinned map[uuid.UUID]struct{}) dto.OrgBoardListResponse {
	items := make([]dto.OrgBoardListItem, 0, len(boards))
	for _, header := range boards {
		_, isPinned := pinned[header.ID]

		item := dto.OrgBoardListItem{
			BoardListItem: boardListItem(header),
			TemplateKey:   header.Board.TemplateKey,
			Pinned:        isPinned,
		}

		if header.TeamID != nil {
			item.TeamName = teamNames[*header.TeamID]
		}

		items = append(items, item)
	}

	return dto.OrgBoardListResponse{Boards: items}
}

func Snapshot(state model.BoardState, viewer Viewer) dto.BoardSnapshot {
	header := state.Header
	board := header.Board

	profiles := make(map[uuid.UUID]model.BoardParticipantProfile, len(state.Participants))
	participants := make([]dto.ParticipantView, 0, len(state.Participants))
	for _, profile := range state.Participants {
		profiles[profile.ID] = profile

		if profile.Removed() {
			continue
		}
		participants = append(participants, dto.ParticipantView{
			ParticipantID: profile.ID,
			DisplayName:   ParticipantName(board, profile),
			Role:          profile.Role,
		})
	}

	columns := make([]dto.ColumnView, 0, len(state.Columns))
	for _, column := range state.Columns {
		if !ColumnShown(board, column.ID, viewer) {
			continue
		}

		columns = append(columns, columnView(board, column))
	}

	reactions := reactionsByCard(state.Reactions)

	cards := make([]dto.CardView, 0, len(state.Cards))
	shown := make(map[uuid.UUID]struct{}, len(state.Cards))
	hiddenCounts := map[uuid.UUID]int{}
	for _, card := range state.Cards {
		if !ColumnShown(board, card.ColumnID, viewer) {
			continue
		}

		if !Visible(board, card, viewer) {
			hiddenCounts[card.ColumnID]++
			continue
		}
		shown[card.ID] = struct{}{}
		cards = append(cards, cardViewWithReactions(board, card, authorProfile(card, profiles), viewer, reactions[card.ID]))
	}

	comments := make([]dto.CommentView, 0, len(state.Comments))
	for _, comment := range state.Comments {
		if _, ok := shown[comment.CardID]; !ok {
			continue
		}
		comments = append(comments, commentView(board, comment, commentAuthorProfile(comment, profiles), viewer))
	}

	cardShown := func(cardID uuid.UUID) bool {
		_, ok := shown[cardID]

		return ok
	}

	return dto.BoardSnapshot{
		BoardID:         board.ID,
		Version:         header.Version,
		Board:           boardView(header),
		Settings:        Settings(board),
		Viewer:          viewerView(viewer),
		Me:              Me(board, state.Votes, viewer),
		Columns:         columns,
		Cards:           cards,
		Comments:        comments,
		HiddenCounts:    hiddenCounts,
		Participants:    participants,
		Timer:           Timer(board),
		ActionItems:     actionItemViews(board, state.ActionItems, cardShown, viewer),
		PreviousActions: previousActionViews(state.PreviousActions),
		Spotlight:       Spotlight(board, cardShown, viewer),
		Surveys:         surveyViews(state.Surveys, state.SurveyResponses, viewer),
	}
}

func CardResponse(board model.Board, card model.Card, author *model.BoardParticipantProfile, viewer Viewer, version int64) dto.BoardCommandResponse {
	response := dto.BoardCommandResponse{Version: version}
	if !Visible(board, card, viewer) {
		return response
	}

	view := cardView(board, card, author, viewer)
	response.Card = &view

	return response
}

func ReplayCardResponse(board model.Board, card model.Card, author *model.BoardParticipantProfile, viewer Viewer, version int64) dto.BoardCommandResponse {
	response := ReplayResponse(version, new(card.ID))
	if !Owns(card, viewer) || !Visible(board, card, viewer) {
		return response
	}

	view := cardView(board, card, author, viewer)
	response.Card = &view

	return response
}

func ColumnResponse(board model.Board, column model.BoardColumn, version int64) dto.BoardCommandResponse {
	view := columnView(board, column)

	return dto.BoardCommandResponse{Version: version, Column: &view}
}

func VersionResponse(version int64) dto.BoardCommandResponse {
	return dto.BoardCommandResponse{Version: version}
}

func ReplayResponse(version int64, id *uuid.UUID) dto.BoardCommandResponse {
	return dto.BoardCommandResponse{Version: version, Replayed: true, ID: id}
}

func ParticipantName(board model.Board, profile model.BoardParticipantProfile) string {
	if !profile.Guest() {
		return cmp.Or(profile.DisplayName, profile.AccountName)
	}

	if !board.GuestNamesEnabled || profile.DisplayName == "" {
		return AnonymousName
	}

	return profile.DisplayName
}

func Owns(card model.Card, viewer Viewer) bool {
	if viewer.ParticipantID == nil || card.AuthorParticipantID == nil {
		return false
	}

	return *viewer.ParticipantID == *card.AuthorParticipantID
}

func boardListItem(header model.BoardHeader) dto.BoardListItem {
	item := dto.BoardListItem{
		ID:          header.ID,
		TeamID:      teamOf(header.Board),
		Title:       header.Title,
		CreatorName: header.CreatorName,
		Ended:       header.Closed(),
	}

	if !header.Board.Anonymous() {
		item.CreatedAt = header.CreatedAt.UTC().Format(timeLayout)
	}

	if header.ArchivedAt != nil {
		item.ArchivedAt = header.ArchivedAt.UTC().Format(timeLayout)
	}

	return item
}

func boardView(header model.BoardHeader) dto.BoardView {
	board := header.Board

	view := dto.BoardView{
		ID:                 board.ID,
		TeamID:             teamOf(board),
		Title:              header.Title,
		Context:            board.Context,
		TemplateKey:        board.TemplateKey,
		Anonymity:          board.Anonymity,
		Stage:              board.Stage,
		CardsHidden:        board.CardsHidden,
		Closed:             board.Closed(),
		ShowPrimeDirective: board.ShowPrimeDirective,
		CreatorName:        header.CreatorName,
	}

	if header.ArchivedAt != nil {
		view.ArchivedAt = header.ArchivedAt.UTC().Format(timeLayout)
	}

	if board.BackgroundMediaID != nil {
		view.BackgroundPath = fmt.Sprintf(model.MediaPathFormat, board.ID, *board.BackgroundMediaID)
	}

	if !board.Anonymous() {
		view.CreatedAt = header.CreatedAt.UTC().Format(timeLayout)
	}

	return view
}

func teamOf(board model.Board) *uuid.UUID {
	if board.TeamID == nil {
		return nil
	}

	return new(*board.TeamID)
}

func viewerView(viewer Viewer) dto.ViewerView {
	return dto.ViewerView{
		ParticipantID: viewer.ParticipantID,
		Role:          viewer.Role,
		Capabilities:  append([]model.BoardCapability{}, viewer.Capabilities...),
	}
}

func columnView(board model.Board, column model.BoardColumn) dto.ColumnView {
	view := dto.ColumnView{
		ID:            column.ID,
		Title:         column.Title,
		Description:   column.Description,
		Colour:        column.Colour,
		Position:      column.Position,
		CardsRevealed: board.CardsHidden && slices.Contains(board.RevealedColumnIDs, column.ID),
		Hidden:        slices.Contains(board.HiddenColumnIDs, column.ID),
	}

	if column.MaxVotes != nil {
		view.MaxVotes = new(*column.MaxVotes)
	}

	return view
}

func cardView(board model.Board, card model.Card, author *model.BoardParticipantProfile, viewer Viewer) dto.CardView {
	view := dto.CardView{
		ID:       card.ID,
		ColumnID: card.ColumnID,
		Position: card.Position,
		Body:     card.Body,
		Colour:   card.Colour,
		IsMine:   Owns(card, viewer),
		Hidden:   board.CardsHiddenIn(card.ColumnID),
		Votes:    votesView(board, card),
		Mentions: mentionedPeople(card.Mentions),
		Pinned:   card.PinnedAt != nil,
	}

	if card.ImageMediaID != nil {
		view.ImagePath = fmt.Sprintf(model.MediaPathFormat, board.ID, *card.ImageMediaID)
	}

	if !board.CardsHiddenIn(card.ColumnID) && card.ParentCardID != nil {
		view.ParentCardID = new(*card.ParentCardID)
	}

	if board.Anonymous() {
		return view
	}

	view.Author = authorView(board, card, author)
	view.CreatedAt = card.CreatedAt.UTC().Format(timeLayout)
	if card.UpdatedAt != nil {
		view.UpdatedAt = card.UpdatedAt.UTC().Format(timeLayout)
	}

	return view
}

func authorProfile(card model.Card, profiles map[uuid.UUID]model.BoardParticipantProfile) *model.BoardParticipantProfile {
	if card.AuthorParticipantID == nil {
		return nil
	}

	profile, ok := profiles[*card.AuthorParticipantID]
	if !ok {
		return nil
	}

	return &profile
}

func authorView(board model.Board, card model.Card, author *model.BoardParticipantProfile) *dto.AuthorView {
	if card.AuthorParticipantID == nil {
		return nil
	}

	view := &dto.AuthorView{ParticipantID: *card.AuthorParticipantID}
	if author != nil && author.ID == *card.AuthorParticipantID {
		view.DisplayName = ParticipantName(board, *author)
	}

	return view
}
