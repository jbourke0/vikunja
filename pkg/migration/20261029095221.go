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
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

// Johnny's Brain: bookmarks, their comments, labels and the "sources" links to
// projects and tasks. All tables are created together so later features
// (link previews, snapshots, broken-link checks) need no further schema changes.

type bookmarks20261029095221 struct {
	ID          int64      `xorm:"bigint autoincr not null unique pk"`
	URL         string     `xorm:"varchar(2048) not null"`
	Title       string     `xorm:"varchar(250) null"`
	Description string     `xorm:"longtext null"`
	FaviconURL  string     `xorm:"varchar(2048) null"`
	ImageURL    string     `xorm:"varchar(2048) null"`
	SiteName    string     `xorm:"varchar(250) null"`
	Status      string     `xorm:"varchar(20) not null default 'to_read' INDEX"`
	LinkStatus  int        `xorm:"int not null default 0"`
	CheckedAt   *time.Time `xorm:"datetime null"`
	CreatedByID int64      `xorm:"bigint not null INDEX"`
	Created     time.Time  `xorm:"created not null"`
	Updated     time.Time  `xorm:"updated not null"`
}

func (bookmarks20261029095221) TableName() string {
	return "bookmarks"
}

type bookmarkSnapshots20261029095221 struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	BookmarkID int64     `xorm:"bigint not null unique"`
	Text       string    `xorm:"longtext null"`
	FetchedAt  time.Time `xorm:"datetime not null"`
}

func (bookmarkSnapshots20261029095221) TableName() string {
	return "bookmark_snapshots"
}

type bookmarkProjects20261029095221 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	BookmarkID  int64     `xorm:"bigint not null INDEX unique(bookmark_project)"`
	ProjectID   int64     `xorm:"bigint not null INDEX unique(bookmark_project)"`
	CreatedByID int64     `xorm:"bigint not null"`
	Created     time.Time `xorm:"created not null"`
}

func (bookmarkProjects20261029095221) TableName() string {
	return "bookmark_projects"
}

type bookmarkTasks20261029095221 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	BookmarkID  int64     `xorm:"bigint not null INDEX unique(bookmark_task)"`
	TaskID      int64     `xorm:"bigint not null INDEX unique(bookmark_task)"`
	CreatedByID int64     `xorm:"bigint not null"`
	Created     time.Time `xorm:"created not null"`
}

func (bookmarkTasks20261029095221) TableName() string {
	return "bookmark_tasks"
}

type labelBookmarks20261029095221 struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	BookmarkID int64     `xorm:"bigint not null INDEX unique(label_bookmark)"`
	LabelID    int64     `xorm:"bigint not null INDEX unique(label_bookmark)"`
	Created    time.Time `xorm:"created not null"`
}

func (labelBookmarks20261029095221) TableName() string {
	return "label_bookmarks"
}

type bookmarkComments20261029095221 struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	Comment    string    `xorm:"text not null"`
	AuthorID   int64     `xorm:"bigint not null"`
	BookmarkID int64     `xorm:"bigint not null INDEX"`
	Created    time.Time `xorm:"created not null"`
	Updated    time.Time `xorm:"updated not null"`
}

func (bookmarkComments20261029095221) TableName() string {
	return "bookmark_comments"
}

func bookmarkTables20261029095221() []interface{} {
	return []interface{}{
		bookmarks20261029095221{},
		bookmarkSnapshots20261029095221{},
		bookmarkProjects20261029095221{},
		bookmarkTasks20261029095221{},
		labelBookmarks20261029095221{},
		bookmarkComments20261029095221{},
	}
}

func migrateBookmarks20261029095221(tx *xorm.Engine) error {
	return tx.Sync(bookmarkTables20261029095221()...) //nolint:forbidigo // brand-new tables (Rollback drops them), so nothing to preserve; Sync also creates the unique indexes
}

func rollbackBookmarks20261029095221(tx *xorm.Engine) error {
	return tx.DropTables(bookmarkTables20261029095221()...)
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261029095221",
		Description: "Johnny's Brain: add bookmarks, bookmark comments, labels, snapshots and project/task sources",
		Migrate:     migrateBookmarks20261029095221,
		Rollback:    rollbackBookmarks20261029095221,
	})
}
