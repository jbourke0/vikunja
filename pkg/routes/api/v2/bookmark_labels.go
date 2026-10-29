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

// RegisterBookmarkLabelRoutes wires adding and removing labels on a bookmark. There is no
// list: a bookmark's labels come back on the bookmark itself.
func RegisterBookmarkLabelRoutes(api huma.API) {
	tags := []string{"bookmarks"}

	Register(api, huma.Operation{
		OperationID: "bookmark-labels-create",
		Summary:     "Add a label to a bookmark",
		Description: "Puts an existing label on the given bookmark. Requires owning the bookmark and being able to see the label. Fails with 409 if the label is already on it.",
		Method:      http.MethodPost,
		Path:        "/bookmarks/{bookmark}/labels",
		Tags:        tags,
	}, bookmarkLabelsCreate)

	Register(api, huma.Operation{
		OperationID: "bookmark-labels-delete",
		Summary:     "Remove a label from a bookmark",
		Description: "Takes a label off the given bookmark. Requires owning the bookmark.",
		Method:      http.MethodDelete,
		Path:        "/bookmarks/{bookmark}/labels/{label}",
		Tags:        tags,
	}, bookmarkLabelsDelete)
}

func init() { AddRouteRegistrar(RegisterBookmarkLabelRoutes) }

func bookmarkLabelsCreate(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	Body       models.LabelBookmark
}) (*singleBody[models.LabelBookmark], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.BookmarkID = in.BookmarkID // parent from the path, not the body
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.LabelBookmark]{Body: &in.Body}, nil
}

func bookmarkLabelsDelete(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	LabelID    int64 `path:"label"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.LabelBookmark{BookmarkID: in.BookmarkID, LabelID: in.LabelID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
