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
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openFetcher reaches the loopback test servers; production code always uses New().
func openFetcher() *Fetcher {
	return newFetcher(func(netip.Addr) bool { return true })
}

func TestFetchPage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Kids martial arts</title>
			<meta property="og:description" content="Classes for ages 5 to 12"><meta property="og:image" content="/kids.jpg">
			<link rel="icon" href="/fav.png"></head><body><p>Term one starts in February.</p></body></html>`))
	}))
	defer srv.Close()

	page, err := openFetcher().FetchPage(context.Background(), srv.URL+"/kids")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, page.StatusCode)
	assert.Equal(t, "Kids martial arts", page.Title)
	assert.Equal(t, "Classes for ages 5 to 12", page.Description)
	assert.Equal(t, srv.URL+"/kids.jpg", page.ImageURL)
	assert.Equal(t, srv.URL+"/fav.png", page.FaviconURL)
	assert.Equal(t, "Term one starts in February.", page.Text)
}

func TestFetchPage_FollowsRedirectsAndReportsFinalURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<title>New place</title>`))
	}))
	defer srv.Close()

	page, err := openFetcher().FetchPage(context.Background(), srv.URL+"/old")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/new", page.URL)
	assert.Equal(t, "New place", page.Title)
}

func TestFetchPage_StopsAfterTooManyRedirects(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	defer srv.Close()

	_, err := openFetcher().FetchPage(context.Background(), srv.URL)
	require.Error(t, err)
	assert.LessOrEqual(t, int(hits.Load()), maxRedirects+1)
}

func TestFetchPage_ErrorStatusesAndOtherTypesReturnOnlyTheStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<title>Page not found</title><p>Oops</p>`))
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.7 <title>not html</title>"))
		}
	}))
	defer srv.Close()

	missing, err := openFetcher().FetchPage(context.Background(), srv.URL+"/missing")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, missing.StatusCode)
	assert.Empty(t, missing.Title, "an error page's title is not the bookmark's title")
	assert.Empty(t, missing.Text)

	pdf, err := openFetcher().FetchPage(context.Background(), srv.URL+"/pdf")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, pdf.StatusCode)
	assert.Empty(t, pdf.Title)
	assert.Empty(t, pdf.Text)
}

func TestFetchPage_ConvertsOtherCharsets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write([]byte("<title>Caf\xe9 menu</title>")) // 0xE9 is é in ISO-8859-1
	}))
	defer srv.Close()

	page, err := openFetcher().FetchPage(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "Café menu", page.Title)
}

func TestFetchPage_ReadsAtMostTheBodyLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>Big</title><body>"))
		chunk := []byte(strings.Repeat("a ", 32<<10))
		for i := 0; i < 200; i++ { // about 13 MB offered
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	page, err := openFetcher().FetchPage(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "Big", page.Title)
	assert.LessOrEqual(t, len(page.Text), maxBodyBytes)
}

func TestFetchPage_HonoursTheContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := openFetcher().FetchPage(ctx, srv.URL)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}
