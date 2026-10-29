// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package pagefetch

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func parse(t *testing.T, src string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestExtract_Metadata(t *testing.T) {
	base, _ := url.Parse("https://shop.example.com/products/widget?ref=x")
	info := extract(parse(t, `<!doctype html><html><head>
		<title>  Widget   | Shop  </title>
		<meta property="og:title" content="Widget Deluxe">
		<meta property="og:title" content="Duplicate from a plugin">
		<meta name="description" content="Plain description">
		<meta property="og:description" content="Open Graph description">
		<meta property="og:site_name" content="Shop">
		<meta property="og:image" content="/img/widget.jpg">
		<meta name="twitter:image" content="https://cdn.example.com/twitter.jpg">
		<link rel="apple-touch-icon" href="/apple.png">
		<link rel="shortcut icon" href="icons/fav.ico">
		</head><body><svg><title>icon title</title></svg><p>Hello</p></body></html>`), base)

	assert.Equal(t, "Widget Deluxe", info.title, "og:title wins over <title> and keeps the first value")
	assert.Equal(t, "Open Graph description", info.description)
	assert.Equal(t, "Shop", info.siteName)
	assert.Equal(t, "https://shop.example.com/img/widget.jpg", info.imageURL, "relative image resolved against the page")
	assert.Equal(t, "https://shop.example.com/products/icons/fav.ico", info.faviconURL, "icon wins over apple-touch-icon")
}

func TestExtract_Fallbacks(t *testing.T) {
	base, _ := url.Parse("http://example.org/a/b")
	info := extract(parse(t, `<html><head><title>Only a title</title><meta name="description" content="Meta description"></head><body></body></html>`), base)
	assert.Equal(t, "Only a title", info.title)
	assert.Equal(t, "Meta description", info.description)
	assert.Equal(t, "http://example.org/favicon.ico", info.faviconURL, "falls back to /favicon.ico")
	assert.Empty(t, info.imageURL)

	svgOnly := extract(parse(t, `<html><body><svg><title>not the page title</title></svg></body></html>`), base)
	assert.Empty(t, svgOnly.title, "an SVG's <title> is not the page title")
}

func TestExtract_RejectsUnsafeURLs(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	info := extract(parse(t, `<html><head>
		<meta property="og:image" content="javascript:alert(1)">
		<link rel="icon" href="data:image/png;base64,AAAA">
		</head></html>`), base)
	assert.Empty(t, info.imageURL)
	assert.Equal(t, "https://example.com/favicon.ico", info.faviconURL, "a data: icon is dropped in favour of the default")

	assert.Empty(t, resolveWebURL(base, "file:///etc/passwd"))
	assert.Empty(t, resolveWebURL(base, "ftp://example.com/x.png"))
	assert.Empty(t, resolveWebURL(base, "https://example.com/"+strings.Repeat("a", maxURLLength)))
	assert.Equal(t, "https://cdn.example.com/x.png", resolveWebURL(base, "//cdn.example.com/x.png"))
}

func TestExtract_ReadableText(t *testing.T) {
	long := strings.Repeat("This is the real article sentence. ", 12)
	info := extract(parse(t, `<html><head><title>T</title><style>.x{color:red}</style></head><body>
		<nav>Home About Contact</nav>
		<header><h1>Site name</h1></header>
		<article><h2>Headline</h2><p>`+long+`</p><script>var secret = 1;</script><p>Second   paragraph
		with   odd   spacing.</p><div hidden>hidden text</div></article>
		<aside>Related links</aside><footer>Copyright</footer></body></html>`), nil)

	assert.Contains(t, info.text, "Headline")
	assert.Contains(t, info.text, "This is the real article sentence.")
	assert.Contains(t, info.text, "Second paragraph with odd spacing.")
	for _, noise := range []string{"Home About", "secret", "color:red", "Related links", "Copyright", "hidden text", "Site name"} {
		assert.NotContains(t, info.text, noise)
	}
	assert.Equal(t, 3, strings.Count(info.text, "\n")+1, "block elements become separate lines: headline and two paragraphs")
}

func TestExtract_TextWithoutArticleUsesBody(t *testing.T) {
	info := extract(parse(t, `<html><body><div>Short page.</div><nav>menu</nav><p>Another line.</p></body></html>`), nil)
	assert.Equal(t, "Short page.\nAnother line.", info.text)
}

func TestExtract_TextIsCapped(t *testing.T) {
	big := strings.Repeat("word ", maxTextRunes) // far more than the cap
	info := extract(parse(t, `<html><body><p>`+big+`</p></body></html>`), nil)
	assert.LessOrEqual(t, len([]rune(info.text)), maxTextRunes)
	assert.NotEmpty(t, info.text)
}
