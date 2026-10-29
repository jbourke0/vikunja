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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelForFolders(t *testing.T) {
	cases := []struct {
		folders []string
		want    string
	}{
		{[]string{"Bookmarks bar", "Work", "Tools"}, "Tools"},
		{[]string{"Bookmarks bar", "Work"}, "Work"},
		{[]string{"Bookmarks bar"}, ""},
		{[]string{"Other bookmarks"}, ""},
		{[]string{"Bookmarks Menu", "Imported from Chrome", "Recipes"}, "Recipes"},
		{[]string{"Work", "Bookmarks bar"}, "Work"},
		{[]string{"Imported from Firefox"}, ""},
		{[]string{"  Gym   stuff  "}, "Gym stuff"},
		{[]string{"", "  "}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, labelForFolders(tc.folders), "%v", tc.folders)
	}
	assert.Len(t, []rune(labelForFolders([]string{string(make([]rune, 0)) + fmt.Sprintf("%090d", 1)})), bookmarkImportLabelMaxLen)
}

func importAt(t *testing.T, s interface{ Commit() error }, fn func() (*BookmarkImportResult, error)) *BookmarkImportResult {
	t.Helper()
	res, err := fn()
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	return res
}

func TestImportBookmarks(t *testing.T) {
	u1 := &user.User{ID: 1}
	past := time.Date(2019, time.March, 4, 9, 30, 0, 0, time.UTC)
	future := time.Now().Add(48 * time.Hour)
	ancient := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

	t.Run("saves new links and counts the rest", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		events.ClearDispatchedEvents()
		s := db.NewSession()
		defer s.Close()

		res := importAt(t, s, func() (*BookmarkImportResult, error) {
			return ImportBookmarks(s, u1, &BookmarkImport{Status: BookmarkStatusReference, Items: []BookmarkImportItem{
				{URL: "https://www.example.net/a", Title: "Page A", Added: &past},
				{URL: "https://example.net/b"},
				{URL: "https://example.net/a/?utm_source=x"}, // same page as A, in the batch
				{URL: "http://www.example.com/one/"},         // the user already has this one
				{URL: "javascript:alert(1)"},
				{URL: "ftp://example.net/file"},
				{URL: ""},
				{URL: "not a url"},
				{URL: "https://example.net/future", Added: &future},
				{URL: "https://example.net/ancient", Added: &ancient},
			}})
		})
		assert.Equal(t, 4, res.Created)
		assert.Equal(t, 2, res.Duplicates)
		assert.Equal(t, 4, res.Invalid)
		assert.Zero(t, res.LabelsCreated)

		a := loadBookmarkByURL(t, "https://www.example.net/a")
		assert.Equal(t, "Page A", a.Title)
		assert.Equal(t, BookmarkStatusReference, a.Status)
		assert.True(t, a.Created.Equal(past), "the original bookmarked date is kept: %s", a.Created)
		assert.Equal(t, int64(1), a.CreatedByID)
		assert.NotEmpty(t, a.URLHash)

		b := loadBookmarkByURL(t, "https://example.net/b")
		assert.Equal(t, "example.net", b.Title, "no title falls back to the host name, to be replaced by the page title")
		assert.WithinDuration(t, time.Now(), b.Created, time.Minute)

		assert.WithinDuration(t, time.Now(), loadBookmarkByURL(t, "https://example.net/future").Created, time.Minute, "a future date is ignored")
		assert.WithinDuration(t, time.Now(), loadBookmarkByURL(t, "https://example.net/ancient").Created, time.Minute, "a date before 1995 is ignored")

		events.DispatchPending(context.Background(), s)
		sent := events.GetDispatchedEvents((&BookmarkBatchSavedEvent{}).Name())
		require.Len(t, sent, 1)
		assert.Len(t, sent[0].(*BookmarkBatchSavedEvent).BookmarkIDs, 4)
		assert.Zero(t, events.CountDispatchedEvents((&BookmarkSavedEvent{}).Name()), "no per-bookmark events: one batch job instead")
	})

	t.Run("another user's bookmark is not a duplicate", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// user 2 has https://example.com/two
		res := importAt(t, s, func() (*BookmarkImportResult, error) {
			return ImportBookmarks(s, u1, &BookmarkImport{Items: []BookmarkImportItem{{URL: "https://example.com/two"}}})
		})
		assert.Equal(t, 1, res.Created)
		assert.Zero(t, res.Duplicates)
	})

	t.Run("folders become labels, reusing the owner's", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Fixture label 1 is "Label #1", created by user 1.
		res := importAt(t, s, func() (*BookmarkImportResult, error) {
			return ImportBookmarks(s, u1, &BookmarkImport{FoldersAsLabels: true, Items: []BookmarkImportItem{
				{URL: "https://example.net/w1", Folders: []string{"Bookmarks bar", "Work", "Tools"}},
				{URL: "https://example.net/w2", Folders: []string{"Bookmarks bar", "tools"}},
				{URL: "https://example.net/r1", Folders: []string{"Label #1"}},
				{URL: "https://example.net/r2", Folders: []string{"label #1"}},
				{URL: "https://example.net/none", Folders: []string{"Bookmarks bar"}},
			}})
		})
		assert.Equal(t, 5, res.Created)
		assert.Equal(t, 1, res.LabelsCreated, "only 'Tools' is new; matching ignores case")

		labelsOf := func(url string) []string {
			b := loadBookmarkByURL(t, url)
			sess := db.NewSession()
			defer sess.Close()
			require.NoError(t, addBookmarkDetails(sess, []*Bookmark{b}))
			titles := []string{}
			for _, l := range b.Labels {
				titles = append(titles, l.Title)
			}
			return titles
		}
		assert.Equal(t, []string{"Tools"}, labelsOf("https://example.net/w1"))
		assert.Equal(t, []string{"Tools"}, labelsOf("https://example.net/w2"), "the same label, not a second 'tools'")
		assert.Equal(t, []string{"Label #1"}, labelsOf("https://example.net/r1"))
		assert.Equal(t, []string{"Label #1"}, labelsOf("https://example.net/r2"))
		assert.Empty(t, labelsOf("https://example.net/none"))
		db.AssertExists(t, "labels", map[string]interface{}{"title": "Tools", "created_by_id": 1}, false)
	})

	t.Run("without the option folders are ignored", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		res := importAt(t, s, func() (*BookmarkImportResult, error) {
			return ImportBookmarks(s, u1, &BookmarkImport{Items: []BookmarkImportItem{{URL: "https://example.net/x", Folders: []string{"Work"}}}})
		})
		assert.Zero(t, res.LabelsCreated)
		db.AssertMissing(t, "labels", map[string]interface{}{"title": "Work"})
	})

	t.Run("limits and permissions", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		items := make([]BookmarkImportItem, bookmarkImportMaxItems+1)
		for i := range items {
			items[i] = BookmarkImportItem{URL: fmt.Sprintf("https://example.net/%d", i)}
		}
		_, err := ImportBookmarks(s, u1, &BookmarkImport{Items: items})
		assert.True(t, IsErrBookmarkImportTooLarge(err))

		_, err = ImportBookmarks(s, &LinkSharing{ID: 1}, &BookmarkImport{Items: items[:1]})
		assert.True(t, IsErrGenericForbidden(err))

		_, err = ImportBookmarks(s, u1, &BookmarkImport{Status: "someday", Items: items[:1]})
		assert.True(t, IsErrBookmarkInvalidStatus(err))
	})
}

