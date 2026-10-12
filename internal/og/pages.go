package og

import (
	"retrotide/internal/boardtemplate"
)

var (
	marketingPages = []page{
		{
			path:        "/",
			title:       "RetroTide: sprint retrospectives your team will use",
			description: "Run sprint retrospectives with hidden cards until the reveal, anonymous boards, voting, timers and action items that carry over to the next retro. Free for one team.",
		},
		{
			path:        "/pricing",
			title:       "Pricing" + siteSuffix,
			description: "RetroTide is free for one team, with unlimited guests and boards, exports and action carry-over. Team and Organisation plans are coming soon.",
		},
		{
			path:        "/compare/easyretro",
			title:       "RetroTide and EasyRetro compared" + siteSuffix,
			description: "A dated, factual comparison of RetroTide and EasyRetro: hidden cards that no RetroTide facilitator can read before the reveal, free action carry-over, and the free plans side by side.",
		},
		{
			path:        "/self-host",
			title:       "Self-host RetroTide" + siteSuffix,
			description: "Run RetroTide on your own server with Docker: one compose file, a few settings, and your team's retrospectives stay on your infrastructure.",
		},
		{
			path:        "/templates",
			title:       "Retrospective templates" + siteSuffix,
			description: "Free sprint retrospective templates, from Start, Stop, Continue to Sailboat and 4Ls. Pick one and start a board in seconds.",
		},
		{
			path:        "/legal/privacy",
			title:       "Privacy notice" + siteSuffix,
			description: "What personal data RetroTide collects, why it uses it, how long it keeps it, who processes it, and the rights you have over it.",
		},
		{
			path:        "/legal/terms",
			title:       "Terms of service" + siteSuffix,
			description: "The terms that apply when you use RetroTide.",
		},
		{
			path:        "/legal/acceptable-use",
			title:       "Acceptable use policy" + siteSuffix,
			description: "What you may and may not do on RetroTide, and how to report abuse.",
		},
		{
			path:        "/legal/cookies",
			title:       "Cookie statement" + siteSuffix,
			description: "The cookies RetroTide sets, what each one is for and how long it lasts.",
		},
		{
			path:        "/legal/sub-processors",
			title:       "Sub-processors" + siteSuffix,
			description: "The companies that process customer data on RetroTide's behalf, and what each one does.",
		},
		{
			path:        "/legal/security",
			title:       "Security" + siteSuffix,
			description: "How RetroTide protects your account and your boards, and how to report a vulnerability.",
		},
	}
)

func IndexablePaths() []string {
	templates := boardtemplate.All()

	paths := make([]string, 0, len(marketingPages)+len(templates))
	for _, candidate := range marketingPages {
		paths = append(paths, candidate.path)
	}

	for _, template := range templates {
		if template.Key == boardtemplate.BlankKey {
			continue
		}

		paths = append(paths, templatesPrefix+template.Key)
	}

	return paths
}
