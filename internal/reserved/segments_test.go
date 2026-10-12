package reserved

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPathSegment(t *testing.T) {
	// given
	cases := []struct {
		name    string
		segment string
		want    bool
	}{
		{name: "the api prefix", segment: "api", want: true},
		{name: "the board share path", segment: "b", want: true},
		{name: "the board path", segment: "boards", want: true},
		{name: "the organisation path", segment: "o", want: true},
		{name: "the probe paths", segment: "livez", want: true},
		{name: "the metrics path", segment: "metrics", want: true},
		{name: "the pprof prefix", segment: "debug", want: true},
		{name: "the hashed assets", segment: "assets", want: true},
		{name: "a spa page", segment: "onboarding", want: true},
		{name: "an auth page an email links to", segment: "reset-password", want: true},
		{name: "the invitation page an email links to", segment: "accept-invitation", want: true},
		{name: "the marketing pricing page", segment: "pricing", want: true},
		{name: "the competitor comparison pages", segment: "compare", want: true},
		{name: "the self-hosting page and its download", segment: "self-host", want: true},
		{name: "the template pages", segment: "templates", want: true},
		{name: "the two-factor sign-in step", segment: "two-factor", want: true},
		{name: "the legal pages", segment: "legal", want: true},
		{name: "the crawler rules", segment: "robots.txt", want: true},
		{name: "the sitemap", segment: "sitemap.xml", want: true},
		{name: "the crawler rules in another case", segment: "Robots.TXT", want: true},
		{name: "matched regardless of case", segment: "ADMIN", want: true},
		{name: "an ordinary slug", segment: "acme-retros", want: false},
		{name: "a prefix of a reserved name", segment: "boardside", want: false},
		{name: "empty", segment: "", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := IsPathSegment(tc.segment)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
