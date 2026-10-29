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

package migration

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestBookmarks20261029095221(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, rollbackBookmarks20261029095221(x))

	require.NoError(t, migrateBookmarks20261029095221(x))

	tables, err := x.DBMetas()
	require.NoError(t, err)
	byName := map[string]*schemas.Table{}
	for _, table := range tables {
		byName[table.Name] = table
	}
	for _, name := range []string{"bookmarks", "bookmark_snapshots", "bookmark_projects", "bookmark_tasks", "label_bookmarks", "bookmark_comments"} {
		assert.Contains(t, byName, name)
	}

	uniqueOn := func(table string, cols ...string) bool {
		for _, index := range byName[table].Indexes {
			if index.Type == schemas.UniqueType && assert.ObjectsAreEqual(cols, index.Cols) {
				return true
			}
		}
		return false
	}
	assert.True(t, uniqueOn("label_bookmarks", "bookmark_id", "label_id"))
	assert.True(t, uniqueOn("bookmark_projects", "bookmark_id", "project_id"))
	assert.True(t, uniqueOn("bookmark_tasks", "bookmark_id", "task_id"))
	assert.True(t, uniqueOn("bookmark_snapshots", "bookmark_id"))

	// The unique index must actually reject a duplicate link.
	now := time.Now()
	_, err = x.Insert(&labelBookmarks20261029095221{BookmarkID: 1, LabelID: 1, Created: now})
	require.NoError(t, err)
	_, err = x.Insert(&labelBookmarks20261029095221{BookmarkID: 1, LabelID: 1, Created: now})
	require.Error(t, err)

	// Status defaults to to_read when not given.
	_, err = x.Exec("INSERT INTO bookmarks (url, created_by_id, created, updated) VALUES (?, ?, ?, ?)", "https://example.com", 1, now, now)
	require.NoError(t, err)
	var status string
	_, err = x.SQL("SELECT status FROM bookmarks WHERE url = ?", "https://example.com").Get(&status)
	require.NoError(t, err)
	assert.Equal(t, "to_read", status)

	require.NoError(t, rollbackBookmarks20261029095221(x))
	exists, err := x.IsTableExist("bookmarks")
	require.NoError(t, err)
	assert.False(t, exists)
}
