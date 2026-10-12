package policy

import (
	"testing"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestCanArchiveDeleteAndMoveBoard(t *testing.T) {
	checks := map[string]func(model.BoardAccess) error{
		"archive": CanArchiveBoard,
		"delete":  CanDeleteBoard,
		"move":    CanMoveBoard,
	}

	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator", access: access(host), want: nil},
		{name: "a facilitator on an archived board", access: access(host, archived), want: nil},
		{name: "a facilitator on a closed board", access: access(host, closed), want: nil},
		{name: "a facilitator on a locked board", access: access(host, locked), want: nil},
		{name: "a participant", access: access(writer), want: ErrNotAllowed},
		{name: "a viewer with no capability", access: access(host, noCapabilities), want: ErrNotAllowed},
	}

	for name, check := range checks {
		for _, tc := range cases {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				// when
				err := check(tc.access)

				// then
				assert.ErrorIs(t, err, tc.want)
			})
		}
	}
}

func TestCanImportCards(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator in the Write stage", access: access(host), want: nil},
		{name: "a facilitator outside the Write stage", access: access(host, in(model.StageVote)), want: ErrCardsOutOfStage},
		{name: "a participant", access: access(writer), want: ErrNotAllowed},
		{name: "a facilitator with no seat", access: access(host, noParticipant), want: ErrNotParticipant},
		{name: "a locked board", access: access(host, locked), want: ErrReadOnly},
		{name: "an archived board", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board", access: access(host, closed), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanImportCards(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
