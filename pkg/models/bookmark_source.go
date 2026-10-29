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

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/web"

	"github.com/ThreeDotsLabs/watermill/message"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// Johnny's Brain "sources": bookmarks linked to the projects and tasks they inform.
// Linking needs the bookmark's owner to also have write access to the project or task,
// because a link makes the bookmark visible to everyone who can see that project or task.

// BookmarkProject links a bookmark to a project as a source.
type BookmarkProject struct {
	ID         int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID int64 `xorm:"bigint not null INDEX unique(bookmark_project)" json:"-" param:"bookmark"`
	// The project to link the bookmark to.
	ProjectID   int64     `xorm:"bigint not null INDEX unique(bookmark_project)" json:"project_id" param:"project" doc:"The project to link the bookmark to. You need write access to it."`
	CreatedByID int64     `xorm:"bigint not null" json:"-"`
	Created     time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When the link was made. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for bookmark to project links.
func (*BookmarkProject) TableName() string {
	return "bookmark_projects"
}

// BookmarkTask links a bookmark to a task as a source.
type BookmarkTask struct {
	ID         int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID int64 `xorm:"bigint not null INDEX unique(bookmark_task)" json:"-" param:"bookmark"`
	// The task to link the bookmark to.
	TaskID      int64     `xorm:"bigint not null INDEX unique(bookmark_task)" json:"task_id" param:"task" doc:"The task to link the bookmark to. You need write access to it."`
	CreatedByID int64     `xorm:"bigint not null" json:"-"`
	Created     time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"When the link was made. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for bookmark to task links.
func (*BookmarkTask) TableName() string {
	return "bookmark_tasks"
}

// BookmarkLinkedProject is the short form of a linked project shown on a bookmark.
type BookmarkLinkedProject struct {
	ID       int64  `xorm:"id" json:"id" doc:"The project id."`
	Title    string `xorm:"title" json:"title" doc:"The project title."`
	HexColor string `xorm:"hex_color" json:"hex_color" doc:"The project colour in hex, without #."`
}

// BookmarkLinkedTask is the short form of a linked task shown on a bookmark.
type BookmarkLinkedTask struct {
	ID        int64  `xorm:"id" json:"id" doc:"The task id."`
	Title     string `xorm:"title" json:"title" doc:"The task title."`
	ProjectID int64  `xorm:"project_id" json:"project_id" doc:"The project the task is in."`
	Done      bool   `xorm:"done" json:"done" doc:"Whether the task is done."`
}

// Create links the bookmark to the project.
func (bp *BookmarkProject) Create(s *xorm.Session, a web.Auth) error {
	exists, err := s.Where("bookmark_id = ? AND project_id = ?", bp.BookmarkID, bp.ProjectID).Exist(&BookmarkProject{})
	if err != nil {
		return err
	}
	if exists {
		return ErrBookmarkSourceAlreadyLinked{BookmarkID: bp.BookmarkID}
	}
	bp.ID = 0
	bp.CreatedByID = a.GetID()
	_, err = s.Insert(bp)
	return err
}

// Delete unlinks the bookmark from the project.
func (bp *BookmarkProject) Delete(s *xorm.Session, _ web.Auth) error {
	deleted, err := s.Where("bookmark_id = ? AND project_id = ?", bp.BookmarkID, bp.ProjectID).Delete(&BookmarkProject{})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrBookmarkSourceNotLinked{BookmarkID: bp.BookmarkID}
	}
	return nil
}

// CanCreate: the caller owns the bookmark and can write to the project.
func (bp *BookmarkProject) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	owns, err := (&Bookmark{ID: bp.BookmarkID}).CanUpdate(s, a)
	if err != nil || !owns {
		return false, err
	}
	if _, err := GetProjectSimpleByID(s, bp.ProjectID); err != nil {
		return false, err
	}
	return (&Project{ID: bp.ProjectID}).CanWrite(s, a)
}

// CanDelete: the bookmark's owner, or anyone who can write to the project, may remove the link.
func (bp *BookmarkProject) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	owns, err := (&Bookmark{ID: bp.BookmarkID}).CanUpdate(s, a)
	if err != nil || owns {
		return owns, err
	}
	if _, err := GetProjectSimpleByID(s, bp.ProjectID); err != nil {
		if IsErrProjectDoesNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return (&Project{ID: bp.ProjectID}).CanWrite(s, a)
}