func loadBookmarkByURL(t *testing.T, url string) *Bookmark {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	b := &Bookmark{}
	has, err := s.Where("url_hash = ?", BookmarkURLHash(url)).Get(b)
	require.NoError(t, err)
	require.True(t, has, "no bookmark for %s", url)
	return b
}

// concurrentFetcher is a page fetcher safe to call from several goroutines.
type concurrentFetcher struct {
	mu          sync.Mutex
	calls       []string
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
	delay       time.Duration
}

func (f *concurrentFetcher) FetchPage(_ context.Context, rawURL string) (*BookmarkPage, error) {
	now := f.inFlight.Add(1)
	for {
		prev := f.maxInFlight.Load()
		if now <= prev || f.maxInFlight.CompareAndSwap(prev, now) {
			break
		}
	}
	time.Sleep(f.delay)
	f.inFlight.Add(-1)

	f.mu.Lock()
	f.calls = append(f.calls, rawURL)
	f.mu.Unlock()
	return &BookmarkPage{StatusCode: 200, Title: "Fetched " + rawURL}, nil
}

func (f *concurrentFetcher) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func TestRefreshBookmarksInBackground_LimitsConcurrency(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	u1 := &user.User{ID: 1}
	ids := []int64{}
	for i := 0; i < 8; i++ {
		ids = append(ids, newBookmark(t, u1, &Bookmark{URL: fmt.Sprintf("https://example.net/many/%d", i)}).ID)
	}

	// Starts are paced about 150ms apart, so a fetch has to take longer than that to overlap.
	f := &concurrentFetcher{delay: 500 * time.Millisecond}
	refreshBookmarksInBackground(context.Background(), f, ids)

	assert.Len(t, f.called(), 8, "every bookmark was fetched")
	assert.Equal(t, 3, int(f.maxInFlight.Load()), "three at a time, and never more")
	for _, id := range ids {
		assert.Equal(t, 200, loadBookmark(t, id).LinkStatus)
	}
}

