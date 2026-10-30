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
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Johnny's Brain duplicate detection.
//
// Saving the same page twice is refused outright (see ensureURLNotSavedYet). Bookmarks about
// the same thing behind different links are found after each save: a cheap local score picks
// a handful of candidates from the owner's bookmarks, then an AI judge (when configured)
// decides which are really the same product, story or information. Matches become
// suggestions the owner can merge or dismiss; nothing is merged or deleted automatically.

// Duplicate suggestion reasons and statuses.
const (
	BookmarkDuplicateReasonSimilar = "similar"
	BookmarkDuplicateReasonAI      = "ai"

	BookmarkDuplicateStatusOpen      = "open"
	BookmarkDuplicateStatusDismissed = "dismissed"
)

const (
	duplicateCandidateLimit = 8
	duplicateCandidateFloor = 0.12
	duplicateLocalThreshold = 0.55
	duplicateAIThreshold    = 0.6
	duplicateMaxCompared    = 2000
	duplicateSnapshotChars  = 2000
)

// BookmarkDuplicate is a suggested pair of bookmarks about the same thing. The lower id is
// always stored in BookmarkID so each pair has one row.
type BookmarkDuplicate struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	BookmarkID  int64     `xorm:"bigint not null INDEX unique(bookmark_duplicate)" json:"-"`
	DuplicateID int64     `xorm:"bigint not null INDEX unique(bookmark_duplicate)" json:"-"`
	Score       float64   `xorm:"double not null default 0" json:"-"`
	Reason      string    `xorm:"varchar(20) not null" json:"-"`
	Explanation string    `xorm:"text null" json:"-"`
	Status      string    `xorm:"varchar(20) not null default 'open' INDEX" json:"-"`
	DetectedAt  time.Time `xorm:"datetime not null" json:"-"`
}

// TableName returns the table name for bookmark duplicate suggestions.
func (*BookmarkDuplicate) TableName() string {
	return "bookmark_duplicates"
}

func orderedPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

// BookmarkDuplicateSuggestion is one possible duplicate of a bookmark, as returned by the API.
type BookmarkDuplicateSuggestion struct {
	Bookmark    *Bookmark `json:"bookmark" doc:"The other bookmark that looks like a duplicate."`
	Score       float64   `json:"score" doc:"How sure the check is, from 0 to 1."`
	Reason      string    `json:"reason" enum:"similar,ai" doc:"How it was found: similar (local comparison of link, title and description) or ai (judged by the AI model)."`
	Explanation string    `json:"explanation" doc:"A short note on why the two look like the same thing. May be empty."`
	DetectedAt  time.Time `json:"detected_at" doc:"When the suggestion was made."`
}

// ---------------------------------------------------------------------------
// AI judge
// ---------------------------------------------------------------------------

// BookmarkForDuplicateCheck is what a judge sees of a bookmark.
type BookmarkForDuplicateCheck struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	SiteName    string `json:"site_name,omitempty"`
	Description string `json:"description,omitempty"`
	PageText    string `json:"page_text,omitempty"`
}

// BookmarkDuplicateMatch is a judge's verdict that a candidate is the same thing as the subject.
type BookmarkDuplicateMatch struct {
	ID         int64
	Confidence float64
	Reason     string
}

// BookmarkDuplicateJudge decides which candidates are about the same thing as the subject.
// It returns only the matches; candidates it leaves out are not duplicates.
type BookmarkDuplicateJudge interface {
	FindDuplicates(ctx context.Context, subject BookmarkForDuplicateCheck, candidates []BookmarkForDuplicateCheck) ([]BookmarkDuplicateMatch, error)
}

var (
	duplicateJudgeMu sync.RWMutex
	duplicateJudge   BookmarkDuplicateJudge
)

// SetBookmarkDuplicateJudge installs the AI judge. Pass nil to fall back to local matching only.
func SetBookmarkDuplicateJudge(j BookmarkDuplicateJudge) {
	duplicateJudgeMu.Lock()
	defer duplicateJudgeMu.Unlock()
	duplicateJudge = j
}

func currentBookmarkDuplicateJudge() BookmarkDuplicateJudge {
	duplicateJudgeMu.RLock()
	defer duplicateJudgeMu.RUnlock()
	return duplicateJudge
}

// ---------------------------------------------------------------------------
// Detection
// ---------------------------------------------------------------------------

