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
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// GetBookmarkSnapshot returns the saved page text of a bookmark the caller can read.
func GetBookmarkSnapshot(s *xorm.Session, a web.Auth, bookmarkID int64) (*BookmarkSnapshot, error) {
	canRead, _, err := (&Bookmark{ID: bookmarkID}).CanRead(s, a)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ErrGenericForbidden{}
	}

	snap := &BookmarkSnapshot{}
	has, err := s.Where("bookmark_id = ?", bookmarkID).Get(snap)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrBookmarkSnapshotDoesNotExist{BookmarkID: bookmarkID}
	}
	return snap, nil
}

// deleteUserBookmarks removes everything a deleted user saved or wrote.
func deleteUserBookmarks(s *xorm.Session, userID int64) error {
	owned := []*Bookmark{}
	if err := s.Cols("id").Where("created_by_id = ?", userID).Find(&owned); err != nil {
		return err
	}
	for _, b := range owned {
		if err := (&Bookmark{ID: b.ID}).Delete(s, nil); err != nil {
			return err
		}
	}
	_, err := s.Where("author_id = ?", userID).Delete(&BookmarkComment{})
	return err
}
