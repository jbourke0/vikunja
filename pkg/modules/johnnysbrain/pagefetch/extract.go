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
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	maxTextRunes   = 200_000
	minArticleText = 200
	maxURLLength   = 2048
)

// pageInfo is what is read out of a parsed page.
type pageInfo struct {
	title, description, siteName string
	faviconURL, imageURL         string
	text                         string
}

// firstValues keeps the first value seen for each key; metadata that appears twice is usually
// a plugin repeating what the theme already said.
type firstValues map[string]string

func (f firstValues) set(key, value string) {
	if _, seen := f[key]; !seen && strings.TrimSpace(value) != "" {
		f[key] = cleanText(value)
	}
}

func (f firstValues) first(keys ...string) string {
	for _, k := range keys {
		if v := f[k]; v != "" {
			return v
		}
	}
	return ""
}

// extract reads the metadata and readable text out of a parsed page. base is the address the
// page was served from, used to turn relative icon and image links into absolute ones.
func extract(doc *html.Node, base *url.URL) pageInfo {
	meta := firstValues{}
	var pageTitle string
	icons := firstValues{}

	var walk func(n *html.Node, inSVG bool)
	walk = func(n *html.Node, inSVG bool) {
		if n.Type == html.ElementNode {
			switch n.DataAtom { //nolint:exhaustive // only a few elements carry metadata
			case atom.Svg:
				inSVG = true
			case atom.Title:
				if !inSVG && pageTitle == "" {
					pageTitle = cleanText(textOf(n))
				}
			case atom.Meta:
				key := strings.ToLower(attr(n, "property"))
				if key == "" {
					key = strings.ToLower(attr(n, "name"))
				}
				if key != "" {
					meta.set(key, attr(n, "content"))
				}
			case atom.Link:
				for _, rel := range strings.Fields(strings.ToLower(attr(n, "rel"))) {
					switch rel {
					case "icon":
						icons.set("icon", attr(n, "href"))
					case "apple-touch-icon":
						icons.set("apple", attr(n, "href"))
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inSVG)
		}
	}
	walk(doc, false)

	info := pageInfo{
		title:       meta.first("og:title", "twitter:title"),
		description: meta.first("og:description", "twitter:description", "description"),
		siteName:    meta.first("og:site_name", "application-name"),
		text:        extractText(doc),
	}
	if info.title == "" {
		info.title = pageTitle
	}
	info.imageURL = resolveWebURL(base, meta.first("og:image", "og:image:secure_url", "og:image:url", "twitter:image", "twitter:image:src"))
	info.faviconURL = resolveWebURL(base, icons.first("icon", "apple"))
	if info.faviconURL == "" && base != nil && base.Host != "" {
		info.faviconURL = (&url.URL{Scheme: base.Scheme, Host: base.Host, Path: "/favicon.ico"}).String()
	}
	return info
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return true
		}
	}
	return false
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

// cleanText collapses runs of whitespace, including newlines, into single spaces.
func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// resolveWebURL turns a possibly relative link into an absolute http(s) URL, or "" when it is
// not one. data:, javascript: and other schemes never get through.
func resolveWebURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || base == nil {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(parsed)
	if (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.Host == "" {
		return ""
	}
	if out := resolved.String(); len(out) <= maxURLLength {
		return out
	}
	return ""
}

var skippedElements = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Template: true, atom.Svg: true,
	atom.Canvas: true, atom.Nav: true, atom.Footer: true, atom.Aside: true, atom.Iframe: true,
	atom.Select: true, atom.Button: true, atom.Head: true,
}

var blockElements = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true,
	atom.H5: true, atom.H6: true, atom.Li: true, atom.Ul: true, atom.Ol: true, atom.Br: true,
	atom.Tr: true, atom.Section: true, atom.Article: true, atom.Main: true, atom.Header: true,
	atom.Blockquote: true, atom.Pre: true, atom.Table: true, atom.Dd: true, atom.Dt: true,
	atom.Figure: true, atom.Figcaption: true, atom.Hr: true,
}

// extractText returns the readable text of a page: the article or main content when the page
// marks one up, otherwise the whole body, without scripts, navigation and footers.
func extractText(doc *html.Node) string {
	for _, tag := range []atom.Atom{atom.Article, atom.Main} {
		if n := findElement(doc, tag); n != nil {
			if text := textFrom(n); utf8.RuneCountInString(text) >= minArticleText {
				return text
			}
		}
	}
	if body := findElement(doc, atom.Body); body != nil {
		return textFrom(body)
	}
	return textFrom(doc)
}

func findElement(n *html.Node, tag atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func textFrom(root *html.Node) string {
	var b strings.Builder
	// Stop collecting well past the cap; whitespace collapsing only ever shrinks the text.
	limit := maxTextRunes * 4

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if b.Len() >= limit {
			return
		}
		switch n.Type { //nolint:exhaustive // other node types have nothing to read
		case html.TextNode:
			// Newlines inside a text node are source formatting, not line breaks.
			b.WriteString(cleanText(n.Data))
			b.WriteByte(' ')
			return
		case html.ElementNode:
			if skippedElements[n.DataAtom] || hasAttr(n, "hidden") {
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockElements[n.DataAtom] {
			b.WriteByte('\n')
		}
	}
	walk(root)

	lines := strings.Split(b.String(), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line = cleanText(line); line != "" {
			kept = append(kept, line)
		}
	}
	return truncate(strings.Join(kept, "\n"), maxTextRunes)
}

func truncate(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}
