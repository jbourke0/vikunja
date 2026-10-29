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

type bookmarkCommentListBody struct {
	Body Paginated[*models.BookmarkComment]
}

// RegisterBookmarkCommentRoutes wires comments on bookmarks onto the Huma API.
func RegisterBookmarkCommentRoutes(api huma.API) {
	tags := []string{"bookmarks"}

	Register(api, huma.Operation{
		OperationID: "bookmark-comments-list",
		Summary:     "List the comments of a bookmark",
		Description: "Returns the comments of the given bookmark, oldest first. Requires read access to the bookmark.",
		Method:      http.MethodGet,
		Path:        "/bookmarks/{bookmark}/comments",
		Tags:        tags,
	}, bookmarkCommentsList)

	Register(api, huma.Operation{
		OperationID: "bookmark-comments-read",
		Summary:     "Get a single comment of a bookmark",
		Description: "Returns one comment. The comment must belong to the bookmark in the path. Sends an ETag; pass it as If-None-Match on a later read to get a 304 Not Modified.",
		Method:      http.MethodGet,
		Path:        "/bookmarks/{bookmark}/comments/{commentid}",
		Tags:        tags,
	}, bookmarkCommentsRead)

	Register(api, huma.Operation{
		OperationID: "bookmark-comments-create",
		Summary:     "Comment on a bookmark",
		Description: "Adds a comment to the given bookmark as the authenticated user. Anyone who can read the bookmark may comment.",
		Method:      http.MethodPost,
		Path:        "/bookmarks/{bookmark}/comments",
		Tags:        tags,
	}, bookmarkCommentsCreate)

	Register(api, huma.Operation{
		OperationID: "bookmark-comments-update",
		Summary:     "Update a comment of a bookmark",
		Description: "Replaces a comment's text. Only its author may update it. Use PATCH for a partial update.",
		Method:      http.MethodPut,
		Path:        "/bookmarks/{bookmark}/comments/{commentid}",
		Tags:        tags,
	}, bookmarkCommentsUpdate)

	Register(api, huma.Operation{
		OperationID: "bookmark-comments-delete",
		Summary:     "Delete a comment of a bookmark",
		Description: "Deletes a comment. Only its author may delete it.",
		Method:      http.MethodDelete,
		Path:        "/bookmarks/{bookmark}/comments/{commentid}",
		Tags:        tags,
	}, bookmarkCommentsDelete)
}

func init() { AddRouteRegistrar(RegisterBookmarkCommentRoutes) }

func bookmarkCommentsList(ctx context.Context, in *struct {
	BookmarkID int64  `path:"bookmark"`
	Format     string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
	ListParams
}) (*bookmarkCommentListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.BookmarkComment{BookmarkID: in.BookmarkID}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.BookmarkComment)
	if !ok {
		return nil, fmt.Errorf("bookmarkComments.ReadAll returned unexpected type %T (expected []*models.BookmarkComment)", result)
	}
	for _, c := range items {
		convertToMarkdown(ctx, &c.Comment)
	}
	return &bookmarkCommentListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type bookmarkCommentReadBody struct {
	models.BookmarkComment
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on the parent bookmark. Editing or deleting a comment also requires being its author."`
}

func bookmarkCommentsRead(ctx context.Context, in *struct {
	BookmarkID int64  `path:"bookmark"`
	ID         int64  `path:"commentid"`
	Format     string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
	conditional.Params
}) (*singleReadBody[bookmarkCommentReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	comment := &models.BookmarkComment{ID: in.ID, BookmarkID: in.BookmarkID}
	maxPermission, err := handler.DoReadOne(ctx, comment, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &bookmarkCommentReadBody{BookmarkComment: *comment, MaxPermission: models.Permission(maxPermission)}
	convertToMarkdown(ctx, &body.Comment)
	return conditionalReadResponse(&in.Params, body, comment.Updated, maxPermission)
}

func bookmarkCommentsCreate(ctx context.Context, in *struct {
	BookmarkID int64  `path:"bookmark"`
	Format     string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
	Body       models.BookmarkComment
}) (*singleBody[models.BookmarkComment], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.BookmarkID = in.BookmarkID // URL wins over body
	if err := convertToHTML(ctx, &in.Body.Comment); err != nil {
		return nil, translateDomainError(err)
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	convertToMarkdown(ctx, &in.Body.Comment)
	return &singleBody[models.BookmarkComment]{Body: &in.Body}, nil
}

func bookmarkCommentsUpdate(ctx context.Context, in *struct {
	BookmarkID int64  `path:"bookmark"`
	ID         int64  `path:"commentid"`
	Format     string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
	Body       bookmarkCommentReadBody
}) (*singleBody[models.BookmarkComment], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	comment := &in.Body.BookmarkComment
	comment.ID = in.ID                 // URL wins over body
	comment.BookmarkID = in.BookmarkID // parent from the path scopes the update
	if err := convertToHTML(ctx, &comment.Comment); err != nil {
		return nil, translateDomainError(err)
	}
	if err := handler.DoUpdate(ctx, comment, a); err != nil {
		return nil, translateDomainError(err)
	}
	convertToMarkdown(ctx, &comment.Comment)
	return &singleBody[models.BookmarkComment]{Body: comment}, nil
}

func bookmarkCommentsDelete(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	ID         int64 `path:"commentid"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.BookmarkComment{ID: in.ID, BookmarkID: in.BookmarkID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
