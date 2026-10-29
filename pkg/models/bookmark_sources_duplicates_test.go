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
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBookmarkURL(t *testing.T) {
	same := [][]string{
		{"https://www.example.com/story/", "http://example.com/story"},
		{"https://example.com/story?utm_source=x&utm_medium=y", "https://example.com/story"},
		{"https://example.com/story?fbclid=abc#comments", "https://example.com/story"},
		{"https://m.example.com/story/amp", "https://example.com/story"},
		{"https://example.com:443/a?b=2&a=1", "https://example.com/a?a=1&b=2"},
		{"HTTPS://EXAMPLE.COM/story", "https://example.com/story"},
	}
	for _, pair := range same {
		assert.Equal(t, NormalizeBookmarkURL(pair[0]), NormalizeBookmarkURL(pair[1]), "%s vs %s", pair[0], pair[1])
		assert.Equal(t, BookmarkURLHash(pair[0]), BookmarkURLHash(pair[1]))
	}
	different := [][]string{
		{"https://example.com/story", "https://example.com/other-story"},
		{"https://example.com/watch?v=1", "https://example.com/watch?v=2"},
		{"https://example.com/story", "https://example.org/story"},
		{"https://example.com/Story", "https://example.com/story"},
	}
	for _, pair := range different {
		assert.NotEqual(t, NormalizeBookmarkURL(pair[0]), NormalizeBookmarkURL(pair[1]), "%s vs %s", pair[0], pair[1])
	}
}

func TestBookmark_SameLinkTwice(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("refused on create, even with tracking parameters", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Bookmark{URL: "http://www.example.com/one/?utm_source=newsletter"}).Create(s, u1)
		var saved ErrBookmarkAlreadySaved
		require.ErrorAs(t, err, &saved)
		assert.Equal(t, int64(1), saved.ExistingID)
	})
	t.Run("allowed for a different owner", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, (&Bookmark{URL: "https://example.com/one"}).Create(s, &user.User{ID: 13}))
	})
	t.Run("refused when an edit makes it match another bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Bookmark{ID: 6, URL: "https://example.com/one"}).Update(s, u1)
		assert.True(t, IsErrBookmarkAlreadySaved(err), "got %v", err)
	})
	t.Run("editing a bookmark without changing its link is fine", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, (&Bookmark{ID: 1, URL: "https://example.com/one", Title: "Renamed"}).Update(s, u1))
	})
	t.Run("list filter finds a saved link", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Bookmark{FilterURL: "https://www.example.com/one?utm_campaign=x"}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{1}, bookmarkIDs(t, result))
	})
}

