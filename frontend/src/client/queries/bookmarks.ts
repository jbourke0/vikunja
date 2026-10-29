// Johnny's Brain: bookmarks, their labels, comments, sources and duplicates.
import {queryOptions, useMutation} from '@tanstack/vue-query'
import type {QueryClient} from '@tanstack/vue-query'

import {
	bookmarkCommentsCreate,
	bookmarkCommentsDelete,
	bookmarkCommentsList,
	bookmarkDuplicatesDismiss,
	bookmarkDuplicatesList,
	bookmarkLabelsCreate,
	bookmarkLabelsDelete,
	bookmarkProjectsCreate,
	bookmarkProjectsDelete,
	bookmarkTasksCreate,
	bookmarkTasksDelete,
	bookmarksCreate,
	bookmarksDelete,
	bookmarksImport,
	bookmarksList,
	bookmarksMerge,
	bookmarksRead,
	bookmarksRefresh,
	bookmarksSnapshot,
	bookmarksUpdate,
} from '@/client/generated'
import type {
	Bookmark,
	BookmarkComment,
	BookmarkLinkedProject,
	BookmarkLinkedTask,
	BookmarkWritable,
	Label,
} from '@/client/generated'
import {ApiError} from '@/client/problem'
import {contextMutationOptions} from './contextMutation'
import {fetchAllPages} from './fetchAllPages'
import {i18n} from '@/i18n'

export type BookmarkStatus = NonNullable<Bookmark['status']>

export const BOOKMARK_STATUSES: BookmarkStatus[] = ['to_read', 'reference', 'archived']

export type BookmarkResponse = Bookmark & {
	id: number
	url: string
	title: string
	status: BookmarkStatus
	labels: Label[]
	projects: BookmarkLinkedProject[]
	tasks: BookmarkLinkedTask[]
	duplicate_count: number
	link_status: number
	has_snapshot: boolean
}

export function normalizeBookmark(bookmark: Bookmark): BookmarkResponse {
	return {
		...bookmark,
		id: bookmark.id ?? 0,
		url: bookmark.url ?? '',
		title: bookmark.title ?? '',
		status: bookmark.status ?? 'to_read',
		labels: bookmark.labels ?? [],
		projects: bookmark.projects ?? [],
		tasks: bookmark.tasks ?? [],
		duplicate_count: bookmark.duplicate_count ?? 0,
		link_status: bookmark.link_status ?? 0,
		has_snapshot: bookmark.has_snapshot ?? false,
	}
}

export type BookmarkFilters = {
	status?: BookmarkStatus
	q?: string
	label_id?: number
	project_id?: number
	task_id?: number
	broken?: boolean
}

export const bookmarkKeys = {
	all: ['bookmarks'] as const,
	lists: () => [...bookmarkKeys.all, 'list'] as const,
	list: (filters: BookmarkFilters) => [...bookmarkKeys.lists(), filters] as const,
	detail: (id: number) => [...bookmarkKeys.all, 'detail', id] as const,
	comments: (id: number) => [...bookmarkKeys.all, 'comments', id] as const,
	duplicates: (id: number) => [...bookmarkKeys.all, 'duplicates', id] as const,
	snapshot: (id: number) => [...bookmarkKeys.all, 'snapshot', id] as const,
	brokenCount: () => [...bookmarkKeys.all, 'broken-count'] as const,
}

function cleanFilters(filters: BookmarkFilters): BookmarkFilters {
	return {
		...(filters.status ? {status: filters.status} : {}),
		...(filters.q?.trim() ? {q: filters.q.trim()} : {}),
		...(filters.label_id ? {label_id: filters.label_id} : {}),
		...(filters.project_id ? {project_id: filters.project_id} : {}),
		...(filters.task_id ? {task_id: filters.task_id} : {}),
		...(filters.broken ? {broken: true} : {}),
	}
}

export function bookmarksQuery(filters: BookmarkFilters) {
	const query = cleanFilters(filters)
	return queryOptions({
		queryKey: bookmarkKeys.list(query),
		queryFn: async () => (await fetchAllPages(async page =>
			(await bookmarksList({query: {...query, page}})).data,
		)).map(normalizeBookmark),
	})
}

export function bookmarkQuery(id: number) {
	return queryOptions({
		queryKey: bookmarkKeys.detail(id),
		queryFn: async () => {
			const {data} = await bookmarksRead({path: {id}})
			return {...normalizeBookmark(data), max_permission: data.max_permission ?? 0}
		},
	})
}

export function bookmarkCommentsQuery(bookmarkId: number) {
	return queryOptions({
		queryKey: bookmarkKeys.comments(bookmarkId),
		queryFn: () => fetchAllPages(async page =>
			(await bookmarkCommentsList({path: {bookmark: bookmarkId}, query: {page}})).data,
		),
	})
}

