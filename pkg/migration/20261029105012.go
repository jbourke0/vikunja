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
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// Johnny's Brain: duplicate detection for bookmarks. Adds a hash of the normalised URL
// (for instant same-link checks) and a table of suggested duplicate pairs.

type bookmarksURLHash20261029105012 struct {
	URLHash string `xorm:"varchar(64) null INDEX"`
}

func (bookmarksURLHash20261029105012) TableName() string {
	return "bookmarks"
}

type bookmarkDuplicates20261029105012 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	BookmarkID  int64     `xorm:"bigint not null INDEX unique(bookmark_duplicate)"`
	DuplicateID int64     `xorm:"bigint not null INDEX unique(bookmark_duplicate)"`
	Score       float64   `xorm:"double not null default 0"`
	Reason      string    `xorm:"varchar(20) not null"`
	Explanation string    `xorm:"text null"`
	Status      string    `xorm:"varchar(20) not null default 'open' INDEX"`
	DetectedAt  time.Time `xorm:"datetime not null"`
}

func (bookmarkDuplicates20261029105012) TableName() string {
	return "bookmark_duplicates"
}

// normalizeBookmarkURL20261029105012 is a frozen copy of models.NormalizeBookmarkURL as it
// was when this migration was written, used only to backfill url_hash.
func normalizeBookmarkURL20261029105012(raw string) string {
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
		if strings.HasPrefix(lower, "utm_") || map[string]bool{
			"fbclid": true, "gclid": true, "dclid": true, "msclkid": true, "mc_cid": true, "mc_eid": true,
			"igshid": true, "si": true, "ref": true, "ref_src": true, "amp": true, "_ga": true, "yclid": true,
		}[lower] {
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

func migrateBookmarkDuplicates20261029105012(tx *xorm.Engine) error {
	if err := partialSync(tx, bookmarksURLHash20261029105012{}); err != nil {
		return err
	}
	if err := tx.Sync(bookmarkDuplicates20261029105012{}); err != nil { //nolint:forbidigo // brand-new table (Rollback drops it); Sync also creates the unique index
		return err
	}

	type row struct {
		ID  int64  `xorm:"id"`
		URL string `xorm:"url"`
	}
	rows := []row{}
	if err := tx.Table("bookmarks").Cols("id", "url").Find(&rows); err != nil {
		return err
	}
	for _, r := range rows {
		sum := sha256.Sum256([]byte(normalizeBookmarkURL20261029105012(r.URL)))
		if _, err := tx.Exec("UPDATE bookmarks SET url_hash = ? WHERE id = ?", hex.EncodeToString(sum[:]), r.ID); err != nil {
			return err
		}
	}
	return nil
}

func rollbackBookmarkDuplicates20261029105012(tx *xorm.Engine) error {
	if err := tx.DropTables(bookmarkDuplicates20261029105012{}); err != nil {
		return err
	}
	// MySQL and Postgres drop a column's single-column index with it; SQLite refuses to drop
	// an indexed column, so the index goes first there.
	if tx.Dialect().URI().DBType == schemas.SQLITE {
		if _, err := tx.Exec("DROP INDEX IF EXISTS " + tx.Dialect().Quoter().Quote("IDX_bookmarks_url_hash")); err != nil {
			return err
		}
	}
	return dropTableColum(tx, "bookmarks", "url_hash")
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261029105012",
		Description: "Johnny's Brain: bookmark URL hashes and suggested duplicate pairs",
		Migrate:     migrateBookmarkDuplicates20261029105012,
		Rollback:    rollbackBookmarkDuplicates20261029105012,
	})
}
