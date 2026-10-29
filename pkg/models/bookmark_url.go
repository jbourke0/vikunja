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

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// trackingParams are query parameters that identify a click or campaign, not the page.
var trackingParams = map[string]bool{
	"fbclid": true, "gclid": true, "dclid": true, "msclkid": true, "mc_cid": true, "mc_eid": true,
	"igshid": true, "si": true, "ref": true, "ref_src": true, "amp": true, "_ga": true, "yclid": true,
}

// NormalizeBookmarkURL reduces a URL to the form two links to the same page share: no scheme,
// lower-case host without www/m/mobile, default ports dropped, no trailing slash or /amp,
// no fragment, tracking parameters removed and the rest sorted.
//
// Keep in step with the frozen copy in migration 20261029105012 only where a change should
// also apply to rows saved before it; otherwise add a migration that re-hashes.
func NormalizeBookmarkURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	host = strings.TrimPrefix(host, "mobile.")
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		host += ":" + port
	}
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	path = strings.TrimSuffix(path, "/amp")
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || trackingParams[lower] {
			query.Del(key)
		}
	}
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := query[key]
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(v))
		}
	}
	normalized := host + path
	if len(parts) > 0 {
		normalized += "?" + strings.Join(parts, "&")
	}
	return normalized
}

// BookmarkURLHash is the stored lookup key for a URL: the SHA-256 of its normalised form.
func BookmarkURLHash(raw string) string {
	sum := sha256.Sum256([]byte(NormalizeBookmarkURL(raw)))
	return hex.EncodeToString(sum[:])
}
