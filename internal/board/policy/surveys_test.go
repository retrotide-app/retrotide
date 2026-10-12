package policy

import (
	"testing"

	"retrotide/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestCanManageSurveys(t *testing.T) {
	cases := []struct {
		name   string
		access model.BoardAccess
		want   error
	}{
		{name: "a facilitator", access: access(host), want: nil},
		{name: "a facilitator in any stage", access: access(host, in(model.StageActions)), want: nil},
		{name: "a participant", access: access(writer), want: ErrNotAllowed},
		{name: "an archived board", access: access(host, archived), want: ErrReadOnly},
		{name: "a closed board", access: access(host, closed), want: ErrReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanManageSurveys(tc.access)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCanRespondToSurvey(t *testing.T) {
	open := model.Survey{Status: model.SurveyOpen}
	shut := model.Survey{Status: model.SurveyClosed}

	cases := []struct {
		name   string
		access model.BoardAccess
		survey model.Survey
		want   error
	}{
		{name: "a participant on an open survey", access: access(writer), survey: open, want: nil},
		{name: "a guest in any stage", access: access(writer, notMember, in(model.StageDiscuss)), survey: open, want: nil},
		{name: "a closed survey", access: access(writer), survey: shut, want: ErrSurveyClosed},
		{name: "a viewer with no seat", access: access(writer, noParticipant), survey: open, want: ErrNotParticipant},
		{name: "a locked board", access: access(writer, locked), survey: open, want: ErrReadOnly},
		{name: "a closed board", access: access(writer, closed), survey: open, want: ErrReadOnly},
		{name: "a viewer who cannot participate", access: access(writer, noCapabilities), survey: open, want: ErrNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			err := CanRespondToSurvey(tc.access, tc.survey)

			// then
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
