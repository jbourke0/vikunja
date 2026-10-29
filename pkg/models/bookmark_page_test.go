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
	"errors"
	"fmt"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePageFetcher struct {
	page  *BookmarkPage
	err   error
	calls []string
	// during runs inside FetchPage, to simulate something happening while the page downloads.
	during func()
}

func (f *fakePageFetcher) FetchPage(_ context.Context, rawURL string) (*BookmarkPage, error) {
	f.calls = append(f.calls, rawURL)
	if f.during != nil {
		f.during()
	}
	return f.page, f.err
}

func loadBookmark(t *testing.T, id int64) *Bookmark {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	b, err := getBookmarkByIDSimple(s, id)
	require.NoError(t, err)
	return b
}

func loadSnapshot(t *testing.T, id int64) *BookmarkSnapshot {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	snap := &BookmarkSnapshot{}
	has, err := s.Where("bookmark_id = ?", id).Get(snap)
	require.NoError(t, err)
	if !has {
		return nil
	}
	return snap
}

func newBookmark(t *testing.T, u *user.User, b *Bookmark) *Bookmark {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	require.NoError(t, b.Create(s, u))
	require.NoError(t, s.Commit())
	return b
}

func fullPage() *BookmarkPage {
	return &BookmarkPage{
		URL:         "https://www.example.net/news",
		StatusCode:  200,
		Title:       "Real page title",
		Description: "What the page says about itself.",
		SiteName:    "Example News",
		FaviconURL:  "https://www.example.net/fav.png",
		ImageURL:    "https://www.example.net/share.jpg",
		Text:        "The readable text of the page.",
	}
}

func TestRefreshBookmarkFromPage_FillsOnlyBlanks(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("a new bookmark is filled in from the page", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		b := newBookmark(t, u1, &Bookmark{URL: "https://www.example.net/news"})
		require.Equal(t, "example.net", b.Title, "default title is the host name")

		f := &fakePageFetcher{page: fullPage()}
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), f, b.ID))

		got := loadBookmark(t, b.ID)
		assert.Equal(t, "Real page title", got.Title)
		assert.Equal(t, "What the page says about itself.", got.Description)
		assert.Equal(t, "Example News", got.SiteName)
		assert.Equal(t, "https://www.example.net/fav.png", got.FaviconURL)
		assert.Equal(t, "https://www.example.net/share.jpg", got.ImageURL)
		assert.Equal(t, 200, got.LinkStatus)
		assert.NotNil(t, got.CheckedAt)
		assert.Equal(t, []string{"https://www.example.net/news"}, f.calls)

		snap := loadSnapshot(t, b.ID)
		require.NotNil(t, snap)
		assert.Equal(t, "The readable text of the page.", snap.Text)
	})

	t.Run("what the owner typed is never overwritten", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		// Fixture bookmark 1 has its own title and description but no icon, image or site name.
		f := &fakePageFetcher{page: fullPage()}
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), f, 1))

		got := loadBookmark(t, 1)
		assert.Equal(t, "Bookmark #1", got.Title)
		assert.Equal(t, "First example bookmark", got.Description)
		assert.Equal(t, "Example News", got.SiteName, "blanks are still filled")
		assert.Equal(t, "https://www.example.net/share.jpg", got.ImageURL)
	})

	t.Run("a second refresh does not replace an earlier fill", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		b := newBookmark(t, u1, &Bookmark{URL: "https://www.example.net/news"})
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), &fakePageFetcher{page: fullPage()}, b.ID))

		changed := fullPage()
		changed.Title, changed.ImageURL, changed.Text = "Retitled page", "https://www.example.net/new.jpg", "Newer text."
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), &fakePageFetcher{page: changed}, b.ID))

		got := loadBookmark(t, b.ID)
		assert.Equal(t, "Real page title", got.Title, "already filled, so left alone")
		assert.Equal(t, "https://www.example.net/share.jpg", got.ImageURL)
		assert.Equal(t, "Newer text.", loadSnapshot(t, b.ID).Text, "the snapshot is the one thing a refresh replaces")
	})

	t.Run("limits are respected", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		b := newBookmark(t, u1, &Bookmark{URL: "https://www.example.net/news"})
		page := fullPage()
		page.Title = string(make([]rune, 0)) + fmt.Sprintf("%0400d", 1)
		page.Text = fmt.Sprintf("%0*d", bookmarkSnapshotMaxChars+5000, 7)
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), &fakePageFetcher{page: page}, b.ID))

		assert.Len(t, []rune(loadBookmark(t, b.ID).Title), 250)
		assert.Len(t, []rune(loadSnapshot(t, b.ID).Text), bookmarkSnapshotMaxChars)
	})
}