func TestBookmarkSources(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("owner links to a writable project and task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		bp := &BookmarkProject{BookmarkID: 1, ProjectID: 1}
		can, err := bp.CanCreate(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, bp.Create(s, u1))

		bt := &BookmarkTask{BookmarkID: 1, TaskID: 1}
		can, err = bt.CanCreate(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, bt.Create(s, u1))
		require.NoError(t, s.Commit())

		b := &Bookmark{ID: 1}
		require.NoError(t, b.ReadOne(s, u1))
		require.Len(t, b.Projects, 1)
		assert.Equal(t, int64(1), b.Projects[0].ID)
		require.Len(t, b.Tasks, 1)
		assert.Equal(t, int64(1), b.Tasks[0].ID)
	})
	t.Run("linking twice is a conflict", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&BookmarkProject{BookmarkID: 3, ProjectID: 1}).Create(s, &user.User{ID: 2})
		assert.True(t, IsErrBookmarkSourceAlreadyLinked(err))
	})
	t.Run("read-only project or task cannot be linked", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&BookmarkProject{BookmarkID: 1, ProjectID: 9}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&BookmarkTask{BookmarkID: 1, TaskID: 18}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("someone else's bookmark cannot be linked", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&BookmarkProject{BookmarkID: 4, ProjectID: 1}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("missing project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := (&BookmarkProject{BookmarkID: 1, ProjectID: 99999}).CanCreate(s, u1)
		assert.True(t, IsErrProjectDoesNotExist(err))
	})
	t.Run("a project writer can remove a link they do not own", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Bookmark 3 is user 2's, linked to project 1, which user 1 owns.
		bp := &BookmarkProject{BookmarkID: 3, ProjectID: 1}
		can, err := bp.CanDelete(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, bp.Delete(s, u1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "bookmark_projects", map[string]interface{}{"bookmark_id": 3, "project_id": 1})
	})
	t.Run("an unrelated user cannot remove a link", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&BookmarkTask{BookmarkID: 5, TaskID: 34}).CanDelete(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("removing a link that does not exist", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&BookmarkTask{BookmarkID: 1, TaskID: 1}).Delete(s, u1)
		assert.True(t, IsErrBookmarkSourceNotLinked(err))
	})
	t.Run("detail only shows sources the caller can see", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Bookmark 5 (user 13) is linked to project 20 and task 34, both private to user 13.
		b := &Bookmark{ID: 5}
		require.NoError(t, b.ReadOne(s, &user.User{ID: 13}))
		assert.Len(t, b.Projects, 1)
		assert.Len(t, b.Tasks, 1)
	})
	t.Run("cleanup removes links to deleted projects and tasks", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := s.Insert(&BookmarkProject{BookmarkID: 1, ProjectID: 99999, CreatedByID: 1})
		require.NoError(t, err)
		_, err = s.Insert(&BookmarkTask{BookmarkID: 1, TaskID: 99999, CreatedByID: 1})
		require.NoError(t, err)
		require.NoError(t, s.Commit())

		require.NoError(t, (&RemoveOrphanedBookmarkSources{}).Handle(nil))

		db.AssertMissing(t, "bookmark_projects", map[string]interface{}{"project_id": 99999})
		db.AssertMissing(t, "bookmark_tasks", map[string]interface{}{"task_id": 99999})
		db.AssertExists(t, "bookmark_projects", map[string]interface{}{"bookmark_id": 3, "project_id": 1}, false)
		db.AssertExists(t, "bookmark_tasks", map[string]interface{}{"bookmark_id": 4, "task_id": 1}, false)
	})
}

func TestLocalDuplicateScore(t *testing.T) {
	b := func(url, title, desc string) *Bookmark { return &Bookmark{URL: url, Title: title, Description: desc} }

	sameQuery := localDuplicateScore(b("https://shop.example/p/widget?colour=red", "Widget", ""), b("https://shop.example/p/widget?colour=blue", "Widget", ""))
	assert.InDelta(t, 0.9, sameQuery, 0.001)

	story := localDuplicateScore(
		b("https://news-one.example/2026/10/geelong-council-approves-waterfront-plan", "Geelong council approves waterfront plan", ""),
		b("https://other-news.example/local/waterfront-plan-approved-geelong", "Waterfront plan approved by Geelong council", ""),
	)
	assert.GreaterOrEqual(t, story, duplicateLocalThreshold, "reworded headline of the same story")

	unrelated := localDuplicateScore(
		b("https://example.com/kids-martial-arts", "Kids martial arts in Geelong", ""),
		b("https://example.org/sourdough", "Easy sourdough starter", ""),
	)
	assert.Less(t, unrelated, duplicateCandidateFloor)

	sameSiteDifferentPage := localDuplicateScore(
		b("https://carbongym.example/timetable", "Timetable", ""),
		b("https://carbongym.example/pricing", "Pricing", ""),
	)
	assert.Less(t, sameSiteDifferentPage, duplicateLocalThreshold)
}

type fakeJudge struct {
	matches  []BookmarkDuplicateMatch
	err      error
	offered  []int64
	subjects []int64
}

func (f *fakeJudge) FindDuplicates(_ context.Context, subject BookmarkForDuplicateCheck, candidates []BookmarkForDuplicateCheck) ([]BookmarkDuplicateMatch, error) {
	f.subjects = append(f.subjects, subject.ID)
	for _, c := range candidates {
		f.offered = append(f.offered, c.ID)
	}
	return f.matches, f.err
}

