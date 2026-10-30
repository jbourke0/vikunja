// Reads the bookmarks file a browser exports (the old "Netscape" HTML format that Chrome, Edge,
// Firefox, Safari and Brave all still write) into a flat list of links with their folders.

export type ImportItem = {
	url: string
	title: string
	/** The note under the link (the DD line), when the file has one. */
	description?: string
	/** ISO date the link was first bookmarked, when the file says. */
	added?: string
	/** The folders the link sat in, outermost first. */
	folders: string[]
}

export type ParsedBookmarks = {
	items: ImportItem[]
	/** Links that are not web addresses: browser pages, place: queries, javascript: bookmarklets. */
	skipped: number
	/** How many folders the file holds. */
	folderCount: number
}

export const MAX_IMPORT_FILE_BYTES = 25 * 1024 * 1024

/** The server keeps at most this many characters of a note. */
export const MAX_NOTE_LENGTH = 2000

function isWebLink(href: string): boolean {
	return /^https?:\/\/\S+/i.test(href.trim())
}

/** ADD_DATE is seconds since 1970 in most exports, but some write milliseconds or microseconds. */
export function parseAddDate(raw: string | null): string | undefined {
	if (!raw || !/^\d+$/.test(raw.trim())) return undefined
	let seconds = Number(raw)
	if (seconds > 1e14) seconds /= 1e6
	else if (seconds > 1e11) seconds /= 1e3
	const date = new Date(seconds * 1000)
	return Number.isNaN(date.getTime()) || seconds <= 0 ? undefined : date.toISOString()
}

/** A DD's own text, leaving out any list nested inside it. */
function noteText(dd: Element): string {
	return Array.from(dd.childNodes)
		.filter(n => n.nodeType === Node.TEXT_NODE || !['DL', 'DT', 'DD'].includes((n as Element).tagName))
		.map(n => n.textContent ?? '')
		.join(' ')
}

function clean(text: string | null | undefined): string {
	return (text ?? '').replace(/\s+/g, ' ').trim()
}

/**
 * Walks the file's lists. The format leaves most tags unclosed, and parsers nest the results
 * differently, so this does not depend on exact nesting: a folder's name (an H3) applies to the
 * list (DL) that follows it, wherever the parser put that list.
 */
export function parseBookmarksHtml(html: string): ParsedBookmarks {
	const doc = new DOMParser().parseFromString(html, 'text/html')
	const items: ImportItem[] = []
	let skipped = 0
	let folderCount = 0

	// The note (DD) that follows a link belongs to that link; a folder or list in between ends it.
	let last: ImportItem | null = null

	type Level = {folder: string | null}
	function walk(node: Element, path: string[], level: Level) {
		for (const child of Array.from(node.children)) {
			switch (child.tagName) {
				case 'H3':
					level.folder = clean(child.textContent)
					folderCount++
					last = null
					break
				case 'A': {
					const href = (child.getAttribute('href') ?? '').trim()
					if (href === '') break
					if (!isWebLink(href)) {
						skipped++
						break
					}
					const added = parseAddDate(child.getAttribute('add_date'))
					last = {
						url: href,
						title: clean(child.textContent),
						...(added ? {added} : {}),
						folders: path,
					}
					items.push(last)
					break
				}
				case 'DD': {
					// Parsers nest the list that follows a folder's description inside the DD,
					// so take only the note's own text and still walk what is inside.
					const note = clean(noteText(child)).slice(0, MAX_NOTE_LENGTH)
					if (last && note !== '' && !last.description) last.description = note
					walk(child, path, level)
					break
				}
				case 'DL': {
					const inner = level.folder ? [...path, level.folder] : path
					level.folder = null
					last = null
					walk(child, inner, {folder: null})
					last = null
					break
				}
				default:
					walk(child, path, level)
			}
		}
	}
	walk(doc.body, [], {folder: null})

	return {items, skipped, folderCount}
}

/** Splits a list into batches of a given size. */
export function inBatches<T>(items: T[], size: number): T[][] {
	const batches: T[][] = []
	for (let i = 0; i < items.length; i += size) {
		batches.push(items.slice(i, i + size))
	}
	return batches
}
