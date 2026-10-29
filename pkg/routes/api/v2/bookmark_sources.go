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

package apiv2

import (
	"context"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterBookmarkSourceRoutes wires linking bookmarks to projects and tasks ("sources").
// Linked bookmarks are listed with GET /bookmarks?project_id= or ?task_id=.
func RegisterBookmarkSourceRoutes(api huma.API) {
	tags := []string{"bookmarks"}

	Register(api, huma.Operation{
		OperationID: "bookmark-projects-create",
		Summary:     "Link a bookmark to a project",
		Description: "Adds the bookmark as a source on the project. Requires owning the bookmark and write access to the project; everyone who can see the project can then read the bookmark and comment on it. Fails with 409 if already linked.",
		Method:      http.MethodPost,
		Path:        "/bookmarks/{bookmark}/projects",
		Tags:        tags,
	}, bookmarkProjectsCreate)

	Register(api, huma.Operation{
		OperationID: "bookmark-projects-delete",
		Summary:     "Unlink a bookmark from a project",
		Description: "Removes the bookmark as a source on the project. The bookmark's owner or anyone with write access to the project may do this.",
		Method:      http.MethodDelete,
		Path:        "/bookmarks/{bookmark}/projects/{project}",
		Tags:        tags,
	}, bookmarkProjectsDelete)

	Register(api, huma.Operation{
		OperationID: "bookmark-tasks-create",
		Summary:     "Link a bookmark to a task",
		Description: "Adds the bookmark as a source on the task. Requires owning the bookmark and write access to the task; everyone who can see the task can then read the bookmark and comment on it. Fails with 409 if already linked.",
		Method:      http.MethodPost,
		Path:        "/bookmarks/{bookmark}/tasks",
		Tags:        tags,
	}, bookmarkTasksCreate)

	Register(api, huma.Operation{
		OperationID: "bookmark-tasks-delete",
		Summary:     "Unlink a bookmark from a task",
		Description: "Removes the bookmark as a source on the task. The bookmark's owner or anyone with write access to the task may do this.",
		Method:      http.MethodDelete,
		Path:        "/bookmarks/{bookmark}/tasks/{task}",
		Tags:        tags,
	}, bookmarkTasksDelete)
}

func init() { AddRouteRegistrar(RegisterBookmarkSourceRoutes) }

func bookmarkProjectsCreate(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	Body       models.BookmarkProject
}) (*singleBody[models.BookmarkProject], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.BookmarkID = in.BookmarkID // parent from the path, not the body
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.BookmarkProject]{Body: &in.Body}, nil
}

func bookmarkProjectsDelete(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	ProjectID  int64 `path:"project"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.BookmarkProject{BookmarkID: in.BookmarkID, ProjectID: in.ProjectID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

func bookmarkTasksCreate(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	Body       models.BookmarkTask
}) (*singleBody[models.BookmarkTask], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.BookmarkID = in.BookmarkID // parent from the path, not the body
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.BookmarkTask]{Body: &in.Body}, nil
}

func bookmarkTasksDelete(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	TaskID     int64 `path:"task"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.BookmarkTask{BookmarkID: in.BookmarkID, TaskID: in.TaskID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
