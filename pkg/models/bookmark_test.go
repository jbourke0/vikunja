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
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bookmarkIDs(t *testing.T, result interface{}) []int64 {
	t.Helper()
	items, ok := result.([]*Bookmark)
	require.True(t, ok, "ReadAll returned %T", result)
	ids := make([]int64, 0, len(items))
	for _, b := range items {
		ids = append(ids, b.ID)
	}
	return ids
}

func TestBookmark_ReadAll(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("own and linked through accessible project or task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, total, err := (&Bookmark{}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 3, 4, 6}, bookmarkIDs(t, result))
		assert.Equal(t, int64(4), total)
	})
	t.Run("newest first with labels and creator", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Bookmark{}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		items := result.([]*Bookmark)
		assert.Equal(t, []int64{6, 4, 3, 1}, bookmarkIDs(t, result))
		last := items[len(items)-1]
		require.Len(t, last.Labels, 1)
		assert.Equal(t, int64(1), last.Labels[0].ID)
		require.NotNil(t, last.CreatedBy)
		assert.Equal(t, int64(1), last.CreatedBy.ID)
	})
	t.Run("search", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Bookmark{}).ReadAll(s, u1, "example.org", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{6}, bookmarkIDs(t, result))
	})
	t.Run("filters", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Bookmark{FilterStatus: BookmarkStatusArchived}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{6}, bookmarkIDs(t, result))

		result, _, _, err = (&Bookmark{FilterLabelID: 1}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{1}, bookmarkIDs(t, result))

		result, _, _, err = (&Bookmark{FilterProjectID: 1}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{3}, bookmarkIDs(t, result))

		result, _, _, err = (&Bookmark{FilterTaskID: 1}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{4}, bookmarkIDs(t, result))
	})
	t.Run("filters cannot widen visibility", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Bookmark{FilterProjectID: 20}).ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		assert.Empty(t, bookmarkIDs(t, result))
	})
	t.Run("other user sees only their own", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Bookmark{}).ReadAll(s, &user.User{ID: 13}, "", 0, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{5}, bookmarkIDs(t, result))
	})
	t.Run("pagination", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&Bookmark{}).ReadAll(s, u1, "", 2, 3)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, int64(4), total)
		assert.Equal(t, []int64{1}, bookmarkIDs(t, result))
	})
}

func TestBookmark_CanRead(t *testing.T) {
	cases := []struct {
		name       string
		bookmarkID int64
		userID     int64
		wantCan    bool
		wantPerm   Permission
	}{
		{"owner", 1, 1, true, PermissionAdmin},
		{"via accessible project", 3, 1, true, PermissionRead},
		{"via accessible task", 4, 1, true, PermissionRead},
		{"private bookmark of another user", 2, 1, false, 0},
		{"linked only to an inaccessible project and task", 5, 1, false, 0},
		{"owner of a linked bookmark", 3, 2, true, PermissionAdmin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db.LoadAndAssertFixtures(t)
			s := db.NewSession()
			defer s.Close()

			can, perm, err := (&Bookmark{ID: tc.bookmarkID}).CanRead(s, &user.User{ID: tc.userID})
			require.NoError(t, err)
			assert.Equal(t, tc.wantCan, can)
			if tc.wantCan {
				assert.Equal(t, int(tc.wantPerm), perm)
			}
		})
	}

	t.Run("nonexistent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := (&Bookmark{ID: 9999}).CanRead(s, &user.User{ID: 1})
		assert.True(t, IsErrBookmarkDoesNotExist(err))
	})
}

func TestBookmark_Create(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		b := &Bookmark{URL: "  https://www.carbongym.com.au/blog  ", Description: "Blog"}
		can, err := b.CanCreate(s, u1)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, b.Create(s, u1))
		require.NoError(t, s.Commit())

		assert.NotZero(t, b.ID)
		assert.Equal(t, "https://www.carbongym.com.au/blog", b.URL)
		assert.Equal(t, "carbongym.com.au", b.Title, "title defaults to the host without www")
		assert.Equal(t, BookmarkStatusToRead, b.Status)
		db.AssertExists(t, "bookmarks", map[string]interface{}{
			"id":            b.ID,
			"url":           "https://www.carbongym.com.au/blog",
			"status":        BookmarkStatusToRead,
			"created_by_id": 1,
		}, false)
	})
	t.Run("server-controlled fields are ignored", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		b := &Bookmark{ID: 1, URL: "https://example.net", LinkStatus: 200, CreatedByID: 2}
		require.NoError(t, b.Create(s, u1))
		assert.NotEqual(t, int64(1), b.ID)
		assert.Equal(t, int64(1), b.CreatedByID)
		assert.Zero(t, b.LinkStatus)
	})
	t.Run("invalid urls", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		for _, raw := range []string{"", "not a url", "ftp://example.com/file", "https://", "javascript:alert(1)"} {
			err := (&Bookmark{URL: raw}).Create(s, u1)
			assert.True(t, IsErrBookmarkInvalidURL(err), "url %q: got %v", raw, err)
		}
	})
	t.Run("invalid status", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Bookmark{URL: "https://example.com", Status: "someday"}).Create(s, u1)
		assert.True(t, IsErrBookmarkInvalidStatus(err))
	})
	t.Run("link share cannot create", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Bookmark{}).CanCreate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestBookmark_Update(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		b := &Bookmark{ID: 1, URL: "https://example.com/one-updated", Title: "Renamed", Status: BookmarkStatusReference}
		can, err := b.CanUpdate(s, &user.User{ID: 1})
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, b.Update(s, &user.User{ID: 1}))
		require.NoError(t, s.Commit())

		assert.Len(t, b.Labels, 1, "update returns the reloaded bookmark")
		db.AssertExists(t, "bookmarks", map[string]interface{}{
			"id":     1,
			"title":  "Renamed",
			"status": BookmarkStatusReference,
		}, false)
	})
	t.Run("reader through a source link cannot update", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Bookmark{ID: 3}).CanUpdate(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("unrelated user cannot update", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Bookmark{ID: 2}).CanUpdate(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share cannot update", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Bookmark{ID: 1}).CanUpdate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := (&Bookmark{ID: 9999}).CanUpdate(s, &user.User{ID: 1})
		assert.True(t, IsErrBookmarkDoesNotExist(err))
	})
}

