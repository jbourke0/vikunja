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
	"sync"
	"time"

	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"

	"xorm.io/builder"
)

// Johnny's Brain weekly broken-link check.

const (
	bookmarkLinkCheckSchedule = "0 3 * * 0" // Sundays, 3am server time
	bookmarkLinkCheckLimit    = 1000        // bookmarks per run, oldest check first
	bookmarkLinkCheckMinAge   = 6 * 24 * time.Hour
)

// brokenLinkCond matches bookmarks whose last check found the page gone or failing: not found,
// gone, a server error, or no response at all. Logins, bot blocks and rate limits (401, 403,
// 429) are not broken links, so they are left out.
func brokenLinkCond() builder.Cond {
	return builder.Or(
		builder.In("bookmarks.link_status", 404, 410, BookmarkLinkStatusUnreachable),
		builder.Gte{"bookmarks.link_status": 500},
	)
}

// refreshBookmarksInBackground fetches the pages of the given bookmarks with a few at a time,
// pausing between starts so a long list does not hammer anyone. Failures are logged, not returned.
func refreshBookmarksInBackground(ctx context.Context, fetcher BookmarkPageFetcher, ids []int64) {
	slots := make(chan struct{}, bookmarkImportConcurrency)
	var wg sync.WaitGroup
	for _, id := range ids {
		if ctx.Err() != nil { // select picks at random when both are ready, so look first
			wg.Wait()
			return
		}
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case slots <- struct{}{}:
		}
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			defer func() { <-slots }()
			if err := RefreshBookmarkFromPage(ctx, fetcher, id); err != nil {
				log.Errorf("Bookmark page fetch: could not update bookmark %d: %s", id, err)
			}
		}(id)
		select {
		case <-ctx.Done():
		case <-time.After(150 * time.Millisecond):
		}
	}
	wg.Wait()
}

// dueForLinkCheck lists the bookmarks that have gone longest without a check, never ones
// checked in the last few days or archived ones. Its session is closed before any fetching, so
// no transaction stays open for the hours a long run can take.
func dueForLinkCheck(now time.Time) ([]int64, error) {
	s := db.NewSession()
	defer s.Close()

	due := []*Bookmark{}
	err := s.Cols("id").
		Where(builder.And(
			builder.Neq{"status": BookmarkStatusArchived},
			builder.Or(builder.IsNull{"checked_at"}, builder.Lt{"checked_at": now.Add(-bookmarkLinkCheckMinAge)}),
		)).
		OrderBy("CASE WHEN checked_at IS NULL THEN 0 ELSE 1 END, checked_at ASC, id ASC").
		Limit(bookmarkLinkCheckLimit).
		Find(&due)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(due))
	for _, b := range due {
		ids = append(ids, b.ID)
	}
	return ids, nil
}

// RunBookmarkLinkCheck re-checks the bookmarks that are due and returns how many it checked.
func RunBookmarkLinkCheck(ctx context.Context, fetcher BookmarkPageFetcher, now time.Time) (checked int, err error) {
	ids, err := dueForLinkCheck(now)
	if err != nil {
		return 0, err
	}
	refreshBookmarksInBackground(ctx, fetcher, ids)
	return len(ids), nil
}

func runBookmarkLinkCheckJob() {
	fetcher := currentBookmarkPageFetcher()
	if fetcher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	checked, err := RunBookmarkLinkCheck(ctx, fetcher, time.Now())
	if err != nil {
		log.Errorf("Bookmark link check failed: %s", err)
		return
	}

	s := db.NewSession()
	defer s.Close()
	broken, err := s.Where(brokenLinkCond()).Count(&Bookmark{})
	if err != nil {
		log.Errorf("Bookmark link check: could not count broken links: %s", err)
		return
	}
	log.Infof("Bookmark link check: checked %d bookmarks, %d now have a broken link", checked, broken)
}

// RegisterBookmarkLinkCheckCron schedules the weekly broken-link check.
func RegisterBookmarkLinkCheckCron() {
	if err := cron.Schedule(bookmarkLinkCheckSchedule, runBookmarkLinkCheckJob); err != nil {
		log.Errorf("Could not register bookmark link check cron: %s", err)
	}
}
