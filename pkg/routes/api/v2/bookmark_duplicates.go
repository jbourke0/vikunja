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

type bookmarkDuplicatesBody struct {
	Body []*models.BookmarkDuplicateSuggestion
}

type bookmarkMergeInput struct {
	SourceID int64 `json:"source_id" minimum:"1" doc:"The bookmark to fold into this one. It is deleted after the merge."`
}

// RegisterBookmarkDuplicateRoutes wires duplicate suggestions and merging. These are not
// plain CRUD, so the handlers open their own session and the model functions check access.
func RegisterBookmarkDuplicateRoutes(api huma.API) {
	tags := []string{"bookmarks"}

	Register(api, huma.Operation{
		OperationID: "bookmark-duplicates-list",
		Summary:     "List possible duplicates of a bookmark",
		Description: "Returns the open suggestions of other bookmarks that look like the same product, story or information, most likely first. Suggestions are made in the background after each save; saving the exact same link twice is refused instead (409).",
		Method:      http.MethodGet,
		Path:        "/bookmarks/{bookmark}/duplicates",
		Tags:        tags,
	}, bookmarkDuplicatesList)

	Register(api, huma.Operation{
		OperationID:   "bookmark-duplicates-dismiss",
		Summary:       "Mark two bookmarks as not duplicates",
		Description:   "Dismisses the suggestion so the pair is never suggested again. Only the bookmark's owner may do this.",
		Method:        http.MethodPost,
		Path:          "/bookmarks/{bookmark}/duplicates/{other}/dismiss",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, bookmarkDuplicatesDismiss)

	Register(api, huma.Operation{
		OperationID:   "bookmarks-merge",
		Summary:       "Merge another bookmark into this one",
		Description:   "Moves the source bookmark's labels, comments, project and task links and page snapshot onto this bookmark, fills this bookmark's empty fields from it, records the merged link in a comment and deletes the source. The caller must own both. Returns the merged bookmark.",
		Method:        http.MethodPost,
		Path:          "/bookmarks/{bookmark}/merge",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, bookmarksMerge)
}

func init() { AddRouteRegistrar(RegisterBookmarkDuplicateRoutes) }

func bookmarkDuplicatesList(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
}) (*bookmarkDuplicatesBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	suggestions, err := models.GetBookmarkDuplicateSuggestions(s, a, in.BookmarkID)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &bookmarkDuplicatesBody{Body: suggestions}, nil
}

func bookmarkDuplicatesDismiss(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	OtherID    int64 `path:"other"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	if err := models.DismissBookmarkDuplicate(s, a, in.BookmarkID, in.OtherID); err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

func bookmarksMerge(ctx context.Context, in *struct {
	BookmarkID int64 `path:"bookmark"`
	Body       bookmarkMergeInput
}) (*singleBody[models.Bookmark], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	merged, err := models.MergeBookmarks(s, a, in.BookmarkID, in.Body.SourceID)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Bookmark]{Body: merged}, nil
}