func TestRefreshBookmarkFromPage_FailuresKeepWhatWeHave(t *testing.T) {
	setup := func(t *testing.T) *Bookmark {
		t.Helper()
		db.LoadAndAssertFixtures(t)
		b := newBookmark(t, &user.User{ID: 1}, &Bookmark{URL: "https://www.example.net/news"})
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), &fakePageFetcher{page: fullPage()}, b.ID))
		return b
	}

	cases := []struct {
		name       string
		fetcher    *fakePageFetcher
		wantStatus int
	}{
		{"page not found", &fakePageFetcher{page: &BookmarkPage{StatusCode: 404, Title: "Page not found", Text: "Oops"}}, 404},
		{"server error", &fakePageFetcher{page: &BookmarkPage{StatusCode: 503}}, 503},
		{"not html", &fakePageFetcher{page: &BookmarkPage{StatusCode: 200}}, 200},
		{"unreachable", &fakePageFetcher{err: errors.New("dial tcp: i/o timeout")}, BookmarkLinkStatusUnreachable},
		{"blocked address", &fakePageFetcher{err: fmt.Errorf("%w: 192.168.1.1", ErrBookmarkPageBlocked)}, BookmarkLinkStatusPrivate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := setup(t)
			require.NoError(t, RefreshBookmarkFromPage(context.Background(), tc.fetcher, b.ID))

			got := loadBookmark(t, b.ID)
			assert.Equal(t, tc.wantStatus, got.LinkStatus)
			assert.NotNil(t, got.CheckedAt)
			assert.Equal(t, "Real page title", got.Title, "an error page never retitles the bookmark")
			snap := loadSnapshot(t, b.ID)
			require.NotNil(t, snap, "the saved text survives a dead link")
			assert.Equal(t, "The readable text of the page.", snap.Text)
		})
	}
}

func TestRefreshBookmarkFromPage_EdgeCases(t *testing.T) {
	t.Run("a bookmark deleted before the fetch is ignored", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		f := &fakePageFetcher{page: fullPage()}
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), f, 9999))
		assert.Empty(t, f.calls)
	})

	t.Run("a link edited during the fetch is left to the newer fetch", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		b := newBookmark(t, &user.User{ID: 1}, &Bookmark{URL: "https://www.example.net/news"})
		f := &fakePageFetcher{page: fullPage()}
		f.during = func() {
			s := db.NewSession()
			defer s.Close()
			require.NoError(t, (&Bookmark{ID: b.ID, URL: "https://www.example.net/other", Title: "example.net"}).Update(s, &user.User{ID: 1}))
			require.NoError(t, s.Commit())
		}
		require.NoError(t, RefreshBookmarkFromPage(context.Background(), f, b.ID))

		got := loadBookmark(t, b.ID)
		assert.Equal(t, "example.net", got.Title, "the old page's title is not applied to the new link")
		assert.Zero(t, got.LinkStatus)
		assert.Nil(t, loadSnapshot(t, b.ID))
	})
}

func TestBookmarkLinkChangeResetsPageData(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	u1 := &user.User{ID: 1}
	b := newBookmark(t, u1, &Bookmark{URL: "https://www.example.net/news"})
	require.NoError(t, RefreshBookmarkFromPage(context.Background(), &fakePageFetcher{page: fullPage()}, b.ID))
	require.NotNil(t, loadSnapshot(t, b.ID))

	events.ClearDispatchedEvents()
	s := db.NewSession()
	defer s.Close()
	require.NoError(t, (&Bookmark{ID: b.ID, URL: "https://www.example.net/different", Title: "Real page title"}).Update(s, u1))
	require.NoError(t, s.Commit())
	events.DispatchPending(context.Background(), s)

	got := loadBookmark(t, b.ID)
	assert.Zero(t, got.LinkStatus, "the old address's check is cleared")
	assert.Nil(t, got.CheckedAt)
	assert.Nil(t, loadSnapshot(t, b.ID), "the old address's text is dropped")
	sent := events.GetDispatchedEvents((&BookmarkSavedEvent{}).Name())
	require.Len(t, sent, 1)
	assert.True(t, sent[0].(*BookmarkSavedEvent).FetchPage, "a new address is fetched")
}