export function hostOf(url: string): string {
	try {
		return new URL(url).hostname.replace(/^www\./, '')
	} catch {
		return url
	}
}

function writableBody(bookmark: BookmarkWritable): BookmarkWritable {
	return {
		url: bookmark.url,
		title: bookmark.title,
		description: bookmark.description,
		favicon_url: bookmark.favicon_url,
		image_url: bookmark.image_url,
		site_name: bookmark.site_name,
		status: bookmark.status,
	}
}

const PAGE_FETCH_REFRESH_DELAYS = [3000, 8000]

export type CreateBookmarkInput = BookmarkWritable & {
	/** Set when the caller handles a 409 "already saved" itself, so no error toast is shown. */
	handlesAlreadySaved?: boolean
}

export function createBookmarkMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({handlesAlreadySaved: _handles, ...input}: CreateBookmarkInput) => {
			const {data} = await bookmarksCreate({body: writableBody(input)})
			return normalizeBookmark(data)
		},
		onSettled: async (_input, client) => {
			await client.invalidateQueries({queryKey: bookmarkKeys.lists()})
			// The server fetches the page in the background; pick up its title and icon when they land.
			for (const delay of PAGE_FETCH_REFRESH_DELAYS) {
				setTimeout(() => void client.invalidateQueries({queryKey: bookmarkKeys.lists()}), delay)
			}
		},
		successMessage: () => i18n.global.t('johnnysBrain.bookmarks.saved'),
		toastError: input => !input.handlesAlreadySaved,
	})
}

export type UpdateBookmarkInput = BookmarkWritable & {id: number}

export function updateBookmarkMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({id, ...bookmark}: UpdateBookmarkInput) => {
			const {data} = await bookmarksUpdate({path: {id}, body: writableBody(bookmark)})
			return normalizeBookmark(data)
		},
		onSuccess: (updated, _input, client) => {
			client.setQueryData<BookmarkResponse & {max_permission: number}>(bookmarkKeys.detail(updated.id),
				current => current ? {...current, ...updated} : current)
		},
		onSettled: async ({id}, client) => {
			await Promise.all([
				client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
				client.invalidateQueries({queryKey: bookmarkKeys.detail(id), exact: true}),
			])
		},
	})
}

export function deleteBookmarkMutationOptions() {
	return contextMutationOptions({
		mutationFn: async (id: number) => {
			await bookmarksDelete({path: {id}})
		},
		onSuccess: (_data, id, client) => {
			client.removeQueries({queryKey: bookmarkKeys.detail(id), exact: true})
			client.removeQueries({queryKey: bookmarkKeys.comments(id), exact: true})
		},
		onSettled: (_id, client) => client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
		successMessage: () => i18n.global.t('johnnysBrain.bookmarks.deleted'),
	})
}

export type BookmarkLabelInput = {bookmarkId: number, labelId: number}

function invalidateBookmark(client: QueryClient, bookmarkId: number) {
	return Promise.all([
		client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
		client.invalidateQueries({queryKey: bookmarkKeys.detail(bookmarkId), exact: true}),
	])
}

export function addBookmarkLabelMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, labelId}: BookmarkLabelInput) => {
			await bookmarkLabelsCreate({path: {bookmark: bookmarkId}, body: {label_id: labelId}})
		},
		onSettled: ({bookmarkId}, client) => invalidateBookmark(client, bookmarkId),
	})
}

export function removeBookmarkLabelMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, labelId}: BookmarkLabelInput) => {
			await bookmarkLabelsDelete({path: {bookmark: bookmarkId, label: labelId}})
		},
		onSettled: ({bookmarkId}, client) => invalidateBookmark(client, bookmarkId),
	})
}

export type CreateBookmarkCommentInput = {bookmarkId: number, comment: string}

export function createBookmarkCommentMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, comment}: CreateBookmarkCommentInput) => {
			const {data} = await bookmarkCommentsCreate({path: {bookmark: bookmarkId}, body: {comment}})
			return data
		},
		onSuccess: (created, {bookmarkId}, client) => {
			client.setQueryData<BookmarkComment[]>(bookmarkKeys.comments(bookmarkId),
				current => current ? [...current, created] : current)
		},
		onSettled: ({bookmarkId}, client) =>
			client.invalidateQueries({queryKey: bookmarkKeys.comments(bookmarkId), exact: true}),
	})
}

export type DeleteBookmarkCommentInput = {bookmarkId: number, commentId: number}

export function deleteBookmarkCommentMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, commentId}: DeleteBookmarkCommentInput) => {
			await bookmarkCommentsDelete({path: {bookmark: bookmarkId, commentid: commentId}})
		},
		optimistic: {
			queryKeys: ({bookmarkId}) => [bookmarkKeys.comments(bookmarkId)],
			update: ({bookmarkId, commentId}, client) => {
				client.setQueryData<BookmarkComment[]>(bookmarkKeys.comments(bookmarkId),
					current => current?.filter(c => c.id !== commentId))
			},
		},
		onSettled: ({bookmarkId}, client) =>
			client.invalidateQueries({queryKey: bookmarkKeys.comments(bookmarkId), exact: true}),
	})
}

