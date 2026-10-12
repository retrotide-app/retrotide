package identity

import (
	"net/url"
	"path"
	"strings"
	"unicode"
)

const (
	apiRoot   = "/api"
	apiPrefix = apiRoot + "/"
)

func CleanReturnTo(raw string) string {
	cleaned := cleanOnce(raw)
	if cleaned == "" || cleanOnce(cleaned) != cleaned {
		return ""
	}

	return cleaned
}

func cleanOnce(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}

	if strings.ContainsRune(raw, '\\') || strings.ContainsFunc(raw, unicode.IsControl) || strings.HasPrefix(raw, apiPrefix) {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" || namesAPI(parsed.Path) {
		return ""
	}

	cleaned := parsed.EscapedPath()
	if parsed.RawQuery != "" {
		cleaned += "?" + parsed.RawQuery
	}

	return cleaned
}

func namesAPI(decoded string) bool {
	resolved := path.Clean(decoded)

	return resolved == apiRoot || strings.HasPrefix(resolved, apiPrefix)
}
