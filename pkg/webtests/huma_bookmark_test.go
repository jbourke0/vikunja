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

package webtests

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixture topology (pkg/db/fixtures/bookmarks.yml and friends):
//   - #1, #6: owned by user1 (#1 has label 1, #6 is archived with label 2).
//   - #2: owned by user2, not linked anywhere - invisible to user1.
//   - #3: owned by user2, source on project 1 (user1's) - user1 can read, not edit.
//   - #4: owned by user2, source on task 1 (project 1) - user1 can read, not edit.
//   - #5: owned by user13, source on project 20 and task 34 (user13 only) - invisible to user1.
func TestHumaBookmark(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/bookmarks",
		idParam:  "bookmark",
		t:        t,
	}

	t.Run("ReadAll", func(t *testing.T) {
		t.Run("exact visible set for user1, newest first", func(t *testing.T) {
			rec, err := testHandler.testReadAllWithUser(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, []int64{6, 4, 3, 1}, labelIDsFromReadAll(t, rec.Body.Bytes()), rec.Body.String())
			assert.EqualValues(t, 4, totalFromReadAll(t, rec.Body.Bytes()))
		})
		t.Run("filters", func(t *testing.T) {
			for query, want := range map[string][]int64{
				"status=archived": {6},
				"label_id=1":      {1},
				"project_id=1":    {3},
				"task_id=1":       {4},
				"project_id=20":   {},
				"q=example.org":   {6},
			} {
				values, err := url.ParseQuery(query)
				require.NoError(t, err)
				rec, err := testHandler.testReadAllWithUser(values, nil)
				require.NoError(t, err, query)
				assert.Equal(t, want, labelIDsFromReadAll(t, rec.Body.Bytes()), query)
			}
		})
		t.Run("unknown status is rejected", func(t *testing.T) {
			_, err := testHandler.testReadAllWithUser(url.Values{"status": {"someday"}}, nil)
			require.Error(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
		})
	})

	t.Run("ReadOne", func(t *testing.T) {
		t.Run("owner", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"bookmark": "1"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Bookmark #1"`)
			assert.Contains(t, rec.Body.String(), `"max_permission":2`)
			assert.Contains(t, rec.Body.String(), `"labels":[{"id":1`)
			assert.NotEmpty(t, rec.Result().Header.Get("ETag"))
		})
		t.Run("reader through a source link", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"bookmark": "3"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"max_permission":0`)
		})
		t.Run("forbidden", func(t *testing.T) {
			for _, id := range []string{"2", "5"} {
				_, err := testHandler.testReadOneWithUser(nil, map[string]string{"bookmark": id})
				require.Error(t, err, id)
				assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err), id)
			}
		})
		t.Run("nonexistent", func(t *testing.T) {
			_, err := testHandler.testReadOneWithUser(nil, map[string]string{"bookmark": "9999"})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("normal", func(t *testing.T) {
			rec, err := testHandler.testCreateWithUser(nil, nil, `{"url":"https://www.carbongym.com.au/timetable"}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"title":"carbongym.com.au"`)
			assert.Contains(t, rec.Body.String(), `"status":"to_read"`)
			assert.Contains(t, rec.Body.String(), `"created_by":{"id":1`)
		})
		t.Run("invalid url", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"url":"ftp://example.com"}`)
			require.Error(t, err)
			assertHandlerErrorCode(t, err, 90002)
		})
		t.Run("invalid status", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"url":"https://example.com","status":"someday"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("owner", func(t *testing.T) {
			rec, err := testHandler.testUpdateWithUser(nil, map[string]string{"bookmark": "1"}, `{"url":"https://example.com/one","title":"Renamed","status":"reference"}`)
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Renamed"`)
			assert.Contains(t, rec.Body.String(), `"status":"reference"`)
		})
		t.Run("reader cannot update", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"bookmark": "3"}, `{"url":"https://example.com/three","title":"Nope"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		})
		t.Run("unrelated user cannot update", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"bookmark": "2"}, `{"url":"https://example.com/two","title":"Nope"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		})
		t.Run("nonexistent", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"bookmark": "9999"}, `{"url":"https://example.com"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("reader cannot delete", func(t *testing.T) {
			_, err := testHandler.testDeleteWithUser(nil, map[string]string{"bookmark": "4"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		})
		t.Run("owner", func(t *testing.T) {
			rec, err := testHandler.testDeleteWithUser(nil, map[string]string{"bookmark": "6"})
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, rec.Code)
			db.AssertMissing(t, "bookmarks", map[string]interface{}{"id": 6})
			db.AssertMissing(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 6})
		})
	})
}

func TestHumaBookmark_PatchStatus(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	rec := humaRequest(t, e, http.MethodPatch, "/api/v2/bookmarks/1", `{"status":"archived"}`, humaTokenFor(t, &testuser1), "application/merge-patch+json")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"archived"`)
	assert.Contains(t, rec.Body.String(), `"title":"Bookmark #1"`, "PATCH keeps the fields it does not mention")
	db.AssertExists(t, "bookmarks", map[string]interface{}{"id": 1, "status": "archived", "url": "https://example.com/one"}, false)

	rec = humaRequest(t, e, http.MethodPatch, "/api/v2/bookmarks/3", `{"status":"archived"}`, humaTokenFor(t, &testuser1), "application/merge-patch+json")
	assert.Equal(t, http.StatusForbidden, rec.Code, "a reader cannot patch")
}