func TestDetectDuplicatesForBookmark(t *testing.T) {
	u1 := &user.User{ID: 1}
	ctx := context.Background()

	setup := func(t *testing.T) (int64, int64, int64) {
		t.Helper()
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		story1 := &Bookmark{URL: "https://news-one.example/geelong-council-approves-waterfront-plan", Title: "Geelong council approves waterfront plan"}
		story2 := &Bookmark{URL: "https://other-news.example/waterfront-plan-approved-geelong", Title: "Waterfront plan approved by Geelong council"}
		other := &Bookmark{URL: "https://example.org/sourdough", Title: "Easy sourdough starter"}
		for _, b := range []*Bookmark{story1, story2, other} {
			require.NoError(t, b.Create(s, u1))
		}
		require.NoError(t, s.Commit())
		return story1.ID, story2.ID, other.ID
	}

	t.Run("local matching without a judge", func(t *testing.T) {
		SetBookmarkDuplicateJudge(nil)
		story1, story2, other := setup(t)
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story2))
		require.NoError(t, s.Commit())

		lo, hi := orderedPair(story1, story2)
		db.AssertExists(t, "bookmark_duplicates", map[string]interface{}{"bookmark_id": lo, "duplicate_id": hi, "reason": "similar", "status": "open"}, false)
		db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"duplicate_id": other})

		suggestions, err := GetBookmarkDuplicateSuggestions(s, u1, story1)
		require.NoError(t, err)
		require.Len(t, suggestions, 1)
		assert.Equal(t, story2, suggestions[0].Bookmark.ID)

		b := &Bookmark{ID: story1}
		require.NoError(t, b.ReadOne(s, u1))
		assert.Equal(t, 1, b.DuplicateCount)
	})

	t.Run("AI judge decides, and only offered ids are accepted", func(t *testing.T) {
		story1, story2, _ := setup(t)
		judge := &fakeJudge{matches: []BookmarkDuplicateMatch{
			{ID: story1, Confidence: 0.92, Reason: "Same council decision reported by two outlets."},
			{ID: 2, Confidence: 0.99, Reason: "made-up id the judge was never shown"},
			{ID: story1 + 1000, Confidence: 0.99, Reason: "another made-up id"},
		}}
		SetBookmarkDuplicateJudge(judge)
		defer SetBookmarkDuplicateJudge(nil)

		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story2))
		require.NoError(t, s.Commit())

		assert.Contains(t, judge.offered, story1)
		assert.NotContains(t, judge.offered, int64(2), "other users' bookmarks are never compared")
		lo, hi := orderedPair(story1, story2)
		db.AssertExists(t, "bookmark_duplicates", map[string]interface{}{
			"bookmark_id": lo, "duplicate_id": hi, "reason": "ai",
			"explanation": "Same council decision reported by two outlets.",
		}, false)
		db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"duplicate_id": 2})
		db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"bookmark_id": 2})
	})

	t.Run("AI judge saying no beats a high local score", func(t *testing.T) {
		_, story2, _ := setup(t)
		SetBookmarkDuplicateJudge(&fakeJudge{})
		defer SetBookmarkDuplicateJudge(nil)

		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story2))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"duplicate_id": story2})
	})

	t.Run("low-confidence AI matches are dropped", func(t *testing.T) {
		story1, story2, _ := setup(t)
		SetBookmarkDuplicateJudge(&fakeJudge{matches: []BookmarkDuplicateMatch{{ID: story1, Confidence: 0.4}}})
		defer SetBookmarkDuplicateJudge(nil)

		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story2))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"duplicate_id": story2})
	})

	t.Run("a failing judge falls back to local matching", func(t *testing.T) {
		story1, story2, _ := setup(t)
		SetBookmarkDuplicateJudge(&fakeJudge{err: errors.New("api down")})
		defer SetBookmarkDuplicateJudge(nil)

		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story2))
		require.NoError(t, s.Commit())
		lo, hi := orderedPair(story1, story2)
		db.AssertExists(t, "bookmark_duplicates", map[string]interface{}{"bookmark_id": lo, "duplicate_id": hi, "reason": "similar"}, false)
	})

	t.Run("dismissed pairs are not suggested again", func(t *testing.T) {
		SetBookmarkDuplicateJudge(nil)
		story1, story2, _ := setup(t)
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story2))
		require.NoError(t, DismissBookmarkDuplicate(s, u1, story2, story1))
		require.NoError(t, DetectDuplicatesForBookmark(ctx, s, story1))
		require.NoError(t, s.Commit())

		suggestions, err := GetBookmarkDuplicateSuggestions(s, u1, story1)
		require.NoError(t, err)
		assert.Empty(t, suggestions)
		lo, hi := orderedPair(story1, story2)
		db.AssertExists(t, "bookmark_duplicates", map[string]interface{}{"bookmark_id": lo, "duplicate_id": hi, "status": "dismissed"}, false)
	})

	t.Run("dismissing needs ownership and an existing suggestion", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		assert.True(t, IsErrGenericForbidden(DismissBookmarkDuplicate(s, u1, 2, 1)))
		assert.True(t, IsErrBookmarkDuplicateNotFound(DismissBookmarkDuplicate(s, u1, 1, 4)))
	})

	t.Run("saving queues a check, a status-only edit does not", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		events.ClearDispatchedEvents()
		s := db.NewSession()
		defer s.Close()

		b := &Bookmark{URL: "https://example.net/new"}
		require.NoError(t, b.Create(s, u1))
		require.NoError(t, s.Commit())
		events.DispatchPending(ctx, s)
		require.Equal(t, 1, events.CountDispatchedEvents((&BookmarkSavedEvent{}).Name()))
		saved := events.GetDispatchedEvents((&BookmarkSavedEvent{}).Name())[0].(*BookmarkSavedEvent)
		assert.Equal(t, b.ID, saved.BookmarkID)

		events.ClearDispatchedEvents()
		s2 := db.NewSession()
		defer s2.Close()
		require.NoError(t, (&Bookmark{ID: b.ID, URL: b.URL, Title: b.Title, Status: BookmarkStatusArchived}).Update(s2, u1))
		require.NoError(t, s2.Commit())
		events.DispatchPending(ctx, s2)
		assert.Equal(t, 0, events.CountDispatchedEvents((&BookmarkSavedEvent{}).Name()))
	})
}