func TestBookmark_Delete(t *testing.T) {
	t.Run("owner deletes bookmark and its associations", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		b := &Bookmark{ID: 3}
		can, err := b.CanDelete(s, &user.User{ID: 2})
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, b.Delete(s, &user.User{ID: 2}))
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "bookmarks", map[string]interface{}{"id": 3})
		db.AssertMissing(t, "bookmark_projects", map[string]interface{}{"bookmark_id": 3})
		db.AssertMissing(t, "bookmark_comments", map[string]interface{}{"bookmark_id": 3})
		db.AssertExists(t, "bookmarks", map[string]interface{}{"id": 1}, false)
	})
	t.Run("labels go with the bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, (&Bookmark{ID: 1}).Delete(s, &user.User{ID: 1}))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 1})
		db.AssertExists(t, "labels", map[string]interface{}{"id": 1}, false)
	})
	t.Run("reader cannot delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Bookmark{ID: 4}).CanDelete(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestBookmarkComment(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("reader can list comments on a linked bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &BookmarkComment{BookmarkID: 3}
		can, _, err := c.CanRead(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		result, _, total, err := c.ReadAll(s, u1, "", 0, 0)
		require.NoError(t, err)
		comments := result.([]*BookmarkComment)
		require.Len(t, comments, 1)
		assert.Equal(t, int64(1), total)
		require.NotNil(t, comments[0].Author)
		assert.Equal(t, int64(2), comments[0].Author.ID)
	})
	t.Run("cannot read comments on a private bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, _, err := (&BookmarkComment{BookmarkID: 2}).CanRead(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("listing comments on a private bookmark is forbidden", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&BookmarkComment{BookmarkID: 2}).ReadAll(s, u1, "", 0, 0)
		assert.True(t, IsErrGenericForbidden(err), "got %v", err)
	})
	t.Run("reader can comment", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &BookmarkComment{BookmarkID: 3, Comment: " <p>Useful for the funnel review</p> "}
		can, err := c.CanCreate(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, c.Create(s, u1))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "bookmark_comments", map[string]interface{}{
			"id":          c.ID,
			"bookmark_id": 3,
			"author_id":   1,
			"comment":     "<p>Useful for the funnel review</p>",
		}, false)
	})
	t.Run("cannot comment on a private bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&BookmarkComment{BookmarkID: 2}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("empty comment", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&BookmarkComment{BookmarkID: 1, Comment: "   "}).Create(s, u1)
		assert.True(t, IsErrBookmarkCommentEmpty(err))
	})
	t.Run("author can edit and delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &BookmarkComment{ID: 1, BookmarkID: 1, Comment: "Edited"}
		can, err := c.CanUpdate(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, c.Update(s, u1))
		assert.Equal(t, "Edited", c.Comment)

		can, err = c.CanDelete(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, c.Delete(s, u1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "bookmark_comments", map[string]interface{}{"id": 1})
	})
	t.Run("only the author can edit", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&BookmarkComment{ID: 2, BookmarkID: 3}).CanUpdate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&BookmarkComment{ID: 2, BookmarkID: 3}).CanDelete(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("comment id must belong to the bookmark", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&BookmarkComment{ID: 2, BookmarkID: 1}).ReadOne(s, u1)
		assert.True(t, IsErrBookmarkCommentDoesNotExist(err))
	})
}

func TestLabelBookmark(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("owner adds a visible label", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		lb := &LabelBookmark{BookmarkID: 1, LabelID: 2}
		can, err := lb.CanCreate(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, lb.Create(s, u1))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 1, "label_id": 2}, false)
	})
	t.Run("duplicate", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&LabelBookmark{BookmarkID: 1, LabelID: 1}).Create(s, u1)
		assert.True(t, IsErrLabelIsAlreadyOnBookmark(err))
	})
	t.Run("not the bookmark owner", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&LabelBookmark{BookmarkID: 3, LabelID: 1}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("label the caller cannot see", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Label 6 is private to user 13.
		can, err := (&LabelBookmark{BookmarkID: 1, LabelID: 6}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent label", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := (&LabelBookmark{BookmarkID: 1, LabelID: 9999}).CanCreate(s, u1)
		assert.True(t, IsErrLabelDoesNotExist(err))
	})
	t.Run("owner removes a label", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		lb := &LabelBookmark{BookmarkID: 1, LabelID: 1}
		can, err := lb.CanDelete(s, u1)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, lb.Delete(s, u1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 1, "label_id": 1})

		err = (&LabelBookmark{BookmarkID: 1, LabelID: 1}).Delete(s, u1)
		assert.True(t, IsErrLabelIsNotOnBookmark(err))
	})
	t.Run("reader cannot remove labels", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&LabelBookmark{BookmarkID: 3, LabelID: 1}).CanDelete(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
}
