// Johnny's Brain: bookmarks, their labels and comments.
import {queryOptions, useMutation} from '@tanstack/vue-query'
import type {QueryClient} from '@tanstack/vue-query'

import {
	bookmarkCommentsCreate,
	bookmarkCommentsDelete,
	bookmarkCommentsList,
	bookmarkLabelsCreate,
	bookmarkLabelsDelete,
	bookmarksCreate,
	bookmarksDelete,
	bookmarksList,
	bookmarksRead,
	bookmarksUpdate,
} from '@/client/generated'
import type {Bookmark, BookmarkComment, BookmarkWritable, Label} from '@/client/generated'
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
}

export function normalizeBookmark(bookmark: Bookmark): BookmarkResponse {
	return {
		...bookmark,
		id: bookmark.id ?? 0,
		url: bookmark.url ?? '',
		title: bookmark.title ?? '',
		status: bookmark.status ?? 'to_read',
		labels: bookmark.labels ?? [],
	}
}

export type BookmarkFilters = {
	status?: BookmarkStatus
	q?: string
	label_id?: number
}

export const bookmarkKeys = {
	all: ['bookmarks'] as const,
	lists: () => [...bookmarkKeys.all, 'list'] as const,
	list: (filters: BookmarkFilters) => [...bookmarkKeys.lists(), filters] as const,
	detail: (id: number) => [...bookmarkKeys.all, 'detail', id] as const,
	comments: (id: number) => [...bookmarkKeys.all, 'comments', id] as const,
}

function cleanFilters(filters: BookmarkFilters): BookmarkFilters {
	return {
		...(filters.status ? {status: filters.status} : {}),
		...(filters.q?.trim() ? {q: filters.q.trim()} : {}),
		...(filters.label_id ? {label_id: filters.label_id} : {}),
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

export function createBookmarkMutationOptions() {
	return contextMutationOptions({
		mutationFn: async (input: BookmarkWritable) => {
			const {data} = await bookmarksCreate({body: writableBody(input)})
			return normalizeBookmark(data)
		},
		onSettled: (_input, client) => client.invalidateQueries({queryKey: bookmarkKeys.lists()}),
		successMessage: () => i18n.global.t('johnnysBrain.bookmarks.saved'),
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
