package user

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestClampDisplayName(t *testing.T) {
	// given
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "an ordinary name passes through", in: "Sam Okafor", want: "Sam Okafor"},
		{name: "leading and trailing whitespace is trimmed", in: "   Ali   ", want: "Ali"},
		{name: "internal whitespace collapses to one space", in: "Priya\t\t\tShah\n\nthe   facilitator", want: "Priya Shah the facilitator"},
		{name: "tags are stripped", in: `<img src="x" alt="red">Jo<b>ey</b>`, want: "Joey"},
		{name: "a script element and its text are dropped", in: `Mo<script>alert("xss")</script>`, want: "Mo"},
		{name: "an entity-encoded script is stripped, not revived", in: "&lt;script&gt;x&lt;/script&gt;", want: ""},
		{name: "a doubly encoded tag is stripped, not revived", in: "Jo&amp;lt;b&amp;gt;ey", want: "Joey"},
		{name: "an entity-encoded image is stripped, not revived", in: "Mo&lt;img src=x onerror=alert(1)&gt;", want: "Mo"},
		{name: "an ampersand is kept as written", in: "Tom & Jerry", want: "Tom & Jerry"},
		{name: "a less-than sign is kept as written", in: "a < b", want: "a < b"},
		{name: "an apostrophe is kept as written", in: "Sam's team", want: "Sam's team"},
		{name: "a quote is kept as written", in: `The "Deploy" crew`, want: `The "Deploy" crew`},
		{name: "empty input stays empty", in: "", want: ""},
		{name: "only whitespace becomes empty", in: "   \t\n  ", want: ""},
		{name: "only markup becomes empty", in: "<b></b><img src=''><span></span>", want: ""},
		{name: "markup is stripped before the length cap", in: `<img alt="` + strings.Repeat("🔴", 30) + `">RealName`, want: "RealName"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := ClampDisplayName(tc.in)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClampDisplayName_CapsAtFortyCharacters(t *testing.T) {
	// given
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "ascii", in: strings.Repeat("a", 60), want: strings.Repeat("a", MaxDisplayNameRunes)},
		{name: "multi-byte runes count once each", in: strings.Repeat("🔴", 60), want: strings.Repeat("🔴", MaxDisplayNameRunes)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := ClampDisplayName(tc.in)

			// then
			assert.Equal(t, tc.want, got)
			assert.Equal(t, MaxDisplayNameRunes, utf8.RuneCountInString(got))
		})
	}
}

func TestClampDisplayName_TrimsASpaceLeftAtTheCap(t *testing.T) {
	// given a name whose fortieth character is a space
	in := strings.Repeat("a", MaxDisplayNameRunes-1) + " tail"

	// when
	got := ClampDisplayName(in)

	// then
	assert.Equal(t, strings.Repeat("a", MaxDisplayNameRunes-1), got)
}