export function useCreateBookmarkMutation() {
	return useMutation(createBookmarkMutationOptions())
}

export function useUpdateBookmarkMutation() {
	return useMutation(updateBookmarkMutationOptions())
}

export function useDeleteBookmarkMutation() {
	return useMutation(deleteBookmarkMutationOptions())
}

export function useAddBookmarkLabelMutation() {
	return useMutation(addBookmarkLabelMutationOptions())
}

export function useRemoveBookmarkLabelMutation() {
	return useMutation(removeBookmarkLabelMutationOptions())
}

export function useCreateBookmarkCommentMutation() {
	return useMutation(createBookmarkCommentMutationOptions())
}

export function useDeleteBookmarkCommentMutation() {
	return useMutation(deleteBookmarkCommentMutationOptions())
}

// ---------------------------------------------------------------------------
// Saving a link that may already exist
// ---------------------------------------------------------------------------

export function isAlreadySaved(e: unknown): boolean {
	return e instanceof ApiError && e.status === 409 && e.code === 90008
}

/** The caller's bookmark for a link, compared the way the server does (tracking parameters ignored). */
export async function findBookmarkByUrl(url: string): Promise<BookmarkResponse | undefined> {
	const {data} = await bookmarksList({query: {url, page: 1}})
	const found = data.items?.[0]
	return found ? normalizeBookmark(found) : undefined
}

/** One bookmark with its sources, which the list endpoint leaves out. */
export async function fetchBookmark(id: number): Promise<BookmarkResponse> {
	const {data} = await bookmarksRead({path: {id}})
	return normalizeBookmark(data)
}

// ---------------------------------------------------------------------------
// Sources: links to projects and tasks
// ---------------------------------------------------------------------------

function invalidateBookmarkAndLists(client: QueryClient, bookmarkId: number) {
	return Promise.all([
		client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
		client.invalidateQueries({queryKey: bookmarkKeys.detail(bookmarkId), exact: true}),
	])
}

export type BookmarkProjectInput = {bookmarkId: number, projectId: number}
export type BookmarkTaskInput = {bookmarkId: number, taskId: number}

export function linkBookmarkProjectMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, projectId}: BookmarkProjectInput) => {
			await bookmarkProjectsCreate({path: {bookmark: bookmarkId}, body: {project_id: projectId}})
		},
		onSettled: ({bookmarkId}, client) => invalidateBookmarkAndLists(client, bookmarkId),
	})
}

export function unlinkBookmarkProjectMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, projectId}: BookmarkProjectInput) => {
			await bookmarkProjectsDelete({path: {bookmark: bookmarkId, project: projectId}})
		},
		onSettled: ({bookmarkId}, client) => invalidateBookmarkAndLists(client, bookmarkId),
	})
}

export function linkBookmarkTaskMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, taskId}: BookmarkTaskInput) => {
			await bookmarkTasksCreate({path: {bookmark: bookmarkId}, body: {task_id: taskId}})
		},
		onSettled: ({bookmarkId}, client) => invalidateBookmarkAndLists(client, bookmarkId),
	})
}

export function unlinkBookmarkTaskMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, taskId}: BookmarkTaskInput) => {
			await bookmarkTasksDelete({path: {bookmark: bookmarkId, task: taskId}})
		},
		onSettled: ({bookmarkId}, client) => invalidateBookmarkAndLists(client, bookmarkId),
	})
}

export function useLinkBookmarkProjectMutation() {
	return useMutation(linkBookmarkProjectMutationOptions())
}

export function useUnlinkBookmarkProjectMutation() {
	return useMutation(unlinkBookmarkProjectMutationOptions())
}

export function useLinkBookmarkTaskMutation() {
	return useMutation(linkBookmarkTaskMutationOptions())
}

export function useUnlinkBookmarkTaskMutation() {
	return useMutation(unlinkBookmarkTaskMutationOptions())
}

// ---------------------------------------------------------------------------
// Duplicates
// ---------------------------------------------------------------------------

export function bookmarkDuplicatesQuery(bookmarkId: number) {
	return queryOptions({
		queryKey: bookmarkKeys.duplicates(bookmarkId),
		queryFn: async () => {
			const {data} = await bookmarkDuplicatesList({path: {bookmark: bookmarkId}})
			return (data ?? []).map(suggestion => ({
				...suggestion,
				bookmark: normalizeBookmark(suggestion.bookmark ?? {}),
			}))
		},
	})
}

