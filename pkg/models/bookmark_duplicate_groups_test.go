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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBookmarkDuplicateGroups(t *testing.T) {
	u1 := &user.User{ID: 1}
	note := "PSST Guide resource for 3. CleanYourTV: your TV, minus the ads."

	seed := func(t *testing.T, items []BookmarkImportItem) {
		t.Helper()
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		importAt(t, s, func() (*BookmarkImportResult, error) {
			return ImportBookmarks(s, u1, &BookmarkImport{Items: items})
		})
	}
	scan := func(t *testing.T) {
		t.Helper()
		payload, err := json.Marshal(&BookmarkDuplicateScanEvent{UserID: u1.ID})
		require.NoError(t, err)
		require.NoError(t, (&ScanBookmarkDuplicates{}).Handle(message.NewMessage("scan", payload)))
	}
	groups := func(t *testing.T) []*BookmarkDuplicateGroup {
		t.Helper()
		s := db.NewSession()
		defer s.Close()
		g, err := GetBookmarkDuplicateGroups(s, u1)
		require.NoError(t, err)
		return g
	}
	urlsOf := func(g *BookmarkDuplicateGroup) []string {
		urls := []string{}
		for _, b := range g.Bookmarks {
			urls = append(urls, b.URL)
		}
		return urls
	}

	t.Run("links with the same note are grouped, whatever their titles", func(t *testing.T) {
		seed(t, []BookmarkImportItem{
			{URL: "https://github.com/Gathin23/cleanyourtv", Title: "CleanYourTV on GitHub", Description: note},
			{URL: "https://cleanyour.tv/app", Title: "cleanyour.tv/app", Description: note},
			{URL: "https://cleanyour.tv/not-android", Title: "cleanyour.tv/not-android", Description: note},
			{URL: "https://example.net/unrelated", Title: "Something else entirely", Description: "A completely different note about boxing gyms"},
		})
		scan(t)

		g := groups(t)
		require.Len(t, g, 1)
		assert.ElementsMatch(t, []string{
			"https://github.com/Gathin23/cleanyourtv", "https://cleanyour.tv/app", "https://cleanyour.tv/not-android",
		}, urlsOf(g[0]))
		assert.Len(t, g[0].Pairs, 3, "every pair is listed so the group can be dismissed")
	})

	t.Run("a note shared by many links is boilerplate and groups nothing", func(t *testing.T) {
		words := []string{"Kettlebell", "Sourdough", "Passport", "Telescope", "Mortgage", "Lighthouse", "Harmonica", "Vineyard"}
		items := []BookmarkImportItem{}
		for i := 0; i < duplicateSameNoteMaxGroup+2; i++ {
			items = append(items, BookmarkImportItem{
				URL:         fmt.Sprintf("https://example.net/page-%d", i),
				Title:       words[i],
				Description: "Linked resource in the TWEAK Guide to Claude modifications and workflows.",
			})
		}
		seed(t, items)
		scan(t)
		assert.Empty(t, groups(t))
	})

	t.Run("a short note is not a signal", func(t *testing.T) {
		seed(t, []BookmarkImportItem{
			{URL: "https://example.net/a", Title: "Alpha topic", Description: "Short note"},
			{URL: "https://example.net/b", Title: "Zulu matter", Description: "Short note"},
		})
		scan(t)
		assert.Empty(t, groups(t))
	})

	t.Run("dismissed pairs stay out after a re-scan", func(t *testing.T) {
		seed(t, []BookmarkImportItem{
			{URL: "https://example.net/one", Title: "First", Description: note},
			{URL: "https://example.net/two", Title: "Second", Description: note},
		})
		scan(t)
		g := groups(t)
		require.Len(t, g, 1)

		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DismissBookmarkDuplicate(s, u1, g[0].Pairs[0].BookmarkID, g[0].Pairs[0].DuplicateID))
		require.NoError(t, s.Commit())

		scan(t)
		assert.Empty(t, groups(t))
	})

	t.Run("only your own bookmarks are grouped, and a link share has none", func(t *testing.T) {
		seed(t, []BookmarkImportItem{
			{URL: "https://example.net/one", Title: "First", Description: note},
			{URL: "https://example.net/two", Title: "Second", Description: strings.ToUpper(note)},
		})
		scan(t)

		s := db.NewSession()
		defer s.Close()
		other, err := GetBookmarkDuplicateGroups(s, &user.User{ID: 2})
		require.NoError(t, err)
		assert.Empty(t, other)

		_, err = GetBookmarkDuplicateGroups(s, nil)
		require.Error(t, err)
	})

	t.Run("scan request is queued for the caller", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		events.ClearDispatchedEvents()
		require.NoError(t, RequestBookmarkDuplicateScan(s, u1))
		require.NoError(t, s.Commit())
		events.DispatchPending(context.Background(), s)
		sent := events.GetDispatchedEvents((&BookmarkDuplicateScanEvent{}).Name())
		require.Len(t, sent, 1)
		assert.Equal(t, u1.ID, sent[0].(*BookmarkDuplicateScanEvent).UserID)

		require.Error(t, RequestBookmarkDuplicateScan(s, nil))
	})
}
