import {
	type BookmarkResponse,
	type BookmarkStatus,
	findBookmarkByUrl,
	isAlreadySaved,
	useCreateBookmarkMutation,
} from '@/client/queries/bookmarks'
import {isRequestContextAbort} from '@/client/requestContext'
import {error} from '@/message'

/** Adds https:// when someone types or pastes a bare domain. */
export function withScheme(raw: string): string {
	const trimmed = raw.trim()
	return /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
}

/** Whether text looks like a link rather than a search. */
export function looksLikeLink(raw: string): boolean {
	const trimmed = raw.trim()
	if (/^https?:\/\//i.test(trimmed)) return true
	return /^[^\s/]+\.[a-z]{2,}(\/\S*)?$/i.test(trimmed)
}

export type SavedLink = {bookmark: BookmarkResponse, alreadySaved: boolean}

/**
 * Saves a link as a bookmark. Saving a link that is already saved is not an error here: the
 * existing bookmark comes back with alreadySaved set, so callers can link or open it instead.
 */
export function useSaveLink() {
	const createMutation = useCreateBookmarkMutation()

	async function save(rawUrl: string, status: BookmarkStatus = 'to_read'): Promise<SavedLink | undefined> {
		const url = withScheme(rawUrl)
		try {
			const bookmark = await createMutation.mutateAsync({url, status, handlesAlreadySaved: true})
			return {bookmark, alreadySaved: false}
		} catch (e) {
			if (isRequestContextAbort(e)) return undefined
			if (!isAlreadySaved(e)) {
				error(e)
				return undefined
			}
		}
		try {
			const existing = await findBookmarkByUrl(url)
			return existing ? {bookmark: existing, alreadySaved: true} : undefined
		} catch (e) {
			if (!isRequestContextAbort(e)) error(e)
			return undefined
		}
	}

	return {save, isSaving: createMutation.isPending}
}