func TestHumaBookmarkComment(t *testing.T) {
	commentsOn := func(bookmark string) webHandlerTestV2 {
		return webHandlerTestV2{
			user:     &testuser1,
			basePath: "/api/v2/bookmarks/" + bookmark + "/comments",
			idParam:  "commentid",
			t:        t,
		}
	}

	t.Run("list on a linked bookmark", func(t *testing.T) {
		h := commentsOn("3")
		rec, err := h.testReadAllWithUser(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []int64{2}, labelIDsFromReadAll(t, rec.Body.Bytes()))
		assert.Contains(t, rec.Body.String(), `"author":{"id":2`)
	})
	t.Run("list on a private bookmark is forbidden", func(t *testing.T) {
		h := commentsOn("2")
		_, err := h.testReadAllWithUser(nil, nil)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("create as a reader", func(t *testing.T) {
		h := commentsOn("3")
		rec, err := h.testCreateWithUser(nil, nil, `{"comment":"Worth a read"}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"comment":"Worth a read"`)
		assert.Contains(t, rec.Body.String(), `"author":{"id":1`)
	})
	t.Run("create on a private bookmark is forbidden", func(t *testing.T) {
		h := commentsOn("2")
		_, err := h.testCreateWithUser(nil, nil, `{"comment":"Nope"}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("author updates own comment", func(t *testing.T) {
		h := commentsOn("1")
		rec, err := h.testUpdateWithUser(nil, map[string]string{"commentid": "1"}, `{"comment":"Edited"}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"comment":"Edited"`)
	})
	t.Run("cannot edit or delete someone else's comment", func(t *testing.T) {
		h := commentsOn("3")
		_, err := h.testUpdateWithUser(nil, map[string]string{"commentid": "2"}, `{"comment":"Nope"}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		_, err = h.testDeleteWithUser(nil, map[string]string{"commentid": "2"})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("comment from another bookmark is not found", func(t *testing.T) {
		h := commentsOn("1")
		_, err := h.testReadOneWithUser(nil, map[string]string{"commentid": "2"})
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})
	t.Run("author deletes own comment", func(t *testing.T) {
		h := commentsOn("1")
		rec, err := h.testDeleteWithUser(nil, map[string]string{"commentid": "1"})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestHumaBookmarkLabel(t *testing.T) {
	labelsOn := func(bookmark string) webHandlerTestV2 {
		return webHandlerTestV2{
			user:     &testuser1,
			basePath: "/api/v2/bookmarks/" + bookmark + "/labels",
			idParam:  "label",
			t:        t,
		}
	}

	t.Run("owner adds a label", func(t *testing.T) {
		h := labelsOn("1")
		rec, err := h.testCreateWithUser(nil, nil, `{"label_id":2}`)
		require.NoError(t, err)
		var body struct {
			LabelID int64 `json:"label_id"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, int64(2), body.LabelID)
		db.AssertExists(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 1, "label_id": 2}, false)
	})
	t.Run("duplicate is a conflict", func(t *testing.T) {
		h := labelsOn("1")
		_, err := h.testCreateWithUser(nil, nil, `{"label_id":1}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusConflict, getHTTPErrorCode(err))
	})
	t.Run("reader cannot label", func(t *testing.T) {
		h := labelsOn("3")
		_, err := h.testCreateWithUser(nil, nil, `{"label_id":1}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("invisible label is forbidden", func(t *testing.T) {
		h := labelsOn("1")
		_, err := h.testCreateWithUser(nil, nil, `{"label_id":6}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("owner removes a label", func(t *testing.T) {
		h := labelsOn("1")
		rec, err := h.testDeleteWithUser(nil, map[string]string{"label": "1"})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		db.AssertMissing(t, "label_bookmarks", map[string]interface{}{"bookmark_id": 1, "label_id": 1})
	})
}

func TestHumaBookmark_SameLinkTwice(t *testing.T) {
	h := webHandlerTestV2{user: &testuser1, basePath: "/api/v2/bookmarks", idParam: "bookmark", t: t}

	_, err := h.testCreateWithUser(nil, nil, `{"url":"http://www.example.com/one/?utm_source=x"}`)
	require.Error(t, err)
	assert.Equal(t, http.StatusConflict, getHTTPErrorCode(err))
	assertHandlerErrorCode(t, err, 90008)

	rec, err := h.testReadAllWithUser(url.Values{"url": {"https://example.com/one?fbclid=abc"}}, nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, labelIDsFromReadAll(t, rec.Body.Bytes()), "the url filter finds the existing bookmark")
}

func TestHumaBookmarkSources(t *testing.T) {
	// One environment for every handler: each fresh one reloads the fixtures on first use.
	e, err := setupTestEnv()
	require.NoError(t, err)
	handler := func(basePath, idParam string) webHandlerTestV2 {
		return webHandlerTestV2{user: &testuser1, basePath: basePath, idParam: idParam, t: t, e: e}
	}
	projects := handler("/api/v2/bookmarks/1/projects", "project")
	tasks := handler("/api/v2/bookmarks/1/tasks", "task")

	t.Run("link and unlink", func(t *testing.T) {
		rec, err := projects.testCreateWithUser(nil, nil, `{"project_id":1}`)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		_, err = tasks.testCreateWithUser(nil, nil, `{"task_id":1}`)
		require.NoError(t, err)

		read := handler("/api/v2/bookmarks", "bookmark")
		rec, err = read.testReadOneWithUser(nil, map[string]string{"bookmark": "1"})
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"projects":[{"id":1`)
		assert.Contains(t, rec.Body.String(), `"tasks":[{"id":1`)

		list, err := read.testReadAllWithUser(url.Values{"task_id": {"1"}}, nil)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 4}, labelIDsFromReadAll(t, list.Body.Bytes()))

		rec, err = tasks.testDeleteWithUser(nil, map[string]string{"task": "1"})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
	t.Run("twice is a conflict", func(t *testing.T) {
		// Project 1 was linked in the first subtest; the environment is shared.
		_, err := projects.testCreateWithUser(nil, nil, `{"project_id":1}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusConflict, getHTTPErrorCode(err))
	})
	t.Run("read-only project is forbidden", func(t *testing.T) {
		_, err := projects.testCreateWithUser(nil, nil, `{"project_id":9}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("someone else's bookmark is forbidden", func(t *testing.T) {
		other := handler("/api/v2/bookmarks/4/projects", "project")
		_, err := other.testCreateWithUser(nil, nil, `{"project_id":1}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("project writer removes a link on another user's bookmark", func(t *testing.T) {
		other := handler("/api/v2/bookmarks/3/projects", "project")
		rec, err := other.testDeleteWithUser(nil, map[string]string{"project": "1"})
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

func TestHumaBookmarkDuplicates(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	s := db.NewSession()
	_, err = s.Insert(&models.Bookmark{ID: 50, URL: "https://example.org/one-again", URLHash: models.BookmarkURLHash("https://example.org/one-again"), Title: "One again", Status: "to_read", CreatedByID: 1})
	require.NoError(t, err)
	_, err = s.Insert(&models.BookmarkDuplicate{BookmarkID: 1, DuplicateID: 50, Score: 0.9, Reason: "ai", Explanation: "Same page.", Status: "open", DetectedAt: time.Now()})
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	s.Close()

	rec := humaRequest(t, e, http.MethodGet, "/api/v2/bookmarks/1/duplicates", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"reason":"ai"`)
	assert.Contains(t, rec.Body.String(), `"bookmark":{"id":50`)

	rec = humaRequest(t, e, http.MethodGet, "/api/v2/bookmarks/1", "", token, "")
	assert.Contains(t, rec.Body.String(), `"duplicate_count":1`)

	rec = humaRequest(t, e, http.MethodGet, "/api/v2/bookmarks/2/duplicates", "", token, "")
	assert.Equal(t, http.StatusForbidden, rec.Code, "private bookmark of another user")

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/bookmarks/1/merge", `{"source_id":3}`, token, "")
	assert.Equal(t, http.StatusForbidden, rec.Code, "cannot merge in someone else's bookmark")

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/bookmarks/1/merge", `{"source_id":50}`, token, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"duplicate_count":0`)
	db.AssertMissing(t, "bookmarks", map[string]interface{}{"id": 50})

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/bookmarks/1/duplicates/50/dismiss", "", token, "")
	assert.Equal(t, http.StatusNotFound, rec.Code, "the pair went with the merge")

	rec = humaRequest(t, e, http.MethodPost, "/api/v2/bookmarks/1/duplicates/6/dismiss", "", token, "")
	assert.Equal(t, http.StatusNoContent, rec.Code, "the fixture pair 1/6 can be dismissed again")
}
