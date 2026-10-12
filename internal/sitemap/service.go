package sitemap

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"retrotide/internal/config"
	"retrotide/internal/og"
	"retrotide/internal/settings"
)

type (
	Service interface {
		Sitemap(ctx context.Context) ([]byte, error)
		Robots(ctx context.Context) ([]byte, error)
	}

	Deps struct {
		Settings       settings.Service
		DeploymentMode config.DeploymentMode
	}

	service struct {
		settings       settings.Service
		deploymentMode config.DeploymentMode
	}

	urlSet struct {
		XMLName   xml.Name `xml:"urlset"`
		Namespace string   `xml:"xmlns,attr"`
		URLs      []entry  `xml:"url"`
	}

	entry struct {
		Location string `xml:"loc"`
	}
)

const (
	SitemapPath = "/sitemap.xml"
	RobotsPath  = "/robots.txt"

	sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"
	selfHostedRobots = "User-agent: *\nDisallow: /\n"
	hostedRobots     = "User-agent: *\nDisallow: /api/\n\nSitemap: %s\n"
)

func NewService(deps Deps) Service {
	return &service{settings: deps.Settings, deploymentMode: deps.DeploymentMode}
}

func (s *service) Sitemap(ctx context.Context) ([]byte, error) {
	set := urlSet{Namespace: sitemapNamespace}
	if s.deploymentMode == config.DeploymentSelfHosted {
		return encode(set)
	}

	baseURL, err := s.baseURL(ctx)
	if err != nil {
		return nil, err
	}

	for _, indexable := range og.IndexablePaths() {
		set.URLs = append(set.URLs, entry{Location: baseURL + (&url.URL{Path: indexable}).EscapedPath()})
	}

	return encode(set)
}

func (s *service) Robots(ctx context.Context) ([]byte, error) {
	if s.deploymentMode == config.DeploymentSelfHosted {
		return []byte(selfHostedRobots), nil
	}

	baseURL, err := s.baseURL(ctx)
	if err != nil {
		return nil, err
	}

	return fmt.Appendf(nil, hostedRobots, baseURL+SitemapPath), nil
}

func (s *service) baseURL(ctx context.Context) (string, error) {
	raw, err := s.settings.Get(ctx, config.SettingBaseURL)
	if err != nil {
		return "", fmt.Errorf("read the base url: %w", err)
	}

	return strings.TrimRight(raw, "/"), nil
}

func encode(set urlSet) ([]byte, error) {
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the sitemap: %w", err)
	}

	document := append([]byte(xml.Header), body...)

	return append(document, '\n'), nil
}
