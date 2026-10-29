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
	"time"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// LabelBookmark puts an existing label on a bookmark.
type LabelBookmark struct {
	ID         int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID int64 `xorm:"bigint not null INDEX unique(label_bookmark)" json:"-" param:"bookmark"`
	// The id of the label to put on the bookmark.
	LabelID int64 `xorm:"bigint not null INDEX unique(label_bookmark)" json:"label_id" param:"label" doc:"The id of the label to put on the bookmark. You must be able to see the label."`
	// When the label was added.
	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this label was added to the bookmark. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for bookmark labels.
func (*LabelBookmark) TableName() string {
	return "label_bookmarks"
}

// Create adds the label to the bookmark.
func (lb *LabelBookmark) Create(s *xorm.Session, _ web.Auth) error {
	exists, err := s.Where("bookmark_id = ? AND label_id = ?", lb.BookmarkID, lb.LabelID).Exist(&LabelBookmark{})
	if err != nil {
		return err
	}
	if exists {
		return ErrLabelIsAlreadyOnBookmark{LabelID: lb.LabelID, BookmarkID: lb.BookmarkID}
	}

	lb.ID = 0
	_, err = s.Insert(lb)
	return err
}

// Delete removes the label from the bookmark.
func (lb *LabelBookmark) Delete(s *xorm.Session, _ web.Auth) error {
	deleted, err := s.Where("bookmark_id = ? AND label_id = ?", lb.BookmarkID, lb.LabelID).Delete(&LabelBookmark{})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrLabelIsNotOnBookmark{LabelID: lb.LabelID, BookmarkID: lb.BookmarkID}
	}
	return nil
}

// CanCreate checks if the caller can label a bookmark: they own the bookmark and can see the label.
func (lb *LabelBookmark) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	ownsBookmark, err := (&Bookmark{ID: lb.BookmarkID}).CanUpdate(s, a)
	if err != nil || !ownsBookmark {
		return false, err
	}

	if _, err := getLabelByIDSimple(s, lb.LabelID); err != nil {
		return false, err
	}
	canSeeLabel, _, err := (&Label{ID: lb.LabelID}).CanRead(s, a)
	return canSeeLabel, err
}

// CanDelete checks if the caller can remove a label from a bookmark: they own the bookmark.
func (lb *LabelBookmark) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	return (&Bookmark{ID: lb.BookmarkID}).CanUpdate(s, a)
}