// BookmarkSavedEvent fires after a bookmark is created, or edited in a way that changes what
// duplicate detection compares.
type BookmarkSavedEvent struct {
	BookmarkID int64 `json:"bookmark_id"`
	// FetchPage asks the listener to fetch the page first: set when the link is new or changed,
	// or when the owner asks for a refresh. Edits to the title or description only re-check duplicates.
	FetchPage bool `json:"fetch_page"`
}

// Name defines the name for BookmarkSavedEvent
func (*BookmarkSavedEvent) Name() string {
	return "bookmark.saved"
}

type duplicateCandidate struct {
	bookmark *Bookmark
	score    float64
}

// DetectDuplicatesForBookmark recomputes the open duplicate suggestions involving a bookmark.
// Dismissed pairs are never suggested again.
func DetectDuplicatesForBookmark(ctx context.Context, s *xorm.Session, bookmarkID int64) error {
	return detectDuplicates(ctx, s, bookmarkID, currentBookmarkDuplicateJudge())
}

// detectDuplicates is DetectDuplicatesForBookmark with the judge chosen by the caller. Bulk
// imports pass nil, so thousands of new bookmarks are compared locally and never sent to the AI.
func detectDuplicates(ctx context.Context, s *xorm.Session, bookmarkID int64, judge BookmarkDuplicateJudge) error {
	subject, err := getBookmarkByIDSimple(s, bookmarkID)
	if err != nil {
		if IsErrBookmarkDoesNotExist(err) {
			return nil // deleted before the check ran
		}
		return err
	}

	others := []*Bookmark{}
	err = s.
		Where("created_by_id = ? AND id != ?", subject.CreatedByID, subject.ID).
		OrderBy("created DESC").
		Limit(duplicateMaxCompared).
		Find(&others)
	if err != nil {
		return err
	}

	dismissed, err := dismissedPartners(s, subject.ID)
	if err != nil {
		return err
	}

	// Links that carry the same note are the same thing from different pages, unless the note is
	// shared by so many that it is boilerplate.
	subjectNote := normalisedNote(subject)
	sameNote := map[int64]bool{}
	if subjectNote != "" {
		for _, other := range others {
			if normalisedNote(other) == subjectNote {
				sameNote[other.ID] = true
			}
		}
		if len(sameNote)+1 > duplicateSameNoteMaxGroup {
			sameNote = map[int64]bool{}
		}
	}

	candidates := make([]duplicateCandidate, 0, len(others))
	for _, other := range others {
		if dismissed[other.ID] {
			continue
		}
		score := localDuplicateScore(subject, other)
		if sameNote[other.ID] && score < duplicateSameNoteScore {
			score = duplicateSameNoteScore
		}
		if score >= duplicateCandidateFloor {
			candidates = append(candidates, duplicateCandidate{bookmark: other, score: score})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > duplicateCandidateLimit {
		candidates = candidates[:duplicateCandidateLimit]
	}

	matches := []BookmarkDuplicateMatch{}
	reason := BookmarkDuplicateReasonSimilar
	judged := false
	if judge != nil && len(candidates) > 0 {
		subjectView, candidateViews, err := duplicateCheckViews(s, subject, candidates)
		if err != nil {
			return err
		}
		verdicts, err := judge.FindDuplicates(ctx, subjectView, candidateViews)
		if err != nil {
			// The AI is a bonus; fall back to the local score rather than failing the check.
			log.Errorf("Bookmark duplicate check: AI judge failed for bookmark %d, using local matching: %s", subject.ID, err)
		} else {
			judged = true
			reason = BookmarkDuplicateReasonAI
			allowed := map[int64]bool{}
			for _, c := range candidates {
				allowed[c.bookmark.ID] = true
			}
			for _, v := range verdicts {
				// Only accept ids we actually offered, never one the model made up.
				if allowed[v.ID] && v.ID != subject.ID && v.Confidence >= duplicateAIThreshold {
					matches = append(matches, v)
				}
			}
		}
	}
	if !judged {
		for _, c := range candidates {
			if c.score >= duplicateLocalThreshold {
				matches = append(matches, BookmarkDuplicateMatch{ID: c.bookmark.ID, Confidence: c.score})
			}
		}
	}

	// Replace this bookmark's open suggestions; dismissed ones stay as they are.
	_, err = s.
		Where(builder.And(
			builder.Or(builder.Eq{"bookmark_id": subject.ID}, builder.Eq{"duplicate_id": subject.ID}),
			builder.Eq{"status": BookmarkDuplicateStatusOpen},
		)).
		Delete(&BookmarkDuplicate{})
	if err != nil {
		return err
	}

	now := time.Now()
	for _, m := range matches {
		lo, hi := orderedPair(subject.ID, m.ID)
		if _, err := s.Insert(&BookmarkDuplicate{
			BookmarkID:  lo,
			DuplicateID: hi,
			Score:       m.Confidence,
			Reason:      reason,
			Explanation: truncateRunes(strings.TrimSpace(m.Reason), 500),
			Status:      BookmarkDuplicateStatusOpen,
			DetectedAt:  now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func dismissedPartners(s *xorm.Session, bookmarkID int64) (map[int64]bool, error) {
	rows := []*BookmarkDuplicate{}
	err := s.
		Where(builder.And(
			builder.Or(builder.Eq{"bookmark_id": bookmarkID}, builder.Eq{"duplicate_id": bookmarkID}),
			builder.Eq{"status": BookmarkDuplicateStatusDismissed},
		)).
		Find(&rows)
	if err != nil {
		return nil, err
	}
	partners := map[int64]bool{}
	for _, r := range rows {
		if r.BookmarkID == bookmarkID {
			partners[r.DuplicateID] = true
		} else {
			partners[r.BookmarkID] = true
		}
	}
	return partners, nil
}

func duplicateCheckViews(s *xorm.Session, subject *Bookmark, candidates []duplicateCandidate) (BookmarkForDuplicateCheck, []BookmarkForDuplicateCheck, error) {
	ids := []int64{subject.ID}
	for _, c := range candidates {
		ids = append(ids, c.bookmark.ID)
	}
	snapshots := []*BookmarkSnapshot{}
	if err := s.In("bookmark_id", ids).Find(&snapshots); err != nil {
		return BookmarkForDuplicateCheck{}, nil, err
	}
	pageText := map[int64]string{}
	for _, snap := range snapshots {
		pageText[snap.BookmarkID] = truncateRunes(snap.Text, duplicateSnapshotChars)
	}

	view := func(b *Bookmark) BookmarkForDuplicateCheck {
		return BookmarkForDuplicateCheck{
			ID:          b.ID,
			URL:         b.URL,
			Title:       b.Title,
			SiteName:    b.SiteName,
			Description: truncateRunes(b.Description, 1000),
			PageText:    pageText[b.ID],
		}
	}
	views := make([]BookmarkForDuplicateCheck, 0, len(candidates))
	for _, c := range candidates {
		views = append(views, view(c.bookmark))
	}
	return view(subject), views, nil
}

func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

// ---------------------------------------------------------------------------
// Local similarity
// ---------------------------------------------------------------------------

var duplicateStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "you": true, "your": true, "are": true,
	"this": true, "that": true, "from": true, "how": true, "what": true, "why": true, "when": true,
	"www": true, "com": true, "net": true, "org": true, "http": true, "https": true, "html": true,
	"index": true, "page": true, "home": true, "official": true, "site": true, "website": true,
}

func duplicateTokens(text string) map[string]bool {
	tokens := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(word)) < 3 || duplicateStopWords[word] {
			continue
		}
		tokens[stemWord(word)] = true
	}
	return tokens
}

// stemWord strips common English endings so "approves", "approved" and "approving" match.
func stemWord(word string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if strings.HasSuffix(word, suffix) && len([]rune(word))-len(suffix) >= 4 {
			return strings.TrimSuffix(word, suffix)
		}
	}
	return word
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	shared := 0
	for t := range a {
		if b[t] {
			shared++
		}
	}
	return float64(shared) / float64(len(a)+len(b)-shared)
}

// hostAndPath splits a normalised URL into its host and its path without the query.
func hostAndPath(normalized string) (string, string) {
	withoutQuery, _, _ := strings.Cut(normalized, "?")
	host, path, _ := strings.Cut(withoutQuery, "/")
	return host, path
}

// localDuplicateScore rates how likely two bookmarks are the same thing, from 0 to 1,
// using only the link, title, site name and description.
func localDuplicateScore(a, b *Bookmark) float64 {
	hostA, pathA := hostAndPath(NormalizeBookmarkURL(a.URL))
	hostB, pathB := hostAndPath(NormalizeBookmarkURL(b.URL))
	if hostA == hostB && pathA == pathB && pathA != "" {
		// Same page with different query parameters.
		return 0.9
	}

	titleA := duplicateTokens(a.Title + " " + titleFromPath(pathA))
	titleB := duplicateTokens(b.Title + " " + titleFromPath(pathB))
	titleScore := jaccard(titleA, titleB)

	sameHost := 0.0
	if hostA == hostB {
		sameHost = 1
	}

	descA, descB := duplicateTokens(a.Description), duplicateTokens(b.Description)
	if len(descA) == 0 || len(descB) == 0 {
		return 0.8*titleScore + 0.2*sameHost*titleScore
	}
	return 0.6*titleScore + 0.25*jaccard(descA, descB) + 0.15*sameHost*titleScore
}

// titleFromPath turns the last path segment into words, so /blog/kids-martial-arts-geelong
// still says something when the title is just the site name.
func titleFromPath(path string) string {
	segments := strings.Split(path, "/")
	last, err := url.PathUnescape(segments[len(segments)-1])
	if err != nil {
		last = segments[len(segments)-1]
	}
	return strings.NewReplacer("-", " ", "_", " ", "+", " ").Replace(last)
}

// ---------------------------------------------------------------------------
// Reading, dismissing and merging
// ---------------------------------------------------------------------------

func openDuplicateCounts(s *xorm.Session, ids []int64) (map[int64]int, error) {
	counts := map[int64]int{}
	if len(ids) == 0 {
		return counts, nil
	}
	rows := []*BookmarkDuplicate{}
	err := s.
		Where(builder.And(
			builder.Or(builder.In("bookmark_id", ids), builder.In("duplicate_id", ids)),
			builder.Eq{"status": BookmarkDuplicateStatusOpen},
		)).
		Find(&rows)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.BookmarkID]++
		counts[r.DuplicateID]++
	}
	return counts, nil
}