func TestBookmarkSavedEventFetchFlag(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("create asks for a fetch", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		events.ClearDispatchedEvents()
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, (&Bookmark{URL: "https://example.net/new"}).Create(s, u1))
		require.NoError(t, s.Commit())
		events.DispatchPending(context.Background(), s)

		sent := events.GetDispatchedEvents((&BookmarkSavedEvent{}).Name())
		require.Len(t, sent, 1)
		assert.True(t, sent[0].(*BookmarkSavedEvent).FetchPage)
	})
	t.Run("a title edit only rechecks duplicates", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		events.ClearDispatchedEvents()
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, (&Bookmark{ID: 1, URL: "https://example.com/one", Title: "Renamed"}).Update(s, u1))
		require.NoError(t, s.Commit())
		events.DispatchPending(context.Background(), s)

		sent := events.GetDispatchedEvents((&BookmarkSavedEvent{}).Name())
		require.Len(t, sent, 1)
		assert.False(t, sent[0].(*BookmarkSavedEvent).FetchPage)
	})
}

// recordingJudge remembers what the duplicate judge was shown.
type recordingJudge struct {
	subjects []BookmarkForDuplicateCheck
}

func (r *recordingJudge) FindDuplicates(_ context.Context, subject BookmarkForDuplicateCheck, _ []BookmarkForDuplicateCheck) ([]BookmarkDuplicateMatch, error) {
	r.subjects = append(r.subjects, subject)
	return nil, nil
}

func TestProcessSavedBookmark(t *testing.T) {
	u1 := &user.User{ID: 1}
	setup := func(t *testing.T) (story1, story2 *Bookmark) {
		t.Helper()
		db.LoadAndAssertFixtures(t)
		story1 = newBookmark(t, u1, &Bookmark{URL: "https://news-one.example/geelong-council-approves-waterfront-plan", Title: "Geelong council approves waterfront plan"})
		story2 = newBookmark(t, u1, &Bookmark{URL: "https://other-news.example/waterfront-plan-approved-geelong"})
		return story1, story2
	}
	handle := func(t *testing.T, id int64, fetch bool) {
		t.Helper()
		events.TestListener(t, &BookmarkSavedEvent{BookmarkID: id, FetchPage: fetch}, &ProcessSavedBookmark{})
	}

	t.Run("fetches first, so the duplicate check sees the real title and page text", func(t *testing.T) {
		_, story2 := setup(t)
		fetcher := &fakePageFetcher{page: &BookmarkPage{StatusCode: 200, Title: "Waterfront plan approved by Geelong council", Text: "Council voted on Tuesday."}}
		judge := &recordingJudge{}
		SetBookmarkPageFetcher(fetcher)
		SetBookmarkDuplicateJudge(judge)
		defer SetBookmarkPageFetcher(nil)
		defer SetBookmarkDuplicateJudge(nil)

		handle(t, story2.ID, true)

		require.Len(t, fetcher.calls, 1)
		require.Len(t, judge.subjects, 1, "duplicate detection runs once per save")
		assert.Equal(t, "Waterfront plan approved by Geelong council", judge.subjects[0].Title)
		assert.Equal(t, "Council voted on Tuesday.", judge.subjects[0].PageText)
	})

	t.Run("an edit without a new link does not fetch", func(t *testing.T) {
		_, story2 := setup(t)
		fetcher := &fakePageFetcher{page: fullPage()}
		judge := &recordingJudge{}
		SetBookmarkPageFetcher(fetcher)
		SetBookmarkDuplicateJudge(judge)
		defer SetBookmarkPageFetcher(nil)
		defer SetBookmarkDuplicateJudge(nil)

		handle(t, story2.ID, false)

		assert.Empty(t, fetcher.calls)
		assert.Len(t, judge.subjects, 1)
	})

	t.Run("previews switched off still checks duplicates", func(t *testing.T) {
		_, story2 := setup(t)
		judge := &recordingJudge{}
		SetBookmarkPageFetcher(nil)
		SetBookmarkDuplicateJudge(judge)
		defer SetBookmarkDuplicateJudge(nil)

		handle(t, story2.ID, true)

		assert.Len(t, judge.subjects, 1)
		assert.Zero(t, loadBookmark(t, story2.ID).LinkStatus, "nothing was fetched")
	})

	t.Run("a failed fetch does not stop the duplicate check", func(t *testing.T) {
		_, story2 := setup(t)
		judge := &recordingJudge{}
		SetBookmarkPageFetcher(&fakePageFetcher{err: errors.New("boom")})
		SetBookmarkDuplicateJudge(judge)
		defer SetBookmarkPageFetcher(nil)
		defer SetBookmarkDuplicateJudge(nil)

		handle(t, story2.ID, true)

		assert.Len(t, judge.subjects, 1)
		assert.Equal(t, BookmarkLinkStatusUnreachable, loadBookmark(t, story2.ID).LinkStatus)
	})
}