func TestMergeBookmarks(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("moves everything and deletes the source", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Target 1 has label 1 and comment 1. Source 6 has label 2 and is archived.
		_, err := s.Insert(&LabelBookmark{BookmarkID: 6, LabelID: 1})
		require.NoError(t, err)
		_, err = s.Insert(&BookmarkComment{BookmarkID: 6, AuthorID: 1, Comment: "on six"})
		require.NoError(t, err)
		_, err = s.Insert(&BookmarkProject{BookmarkID: 6, ProjectID: 1, CreatedByID: 1})
		require.NoError(t, err)
		_, err = s.Exec("UPDATE bookmarks SET description = '', image_url = 'https://example.org/six.png' WHERE id = 1")
		require.NoError(t, err)

		merged, err := MergeBookmarks(s, u1, 1, 6)
		require.NoError(t, err)
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "bookmarks", map[string]interface{}{"id": 6})
		assert.ElementsMatch(t, []int64{1, 2}, labelIDsOf(merged.Labels), "labels combined without a duplicate row")
		db.AssertExists(t, "bookmark_comments", map[string]interface{}{"bookmark_id": 1, "comment": "on six"}, false)
		db.AssertExists(t, "bookmark_projects", map[string]interface{}{"bookmark_id": 1, "project_id": 1}, false)
		db.AssertMissing(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 6})
		db.AssertMissing(t, "bookmark_duplicates", map[string]interface{}{"duplicate_id": 6})
		assert.Equal(t, "https://example.org/six.png", merged.ImageURL, "existing values are kept")
		assert.Equal(t, "https://example.com/one", merged.URL)

		comments, _, _, err := (&BookmarkComment{BookmarkID: 1}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		last := comments.([]*BookmarkComment)
		assert.Contains(t, last[len(last)-1].Comment, "https://example.org/six", "merge leaves a note with the merged link")
	})
	t.Run("both bookmarks must be the caller's", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := MergeBookmarks(s, u1, 1, 3)
		assert.True(t, IsErrGenericForbidden(err), "got %v", err)
		_, err = MergeBookmarks(s, u1, 3, 1)
		assert.True(t, IsErrGenericForbidden(err), "got %v", err)
	})
	t.Run("not into itself", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := MergeBookmarks(s, u1, 1, 1)
		assert.True(t, IsErrBookmarkMergeIntoItself(err))
	})
}

func labelIDsOf(labels []*Label) []int64 {
	ids := make([]int64, 0, len(labels))
	for _, l := range labels {
		ids = append(ids, l.ID)
	}
	return ids
}