// GetBookmarkDuplicateSuggestions lists the open possible duplicates of a bookmark that the
// caller can see, most likely first.
func GetBookmarkDuplicateSuggestions(s *xorm.Session, a web.Auth, bookmarkID int64) ([]*BookmarkDuplicateSuggestion, error) {
	canRead, _, err := (&Bookmark{ID: bookmarkID}).CanRead(s, a)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ErrGenericForbidden{}
	}

	rows := []*BookmarkDuplicate{}
	err = s.
		Where(builder.And(
			builder.Or(builder.Eq{"bookmark_id": bookmarkID}, builder.Eq{"duplicate_id": bookmarkID}),
			builder.Eq{"status": BookmarkDuplicateStatusOpen},
		)).
		OrderBy("score DESC, id ASC").
		Find(&rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*BookmarkDuplicateSuggestion{}, nil
	}

	otherIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.BookmarkID == bookmarkID {
			otherIDs = append(otherIDs, r.DuplicateID)
		} else {
			otherIDs = append(otherIDs, r.BookmarkID)
		}
	}

	visible, err := bookmarkVisibleCond(s, a)
	if err != nil {
		return nil, err
	}
	others := []*Bookmark{}
	if err := s.Where(builder.And(builder.In("bookmarks.id", otherIDs), visible)).Find(&others); err != nil {
		return nil, err
	}
	if err := addBookmarkDetails(s, others); err != nil {
		return nil, err
	}
	byID := map[int64]*Bookmark{}
	for _, o := range others {
		byID[o.ID] = o
	}

	suggestions := make([]*BookmarkDuplicateSuggestion, 0, len(rows))
	for i, r := range rows {
		other, ok := byID[otherIDs[i]]
		if !ok {
			continue
		}
		suggestions = append(suggestions, &BookmarkDuplicateSuggestion{
			Bookmark:    other,
			Score:       r.Score,
			Reason:      r.Reason,
			Explanation: r.Explanation,
			DetectedAt:  r.DetectedAt,
		})
	}
	return suggestions, nil
}

