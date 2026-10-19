package og

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"retrotide/internal/boardtemplate"
	"retrotide/internal/config"
	"retrotide/internal/settings"
)

type (
	Resolver interface {
		Resolve(ctx context.Context, requestPath string) (Meta, error)
	}

	Deps struct {
		Settings       settings.Service
		DeploymentMode config.DeploymentMode
	}

	Meta struct {
		BaseURL     string
		Canonical   string
		Title       string
		Description string
		Robots      string
	}

	page struct {
		path        string
		title       string
		description string
		robots      string
		hidden      bool
	}

	resolver struct {
		settings       settings.Service
		deploymentMode config.DeploymentMode
	}
)

const (
	siteSuffix      = " | RetroTide"
	templatesPrefix = "/templates/"

	sharedTitle       = "Join a retrospective on RetroTide"
	sharedDescription = "Someone shared a RetroTide retrospective with you. Open the link to join it."
	sharedRobots      = "noindex, nofollow"
)

var (
	sharedPrefixes = []string{"/b", "/boards", "/o", "/invite"}
)

func NewResolver(deps Deps) Resolver {
	return &resolver{settings: deps.Settings, deploymentMode: deps.DeploymentMode}
}

func (r *resolver) Resolve(ctx context.Context, requestPath string) (Meta, error) {
	raw, err := r.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return Meta{}, fmt.Errorf("read the base url: %w", err)
	}

	baseURL := strings.TrimRight(raw, "/")

	found := r.lookup(path.Clean("/" + requestPath))
	canonical := (&url.URL{Path: found.path}).EscapedPath()

	return Meta{
		BaseURL:     baseURL,
		Canonical:   baseURL + canonical,
		Title:       found.title,
		Description: found.description,
		Robots:      found.robots,
	}, nil
}

func (r *resolver) lookup(cleaned string) page {
	if shared, ok := sharedPage(cleaned); ok {
		return shared
	}

	if r.deploymentMode == config.DeploymentSelfHosted {
		return page{path: cleaned}
	}

	index := slices.IndexFunc(marketingPages, func(candidate page) bool {
		return !candidate.hidden && len(candidate.path) == len(cleaned) && strings.EqualFold(candidate.path, cleaned)
	})
	if index >= 0 {
		return marketingPages[index]
	}

	if template, ok := templatePage(cleaned); ok {
		return template
	}

	return page{path: cleaned}
}

func sharedPage(cleaned string) (page, bool) {
	for _, prefix := range sharedPrefixes {
		if len(cleaned) < len(prefix) || !strings.EqualFold(cleaned[:len(prefix)], prefix) {
			continue
		}

		rest := cleaned[len(prefix):]
		if rest != "" && rest[0] != '/' {
			continue
		}

		return page{path: prefix + rest, title: sharedTitle, description: sharedDescription, robots: sharedRobots}, true
	}

	return page{}, false
}

func templatePage(cleaned string) (page, bool) {
	if len(cleaned) <= len(templatesPrefix) || !strings.EqualFold(cleaned[:len(templatesPrefix)], templatesPrefix) {
		return page{}, false
	}

	key := cleaned[len(templatesPrefix):]
	if key == boardtemplate.BlankKey {
		return page{}, false
	}

	template, ok := boardtemplate.Lookup(key)
	if !ok {
		return page{}, false
	}

	titles := make([]string, 0, len(template.Columns))
	for _, column := range template.Columns {
		titles = append(titles, column.Title)
	}

	return page{
		path:        templatesPrefix + template.Key,
		title:       template.Title + " retrospective template" + siteSuffix,
		description: template.Description + " Columns: " + listOf(titles) + ".",
	}, true
}

func listOf(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}

	last := len(items) - 1

	return strings.Join(items[:last], ", ") + " and " + items[last]
}
