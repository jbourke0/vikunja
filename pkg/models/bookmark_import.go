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
	"context"
	"encoding/json"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"github.com/ThreeDotsLabs/watermill/message"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// Johnny's Brain bookmark import: the browser's exported bookmarks file, in batches.
//
// The file is read in the browser and sent here a few hundred links at a time. Saving is quick;
// fetching every page and comparing for duplicates is slow, so that runs afterwards as one
// background job per batch, with the free local duplicate check only (never the AI judge).

const (
	bookmarkImportMaxItems    = 500
	bookmarkImportLabelMaxLen = 50
	bookmarkImportConcurrency = 3
)

// BookmarkImportItem is one link from an exported bookmarks file.
type BookmarkImportItem struct {
	URL     string     `json:"url" maxLength:"2048" doc:"The address. Items that are not absolute http or https URLs are counted as invalid and skipped."`
	Title   string     `json:"title,omitempty" maxLength:"250" doc:"The title the browser had. Left empty, the page's own title is fetched later."`
	Added   *time.Time `json:"added,omitempty" doc:"When the link was first bookmarked. Used as the saved date unless it is in the future or before 1995."`
	Folders []string   `json:"folders,omitempty" maxItems:"20" doc:"The folders the bookmark sat in, outermost first."`
}

// BookmarkImport is a batch of links to import.
type BookmarkImport struct {
	Items           []BookmarkImportItem `json:"items" maxItems:"500" doc:"The links to import, at most 500 per request."`
	Status          string               `json:"status,omitempty" enum:"to_read,reference,archived" doc:"Reading status for every imported link. Defaults to to_read."`
	FoldersAsLabels bool                 `json:"folders_as_labels,omitempty" doc:"Label each link with the name of the folder it sat in, reusing your labels of that name and creating the rest. Generic folders such as 'Bookmarks bar' are ignored."`
}

// BookmarkImportResult says what an import batch did.
type BookmarkImportResult struct {
	Created       int `json:"created" doc:"New bookmarks saved."`
	Duplicates    int `json:"duplicates" doc:"Links skipped because you already had them, or they appeared twice in the batch."`
	Invalid       int `json:"invalid" doc:"Items skipped because they were not http or https addresses."`
	LabelsCreated int `json:"labels_created" doc:"Labels created from folder names."`
}

var genericBookmarkFolders = map[string]bool{
	"bookmarks bar": true, "bookmarks toolbar": true, "bookmarks menu": true, "bookmarks": true,
	"other bookmarks": true, "mobile bookmarks": true, "favorites bar": true, "favourites bar": true,
	"favorites": true, "favourites": true, "reading list": true, "menu": true, "toolbar": true,
}

// labelForFolders picks the label for a bookmark from its folder path: the innermost folder
// that is not one of the browsers' own top-level folders.
func labelForFolders(folders []string) string {
	for i := len(folders) - 1; i >= 0; i-- {
		name := truncateRunes(strings.Join(strings.Fields(folders[i]), " "), bookmarkImportLabelMaxLen)
		lower := strings.ToLower(name)
		if name == "" || genericBookmarkFolders[lower] || strings.HasPrefix(lower, "imported") {
			continue
		}
		return name
	}
	return ""
}

type preparedImport struct {
	bookmark *Bookmark
	label    string
}

