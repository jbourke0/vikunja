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
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/web"

	"github.com/ThreeDotsLabs/watermill/message"
	"xorm.io/xorm"
)

// Johnny's Brain link previews and page snapshots.
//
// After a bookmark is saved, a background listener fetches the page, fills in the title,
// description, site name, icon and preview image the owner left blank, and stores the page's
// readable text so it can be searched and survives link rot. The fetch itself is done by a
// BookmarkPageFetcher installed at startup (see pkg/modules/johnnysbrain/pagefetch), so models
// never touches the network and tests stay offline.

// Values of Bookmark.LinkStatus besides real HTTP status codes.
const (
	// BookmarkLinkStatusUnchecked means the link has not been checked yet.
	BookmarkLinkStatusUnchecked = 0
	// BookmarkLinkStatusUnreachable means the page could not be reached (DNS, connection or timeout).
	BookmarkLinkStatusUnreachable = -1
	// BookmarkLinkStatusPrivate means the address is on a private or internal network, which the
	// server never fetches.
	BookmarkLinkStatusPrivate = -2
)

const bookmarkSnapshotMaxChars = 200_000

// ErrBookmarkPageBlocked is returned by a fetcher that refused to contact an address because it
// is private, internal or otherwise not allowed.
var ErrBookmarkPageBlocked = errors.New("address is not allowed")

// BookmarkPage is what a fetcher learned about a page.
type BookmarkPage struct {
	// URL is the address the page was finally served from, after redirects.
	URL string
	// StatusCode is the HTTP status of the final response.
	StatusCode  int
	Title       string
	Description string
	SiteName    string
	// FaviconURL and ImageURL are absolute http(s) URLs, or empty.
	FaviconURL string
	ImageURL   string
	// Text is the readable text of the page, empty when it is not an HTML page.
	Text string
}

// BookmarkPageFetcher fetches and summarises a web page. A response with an error status or a
// page that is not HTML is not an error: it comes back with its StatusCode and little else.
// An error means the page could not be fetched at all; ErrBookmarkPageBlocked marks refused
// addresses.
type BookmarkPageFetcher interface {
	FetchPage(ctx context.Context, rawURL string) (*BookmarkPage, error)
}

var (
	pageFetcherMu sync.RWMutex
	pageFetcher   BookmarkPageFetcher
)

// SetBookmarkPageFetcher installs the page fetcher. Pass nil to switch link previews off.
func SetBookmarkPageFetcher(f BookmarkPageFetcher) {
	pageFetcherMu.Lock()
	defer pageFetcherMu.Unlock()
	pageFetcher = f
}

func currentBookmarkPageFetcher() BookmarkPageFetcher {
	pageFetcherMu.RLock()
	defer pageFetcherMu.RUnlock()
	return pageFetcher
}

