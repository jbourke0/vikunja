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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestBookmarkDuplicates20261029105012(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, rollbackBookmarks20261029095221(x))
	require.NoError(t, migrateBookmarks20261029095221(x))

	now := time.Now()
	_, err = x.Insert(&bookmarks20261029095221{URL: "https://www.example.com/story/?utm_source=x", Status: "to_read", CreatedByID: 1, Created: now, Updated: now})
	require.NoError(t, err)

	require.NoError(t, migrateBookmarkDuplicates20261029105012(x))

	var hash string
	_, err = x.SQL("SELECT url_hash FROM bookmarks").Get(&hash)
	require.NoError(t, err)
	sum := sha256.Sum256([]byte("example.com/story"))
	assert.Equal(t, hex.EncodeToString(sum[:]), hash, "www, trailing slash and utm_ parameters are ignored")

	tables, err := x.DBMetas()
	require.NoError(t, err)
	var dupTable, bookmarksTable *schemas.Table
	for _, table := range tables {
		switch table.Name {
		case "bookmark_duplicates":
			dupTable = table
		case "bookmarks":
			bookmarksTable = table
		}
	}
	require.NotNil(t, dupTable)
	require.NotNil(t, bookmarksTable)
	foundUnique := false
	for _, index := range dupTable.Indexes {
		if index.Type == schemas.UniqueType && assert.ObjectsAreEqual([]string{"bookmark_id", "duplicate_id"}, index.Cols) {
			foundUnique = true
		}
	}
	assert.True(t, foundUnique)
	assert.NotEmpty(t, bookmarksTable.Indexes, "existing bookmark indexes survive the partial sync")
	createdByIndexed := false
	for _, index := range bookmarksTable.Indexes {
		if assert.ObjectsAreEqual([]string{"created_by_id"}, index.Cols) {
			createdByIndexed = true
		}
	}
	assert.True(t, createdByIndexed, "the created_by_id index from the first migration is kept")

	require.NoError(t, rollbackBookmarkDuplicates20261029105012(x))
	exists, err := x.IsTableExist("bookmark_duplicates")
	require.NoError(t, err)
	assert.False(t, exists)
	hasColumn, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "bookmarks", "url_hash")
	require.NoError(t, err)
	assert.False(t, hasColumn)
}