func TestRefreshBookmarksInBackground_StopsWhenCancelled(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	u1 := &user.User{ID: 1}
	ids := []int64{}
	for i := 0; i < 6; i++ {
		ids = append(ids, newBookmark(t, u1, &Bookmark{URL: fmt.Sprintf("https://example.net/cancel/%d", i)}).ID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := &concurrentFetcher{}
	refreshBookmarksInBackground(ctx, f, ids)
	assert.Empty(t, f.called())
}

func TestProcessImportedBookmarks(t *testing.T) {
	u1 := &user.User{ID: 1}
	db.LoadAndAssertFixtures(t)

	s := db.NewSession()
	res, err := ImportBookmarks(s, u1, &BookmarkImport{Items: []BookmarkImportItem{
		{URL: "https://news-one.example/geelong-council-approves-waterfront-plan", Title: "Geelong council approves waterfront plan"},
		{URL: "https://other-news.example/waterfront-plan-approved-geelong", Title: "Waterfront plan approved by Geelong council"},
		{URL: "https://example.org/sourdough", Title: "Easy sourdough starter"},
	}})
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	s.Close()
	require.Equal(t, 3, res.Created)

	ids := []int64{
		loadBookmarkByURL(t, "https://news-one.example/geelong-council-approves-waterfront-plan").ID,
		loadBookmarkByURL(t, "https://other-news.example/waterfront-plan-approved-geelong").ID,
		loadBookmarkByURL(t, "https://example.org/sourdough").ID,
	}

	fetcher := &concurrentFetcher{}
	judge := &recordingJudge{}
	SetBookmarkPageFetcher(fetcher)
	SetBookmarkDuplicateJudge(judge)
	defer SetBookmarkPageFetcher(nil)
	defer SetBookmarkDuplicateJudge(nil)

	events.TestListener(t, &BookmarkBatchSavedEvent{BookmarkIDs: ids}, &ProcessImportedBookmarks{})

	assert.Len(t, fetcher.called(), 3, "every imported page is fetched")
	assert.Empty(t, judge.subjects, "an import never spends the AI judge")
	for _, id := range ids {
		assert.Equal(t, 200, loadBookmark(t, id).LinkStatus)
	}
	lo, hi := orderedPair(ids[0], ids[1])
	db.AssertExists(t, "bookmark_duplicates", map[string]interface{}{"bookmark_id": lo, "duplicate_id": hi, "reason": "similar", "status": "open"}, false)
	db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"duplicate_id": ids[2]})
}

