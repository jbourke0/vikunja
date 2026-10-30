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
	"sort"
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

// Johnny's Brain duplicate clean-up: the open duplicate suggestions gathered into groups, so a
// whole set of links about one thing can be reviewed and folded into one bookmark at a time.

const (
	// Links that carry the same note (for example, the same line of a saved guide) are the same
	// thing seen from different pages. A note this short, or shared by this many links, is
	// boilerplate rather than a signal, so it is ignored.
	duplicateSameNoteMinChars = 25
	duplicateSameNoteMaxGroup = 6
	duplicateSameNoteScore    = 0.8

	duplicateGroupsLimit = 200
)

// BookmarkDuplicatePair is two bookmarks of a group that were suggested as duplicates.
type BookmarkDuplicatePair struct {
	BookmarkID  int64 `json:"bookmark_id" doc:"One bookmark of the pair."`
	DuplicateID int64 `json:"duplicate_id" doc:"The other bookmark of the pair."`
}

// BookmarkDuplicateGroup is a set of your bookmarks that are linked by open duplicate suggestions.
type BookmarkDuplicateGroup struct {
	Bookmarks []*Bookmark             `json:"bookmarks" doc:"The bookmarks in the group, oldest first."`
	Pairs     []BookmarkDuplicatePair `json:"pairs" doc:"The suggested pairs within the group, so a group can be dismissed as a whole."`
}

// normalisedNote is a bookmark's description with case and spacing ignored, or empty when it
// is too short to say anything.
func normalisedNote(b *Bookmark) string {
	note := strings.ToLower(strings.Join(strings.Fields(b.Description), " "))
	if len([]rune(note)) < duplicateSameNoteMinChars {
		return ""
	}
	return note
}

// GetBookmarkDuplicateGroups returns your open duplicate suggestions as groups, biggest first.
// Only bookmarks you own are included.
func GetBookmarkDuplicateGroups(s *xorm.Session, a web.Auth) ([]*BookmarkDuplicateGroup, error) {
	u, ok := a.(*user.User)
	if !ok {
		return nil, ErrGenericForbidden{}
	}

	ids := []int64{}
	if err := s.Table("bookmarks").Where("created_by_id = ?", u.ID).Cols("id").Find(&ids); err != nil {
		return nil, err
	}
	groups := []*BookmarkDuplicateGroup{}
	if len(ids) < 2 {
		return groups, nil
	}

	rows := []*BookmarkDuplicate{}
	err := s.
		Where(builder.And(
			builder.In("bookmark_id", ids),
			builder.In("duplicate_id", ids),
			builder.Eq{"status": BookmarkDuplicateStatusOpen},
		)).
		Find(&rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return groups, nil
	}

	// Union-find over the pairs.
	parent := map[int64]int64{}
	var find func(int64) int64
	find = func(x int64) int64 {
		if p, ok := parent[x]; !ok || p == x {
			parent[x] = x
			return x
		}
		root := find(parent[x])
		parent[x] = root
		return root
	}
	for _, r := range rows {
		parent[find(r.BookmarkID)] = find(r.DuplicateID)
	}
	members := map[int64][]int64{}
	for id := range parent {
		root := find(id)
		members[root] = append(members[root], id)
	}
	pairsByRoot := map[int64][]BookmarkDuplicatePair{}
	for _, r := range rows {
		root := find(r.BookmarkID)
		pairsByRoot[root] = append(pairsByRoot[root], BookmarkDuplicatePair{BookmarkID: r.BookmarkID, DuplicateID: r.DuplicateID})
	}

	roots := make([]int64, 0, len(members))
	for root := range members {
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i, j int) bool {
		li, lj := len(members[roots[i]]), len(members[roots[j]])
		if li != lj {
			return li > lj
		}
		return roots[i] < roots[j]
	})
	if len(roots) > duplicateGroupsLimit {
		roots = roots[:duplicateGroupsLimit]
	}

	wanted := []int64{}
	for _, root := range roots {
		wanted = append(wanted, members[root]...)
	}
	loaded := []*Bookmark{}
	if err := s.In("id", wanted).Find(&loaded); err != nil {
		return nil, err
	}
	if err := addBookmarkDetails(s, loaded); err != nil {
		return nil, err
	}
	byID := map[int64]*Bookmark{}
	for _, b := range loaded {
		byID[b.ID] = b
	}

	for _, root := range roots {
		group := &BookmarkDuplicateGroup{Bookmarks: []*Bookmark{}, Pairs: pairsByRoot[root]}
		for _, id := range members[root] {
			if b, ok := byID[id]; ok {
				group.Bookmarks = append(group.Bookmarks, b)
			}
		}
		sort.Slice(group.Bookmarks, func(i, j int) bool { return group.Bookmarks[i].ID < group.Bookmarks[j].ID })
		if len(group.Bookmarks) >= 2 {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// BookmarkDuplicateScanEvent asks for every bookmark of a user to be checked for duplicates again.
type BookmarkDuplicateScanEvent struct {
	UserID int64 `json:"user_id"`
}

// Name defines the name for BookmarkDuplicateScanEvent
func (*BookmarkDuplicateScanEvent) Name() string {
	return "bookmark.duplicates.scan"
}

// RequestBookmarkDuplicateScan queues a background check of all your bookmarks, using only the
// free local comparison. Call events.DispatchPending after committing.
func RequestBookmarkDuplicateScan(s *xorm.Session, a web.Auth) error {
	u, ok := a.(*user.User)
	if !ok {
		return ErrGenericForbidden{}
	}
	events.DispatchOnCommit(s, &BookmarkDuplicateScanEvent{UserID: u.ID})
	return nil
}

// ScanBookmarkDuplicates re-checks all of a user's bookmarks for duplicates.
type ScanBookmarkDuplicates struct{}

// Name defines the name for the ScanBookmarkDuplicates listener
func (*ScanBookmarkDuplicates) Name() string {
	return "bookmark.duplicates.scan.process"
}

// Handle never returns an error: the event system would retry the whole scan.
func (*ScanBookmarkDuplicates) Handle(msg *message.Message) error {
	event := &BookmarkDuplicateScanEvent{}
	if err := json.Unmarshal(msg.Payload, event); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	ids := []int64{}
	s := db.NewSession()
	err := s.Table("bookmarks").Where("created_by_id = ?", event.UserID).OrderBy("id").Cols("id").Find(&ids)
	s.Close()
	if err != nil {
		log.Errorf("Bookmark duplicate scan: could not list bookmarks of user %d: %s", event.UserID, err)
		return nil
	}

	for _, id := range ids {
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
			log.Errorf("Bookmark duplicate scan: check failed for bookmark %d: %s", id, err)
		}
	}
	return nil
}
