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

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterBookmarkRefreshRoutes wires re-fetching a bookmark's page. It is not plain CRUD, so
// the handler opens its own session and the model checks ownership.
func RegisterBookmarkRefreshRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID:   "bookmarks-refresh",
		Summary:       "Refresh a bookmark from its page",
		Description:   "Queues a new fetch of the bookmark's page in the background and returns 202 straight away; read the bookmark again a few seconds later. The fetch fills the title (while it is still the default host name), description, site name, icon and preview image only where they are blank, records the link status and check time, and saves the page's readable text for search. Only the owner may refresh. Fails with 409 when link previews are switched off on the server.",
		Method:        http.MethodPost,
		Path:          "/bookmarks/{bookmark}/refresh",
		Tags:          []string{"bookmarks"},
		DefaultStatus: http.StatusAccepted,
	}, bookmarksRefresh)
}

func init() { AddRouteRegistrar(RegisterBookmarkRefreshRoutes) }

func bookmarksRefresh(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	if err := models.RequestBookmarkRefresh(ctx, s, a, in.BookmarkID); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
