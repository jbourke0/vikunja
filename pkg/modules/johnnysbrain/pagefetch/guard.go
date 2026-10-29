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

// Package pagefetch fetches web pages for Johnny's Brain bookmarks and pulls out a title,
// description, icon, preview image and readable text.
//
// The server runs on a home network, so fetching arbitrary links is a server-side request
// forgery risk: a bookmark could point at the router, Portainer or a cloud metadata address.
// Every connection is therefore checked at dial time, against the address actually being
// connected to, so redirects and DNS tricks cannot slip past a check made earlier.
package pagefetch

import (
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"code.vikunja.io/api/pkg/models"
)

// blockedPrefixes are ranges that are not covered by netip's own predicates but are not the
// public internet either: CGNAT (where Tailscale lives), documentation and benchmarking
// ranges, reserved space, and IPv6 forms that embed an IPv4 address.
var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",
	"100.64.0.0/10", // CGNAT, Tailscale
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",  // reserved, includes broadcast
	"64:ff9b::/96", // NAT64 embeds an IPv4 address
	"64:ff9b:1::/48",
	"100::/64",
	"2001::/23", // Teredo and IETF protocol assignments
	"2001:db8::/32",
	"2002::/16", // 6to4 embeds an IPv4 address
	"fec0::/10", // deprecated site-local
)

func mustPrefixes(cidrs ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		prefixes = append(prefixes, netip.MustParsePrefix(c))
	}
	return prefixes
}

// IsPublicIP reports whether an address is on the public internet, meaning one the server may
// connect to. Loopback, private, link-local, CGNAT, multicast, unspecified and reserved
// addresses are refused, including their IPv4-mapped IPv6 spellings.
func IsPublicIP(addr netip.Addr) bool {
	addr = addr.Unmap().WithZone("") // a zone makes Prefix.Contains answer false for everything
	if !addr.IsValid() ||
		addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// guardedControl returns a net.Dialer Control function that refuses to connect to any address
// allow does not approve. Control runs after name resolution, once per attempted address.
func guardedControl(allow func(netip.Addr) bool) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if network != "tcp4" && network != "tcp6" {
			return fmt.Errorf("%w: network %q", models.ErrBookmarkPageBlocked, network)
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("%w: %q", models.ErrBookmarkPageBlocked, address)
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !allow(ip) {
			return fmt.Errorf("%w: %s", models.ErrBookmarkPageBlocked, host)
		}
		return nil
	}
}