// DismissBookmarkDuplicate marks two bookmarks as not duplicates, so the pair is never
// suggested again. Only the bookmark's owner may do this.
func DismissBookmarkDuplicate(s *xorm.Session, a web.Auth, bookmarkID, otherID int64) error {
	owns, err := (&Bookmark{ID: bookmarkID}).CanUpdate(s, a)
	if err != nil {
		return err
	}
	if !owns {
		return ErrGenericForbidden{}
	}

	lo, hi := orderedPair(bookmarkID, otherID)
	updated, err := s.
		Where("bookmark_id = ? AND duplicate_id = ?", lo, hi).
		Cols("status").
		Update(&BookmarkDuplicate{Status: BookmarkDuplicateStatusDismissed})
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrBookmarkDuplicateNotFound{BookmarkID: bookmarkID, OtherID: otherID}
	}
	return nil
}

// MergeBookmarks folds source into target: labels, comments, source links and the page
// snapshot move across, empty fields on target are filled from source, a comment records the
// merged link, and source is deleted. The caller must own both.
func MergeBookmarks(s *xorm.Session, a web.Auth, targetID, sourceID int64) (*Bookmark, error) {
	if targetID == sourceID {
		return nil, ErrBookmarkMergeIntoItself{BookmarkID: targetID}
	}
	for _, id := range []int64{targetID, sourceID} {
		owns, err := (&Bookmark{ID: id}).CanUpdate(s, a)
		if err != nil {
			return nil, err
		}
		if !owns {
			return nil, ErrGenericForbidden{}
		}
	}

	target, err := getBookmarkByIDSimple(s, targetID)
	if err != nil {
		return nil, err
	}
	source, err := getBookmarkByIDSimple(s, sourceID)
	if err != nil {
		return nil, err
	}

	if err := moveBookmarkLinks(s, "label_bookmarks", "label_id", sourceID, targetID); err != nil {
		return nil, err
	}
	if err := moveBookmarkLinks(s, "bookmark_projects", "project_id", sourceID, targetID); err != nil {
		return nil, err
	}
	if err := moveBookmarkLinks(s, "bookmark_tasks", "task_id", sourceID, targetID); err != nil {
		return nil, err
	}

	if _, err := s.Exec("UPDATE bookmark_comments SET bookmark_id = ? WHERE bookmark_id = ?", targetID, sourceID); err != nil {
		return nil, err
	}

	targetHasSnapshot, err := s.Where("bookmark_id = ?", targetID).Exist(&BookmarkSnapshot{})
	if err != nil {
		return nil, err
	}
	if targetHasSnapshot {
		_, err = s.Where("bookmark_id = ?", sourceID).Delete(&BookmarkSnapshot{})
	} else {
		_, err = s.Exec("UPDATE bookmark_snapshots SET bookmark_id = ? WHERE bookmark_id = ?", targetID, sourceID)
	}
	if err != nil {
		return nil, err
	}

	fill := func(dst *string, src string) {
		if strings.TrimSpace(*dst) == "" {
			*dst = src
		}
	}
	fill(&target.Description, source.Description)
	fill(&target.FaviconURL, source.FaviconURL)
	fill(&target.ImageURL, source.ImageURL)
	fill(&target.SiteName, source.SiteName)
	if _, err := s.ID(targetID).Cols("description", "favicon_url", "image_url", "site_name").Update(target); err != nil {
		return nil, err
	}

	if source.URLHash != target.URLHash {
		note := &BookmarkComment{
			BookmarkID: targetID,
			AuthorID:   a.GetID(),
			Comment: fmt.Sprintf(`<p>Merged in "%s": <a href="%s">%s</a></p>`,
				escapeHTML(source.Title), escapeHTML(source.URL), escapeHTML(source.URL)),
		}
		if _, err := s.Insert(note); err != nil {
			return nil, err
		}
	}

	if _, err := s.Where("bookmark_id = ? OR duplicate_id = ?", sourceID, sourceID).Delete(&BookmarkDuplicate{}); err != nil {
		return nil, err
	}
	if _, err := s.ID(sourceID).Delete(&Bookmark{}); err != nil {
		return nil, err
	}

	merged := &Bookmark{ID: targetID}
	if err := merged.ReadOne(s, a); err != nil {
		return nil, err
	}
	return merged, nil
}

// moveBookmarkLinks re-points a join table's rows from one bookmark to another, dropping the
// ones the target already has so the unique index holds.
func moveBookmarkLinks(s *xorm.Session, table, otherColumn string, fromID, toID int64) error {
	_, err := s.Exec(
		"DELETE FROM "+table+" WHERE bookmark_id = ? AND "+otherColumn+" IN (SELECT "+otherColumn+" FROM (SELECT "+otherColumn+" FROM "+table+" WHERE bookmark_id = ?) AS existing)",
		fromID, toID,
	)
	if err != nil {
		return err
	}
	_, err = s.Exec("UPDATE "+table+" SET bookmark_id = ? WHERE bookmark_id = ?", toID, fromID)
	return err
}

func escapeHTML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;").Replace(s)
}
