package identity

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanReturnTo_DropsAnythingThatCouldLeaveTheOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "protocol relative", raw: "//evil.example"},
		{name: "a slash then a backslash", raw: `/\evil.example`},
		{name: "a backslash further in", raw: `/boards\..\evil`},
		{name: "an absolute url", raw: "https://evil.example"},
		{name: "a script url", raw: "javascript:alert(1)"},
		{name: "a relative path", raw: "boards/42"},
		{name: "a tab before the path", raw: "\t/boards/42"},
		{name: "a tab inside the path", raw: "/\t/evil.example"},
		{name: "a newline inside the path", raw: "/boards\n/42"},
		{name: "a delete character", raw: "/boards\x7f"},
		{name: "an api path", raw: "/api/v1/auth/logout"},
		{name: "an api path hidden behind an escape", raw: "/%61pi/v1/auth/logout"},
		{name: "an api path behind a dot segment", raw: "/./api/v1/auth/identities"},
		{name: "an api path behind an escaped dot segment", raw: "/%2e/api/v1/auth/identities"},
		{name: "an api path behind a parent segment", raw: "/boards/../api/v1/auth/identities"},
		{name: "the api root behind a parent segment", raw: "/boards/../api/"},
		{name: "an escape that does not decode", raw: "/boards/%zz"},
		{name: "escaped slashes the parser escapes again as a literal //", raw: "/%2F%2Fevil.example|"},
		{name: "an escaped slash and backslash the parser escapes again as a literal //", raw: "/%2F%5Cevil.example|"},
		{name: "escaped slashes before a non-ASCII letter", raw: "/%2F%2Fevil.example/é"},
		{name: "escaped slashes before a space", raw: "/%2F%2Fevil.example "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			raw := tc.raw

			// when
			cleaned := CleanReturnTo(raw)

			// then a dropped value is not refused: the user lands on the default page
			assert.Empty(t, cleaned)
		})
	}
}

func TestCleanReturnTo_KeepsAPathOnThisOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "the root", raw: "/", want: "/"},
		{name: "a page", raw: "/invite", want: "/invite"},
		{name: "a page with a query", raw: "/boards/42?view=list&sort=votes", want: "/boards/42?view=list&sort=votes"},
		{name: "a fragment is never stored", raw: "/account#sign-in-methods", want: "/account"},
		{name: "an empty query is left off", raw: "/account?", want: "/account"},
		{name: "escaped slashes stay escaped, so they name a path on this origin", raw: "/%2F%2Fevil.example", want: "/%2F%2Fevil.example"},
		{name: "a path merely starting with api", raw: "/apiary", want: "/apiary"},
		{name: "a non-ASCII page name is escaped", raw: "/boards/é", want: "/boards/%C3%A9"},
		{name: "a parent segment that stays off the api", raw: "/boards/../account", want: "/boards/../account"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			raw := tc.raw

			// when
			cleaned := CleanReturnTo(raw)

			// then
			assert.Equal(t, tc.want, cleaned)
		})
	}
}

func TestCleanReturnTo_StoresOnlyAValueThatPassesItsOwnCheck(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "escaped slashes and an escaped query", raw: "/%2F%2Fevil.example?next=%2F"},
		{name: "escaped slashes before a pipe", raw: "/%2F%2Fevil.example|"},
		{name: "an escaped slash and backslash before a pipe", raw: "/%2F%5Cevil.example|"},
		{name: "escaped slashes before a non-ASCII letter", raw: "/%2F%2Fevil.example/é"},
		{name: "escaped slashes before a space", raw: "/%2F%2Fevil.example "},
		{name: "escaped slashes before a quote", raw: `/%2F%2Fevil.example"`},
		{name: "escaped slashes before a brace", raw: "/%2F%2Fevil.example{"},
		{name: "a non-ASCII page name", raw: "/boards/é"},
		{name: "a space in a page name", raw: "/a b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cleaned := CleanReturnTo(tc.raw)

			// when
			again := CleanReturnTo(cleaned)

			// then the stored value starts with exactly one slash, and a controller that also cleans it cannot change what the service stores
			if cleaned != "" {
				assert.True(t, strings.HasPrefix(cleaned, "/"))
				assert.False(t, strings.HasPrefix(cleaned, "//"))
			}

			assert.Equal(t, cleaned, again)
		})
	}
}