// Create links the bookmark to the task.
func (bt *BookmarkTask) Create(s *xorm.Session, a web.Auth) error {
	exists, err := s.Where("bookmark_id = ? AND task_id = ?", bt.BookmarkID, bt.TaskID).Exist(&BookmarkTask{})
	if err != nil {
		return err
	}
	if exists {
		return ErrBookmarkSourceAlreadyLinked{BookmarkID: bt.BookmarkID}
	}
	bt.ID = 0
	bt.CreatedByID = a.GetID()
	_, err = s.Insert(bt)
	return err
}

// Delete unlinks the bookmark from the task.
func (bt *BookmarkTask) Delete(s *xorm.Session, _ web.Auth) error {
	deleted, err := s.Where("bookmark_id = ? AND task_id = ?", bt.BookmarkID, bt.TaskID).Delete(&BookmarkTask{})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrBookmarkSourceNotLinked{BookmarkID: bt.BookmarkID}
	}
	return nil
}

// CanCreate: the caller owns the bookmark and can write to the task.
func (bt *BookmarkTask) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	owns, err := (&Bookmark{ID: bt.BookmarkID}).CanUpdate(s, a)
	if err != nil || !owns {
		return false, err
	}
	if _, err := GetTaskByIDSimple(s, bt.TaskID); err != nil {
		return false, err
	}
	return (&Task{ID: bt.TaskID}).CanWrite(s, a)
}

// CanDelete: the bookmark's owner, or anyone who can write to the task, may remove the link.
func (bt *BookmarkTask) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	owns, err := (&Bookmark{ID: bt.BookmarkID}).CanUpdate(s, a)
	if err != nil || owns {
		return owns, err
	}
	if _, err := GetTaskByIDSimple(s, bt.TaskID); err != nil {
		if IsErrTaskDoesNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return (&Task{ID: bt.TaskID}).CanWrite(s, a)
}

// addBookmarkSources fills in the projects and tasks a bookmark is linked to, limited to the
// ones the caller can see.
func addBookmarkSources(s *xorm.Session, a web.Auth, b *Bookmark) error {
	projectAccess, err := accessibleProjectIDsCond(s, a, "projects.id")
	if err != nil {
		return err
	}
	projects := []*BookmarkLinkedProject{}
	err = s.Table("projects").
		Select("projects.id, projects.title, projects.hex_color").
		Join("INNER", "bookmark_projects", "bookmark_projects.project_id = projects.id").
		Where(builder.And(builder.Eq{"bookmark_projects.bookmark_id": b.ID}, projectAccess)).
		OrderBy("projects.title ASC").
		Find(&projects)
	if err != nil {
		return err
	}

	taskAccess, err := accessibleProjectIDsCond(s, a, "tasks.project_id")
	if err != nil {
		return err
	}
	tasks := []*BookmarkLinkedTask{}
	err = s.Table("tasks").
		Select("tasks.id, tasks.title, tasks.project_id, tasks.done").
		Join("INNER", "bookmark_tasks", "bookmark_tasks.task_id = tasks.id").
		Where(builder.And(builder.Eq{"bookmark_tasks.bookmark_id": b.ID}, taskAccess, taskNotDeletedCond("tasks"))).
		OrderBy("tasks.done ASC, tasks.title ASC").
		Find(&tasks)
	if err != nil {
		return err
	}

	b.Projects = projects
	b.Tasks = tasks
	return nil
}

// RemoveOrphanedBookmarkSources deletes source links whose project or task no longer exists.
// Soft-deleted tasks keep their links so a restore brings them back.
type RemoveOrphanedBookmarkSources struct{}

// Name defines the name for the RemoveOrphanedBookmarkSources listener
func (*RemoveOrphanedBookmarkSources) Name() string {
	return "bookmark.sources.cleanup"
}

// Handle is executed when a task or project is deleted
func (*RemoveOrphanedBookmarkSources) Handle(_ *message.Message) error {
	s := db.NewSession()
	defer s.Close()

	if _, err := s.Where(builder.NotIn("project_id", builder.Select("id").From("projects"))).Delete(&BookmarkProject{}); err != nil {
		_ = s.Rollback()
		return err
	}
	if _, err := s.Where(builder.NotIn("task_id", builder.Select("id").From("tasks"))).Delete(&BookmarkTask{}); err != nil {
		_ = s.Rollback()
		return err
	}
	return s.Commit()
}

func registerBookmarkListeners() {
	cleanup := &RemoveOrphanedBookmarkSources{}
	events.RegisterListener((&TaskDeletedEvent{}).Name(), cleanup)
	events.RegisterListener((&ProjectDeletedEvent{}).Name(), cleanup)
	events.RegisterListener((&BookmarkSavedEvent{}).Name(), &DetectBookmarkDuplicates{})
}