// ImportBookmarks saves a batch of links for the caller. The page fetches and duplicate checks
// are queued for after the transaction commits.
func ImportBookmarks(s *xorm.Session, a web.Auth, in *BookmarkImport) (*BookmarkImportResult, error) {
	if _, isLinkShare := a.(*LinkSharing); isLinkShare {
		return nil, ErrGenericForbidden{}
	}
	u, err := user.GetFromAuth(a)
	if err != nil {
		return nil, err
	}
	if len(in.Items) > bookmarkImportMaxItems {
		return nil, ErrBookmarkImportTooLarge{Max: bookmarkImportMaxItems}
	}
	status := in.Status
	if status == "" {
		status = BookmarkStatusToRead
	}

	result := &BookmarkImportResult{}
	now := time.Now()
	earliest := time.Date(1995, time.January, 1, 0, 0, 0, 0, time.UTC)

	prepared := make([]preparedImport, 0, len(in.Items))
	seen := map[string]bool{}
	hashes := make([]string, 0, len(in.Items))
	for _, item := range in.Items {
		b := &Bookmark{URL: item.URL, Title: item.Title, Status: status}
		if err := b.normalizeAndValidate(); err != nil {
			if IsErrBookmarkInvalidURL(err) {
				result.Invalid++
				continue
			}
			return nil, err
		}
		if seen[b.URLHash] {
			result.Duplicates++
			continue
		}
		seen[b.URLHash] = true
		hashes = append(hashes, b.URLHash)
		b.CreatedByID = u.ID
		if item.Added != nil && !item.Added.After(now) && item.Added.After(earliest) {
			b.Created, b.Updated = *item.Added, *item.Added
		}
		p := preparedImport{bookmark: b}
		if in.FoldersAsLabels {
			p.label = labelForFolders(item.Folders)
		}
		prepared = append(prepared, p)
	}

	if len(hashes) > 0 {
		already := []*Bookmark{}
		err := s.Cols("url_hash").
			Where(builder.And(builder.Eq{"created_by_id": u.ID}, builder.In("url_hash", hashes))).
			Find(&already)
		if err != nil {
			return nil, err
		}
		have := make(map[string]bool, len(already))
		for _, b := range already {
			have[b.URLHash] = true
		}
		kept := prepared[:0]
		for _, p := range prepared {
			if have[p.bookmark.URLHash] {
				result.Duplicates++
				continue
			}
			kept = append(kept, p)
		}
		prepared = kept
	}

	ids := make([]int64, 0, len(prepared))
	for _, p := range prepared {
		insert := s.Table("bookmarks")
		if !p.bookmark.Created.IsZero() {
			insert = insert.NoAutoTime() // keep the date the link was originally bookmarked
		}
		if _, err := insert.Insert(p.bookmark); err != nil {
			return nil, err
		}
		ids = append(ids, p.bookmark.ID)
	}
	result.Created = len(ids)

	labelsCreated, err := labelImportedBookmarks(s, u, prepared)
	if err != nil {
		return nil, err
	}
	result.LabelsCreated = labelsCreated

	if len(ids) > 0 {
		events.DispatchOnCommit(s, &BookmarkBatchSavedEvent{BookmarkIDs: ids})
	}
	return result, nil
}

// labelImportedBookmarks puts each bookmark's folder label on it, reusing the owner's labels
// of the same name (ignoring case) and creating the ones that do not exist yet.
func labelImportedBookmarks(s *xorm.Session, u *user.User, prepared []preparedImport) (created int, err error) {
	wanted := map[string]bool{}
	for _, p := range prepared {
		if p.label != "" {
			wanted[strings.ToLower(p.label)] = true
		}
	}
	if len(wanted) == 0 {
		return 0, nil
	}

	owned := []*Label{}
	if err := s.Where("created_by_id = ?", u.ID).Find(&owned); err != nil {
		return 0, err
	}
	byName := make(map[string]*Label, len(owned))
	for _, l := range owned {
		byName[strings.ToLower(l.Title)] = l
	}

	for _, p := range prepared {
		if p.label == "" {
			continue
		}
		key := strings.ToLower(p.label)
		label, ok := byName[key]
		if !ok {
			label = &Label{Title: p.label, CreatedByID: u.ID}
			if _, err := s.Insert(label); err != nil {
				return created, err
			}
			byName[key] = label
			created++
		}
		if _, err := s.Insert(&LabelBookmark{BookmarkID: p.bookmark.ID, LabelID: label.ID}); err != nil {
			return created, err
		}
	}
	return created, nil
}

// BookmarkBatchSavedEvent fires after a batch import commits.
type BookmarkBatchSavedEvent struct {
	BookmarkIDs []int64 `json:"bookmark_ids"`
}

// Name defines the name for BookmarkBatchSavedEvent
func (*BookmarkBatchSavedEvent) Name() string {
	return "bookmark.batch.saved"
}

// ProcessImportedBookmarks fetches the pages of an imported batch, gently, then checks the
// batch for duplicates locally.
type ProcessImportedBookmarks struct{}

// Name defines the name for the ProcessImportedBookmarks listener
func (*ProcessImportedBookmarks) Name() string {
	return "bookmark.batch.process"
}

// Handle is executed when an import batch has been saved. It never returns an error: the event
// system would retry the whole batch, fetching every page again.
func (*ProcessImportedBookmarks) Handle(msg *message.Message) error {
	event := &BookmarkBatchSavedEvent{}
	if err := json.Unmarshal(msg.Payload, event); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	if fetcher := currentBookmarkPageFetcher(); fetcher != nil {
		refreshBookmarksInBackground(ctx, fetcher, event.BookmarkIDs)
	}

	for _, id := range event.BookmarkIDs {
		if ctx.Err() != nil {
			break
		}
		err := retryWhenSQLiteBusy(ctx, func() error {
			s := db.NewSession()
			defer s.Close()
			if err := detectDuplicates(ctx, s, id, nil); err != nil {
				_ = s.Rollback()
				return err
			}
			return s.Commit()
		})
		if err != nil {
			log.Errorf("Bookmark import: duplicate check failed for bookmark %d: %s", id, err)
		}
	}
	return nil
}
