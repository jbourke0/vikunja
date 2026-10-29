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
	"strings"
	"time"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// BookmarkComment is a comment on a bookmark.
type BookmarkComment struct {
	ID int64 `xorm:"bigint autoincr not null unique pk" json:"id" param:"commentid" readOnly:"true" doc:"The unique, numeric id of this comment."`
	// The comment text.
	Comment string `xorm:"text not null" json:"comment" valid:"dbtext,required" doc:"The comment text. May contain HTML."`

	AuthorID int64 `xorm:"bigint not null" json:"-"`
	// The user who wrote the comment.
	Author *user.User `xorm:"-" json:"author" readOnly:"true" doc:"The user who wrote the comment. Set from the authenticated user on create; ignored on write."`

	BookmarkID int64 `xorm:"bigint not null INDEX" json:"-" param:"bookmark"`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this comment was created. You cannot change this value."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this comment was last updated. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for bookmark comments.
func (*BookmarkComment) TableName() string {
	return "bookmark_comments"
}

// Create adds a comment to a bookmark as the authenticated user.
func (bc *BookmarkComment) Create(s *xorm.Session, a web.Auth) error {
	author, err := user.GetFromAuth(a)
	if err != nil {
		return err
	}

	bc.Comment = strings.TrimSpace(bc.Comment)
	if bc.Comment == "" {
		return ErrBookmarkCommentEmpty{}
	}

	bc.ID = 0
	bc.AuthorID = author.ID
	bc.Author = author

	_, err = s.Insert(bc)
	return err
}

// Update changes a comment's text. Only the author may do this (see CanUpdate).
func (bc *BookmarkComment) Update(s *xorm.Session, a web.Auth) error {
	bc.Comment = strings.TrimSpace(bc.Comment)
	if bc.Comment == "" {
		return ErrBookmarkCommentEmpty{}
	}

	_, err := s.
		Where("id = ? AND bookmark_id = ?", bc.ID, bc.BookmarkID).
		Cols("comment").
		Update(bc)
	if err != nil {
		return err
	}

	return bc.ReadOne(s, a)
}

// Delete removes a comment.
func (bc *BookmarkComment) Delete(s *xorm.Session, _ web.Auth) error {
	_, err := s.Where("id = ? AND bookmark_id = ?", bc.ID, bc.BookmarkID).Delete(&BookmarkComment{})
	return err
}

// ReadOne loads a single comment with its author.
func (bc *BookmarkComment) ReadOne(s *xorm.Session, _ web.Auth) error {
	stored, err := getBookmarkCommentSimple(s, bc.ID, bc.BookmarkID)
	if err != nil {
		return err
	}
	*bc = *stored

	author, err := user.GetUserByID(s, bc.AuthorID)
	if err != nil && !user.IsErrUserDoesNotExist(err) {
		return err
	}
	bc.Author = author
	return nil
}

// ReadAll lists the comments on a bookmark, oldest first.
func (bc *BookmarkComment) ReadAll(s *xorm.Session, a web.Auth, _ string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	// The generic list handler does not call CanRead, so the check has to happen here.
	canRead, _, err := bc.CanRead(s, a)
	if err != nil {
		return nil, 0, 0, err
	}
	if !canRead {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	limit, start := getLimitFromPageIndex(page, perPage)

	comments := []*BookmarkComment{}
	query := s.Where("bookmark_id = ?", bc.BookmarkID).OrderBy("created ASC, id ASC")
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err := query.Find(&comments); err != nil {
		return nil, 0, 0, err
	}

	authorIDs := make([]int64, 0, len(comments))
	for _, c := range comments {
		authorIDs = append(authorIDs, c.AuthorID)
	}
	authors, err := user.GetUsersByIDs(s, authorIDs)
	if err != nil {
		return nil, 0, 0, err
	}
	for _, c := range comments {
		c.Author = authors[c.AuthorID]
	}

	total, err := s.Where("bookmark_id = ?", bc.BookmarkID).Count(&BookmarkComment{})
	if err != nil {
		return nil, 0, 0, err
	}

	return comments, len(comments), total, nil
}

func getBookmarkCommentSimple(s *xorm.Session, id, bookmarkID int64) (*BookmarkComment, error) {
	c := &BookmarkComment{}
	exists, err := s.Where("id = ? AND bookmark_id = ?", id, bookmarkID).Get(c)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrBookmarkCommentDoesNotExist{CommentID: id, BookmarkID: bookmarkID}
	}
	return c, nil
}

// CanRead checks if the caller can read comments: anyone who can read the bookmark.
func (bc *BookmarkComment) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	return (&Bookmark{ID: bc.BookmarkID}).CanRead(s, a)
}

// CanCreate checks if the caller can comment: any real user who can read the bookmark.
func (bc *BookmarkComment) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	if _, is := a.(*LinkSharing); is {
		return false, nil
	}
	can, _, err := (&Bookmark{ID: bc.BookmarkID}).CanRead(s, a)
	return can, err
}

// CanUpdate checks if the caller can edit a comment: only its author.
func (bc *BookmarkComment) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	return bc.isAuthorWithAccess(s, a)
}

// CanDelete checks if the caller can delete a comment: only its author.
func (bc *BookmarkComment) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	return bc.isAuthorWithAccess(s, a)
}

func (bc *BookmarkComment) isAuthorWithAccess(s *xorm.Session, a web.Auth) (bool, error) {
	if _, is := a.(*LinkSharing); is {
		return false, nil
	}

	canRead, _, err := (&Bookmark{ID: bc.BookmarkID}).CanRead(s, a)
	if err != nil || !canRead {
		return false, err
	}

	stored, err := getBookmarkCommentSimple(s, bc.ID, bc.BookmarkID)
	if err != nil {
		return false, err
	}
	return stored.AuthorID == a.GetID(), nil
}