export type DuplicatePairInput = {bookmarkId: number, otherId: number}

function invalidatePair(client: QueryClient, {bookmarkId, otherId}: DuplicatePairInput) {
	return Promise.all([
		client.invalidateQueries({queryKey: bookmarkKeys.duplicates(bookmarkId), exact: true}),
		client.invalidateQueries({queryKey: bookmarkKeys.duplicates(otherId), exact: true}),
		client.invalidateQueries({queryKey: bookmarkKeys.detail(bookmarkId), exact: true}),
		client.invalidateQueries({queryKey: bookmarkKeys.detail(otherId), exact: true}),
		client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
	])
}

export function dismissBookmarkDuplicateMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, otherId}: DuplicatePairInput) => {
			await bookmarkDuplicatesDismiss({path: {bookmark: bookmarkId, other: otherId}})
		},
		onSettled: (input, client) => invalidatePair(client, input),
	})
}

export function mergeBookmarksMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({bookmarkId, otherId}: DuplicatePairInput) => {
			const {data} = await bookmarksMerge({path: {bookmark: bookmarkId}, body: {source_id: otherId}})
			return normalizeBookmark(data)
		},
		onSuccess: (_merged, {otherId}, client) => {
			client.removeQueries({queryKey: bookmarkKeys.detail(otherId), exact: true})
			client.removeQueries({queryKey: bookmarkKeys.comments(otherId), exact: true})
			client.removeQueries({queryKey: bookmarkKeys.duplicates(otherId), exact: true})
		},
		onSettled: ({bookmarkId}, client) => Promise.all([
			client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
			client.invalidateQueries({queryKey: bookmarkKeys.detail(bookmarkId), exact: true}),
			client.invalidateQueries({queryKey: bookmarkKeys.comments(bookmarkId), exact: true}),
			client.invalidateQueries({queryKey: bookmarkKeys.duplicates(bookmarkId), exact: true}),
		]),
		successMessage: () => i18n.global.t('johnnysBrain.bookmarks.merged'),
	})
}

export function useDismissBookmarkDuplicateMutation() {
	return useMutation(dismissBookmarkDuplicateMutationOptions())
}

export function useMergeBookmarksMutation() {
	return useMutation(mergeBookmarksMutationOptions())
}

// ---------------------------------------------------------------------------
// Link previews
// ---------------------------------------------------------------------------

/** Asks the server to fetch the bookmark's page again; the result arrives a few seconds later. */
export function refreshBookmarkMutationOptions() {
	return contextMutationOptions({
		mutationFn: async (bookmarkId: number) => {
			await bookmarksRefresh({path: {bookmark: bookmarkId}})
		},
	})
}

export type ImportBatchInput = {
	items: {url: string, title?: string, added?: string, folders?: string[]}[]
	status: BookmarkStatus
	foldersAsLabels: boolean
}

/** Imports one batch of links. The caller sends a large file in batches of a few hundred. */
export function importBookmarksMutationOptions() {
	return contextMutationOptions({
		mutationFn: async ({items, status, foldersAsLabels}: ImportBatchInput) =>
			(await bookmarksImport({body: {items, status, folders_as_labels: foldersAsLabels}})).data,
		onSettled: (_input, client) => Promise.all([
			client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
			client.invalidateQueries({queryKey: bookmarkKeys.brokenCount()}),
		]),
	})
}

export function useImportBookmarksMutation() {
	return useMutation(importBookmarksMutationOptions())
}

export function useRefreshBookmarkMutation() {
	return useMutation(refreshBookmarkMutationOptions())
}

export type PageProblem = 'gone' | 'unreachable' | 'error'

/**
 * What is wrong with a bookmark's page, going by its last check, or null when nothing is. A page
 * that blocks bots or wants a login (401, 403, 429) is not a problem, and nor is an address on a
 * private network, which is never checked. Matches the server's notion of a broken link.
 */
export function pageProblem(linkStatus: number): PageProblem | null {
	if (linkStatus === 404 || linkStatus === 410) return 'gone'
	if (linkStatus === -1) return 'unreachable'
	if (linkStatus >= 500) return 'error'
	return null
}

/** The saved readable text of a bookmark's page. Only asked for once someone opens it. */
export function bookmarkSnapshotQuery(bookmarkId: number, enabled: boolean) {
	return queryOptions({
		queryKey: bookmarkKeys.snapshot(bookmarkId),
		queryFn: async () => (await bookmarksSnapshot({path: {bookmark: bookmarkId}})).data,
		enabled,
	})
}

/** How many bookmarks have a broken link, for the badge on the Broken links tab. */
export function brokenCountQuery() {
	return queryOptions({
		queryKey: bookmarkKeys.brokenCount(),
		queryFn: async () => (await bookmarksList({query: {broken: true, per_page: 1}})).data.total ?? 0,
	})
}
