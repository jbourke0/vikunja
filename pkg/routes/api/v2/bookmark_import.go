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
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/models"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterBookmarkImportRoutes wires bulk import and reading a bookmark's saved page text. Neither
// is plain CRUD, so the handlers open their own session and the models check access.
func RegisterBookmarkImportRoutes(api huma.API) {
	tags := []string{"bookmarks"}

	Register(api, huma.Operation{
		OperationID:   "bookmarks-import",
		Summary:       "Import a batch of bookmarks",
		Description:   "Saves up to 500 links at once, for importing a browser's exported bookmarks file in batches. Links you already have, repeated links and anything that is not an http or https address are skipped and counted. Saved dates are kept when given. With folders_as_labels, each link gets the name of its folder as a label. Returns straight away: pages are fetched and checked for duplicates in the background afterwards, using the free local duplicate check only.",
		Method:        http.MethodPost,
		Path:          "/bookmarks/import",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, bookmarksImport)

	Register(api, huma.Operation{
		OperationID: "bookmarks-snapshot",
		Summary:     "Get a bookmark's saved page text",
		Description: "Returns the readable text saved from the page when it was last fetched. It is kept even if the page later disappears. Anyone who can read the bookmark can read it; 404 when nothing is saved.",
		Method:      http.MethodGet,
		Path:        "/bookmarks/{bookmark}/snapshot",
		Tags:        tags,
	}, bookmarksSnapshot)
}

func init() { AddRouteRegistrar(RegisterBookmarkImportRoutes) }

func bookmarksImport(ctx context.Context, in *struct {
	Body models.BookmarkImport
}) (*singleBody[models.BookmarkImportResult], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	result, err := models.ImportBookmarks(s, a, &in.Body)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	events.DispatchPending(ctx, s)
	return &singleBody[models.BookmarkImportResult]{Body: result}, nil
}

func bookmarksSnapshot(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
}) (*singleBody[models.BookmarkSnapshot], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	snapshot, err := models.GetBookmarkSnapshot(s, a, in.BookmarkID)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.BookmarkSnapshot]{Body: snapshot}, nil
}
