package projection

import (
	"retrotide/internal/dto"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

const (
	EventSpotlightChanged = "spotlight_changed"
)

func SpotlightChanged(board model.Board, version int64) Event {
	spotlight := board.Spotlight

	payload := dto.SpotlightChangedEvent{
		BoardEventHead: head(EventSpotlightChanged, version),
		Spotlight: dto.SpotlightView{
			PresentingCardID:  copyID(spotlight.PresentingCardID),
			HighlightedCardID: copyID(spotlight.HighlightedCardID),
			FocusedColumnID:   copyID(spotlight.FocusedColumnID),
		},
	}

	return everyone(payload, version)
}

func Spotlight(board model.Board, shown func(uuid.UUID) bool, viewer Viewer) dto.SpotlightView {
	spotlight := board.Spotlight
	view := dto.SpotlightView{}

	if spotlight.PresentingCardID != nil && shown(*spotlight.PresentingCardID) {
		view.PresentingCardID = new(*spotlight.PresentingCardID)
	}

	if spotlight.HighlightedCardID != nil && shown(*spotlight.HighlightedCardID) {
		view.HighlightedCardID = new(*spotlight.HighlightedCardID)
	}

	if spotlight.FocusedColumnID != nil && ColumnShown(board, *spotlight.FocusedColumnID, viewer) {
		view.FocusedColumnID = new(*spotlight.FocusedColumnID)
	}

	return view
}

func copyID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}

	return new(*id)
}
