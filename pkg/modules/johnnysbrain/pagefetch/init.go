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
	"os"
	"strings"

	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
)

const (
	// EnvLinkPreviews switches link previews off when set to "off".
	EnvLinkPreviews = "JB_LINK_PREVIEWS"
	// EnvLinkCheck switches the weekly broken-link check off when set to "off".
	EnvLinkCheck = "JB_LINK_CHECK"
)

// Init installs the page fetcher unless link previews are switched off.
func Init() {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(EnvLinkPreviews)), "off") {
		log.Infof("Bookmark link previews: off (%s=off)", EnvLinkPreviews)
		return
	}
	models.SetBookmarkPageFetcher(New())
	log.Infof("Bookmark link previews: on (set %s=off to disable)", EnvLinkPreviews)

	if strings.EqualFold(strings.TrimSpace(os.Getenv(EnvLinkCheck)), "off") {
		log.Infof("Bookmark weekly link check: off (%s=off)", EnvLinkCheck)
		return
	}
	models.RegisterBookmarkLinkCheckCron()
	log.Infof("Bookmark weekly link check: on, Sundays 3am (set %s=off to disable)", EnvLinkCheck)
}
