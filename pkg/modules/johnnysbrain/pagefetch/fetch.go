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
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/models"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	userAgent    = "JohnnysBrain/1.0 (+https://tasks.jbourke.com)"
	maxBodyBytes = 2 << 20 // 2 MB of HTML is plenty for metadata and text
	maxRedirects = 5
)

// Fetcher implements models.BookmarkPageFetcher over a client that cannot reach private networks.
type Fetcher struct {
	client    *http.Client
	transport *http.Transport
}

var _ models.BookmarkPageFetcher = (*Fetcher)(nil)

// New returns a fetcher that only connects to public internet addresses.
func New() *Fetcher {
	return newFetcher(IsPublicIP)
}

// newFetcher builds a fetcher with a custom address policy. Production code must use New;
// the policy is a parameter only so tests can reach servers on loopback addresses.
func newFetcher(allow func(netip.Addr) bool) *Fetcher {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: guardedControl(allow)}
	transport := &http.Transport{
		Proxy:                  nil, // never via a proxy from the environment
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		DisableKeepAlives:      true,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  8 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
	return &Fetcher{
		transport: transport,
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return errors.New("too many redirects")
				}
				if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
					return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
				}
				return nil // the address itself is checked when the connection is made
			},
		},
	}
}

// FetchPage fetches a page and summarises it. Responses that are not successful HTML come back
// with their status and nothing else; only a failure to get any response is an error.
func (f *Fetcher) FetchPage(ctx context.Context, rawURL string) (*models.BookmarkPage, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return nil, fmt.Errorf("not a web address: %q", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	page := &models.BookmarkPage{URL: resp.Request.URL.String(), StatusCode: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return page, nil
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || (mediaType != "text/html" && mediaType != "application/xhtml+xml") {
			return page, nil
		}
	}

	body := io.LimitReader(resp.Body, maxBodyBytes)
	if converted, err := charset.NewReader(body, contentType); err == nil {
		body = converted
	}
	doc, err := html.Parse(body)
	if err != nil {
		return page, nil
	}

	info := extract(doc, resp.Request.URL)
	page.Title = info.title
	page.Description = info.description
	page.SiteName = info.siteName
	page.FaviconURL = info.faviconURL
	page.ImageURL = info.imageURL
	page.Text = info.text
	return page, nil
}
