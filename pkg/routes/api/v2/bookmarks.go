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
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

// Johnny's Brain: saved links.

type bookmarkListBody struct {
	Body Paginated[*models.Bookmark]
}

// RegisterBookmarkRoutes wires Bookmark CRUD onto the Huma API.
func RegisterBookmarkRoutes(api huma.API) {
	tags := []string{"bookmarks"}

	Register(api, huma.Operation{
		OperationID: "bookmarks-list",
		Summary:     "List bookmarks",
		Description: "Returns the bookmarks the authenticated user can see, newest first: their own plus any linked as a source to a project or task they can access. q searches the title, URL and description; the other filters narrow the list and never widen it.",
		Method:      http.MethodGet,
		Path:        "/bookmarks",
		Tags:        tags,
	}, bookmarksList)

	Register(api, huma.Operation{
		OperationID: "bookmarks-read",
		Summary:     "Get a bookmark",
		Description: "Returns a single bookmark with its labels. Sends an ETag; pass it as If-None-Match on a later read to get a 304 Not Modified.",
		Method:      http.MethodGet,
		Path:        "/bookmarks/{id}",
		Tags:        tags,
	}, bookmarksRead)

	Register(api, huma.Operation{
		OperationID: "bookmarks-create",
		Summary:     "Save a bookmark",
		Description: "Saves a link; the authenticated user becomes its owner. Only url is required. The title defaults to the site's host name and the status to to_read. Saving a link you already saved fails with 409 (look it up with GET /bookmarks?url=). Possible duplicates behind different links are found in the background; see the duplicates endpoint.",
		Method:      http.MethodPost,
		Path:        "/bookmarks",
		Tags:        tags,
	}, bookmarksCreate)

	Register(api, huma.Operation{
		OperationID: "bookmarks-update",
		Summary:     "Update a bookmark",
		Description: "Replaces a bookmark's editable fields. Only the owner may update it. Use PATCH for a partial update, for example to change only the status. Labels are managed through the bookmark labels endpoints.",
		Method:      http.MethodPut,
		Path:        "/bookmarks/{id}",
		Tags:        tags,
	}, bookmarksUpdate)

	Register(api, huma.Operation{
		OperationID: "bookmarks-delete",
		Summary:     "Delete a bookmark",
		Description: "Deletes a bookmark with its comments, labels and source links. Only the owner may delete it.",
		Method:      http.MethodDelete,
		Path:        "/bookmarks/{id}",
		Tags:        tags,
	}, bookmarksDelete)
}

func init() { AddRouteRegistrar(RegisterBookmarkRoutes) }

func bookmarksList(ctx context.Context, in *struct {
	ListParams
	Status    string `query:"status" enum:"to_read,reference,archived" doc:"Only bookmarks with this status."`
	LabelID   int64  `query:"label_id" minimum:"1" doc:"Only bookmarks with this label."`
	ProjectID int64  `query:"project_id" minimum:"1" doc:"Only bookmarks linked as a source to this project."`
	TaskID    int64  `query:"task_id" minimum:"1" doc:"Only bookmarks linked as a source to this task."`
	Broken    bool   `query:"broken" doc:"Only bookmarks whose last link check found the page gone or failing: not found, gone, a server error or no response."`
	URL       string `query:"url" doc:"Only the bookmark saved for this link, compared after normalising (tracking parameters, www and trailing slashes are ignored). Use it to ask whether a link is already saved."`
}) (*bookmarkListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	filter := &models.Bookmark{
		FilterStatus:    in.Status,
		FilterLabelID:   in.LabelID,
		FilterProjectID: in.ProjectID,
		FilterTaskID:    in.TaskID,
		FilterURL:       in.URL,
		FilterBroken:    in.Broken,
	}
	result, _, total, err := handler.DoReadAll(ctx, filter, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.Bookmark)
	if !ok {
		return nil, fmt.Errorf("bookmarks.ReadAll returned unexpected type %T (expected []*models.Bookmark)", result)
	}
	return &bookmarkListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type bookmarkReadBody struct {
	models.Bookmark
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this bookmark: 2 (admin) for the owner, 0 (read) for anyone who sees it through a source link."`
}

func bookmarksRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[bookmarkReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	bookmark := &models.Bookmark{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, bookmark, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &bookmarkReadBody{Bookmark: *bookmark, MaxPermission: models.Permission(maxPermission)}
	return conditionalReadResponse(&in.Params, body, bookmark.Updated, maxPermission)
}

func bookmarksCreate(ctx context.Context, in *struct {
	Body models.Bookmark
}) (*singleBody[models.Bookmark], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Bookmark]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func bookmarksUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body bookmarkReadBody
}) (*singleBody[models.Bookmark], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	bookmark := &in.Body.Bookmark
	bookmark.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, bookmark, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Bookmark]{Body: bookmark}, nil
}

func bookmarksDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.Bookmark{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