func TestRunBookmarkLinkCheck(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	now := time.Now()

	// Fixture bookmarks 1-5 are active and unchecked; 6 is archived. Make 3 recently checked and 4
	// checked long ago.
	s := db.NewSession()
	_, err := s.Exec("UPDATE bookmarks SET checked_at = ? WHERE id = 3", now.Add(-24*time.Hour))
	require.NoError(t, err)
	_, err = s.Exec("UPDATE bookmarks SET checked_at = ? WHERE id = 4", now.Add(-30*24*time.Hour))
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	s.Close()

	f := &concurrentFetcher{}
	checked, err := RunBookmarkLinkCheck(context.Background(), f, now)
	require.NoError(t, err)

	assert.Equal(t, 4, checked)
	assert.ElementsMatch(t, []string{
		"https://example.com/one", "https://example.com/two", "https://example.com/four", "https://example.com/five",
	}, f.called(), "unchecked and long-unchecked ones, not the one checked yesterday, not the archived one")

	again, err := RunBookmarkLinkCheck(context.Background(), f, now.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 0, again, "nothing is due straight after a run")

	later, err := RunBookmarkLinkCheck(context.Background(), f, now.Add(7*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 5, later, "a week on, everything active is due again (bookmark 3 included)")
}

func TestBrokenLinkFilter(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	u1 := &user.User{ID: 1}

	s := db.NewSession()
	defer s.Close()
	for id, status := range map[int64]int{1: 404, 3: 403, 4: 503, 6: BookmarkLinkStatusUnreachable} {
		_, err := s.Exec("UPDATE bookmarks SET link_status = ? WHERE id = ?", status, id)
		require.NoError(t, err)
	}
	require.NoError(t, s.Commit())

	result, _, total, err := (&Bookmark{FilterBroken: true}).ReadAll(s, u1, "", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{6, 4, 1}, bookmarkIDs(t, result), "gone, server error and unreachable; a 403 is not a broken link")
	assert.Equal(t, int64(3), total)

	for status, broken := range map[int]bool{200: false, 301: false, 401: false, 403: false, 429: false, 404: true, 410: true, 500: true, 503: true, -1: true, -2: false, 0: false} {
		b := &Bookmark{FilterBroken: true}
		_, err := s.Exec("UPDATE bookmarks SET link_status = ? WHERE id = 1", status)
		require.NoError(t, err)
		res, _, _, err := b.ReadAll(s, u1, "example.com/one", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, broken, len(bookmarkIDs(t, res)) == 1, "status %d", status)
	}
}

func TestGetBookmarkSnapshot(t *testing.T) {
	u1 := &user.User{ID: 1}
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	snap, err := GetBookmarkSnapshot(s, u1, 1)
	require.NoError(t, err)
	assert.Equal(t, "Photographs from the grading day", snap.Text)

	_, err = GetBookmarkSnapshot(s, u1, 3)
	assert.True(t, IsErrBookmarkSnapshotDoesNotExist(err), "readable, but nothing saved")
	_, err = GetBookmarkSnapshot(s, u1, 2)
	assert.True(t, IsErrGenericForbidden(err), "user 2's private bookmark, which has a snapshot")
	_, err = GetBookmarkSnapshot(s, u1, 9999)
	assert.True(t, IsErrBookmarkDoesNotExist(err))
}

func TestDeleteUserBookmarks(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	// A comment user 2 left on user 1's bookmark goes too; user 1's own data stays.
	_, err := s.Insert(&BookmarkComment{BookmarkID: 1, AuthorID: 2, Comment: "from user two"})
	require.NoError(t, err)

	require.NoError(t, deleteUserBookmarks(s, 2))
	require.NoError(t, s.Commit())

	for _, id := range []int{2, 3, 4} {
		db.AssertMissing(t, "bookmarks", map[string]interface{}{"id": id})
	}
	db.AssertMissing(t, "bookmark_projects", map[string]interface{}{"bookmark_id": 3})
	db.AssertMissing(t, "bookmark_snapshots", map[string]interface{}{"bookmark_id": 2})
	db.AssertMissing(t, "bookmark_comments", map[string]interface{}{"author_id": 2})
	db.AssertExists(t, "bookmarks", map[string]interface{}{"id": 1}, false)
	db.AssertExists(t, "bookmark_comments", map[string]interface{}{"id": 1}, false)
	db.AssertExists(t, "bookmarks", map[string]interface{}{"id": 5}, false)
}
