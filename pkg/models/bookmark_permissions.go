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
	"fmt"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// CanCreate checks if the caller can save bookmarks. Any real user can; link shares cannot.
func (b *Bookmark) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {
	if _, is := a.(*LinkSharing); is {
		return false, nil
	}
	return true, nil
}

// CanRead checks if the caller can see a bookmark: they own it, or it is linked as a
// source to a project or task they can access.
func (b *Bookmark) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	return b.hasAccessToBookmark(s, a)
}

// CanUpdate checks if the caller can edit a bookmark. Only the owner can.
func (b *Bookmark) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	return b.isBookmarkOwner(s, a)
}

// CanDelete checks if the caller can delete a bookmark. Only the owner can.
func (b *Bookmark) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	return b.isBookmarkOwner(s, a)
}

func (b *Bookmark) isBookmarkOwner(s *xorm.Session, a web.Auth) (bool, error) {
	stored, err := getBookmarkByIDSimple(s, b.ID)
	if err != nil {
		return false, err
	}

	// Link shares can see bookmarks through a shared project but never own one.
	caller, err := user.GetFromAuth(a)
	if user.IsErrMustNotBeLinkShare(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if stored.CreatedByID == caller.ID {
		return true, nil
	}

	creator, err := user.GetUserByID(s, stored.CreatedByID)
	if err != nil {
		if user.IsErrUserDoesNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return creator.IsBotOwnedBy(caller), nil
}

// bookmarkVisibleCond matches every bookmark the caller may see. It is the one definition
// of bookmark visibility, shared by single reads and the list.
//
// Like labelVisibleCond, the branches are combined with builder.Or and handed to Where in
// one piece; chaining session .Where/.Or would flatten the precedence.
func bookmarkVisibleCond(s *xorm.Session, a web.Auth) (builder.Cond, error) {
	projectAccess, err := accessibleProjectIDsCond(s, a, "bookmark_projects.project_id")
	if err != nil {
		return nil, err
	}
	taskProjectAccess, err := accessibleProjectIDsCond(s, a, "tasks.project_id")
	if err != nil {
		return nil, err
	}

	linkedToAccessibleProject := builder.In("bookmarks.id",
		builder.
			Select("bookmark_projects.bookmark_id").
			From("bookmark_projects").
			Where(projectAccess),
	)
	linkedToAccessibleTask := builder.In("bookmarks.id",
		builder.
			Select("bookmark_tasks.bookmark_id").
			From("bookmark_tasks").
			InnerJoin("tasks", "tasks.id = bookmark_tasks.task_id").
			Where(builder.And(taskProjectAccess, taskNotDeletedCond("tasks"))),
	)

	branches := []builder.Cond{linkedToAccessibleProject, linkedToAccessibleTask}
	if _, isLinkShare := a.(*LinkSharing); !isLinkShare {
		caller, err := user.GetFromAuth(a)
		if err != nil {
			return nil, err
		}
		branches = append(branches, user.SameBotIdentityCond(caller, "bookmarks.created_by_id"))
	}

	visible := builder.Or(branches...)
	// Callers embed this in builder.And, which silently drops an invalid cond and would
	// then match every bookmark instead of failing.
	if visible == nil || !visible.IsValid() {
		return nil, fmt.Errorf("refusing to return an empty bookmark visibility condition for auth %d", a.GetID())
	}

	return visible, nil
}

// hasAccessToBookmark reports whether the caller can read a bookmark and their maximum
// permission on it: admin for the owner, read for anyone who sees it through a source link.
func (b *Bookmark) hasAccessToBookmark(s *xorm.Session, a web.Auth) (bool, int, error) {
	// Missing bookmarks are a 404, not a 403.
	if _, err := getBookmarkByIDSimple(s, b.ID); err != nil {
		return false, 0, err
	}

	visible, err := bookmarkVisibleCond(s, a)
	if err != nil {
		return false, 0, err
	}

	has, err := s.Table("bookmarks").
		Where(builder.And(builder.Eq{"bookmarks.id": b.ID}, visible)).
		Exist(&Bookmark{})
	if err != nil || !has {
		return false, 0, err
	}

	owner, err := b.isBookmarkOwner(s, a)
	if err != nil {
		return false, 0, err
	}
	if owner {
		return true, int(PermissionAdmin), nil
	}
	return true, int(PermissionRead), nil
}