func TestBookmarkSearchCoversPageText(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	u1 := &user.User{ID: 1}
	// Fixtures: bookmark 1 has the text "Photographs from the grading day"; user 2's private
	// bookmark 2 has "Private photographs of user two".
	s := db.NewSession()
	defer s.Close()

	result, _, total, err := (&Bookmark{}).ReadAll(s, u1, "photographs", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, bookmarkIDs(t, result), "matches the page text, never another user's private bookmark")
	assert.Equal(t, int64(1), total)

	result, _, _, err = (&Bookmark{}).ReadAll(s, u1, "nothing like this anywhere", 0, 0)
	require.NoError(t, err)
	assert.Empty(t, bookmarkIDs(t, result))

	items := func() []*Bookmark {
		r, _, _, e := (&Bookmark{}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, e)
		return r.([]*Bookmark)
	}()
	byID := map[int64]*Bookmark{}
	for _, b := range items {
		byID[b.ID] = b
	}
	assert.True(t, byID[1].HasSnapshot)
	assert.False(t, byID[3].HasSnapshot)

	one := &Bookmark{ID: 1}
	require.NoError(t, one.ReadOne(s, u1))
	assert.True(t, one.HasSnapshot)
}

func TestRequestBookmarkRefresh(t *testing.T) {
	u1 := &user.User{ID: 1}
	fetcher := &fakePageFetcher{}

	t.Run("owner queues a fetch", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		SetBookmarkPageFetcher(fetcher)
		defer SetBookmarkPageFetcher(nil)
		events.ClearDispatchedEvents()
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, RequestBookmarkRefresh(context.Background(), s, u1, 1))
		sent := events.GetDispatchedEvents((&BookmarkSavedEvent{}).Name())
		require.Len(t, sent, 1)
		assert.Equal(t, &BookmarkSavedEvent{BookmarkID: 1, FetchPage: true}, sent[0])
	})
	t.Run("a reader through a source link cannot", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		SetBookmarkPageFetcher(fetcher)
		defer SetBookmarkPageFetcher(nil)
		events.ClearDispatchedEvents()
		s := db.NewSession()
		defer s.Close()

		assert.True(t, IsErrGenericForbidden(RequestBookmarkRefresh(context.Background(), s, u1, 3)))
		assert.True(t, IsErrGenericForbidden(RequestBookmarkRefresh(context.Background(), s, u1, 2)))
		assert.Equal(t, 0, events.CountDispatchedEvents((&BookmarkSavedEvent{}).Name()))
	})
	t.Run("missing bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		SetBookmarkPageFetcher(fetcher)
		defer SetBookmarkPageFetcher(nil)
		s := db.NewSession()
		defer s.Close()

		assert.True(t, IsErrBookmarkDoesNotExist(RequestBookmarkRefresh(context.Background(), s, u1, 9999)))
	})
	t.Run("previews switched off", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		SetBookmarkPageFetcher(nil)
		s := db.NewSession()
		defer s.Close()

		assert.True(t, IsErrBookmarkPreviewsDisabled(RequestBookmarkRefresh(context.Background(), s, u1, 1)))
	})
}

func TestRetryWhenSQLiteBusy(t *testing.T) {
	t.Run("retries lock errors until they clear", func(t *testing.T) {
		calls := 0
		err := retryWhenSQLiteBusy(context.Background(), func() error {
			calls++
			if calls < 3 {
				return errors.New("database is locked")
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, 3, calls)
	})
	t.Run("does not retry other errors", func(t *testing.T) {
		calls := 0
		boom := errors.New("syntax error")
		err := retryWhenSQLiteBusy(context.Background(), func() error { calls++; return boom })
		require.ErrorIs(t, err, boom)
		assert.Equal(t, 1, calls)
	})
	t.Run("gives up after a bounded number of attempts", func(t *testing.T) {
		calls := 0
		err := retryWhenSQLiteBusy(context.Background(), func() error { calls++; return errors.New("SQLITE_BUSY") })
		require.Error(t, err)
		assert.Equal(t, 6, calls)
	})
	t.Run("stops when the context ends", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		err := retryWhenSQLiteBusy(ctx, func() error { calls++; return errors.New("database table is locked") })
		require.Error(t, err)
		assert.Equal(t, 1, calls)
	})
}
