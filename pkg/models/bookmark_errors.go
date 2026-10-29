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
	"errors"
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/web"
)

// Johnny's Brain error codes live in the 90000 range to stay clear of upstream codes.

// ErrBookmarkDoesNotExist represents an error where a bookmark does not exist.
type ErrBookmarkDoesNotExist struct {
	BookmarkID int64
}

// IsErrBookmarkDoesNotExist checks if an error is ErrBookmarkDoesNotExist.
func IsErrBookmarkDoesNotExist(err error) bool {
	var target ErrBookmarkDoesNotExist
	return errors.As(err, &target)
}

func (err ErrBookmarkDoesNotExist) Error() string {
	return fmt.Sprintf("Bookmark does not exist [BookmarkID: %v]", err.BookmarkID)
}

// ErrCodeBookmarkDoesNotExist holds the unique world-error code of this error
const ErrCodeBookmarkDoesNotExist = 90001

// HTTPError holds the http error description
func (err ErrBookmarkDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeBookmarkDoesNotExist,
		Message:  "This bookmark does not exist.",
	}
}

// ErrBookmarkInvalidURL represents an error where a bookmark URL is not an absolute http(s) URL.
type ErrBookmarkInvalidURL struct {
	URL string
}

// IsErrBookmarkInvalidURL checks if an error is ErrBookmarkInvalidURL.
func IsErrBookmarkInvalidURL(err error) bool {
	var target ErrBookmarkInvalidURL
	return errors.As(err, &target)
}

func (err ErrBookmarkInvalidURL) Error() string {
	return fmt.Sprintf("Bookmark URL is invalid [URL: %q]", err.URL)
}

// ErrCodeBookmarkInvalidURL holds the unique world-error code of this error
const ErrCodeBookmarkInvalidURL = 90002

// HTTPError holds the http error description
func (err ErrBookmarkInvalidURL) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeBookmarkInvalidURL,
		Message:  "The bookmark URL must be a full http or https address.",
	}
}

// ErrBookmarkInvalidStatus represents an error where a bookmark status is unknown.
type ErrBookmarkInvalidStatus struct {
	Status string
}

// IsErrBookmarkInvalidStatus checks if an error is ErrBookmarkInvalidStatus.
func IsErrBookmarkInvalidStatus(err error) bool {
	var target ErrBookmarkInvalidStatus
	return errors.As(err, &target)
}

func (err ErrBookmarkInvalidStatus) Error() string {
	return fmt.Sprintf("Bookmark status is invalid [Status: %q]", err.Status)
}

// ErrCodeBookmarkInvalidStatus holds the unique world-error code of this error
const ErrCodeBookmarkInvalidStatus = 90003

// HTTPError holds the http error description
func (err ErrBookmarkInvalidStatus) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeBookmarkInvalidStatus,
		Message:  "The bookmark status must be to_read, reference or archived.",
	}
}

// ErrBookmarkCommentDoesNotExist represents an error where a bookmark comment does not exist.
type ErrBookmarkCommentDoesNotExist struct {
	CommentID  int64
	BookmarkID int64
}

// IsErrBookmarkCommentDoesNotExist checks if an error is ErrBookmarkCommentDoesNotExist.
func IsErrBookmarkCommentDoesNotExist(err error) bool {
	var target ErrBookmarkCommentDoesNotExist
	return errors.As(err, &target)
}

func (err ErrBookmarkCommentDoesNotExist) Error() string {
	return fmt.Sprintf("Bookmark comment does not exist [CommentID: %v, BookmarkID: %v]", err.CommentID, err.BookmarkID)
}

// ErrCodeBookmarkCommentDoesNotExist holds the unique world-error code of this error
const ErrCodeBookmarkCommentDoesNotExist = 90004

// HTTPError holds the http error description
func (err ErrBookmarkCommentDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeBookmarkCommentDoesNotExist,
		Message:  "This bookmark comment does not exist.",
	}
}

// ErrBookmarkCommentEmpty represents an error where a bookmark comment has no text.
type ErrBookmarkCommentEmpty struct{}

// IsErrBookmarkCommentEmpty checks if an error is ErrBookmarkCommentEmpty.
func IsErrBookmarkCommentEmpty(err error) bool {
	var target ErrBookmarkCommentEmpty
	return errors.As(err, &target)
}

func (err ErrBookmarkCommentEmpty) Error() string {
	return "Bookmark comment is empty"
}

// ErrCodeBookmarkCommentEmpty holds the unique world-error code of this error
const ErrCodeBookmarkCommentEmpty = 90005

// HTTPError holds the http error description
func (err ErrBookmarkCommentEmpty) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeBookmarkCommentEmpty,
		Message:  "A comment needs some text.",
	}
}

// ErrLabelIsAlreadyOnBookmark represents an error where a label is already on a bookmark.
type ErrLabelIsAlreadyOnBookmark struct {
	LabelID    int64
	BookmarkID int64
}

