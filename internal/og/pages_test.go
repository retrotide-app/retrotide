package og

import (
	"testing"

	"retrotide/internal/boardtemplate"
	"retrotide/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexablePaths_ListsTheMarketingPagesThenEveryPublicTemplate(t *testing.T) {
	// when
	paths := IndexablePaths()

	// then
	assert.Equal(t, []string{
		"/",
		"/pricing",
		"/compare/easyretro",
		"/self-host",
		"/templates",
		"/legal/privacy",
		"/legal/terms",
		"/legal/acceptable-use",
		"/legal/cookies",
		"/legal/sub-processors",
		"/legal/security",
		"/templates/start-stop-continue",
		"/templates/mad-sad-glad",
		"/templates/went-well",
		"/templates/four-ls",
		"/templates/sailboat",
		"/templates/starfish",
		"/templates/kalm",
		"/templates/daki",
		"/templates/rose-thorn-bud",
		"/templates/wrap",
		"/templates/lean-coffee",
		"/templates/three-little-pigs",
		"/templates/hot-air-balloon",
		"/templates/speed-car",
		"/templates/mountain-climb",
		"/templates/desert-island",
		"/templates/weather-report",
		"/templates/traffic-lights",
		"/templates/plus-delta",
		"/templates/keep-drop-add",
		"/templates/good-bad-ugly",
		"/templates/fun-done-learn",
		"/templates/highs-lows",
		"/templates/timeline",
		"/templates/learning-matrix",
		"/templates/what-so-what-now-what",
		"/templates/lessons-learned",
		"/templates/post-mortem",
		"/templates/incident-review",
		"/templates/pre-mortem",
		"/templates/swot",
		"/templates/pros-cons",
		"/templates/six-thinking-hats",
		"/templates/kudos",
		"/templates/daily-standup",
		"/templates/kanban",
		"/templates/quarter-in-review",
		"/templates/expectations-reality",
		"/templates/brainstorm",
		"/templates/hopes-and-fears",
	}, paths)
	assert.NotContains(t, paths, templatesPrefix+boardtemplate.BlankKey)
}

func TestIndexablePaths_EachGetsItsOwnMetaFromTheResolver(t *testing.T) {
	for _, indexable := range IndexablePaths() {
		t.Run(indexable, func(t *testing.T) {
			// given
			resolver := newTestResolver(t, config.DeploymentHosted, testBaseURL)

			// when
			meta, err := resolver.Resolve(t.Context(), indexable)

			// then the sitemap and the resolver agree on which pages exist
			require.NoError(t, err)
			assert.NotEmpty(t, meta.Title)
			assert.NotEmpty(t, meta.Description)
			assert.Empty(t, meta.Robots)
			assert.Equal(t, testBaseURL+indexable, meta.Canonical)
		})
	}
}