// defaultBookmarkTitle is the title a bookmark gets when none is given: its host name.
func defaultBookmarkTitle(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// RefreshBookmarkFromPage fetches the bookmark's page and applies the result. Fields the owner
// has filled in are never changed: the title counts as blank while it is still the default
// host name. The link status and check time are always updated; the stored page text is only
// replaced by a newer copy, never removed, so a page that later disappears stays searchable.
//
// No database session is held open during the fetch.
func RefreshBookmarkFromPage(ctx context.Context, fetcher BookmarkPageFetcher, bookmarkID int64) error {
	s := db.NewSession()
	before, err := getBookmarkByIDSimple(s, bookmarkID)
	s.Close()
	if err != nil {
		if IsErrBookmarkDoesNotExist(err) {
			return nil // deleted before the fetch ran
		}
		return err
	}

	page, fetchErr := fetcher.FetchPage(ctx, before.URL)
	linkStatus := BookmarkLinkStatusUnreachable
	switch {
	case errors.Is(fetchErr, ErrBookmarkPageBlocked):
		linkStatus = BookmarkLinkStatusPrivate
		page = nil
	case fetchErr != nil:
		log.Debugf("Bookmark page fetch: bookmark %d: %s", bookmarkID, fetchErr)
		page = nil
	case page != nil:
		linkStatus = page.StatusCode
	}

	return retryWhenSQLiteBusy(ctx, func() error {
		return applyFetchedPage(bookmarkID, before.URLHash, linkStatus, page)
	})
}

// applyFetchedPage writes the outcome of a fetch to the bookmark in one short transaction.
func applyFetchedPage(bookmarkID int64, urlHashFetched string, linkStatus int, page *BookmarkPage) error {
	s := db.NewSession()
	defer s.Close()

	current, err := getBookmarkByIDSimple(s, bookmarkID)
	if err != nil {
		if IsErrBookmarkDoesNotExist(err) {
			return nil
		}
		return err
	}
	if current.URLHash != urlHashFetched {
		return nil // the link was edited meanwhile; the edit queued its own fetch
	}

	now := time.Now()
	current.LinkStatus = linkStatus
	current.CheckedAt = &now
	cols := []string{"link_status", "checked_at"}

	succeeded := page != nil && page.StatusCode >= 200 && page.StatusCode < 300
	if succeeded {
		fill := func(col string, dst *string, value string, blank bool) {
			if blank && strings.TrimSpace(value) != "" {
				*dst = value
				cols = append(cols, col)
			}
		}
		fill("title", &current.Title, truncateRunes(page.Title, 250),
			strings.TrimSpace(current.Title) == "" || current.Title == defaultBookmarkTitle(current.URL))
		fill("description", &current.Description, truncateRunes(page.Description, 2000), strings.TrimSpace(current.Description) == "")
		fill("site_name", &current.SiteName, truncateRunes(page.SiteName, 250), strings.TrimSpace(current.SiteName) == "")
		fill("favicon_url", &current.FaviconURL, page.FaviconURL, strings.TrimSpace(current.FaviconURL) == "")
		fill("image_url", &current.ImageURL, page.ImageURL, strings.TrimSpace(current.ImageURL) == "")
	}

	if _, err := s.ID(current.ID).Cols(cols...).Update(current); err != nil {
		_ = s.Rollback()
		return err
	}

	// Text from an error page is not the page the owner saved, so only a successful fetch
	// replaces the snapshot; a page that has gone keeps its last good copy.
	if succeeded && strings.TrimSpace(page.Text) != "" {
		if err := saveBookmarkSnapshot(s, current.ID, truncateRunes(page.Text, bookmarkSnapshotMaxChars), now); err != nil {
			_ = s.Rollback()
			return err
		}
	}

	return s.Commit()
}

// saveBookmarkSnapshot stores the page text for a bookmark, replacing an older copy.
func saveBookmarkSnapshot(s *xorm.Session, bookmarkID int64, text string, fetchedAt time.Time) error {
	updated, err := s.
		Where("bookmark_id = ?", bookmarkID).
		Cols("text", "fetched_at").
		Update(&BookmarkSnapshot{Text: text, FetchedAt: fetchedAt})
	if err != nil {
		return err
	}
	if updated > 0 {
		return nil
	}
	_, err = s.Insert(&BookmarkSnapshot{BookmarkID: bookmarkID, Text: text, FetchedAt: fetchedAt})
	return err
}

// ProcessSavedBookmark handles BookmarkSavedEvent: fetch the page first when the event asks for
// it, then look for duplicates, so the check sees the real title and page text and runs once.
type ProcessSavedBookmark struct{}

// Name defines the name for the ProcessSavedBookmark listener
func (*ProcessSavedBookmark) Name() string {
	return "bookmark.saved.process"
}

// Handle is executed when a bookmark is saved or a refresh is requested
func (*ProcessSavedBookmark) Handle(msg *message.Message) error {
	event := &BookmarkSavedEvent{}
	if err := json.Unmarshal(msg.Payload, event); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if fetcher := currentBookmarkPageFetcher(); fetcher != nil && event.FetchPage {
		// Previews are a bonus: a failed fetch must not stop the duplicate check.
		if err := RefreshBookmarkFromPage(ctx, fetcher, event.BookmarkID); err != nil {
			log.Errorf("Bookmark page fetch: could not update bookmark %d: %s", event.BookmarkID, err)
		}
	}

	// A lock clash here would make the event system run this whole handler again, fetching the
	// page a second time, so ride it out in place.
	return retryWhenSQLiteBusy(ctx, func() error {
		s := db.NewSession()
		defer s.Close()
		if err := DetectDuplicatesForBookmark(ctx, s, event.BookmarkID); err != nil {
			_ = s.Rollback()
			return err
		}
		return s.Commit()
	})
}

// RequestBookmarkRefresh queues a fresh fetch of the bookmark's page. Only the owner may ask.
func RequestBookmarkRefresh(ctx context.Context, s *xorm.Session, a web.Auth, bookmarkID int64) error {
	owns, err := (&Bookmark{ID: bookmarkID}).CanUpdate(s, a)
	if err != nil {
		return err
	}
	if !owns {
		return ErrGenericForbidden{}
	}
	if currentBookmarkPageFetcher() == nil {
		return ErrBookmarkPreviewsDisabled{}
	}
	return events.DispatchWithContext(ctx, &BookmarkSavedEvent{BookmarkID: bookmarkID, FetchPage: true})
}

// retryWhenSQLiteBusy runs fn again, with a short growing pause, while it fails because SQLite
// is locked. SQLite's busy timeout cannot help a transaction that read first and then tried to
// write after another writer committed, so each attempt must start a fresh transaction. Other
// errors, and running out of attempts, are returned as they are.
func retryWhenSQLiteBusy(ctx context.Context, fn func() error) error {
	delay := 50 * time.Millisecond
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		if err = fn(); err == nil || !isSQLiteBusy(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(delay + time.Duration(time.Now().UnixNano()%int64(delay))):
		}
		delay *= 2
	}
	return err
}

func isSQLiteBusy(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy")
}