// IsErrLabelIsAlreadyOnBookmark checks if an error is ErrLabelIsAlreadyOnBookmark.
func IsErrLabelIsAlreadyOnBookmark(err error) bool {
	var target ErrLabelIsAlreadyOnBookmark
	return errors.As(err, &target)
}

func (err ErrLabelIsAlreadyOnBookmark) Error() string {
	return fmt.Sprintf("Label already exists on bookmark [LabelID: %v, BookmarkID: %v]", err.LabelID, err.BookmarkID)
}

// ErrCodeLabelIsAlreadyOnBookmark holds the unique world-error code of this error
const ErrCodeLabelIsAlreadyOnBookmark = 90006

// HTTPError holds the http error description
func (err ErrLabelIsAlreadyOnBookmark) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusConflict,
		Code:     ErrCodeLabelIsAlreadyOnBookmark,
		Message:  "This label is already on the bookmark.",
	}
}

// ErrLabelIsNotOnBookmark represents an error where a label to remove is not on the bookmark.
type ErrLabelIsNotOnBookmark struct {
	LabelID    int64
	BookmarkID int64
}

// IsErrLabelIsNotOnBookmark checks if an error is ErrLabelIsNotOnBookmark.
func IsErrLabelIsNotOnBookmark(err error) bool {
	var target ErrLabelIsNotOnBookmark
	return errors.As(err, &target)
}

func (err ErrLabelIsNotOnBookmark) Error() string {
	return fmt.Sprintf("Label is not on bookmark [LabelID: %v, BookmarkID: %v]", err.LabelID, err.BookmarkID)
}

// ErrCodeLabelIsNotOnBookmark holds the unique world-error code of this error
const ErrCodeLabelIsNotOnBookmark = 90007

// HTTPError holds the http error description
func (err ErrLabelIsNotOnBookmark) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeLabelIsNotOnBookmark,
		Message:  "This label is not on the bookmark.",
	}
}

// ErrBookmarkAlreadySaved represents an error where the same page is saved twice by the same owner.
type ErrBookmarkAlreadySaved struct {
	ExistingID int64
}

// IsErrBookmarkAlreadySaved checks if an error is ErrBookmarkAlreadySaved.
func IsErrBookmarkAlreadySaved(err error) bool {
	var target ErrBookmarkAlreadySaved
	return errors.As(err, &target)
}

func (err ErrBookmarkAlreadySaved) Error() string {
	return fmt.Sprintf("Bookmark already saved [ExistingID: %v]", err.ExistingID)
}

// ErrCodeBookmarkAlreadySaved holds the unique world-error code of this error
const ErrCodeBookmarkAlreadySaved = 90008

// HTTPError holds the http error description
func (err ErrBookmarkAlreadySaved) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusConflict,
		Code:     ErrCodeBookmarkAlreadySaved,
		Message:  fmt.Sprintf("You already saved this link as bookmark %d.", err.ExistingID),
	}
}

// ErrBookmarkSourceAlreadyLinked represents an error where a bookmark is already linked to that project or task.
type ErrBookmarkSourceAlreadyLinked struct {
	BookmarkID int64
}

// IsErrBookmarkSourceAlreadyLinked checks if an error is ErrBookmarkSourceAlreadyLinked.
func IsErrBookmarkSourceAlreadyLinked(err error) bool {
	var target ErrBookmarkSourceAlreadyLinked
	return errors.As(err, &target)
}

func (err ErrBookmarkSourceAlreadyLinked) Error() string {
	return fmt.Sprintf("Bookmark source already linked [BookmarkID: %v]", err.BookmarkID)
}

// ErrCodeBookmarkSourceAlreadyLinked holds the unique world-error code of this error
const ErrCodeBookmarkSourceAlreadyLinked = 90009

// HTTPError holds the http error description
func (err ErrBookmarkSourceAlreadyLinked) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusConflict,
		Code:     ErrCodeBookmarkSourceAlreadyLinked,
		Message:  "This bookmark is already linked there.",
	}
}

// ErrBookmarkSourceNotLinked represents an error where a link to remove does not exist.
type ErrBookmarkSourceNotLinked struct {
	BookmarkID int64
}

// IsErrBookmarkSourceNotLinked checks if an error is ErrBookmarkSourceNotLinked.
func IsErrBookmarkSourceNotLinked(err error) bool {
	var target ErrBookmarkSourceNotLinked
	return errors.As(err, &target)
}

func (err ErrBookmarkSourceNotLinked) Error() string {
	return fmt.Sprintf("Bookmark source not linked [BookmarkID: %v]", err.BookmarkID)
}

// ErrCodeBookmarkSourceNotLinked holds the unique world-error code of this error
const ErrCodeBookmarkSourceNotLinked = 90010

// HTTPError holds the http error description
func (err ErrBookmarkSourceNotLinked) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeBookmarkSourceNotLinked,
		Message:  "This bookmark is not linked there.",
	}
}

