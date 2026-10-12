package og

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testShell = `<!doctype html>
<html lang="en-GB">
    <head>
        <title>RetroTide</title>
        <meta
            name="description"
            content="Sprint retrospectives for teams."
        />
        <meta name="robots" content="index, follow" />
        <link rel="canonical" href="__BASE_URL__/" />
        <meta property="og:type" content="website" />
        <meta property="og:url" content="__BASE_URL__/" />
        <meta property="og:image" content="__BASE_URL__/logo.png" />
        <meta property="og:title" content="RetroTide" />
        <meta
            property="og:description"
            content="Sprint retrospectives for teams."
        />
        <meta name="twitter:title" content="RetroTide" />
        <meta name="twitter:description" content="Sprint retrospectives for teams." />
    </head>
    <body><div id="root"></div></body>
</html>`
)

func TestRender_RewritesEveryTagTheMetaNames(t *testing.T) {
	// given
	meta := Meta{
		BaseURL:     testBaseURL,
		Canonical:   testBaseURL + "/pricing",
		Title:       "Pricing | RetroTide",
		Description: "Free for one team.",
		Robots:      "noindex, nofollow",
	}

	// when
	rendered := string(Render([]byte(testShell), meta))

	// then
	assert.Equal(t, `<!doctype html>
<html lang="en-GB">
    <head>
        <title>Pricing | RetroTide</title>
        <meta
            name="description"
            content="Free for one team."
        />
        <meta name="robots" content="noindex, nofollow" />
        <link rel="canonical" href="https://retrotide.test/pricing" />
        <meta property="og:type" content="website" />
        <meta property="og:url" content="https://retrotide.test/pricing" />
        <meta property="og:image" content="https://retrotide.test/logo.png" />
        <meta property="og:title" content="Pricing | RetroTide" />
        <meta
            property="og:description"
            content="Free for one team."
        />
        <meta name="twitter:title" content="Pricing | RetroTide" />
        <meta name="twitter:description" content="Free for one team." />
    </head>
    <body><div id="root"></div></body>
</html>`, rendered)
}

func TestRender_KeepsTheShellsCopyWhereTheMetaHasNone(t *testing.T) {
	// given
	meta := Meta{BaseURL: testBaseURL, Canonical: testBaseURL + "/dashboard"}

	// when
	rendered := string(Render([]byte(testShell), meta))

	// then only the canonical url and the base url change
	assert.Contains(t, rendered, "<title>RetroTide</title>")
	assert.Contains(t, rendered, `content="Sprint retrospectives for teams."`)
	assert.Contains(t, rendered, `<meta name="robots" content="index, follow" />`)
	assert.Contains(t, rendered, `<meta property="og:title" content="RetroTide" />`)
	assert.Contains(t, rendered, `<link rel="canonical" href="https://retrotide.test/dashboard" />`)
	assert.Contains(t, rendered, `<meta property="og:url" content="https://retrotide.test/dashboard" />`)
	assert.Contains(t, rendered, `<meta property="og:image" content="https://retrotide.test/logo.png" />`)
	assert.NotContains(t, rendered, baseURLPlaceholder)
}

func TestRender_EscapesEveryInsertedValue(t *testing.T) {
	// given
	meta := Meta{
		BaseURL:     `https://retrotide.test/"><script>x</script>`,
		Canonical:   `https://retrotide.test/a"b`,
		Title:       `Tom & "Jerry" </title><script>alert(1)</script>`,
		Description: `It's <b>bold</b> & "quoted"`,
		Robots:      `noindex"><meta x="`,
	}

	// when
	rendered := string(Render([]byte(testShell), meta))

	// then nothing inserted can close the attribute or the element it sits in
	assert.Contains(t, rendered, "<title>Tom &amp; &#34;Jerry&#34; &lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;</title>")
	assert.Contains(t, rendered, `content="It&#39;s &lt;b&gt;bold&lt;/b&gt; &amp; &#34;quoted&#34;"`)
	assert.Contains(t, rendered, `<meta name="robots" content="noindex&#34;&gt;&lt;meta x=&#34;" />`)
	assert.Contains(t, rendered, `<link rel="canonical" href="https://retrotide.test/a&#34;b" />`)
	assert.Contains(t, rendered, `<meta property="og:image" content="https://retrotide.test/&#34;&gt;&lt;script&gt;x&lt;/script&gt;/logo.png" />`)
	assert.NotContains(t, rendered, "<script>")
	assert.NotContains(t, rendered, "<b>")
}

func TestRender_InsertsDollarSignsLiterally(t *testing.T) {
	// given
	meta := Meta{BaseURL: testBaseURL, Canonical: testBaseURL + "/$1", Title: "Save $2 ${1}", Description: "Costs $1.00"}

	// when
	rendered := string(Render([]byte(testShell), meta))

	// then
	assert.Contains(t, rendered, "<title>Save $2 ${1}</title>")
	assert.Contains(t, rendered, `<meta property="og:title" content="Save $2 ${1}" />`)
	assert.Contains(t, rendered, `<meta name="twitter:description" content="Costs $1.00" />`)
	assert.Contains(t, rendered, `<link rel="canonical" href="https://retrotide.test/$1" />`)
}
