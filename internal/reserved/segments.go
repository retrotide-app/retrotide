package reserved

import "strings"

var (
	pathSegments = map[string]struct{}{
		"accept-invitation": {},
		"account":           {},
		"admin":             {},
		"api":               {},
		"assets":            {},
		"b":                 {},
		"boards":            {},
		"compare":           {},
		"debug":             {},
		"forgot-password":   {},
		"health":            {},
		"invite":            {},
		"legal":             {},
		"livez":             {},
		"login":             {},
		"metrics":           {},
		"notifications":     {},
		"o":                 {},
		"onboarding":        {},
		"pricing":           {},
		"register":          {},
		"reset-password":    {},
		"robots.txt":        {},
		"self-host":         {},
		"sitemap":           {},
		"sitemap.xml":       {},
		"templates":         {},
		"two-factor":        {},
		"verify-email":      {},
	}
)

func IsPathSegment(segment string) bool {
	_, reserved := pathSegments[strings.ToLower(segment)]

	return reserved
}