// ErrBookmarkDuplicateNotFound represents an error where there is no duplicate suggestion for a pair.
type ErrBookmarkDuplicateNotFound struct {
	BookmarkID int64
	OtherID    int64
}

// IsErrBookmarkDuplicateNotFound checks if an error is ErrBookmarkDuplicateNotFound.
func IsErrBookmarkDuplicateNotFound(err error) bool {
	var target ErrBookmarkDuplicateNotFound
	return errors.As(err, &target)
}

func (err ErrBookmarkDuplicateNotFound) Error() string {
	return fmt.Sprintf("Bookmark duplicate suggestion not found [BookmarkID: %v, OtherID: %v]", err.BookmarkID, err.OtherID)
}

// ErrCodeBookmarkDuplicateNotFound holds the unique world-error code of this error
const ErrCodeBookmarkDuplicateNotFound = 90011

// HTTPError holds the http error description
func (err ErrBookmarkDuplicateNotFound) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeBookmarkDuplicateNotFound,
		Message:  "These bookmarks are not suggested as duplicates.",
	}
}

// ErrBookmarkMergeIntoItself represents an error where a merge names the same bookmark twice.
type ErrBookmarkMergeIntoItself struct {
	BookmarkID int64
}

// IsErrBookmarkMergeIntoItself checks if an error is ErrBookmarkMergeIntoItself.
func IsErrBookmarkMergeIntoItself(err error) bool {
	var target ErrBookmarkMergeIntoItself
	return errors.As(err, &target)
}

func (err ErrBookmarkMergeIntoItself) Error() string {
	return fmt.Sprintf("Cannot merge a bookmark into itself [BookmarkID: %v]", err.BookmarkID)
}

// ErrCodeBookmarkMergeIntoItself holds the unique world-error code of this error
const ErrCodeBookmarkMergeIntoItself = 90012

// HTTPError holds the http error description
func (err ErrBookmarkMergeIntoItself) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeBookmarkMergeIntoItself,
		Message:  "A bookmark cannot be merged into itself.",
	}
}

// ErrBookmarkPreviewsDisabled represents an error where link previews are switched off on this server.
type ErrBookmarkPreviewsDisabled struct{}

// IsErrBookmarkPreviewsDisabled checks if an error is ErrBookmarkPreviewsDisabled.
func IsErrBookmarkPreviewsDisabled(err error) bool {
	var target ErrBookmarkPreviewsDisabled
	return errors.As(err, &target)
}

func (err ErrBookmarkPreviewsDisabled) Error() string {
	return "Bookmark link previews are switched off"
}

// ErrCodeBookmarkPreviewsDisabled holds the unique world-error code of this error
const ErrCodeBookmarkPreviewsDisabled = 90013

// HTTPError holds the http error description
func (err ErrBookmarkPreviewsDisabled) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusConflict,
		Code:     ErrCodeBookmarkPreviewsDisabled,
		Message:  "Link previews are switched off on this server.",
	}
}

// ErrBookmarkImportTooLarge represents an error where an import batch has too many links.
type ErrBookmarkImportTooLarge struct {
	Max int
}

// IsErrBookmarkImportTooLarge checks if an error is ErrBookmarkImportTooLarge.
func IsErrBookmarkImportTooLarge(err error) bool {
	var target ErrBookmarkImportTooLarge
	return errors.As(err, &target)
}

func (err ErrBookmarkImportTooLarge) Error() string {
	return fmt.Sprintf("Bookmark import is too large [Max: %v]", err.Max)
}

// ErrCodeBookmarkImportTooLarge holds the unique world-error code of this error
const ErrCodeBookmarkImportTooLarge = 90014

// HTTPError holds the http error description
func (err ErrBookmarkImportTooLarge) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusBadRequest,
		Code:     ErrCodeBookmarkImportTooLarge,
		Message:  fmt.Sprintf("Send at most %d links per import request.", err.Max),
	}
}

// ErrBookmarkSnapshotDoesNotExist represents an error where a bookmark has no saved page text.
type ErrBookmarkSnapshotDoesNotExist struct {
	BookmarkID int64
}

// IsErrBookmarkSnapshotDoesNotExist checks if an error is ErrBookmarkSnapshotDoesNotExist.
func IsErrBookmarkSnapshotDoesNotExist(err error) bool {
	var target ErrBookmarkSnapshotDoesNotExist
	return errors.As(err, &target)
}

func (err ErrBookmarkSnapshotDoesNotExist) Error() string {
	return fmt.Sprintf("Bookmark snapshot does not exist [BookmarkID: %v]", err.BookmarkID)
}

// ErrCodeBookmarkSnapshotDoesNotExist holds the unique world-error code of this error
const ErrCodeBookmarkSnapshotDoesNotExist = 90015

// HTTPError holds the http error description
func (err ErrBookmarkSnapshotDoesNotExist) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusNotFound,
		Code:     ErrCodeBookmarkSnapshotDoesNotExist,
		Message:  "No page text is saved for this bookmark.",
	}
}
