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
	"net/url"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Johnny's Brain: saved links ("bookmarks") that can be tagged with labels, commented on
// and linked as sources to projects and tasks.

// Bookmark statuses.
const (
	BookmarkStatusToRead    = "to_read"
	BookmarkStatusReference = "reference"
	BookmarkStatusArchived  = "archived"
)

// Bookmark is a saved link.
type Bookmark struct {
	// The unique, numeric id of this bookmark.
	ID int64 `xorm:"bigint autoincr not null unique pk" json:"id" param:"bookmark" readOnly:"true" doc:"The unique, numeric id of this bookmark."`
	// The address of the saved page.
	URL string `xorm:"varchar(2048) not null" json:"url" maxLength:"2048" doc:"The address of the saved page. Must be an absolute http or https URL."`
	// The page title. Defaults to the site's host name when empty.
	Title string `xorm:"varchar(250) null" json:"title" maxLength:"250" doc:"The page title. Defaults to the site's host name when left empty."`
	// A short description of the page, usually its meta description.
	Description string `xorm:"longtext null" json:"description" doc:"A short plain-text description of the page, usually its meta or Open Graph description."`
	// The page's favicon.
	FaviconURL string `xorm:"varchar(2048) null" json:"favicon_url" maxLength:"2048" doc:"Absolute URL of the page's favicon."`
	// A preview image for the page.
	ImageURL string `xorm:"varchar(2048) null" json:"image_url" maxLength:"2048" doc:"Absolute URL of a preview image, usually the page's Open Graph image."`
	// The name of the site.
	SiteName string `xorm:"varchar(250) null" json:"site_name" maxLength:"250" doc:"The name of the site, usually its Open Graph site name."`
	// Reading status.
	Status string `xorm:"varchar(20) not null default 'to_read' INDEX" json:"status" enum:"to_read,reference,archived" doc:"Reading status: to_read (default), reference or archived."`
	// HTTP status code from the last link check.
	LinkStatus int `xorm:"int not null default 0" json:"link_status" readOnly:"true" doc:"HTTP status code from the last link check, 0 if the link has not been checked. Set by the server."`
	// When the link was last checked.
	CheckedAt *time.Time `xorm:"datetime null" json:"checked_at" readOnly:"true" doc:"When the link was last checked, null if never. Set by the server."`

	CreatedByID int64 `xorm:"bigint not null INDEX" json:"-"`
	// The user who saved this bookmark.
	CreatedBy *user.User `xorm:"-" json:"created_by" readOnly:"true" doc:"The user who saved this bookmark."`

	// The labels on this bookmark.
	Labels []*Label `xorm:"-" json:"labels" readOnly:"true" doc:"The labels on this bookmark. Managed through the bookmark labels endpoints, not by writing here."`

	// A timestamp when this bookmark was created.
	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this bookmark was created. You cannot change this value."`
	// A timestamp when this bookmark was last updated.
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this bookmark was last updated. You cannot change this value."`

	// List filters, set by the API layer before ReadAll.
	FilterStatus    string `xorm:"-" json:"-"`
	FilterLabelID   int64  `xorm:"-" json:"-"`
	FilterProjectID int64  `xorm:"-" json:"-"`
	FilterTaskID    int64  `xorm:"-" json:"-"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for bookmarks.
func (*Bookmark) TableName() string {
	return "bookmarks"
}

// normalizeAndValidate trims input, checks the URL and status and fills defaults.
func (b *Bookmark) normalizeAndValidate() error {
	b.URL = strings.TrimSpace(b.URL)
	b.Title = strings.TrimSpace(b.Title)
	b.SiteName = strings.TrimSpace(b.SiteName)

	parsed, err := url.Parse(b.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(b.URL) > 2048 {
		return ErrBookmarkInvalidURL{URL: b.URL}
	}

	if b.Status == "" {
		b.Status = BookmarkStatusToRead
	}
	switch b.Status {
	case BookmarkStatusToRead, BookmarkStatusReference, BookmarkStatusArchived:
	default:
		return ErrBookmarkInvalidStatus{Status: b.Status}
	}

	if b.Title == "" {
		b.Title = strings.TrimPrefix(parsed.Hostname(), "www.")
	}
	if len([]rune(b.Title)) > 250 {
		b.Title = string([]rune(b.Title)[:250])
	}

	return nil
}

// Create saves a new bookmark owned by the authenticated user.
func (b *Bookmark) Create(s *xorm.Session, a web.Auth) error {
	u, err := user.GetFromAuth(a)
	if err != nil {
		return err
	}

	if err := b.normalizeAndValidate(); err != nil {
		return err
	}

	b.ID = 0
	b.LinkStatus = 0
	b.CheckedAt = nil
	b.CreatedByID = u.ID
	b.CreatedBy = u
	b.Labels = []*Label{}

	_, err = s.Insert(b)
	return err
}

// Update replaces the editable fields of a bookmark. Only the owner may do this (see CanUpdate).
func (b *Bookmark) Update(s *xorm.Session, a web.Auth) error {
	if err := b.normalizeAndValidate(); err != nil {
		return err
	}

	_, err := s.
		ID(b.ID).
		Cols(
			"url",
			"title",
			"description",
			"favicon_url",
			"image_url",
			"site_name",
			"status",
		).
		Update(b)
	if err != nil {
		return err
	}

	return b.ReadOne(s, a)
}

// Delete removes a bookmark together with its labels, comments, snapshot and source links.
func (b *Bookmark) Delete(s *xorm.Session, _ web.Auth) error {
	// No FK cascade in the schema, so the associations must go explicitly.
	for _, bean := range []interface{}{&LabelBookmark{}, &BookmarkComment{}, &BookmarkProject{}, &BookmarkTask{}, &BookmarkSnapshot{}} {
		if _, err := s.Where("bookmark_id = ?", b.ID).Delete(bean); err != nil {
			return err
		}
	}

	_, err := s.ID(b.ID).Delete(&Bookmark{})
	return err
}

// ReadOne loads a bookmark with its creator and labels. CanRead has already checked access.
func (b *Bookmark) ReadOne(s *xorm.Session, _ web.Auth) error {
	stored, err := getBookmarkByIDSimple(s, b.ID)
	if err != nil {
		return err
	}
	*b = *stored

	return addBookmarkDetails(s, []*Bookmark{b})
}

// ReadAll lists the bookmarks the caller can see, newest first. The search matches the
// title, URL and description; the Filter* fields narrow the list further.
func (b *Bookmark) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	visible, err := bookmarkVisibleCond(s, a)
	if err != nil {
		return nil, 0, 0, err
	}

	conds := []builder.Cond{visible}

	if search = strings.TrimSpace(search); search != "" {
		conds = append(conds, builder.Or(
			db.ILIKE("bookmarks.title", search),
			db.ILIKE("bookmarks.url", search),
			db.ILIKE("bookmarks.description", search),
		))
	}
	if b.FilterStatus != "" {
		conds = append(conds, builder.Eq{"bookmarks.status": b.FilterStatus})
	}
	if b.FilterLabelID > 0 {
		conds = append(conds, builder.In("bookmarks.id",
			builder.Select("bookmark_id").From("label_bookmarks").Where(builder.Eq{"label_id": b.FilterLabelID})))
	}
	if b.FilterProjectID > 0 {
		conds = append(conds, builder.In("bookmarks.id",
			builder.Select("bookmark_id").From("bookmark_projects").Where(builder.Eq{"project_id": b.FilterProjectID})))
	}
	if b.FilterTaskID > 0 {
		conds = append(conds, builder.In("bookmarks.id",
			builder.Select("bookmark_id").From("bookmark_tasks").Where(builder.Eq{"task_id": b.FilterTaskID})))
	}

	cond := builder.And(conds...)

	limit, start := getLimitFromPageIndex(page, perPage)
	query := s.Where(cond).OrderBy("bookmarks.created DESC, bookmarks.id DESC")
	if limit > 0 {
		query = query.Limit(limit, start)
	}

	bookmarks := []*Bookmark{}
	if err := query.Find(&bookmarks); err != nil {
		return nil, 0, 0, err
	}

	if err := addBookmarkDetails(s, bookmarks); err != nil {
		return nil, 0, 0, err
	}

	total, err := s.Where(cond).Count(&Bookmark{})
	if err != nil {
		return nil, 0, 0, err
	}

	return bookmarks, len(bookmarks), total, nil
}

func getBookmarkByIDSimple(s *xorm.Session, id int64) (*Bookmark, error) {
	b := &Bookmark{}
	exists, err := s.Where("id = ?", id).Get(b)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrBookmarkDoesNotExist{BookmarkID: id}
	}
	return b, nil
}

// addBookmarkDetails fills in creators and labels for a batch of bookmarks.
func addBookmarkDetails(s *xorm.Session, bookmarks []*Bookmark) error {
	if len(bookmarks) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(bookmarks))
	creatorIDs := make([]int64, 0, len(bookmarks))
	byID := make(map[int64]*Bookmark, len(bookmarks))
	for _, b := range bookmarks {
		ids = append(ids, b.ID)
		creatorIDs = append(creatorIDs, b.CreatedByID)
		byID[b.ID] = b
		b.Labels = []*Label{}
	}

	creators, err := user.GetUsersByIDs(s, creatorIDs)
	if err != nil {
		return err
	}
	for _, b := range bookmarks {
		b.CreatedBy = creators[b.CreatedByID]
	}

	type labelWithBookmark struct {
		Label      `xorm:"extends"`
		BookmarkID int64 `xorm:"bookmark_id"`
	}
	rows := []*labelWithBookmark{}
	err = s.Table("labels").
		Select("labels.*, label_bookmarks.bookmark_id").
		Join("INNER", "label_bookmarks", "label_bookmarks.label_id = labels.id").
		In("label_bookmarks.bookmark_id", ids).
		OrderBy("labels.title ASC").
		Find(&rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		l := row.Label
		if b, ok := byID[row.BookmarkID]; ok {
			b.Labels = append(b.Labels, &l)
		}
	}

	return nil
}

// BookmarkSnapshot holds the readable text of a page captured when it was saved.
type BookmarkSnapshot struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID int64     `xorm:"bigint not null unique" json:"-"`
	Text       string    `xorm:"longtext null" json:"text"`
	FetchedAt  time.Time `xorm:"datetime not null" json:"fetched_at"`
}

// TableName returns the table name for bookmark snapshots.
func (*BookmarkSnapshot) TableName() string {
	return "bookmark_snapshots"
}

// BookmarkProject links a bookmark to a project as a source.
type BookmarkProject struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID  int64     `xorm:"bigint not null INDEX unique(bookmark_project)" json:"bookmark_id"`
	ProjectID   int64     `xorm:"bigint not null INDEX unique(bookmark_project)" json:"project_id"`
	CreatedByID int64     `xorm:"bigint not null" json:"-"`
	Created     time.Time `xorm:"created not null" json:"created"`
}

// TableName returns the table name for bookmark to project links.
func (*BookmarkProject) TableName() string {
	return "bookmark_projects"
}

// BookmarkTask links a bookmark to a task as a source.
type BookmarkTask struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID  int64     `xorm:"bigint not null INDEX unique(bookmark_task)" json:"bookmark_id"`
	TaskID      int64     `xorm:"bigint not null INDEX unique(bookmark_task)" json:"task_id"`
	CreatedByID int64     `xorm:"bigint not null" json:"-"`
	Created     time.Time `xorm:"created not null" json:"created"`
}

// TableName returns the table name for bookmark to task links.
func (*BookmarkTask) TableName() string {
	return "bookmark_tasks"
}

// bookmarkTables lists the Johnny's Brain bookmark tables for GetTables.
func bookmarkTables() []interface{} {
	return []interface{}{
		&Bookmark{},
		&BookmarkSnapshot{},
		&BookmarkProject{},
		&BookmarkTask{},
		&LabelBookmark{},
		&BookmarkComment{},
	}
}
