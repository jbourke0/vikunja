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
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.255.255.254", "::1", "0.0.0.0", "::",
		"10.0.0.1", "10.255.255.255", "172.16.0.1", "172.31.255.255", "192.168.0.1", "192.168.100.190",
		"169.254.169.254", "169.254.0.1", "fe80::1", "fe80::1%eth0",
		"100.64.0.1", "100.100.100.100", "100.127.255.255", // CGNAT / Tailscale
		"fc00::1", "fd12:3456::1", // unique local
		"224.0.0.1", "ff02::1", "255.255.255.255", "240.0.0.1",
		"192.0.2.1", "198.51.100.1", "203.0.113.1", "198.18.0.1", "192.0.0.1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:192.168.1.1", "::ffff:169.254.169.254", "::ffff:100.64.0.1", // IPv4-mapped
		"64:ff9b::7f00:1", "64:ff9b::a00:1", // NAT64 to loopback / 10.0.0.1
		"2002:7f00:1::1", "2002:c0a8:101::1", // 6to4 to loopback / 192.168.1.1
		"2001:db8::1", "2001::1", "fec0::1",
	}
	for _, s := range blocked {
		addr, err := netip.ParseAddr(s)
		require.NoError(t, err, s)
		assert.False(t, IsPublicIP(addr), "%s must be refused", s)
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "100.63.255.255", "100.128.0.1", "172.15.0.1", "172.32.0.1", "2606:4700:4700::1111", "::ffff:8.8.8.8"}
	for _, s := range allowed {
		addr, err := netip.ParseAddr(s)
		require.NoError(t, err, s)
		assert.True(t, IsPublicIP(addr), "%s must be allowed", s)
	}
}

func TestFetcherRefusesPrivateAddresses(t *testing.T) {
	var hits atomic.Int32
	local := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer local.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(local.URL, "http://"))
	require.NoError(t, err)

	f := New()
	urls := []string{
		"http://127.0.0.1:" + port + "/",
		"http://localhost:" + port + "/",
		"http://[::1]:" + port + "/",
		"http://0.0.0.0:" + port + "/",
		"http://192.168.1.1/",
		"http://192.168.100.190:9000/", // a homelab service
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/", // CGNAT / Tailscale
		"http://[::ffff:127.0.0.1]:" + port + "/",
		"http://[fe80::1]/",
		"https://192.168.1.1/",
	}
	for _, u := range urls {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		page, err := f.FetchPage(ctx, u)
		cancel()
		require.Error(t, err, u)
		require.ErrorIs(t, err, models.ErrBookmarkPageBlocked, "%s must be refused, got: %v", u, err)
		assert.Nil(t, page, u)
	}
	assert.Zero(t, hits.Load(), "the local server must never receive a request")
}

func TestFetcherNeverReachesLoopbackThroughOddNumericForms(t *testing.T) {
	var hits atomic.Int32
	local := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer local.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(local.URL, "http://"))
	require.NoError(t, err)

	f := New()
	// Shorthand, decimal, hex and octal spellings of 127.0.0.1. Depending on the resolver they are
	// either refused at dial time or fail to resolve; either way no connection may be made.
	for _, host := range []string{"127.1", "2130706433", "0x7f.0.0.1", "0x7f000001", "017700000001", "127.0.1"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		page, err := f.FetchPage(ctx, "http://"+host+":"+port+"/")
		cancel()
		require.Error(t, err, host)
		assert.Nil(t, page, host)
	}
	assert.Zero(t, hits.Load(), "the local server must never receive a request")
}

func TestFetcherChecksEveryRedirect(t *testing.T) {
	// 127.0.0.1 stands in for a public host, 127.0.0.2 for a private one: Linux routes all of
	// 127/8 to loopback, so two servers can sit on different "networks" in a test.
	var privateHits atomic.Int32
	private := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		privateHits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>secret</title>"))
	}))
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot listen on 127.0.0.2: %v", err)
	}
	private.Listener = l
	private.Start()
	defer private.Close()

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, private.URL+"/secret", http.StatusFound)
	}))
	defer public.Close()

	f := newFetcher(func(ip netip.Addr) bool { return ip == netip.MustParseAddr("127.0.0.1") })

	// Sanity: the public stand-in works on its own.
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>fine</title>"))
	}))
	defer ok.Close()
	page, err := f.FetchPage(context.Background(), ok.URL)
	require.NoError(t, err)
	assert.Equal(t, "fine", page.Title)

	// Control: with no address policy the very same redirect does reach the private server, so
	// the assertions below can fail if the guard is ever weakened.
	page, err = openFetcher().FetchPage(context.Background(), public.URL)
	require.NoError(t, err)
	assert.Equal(t, "secret", page.Title)
	require.Equal(t, int32(1), privateHits.Load(), "without the guard the redirect is followed")
	privateHits.Store(0)

	page, err = f.FetchPage(context.Background(), public.URL)
	require.ErrorIs(t, err, models.ErrBookmarkPageBlocked, "a redirect into a private address must be refused")
	assert.Nil(t, page)
	assert.Zero(t, privateHits.Load(), "the private server must never receive the redirected request")
}

func TestFetcherIsLockedDown(t *testing.T) {
	f := New()
	assert.Nil(t, f.transport.Proxy, "no proxy from the environment")
	assert.True(t, f.transport.DisableKeepAlives)

	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/x", "gopher://example.com", "javascript:alert(1)", "http://", "//example.com", "", "not a url"} {
		page, err := f.FetchPage(context.Background(), u)
		require.Error(t, err, u)
		assert.Nil(t, page, u)
		assert.NotErrorIs(t, err, models.ErrBookmarkPageBlocked, "%q is invalid, not blocked", u)
	}
}
