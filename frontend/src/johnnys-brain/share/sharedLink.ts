// Works out which link, and which title, a share from another app is about.
//
// Apps hand a share over in different shapes: Chrome sends the address in `url` and the page
// title in `title`, but many Android apps send a sentence like "Have a look at this
// https://example.com/story" in `text` and nothing else.

export type SharedInput = {
	url?: string | null
	text?: string | null
	title?: string | null
}

export type SharedLink = {
	/** An absolute http(s) address, or '' when the share held no link. */
	url: string
	/** The page title the sharing app supplied, or '' when it supplied none worth using. */
	title: string
}

const LINK_IN_TEXT = /https?:\/\/[^\s<>"'`]+/i
const TRAILING_PUNCTUATION = /[.,;:!?'"]+$/

/** Strips punctuation a sentence puts after a link, keeping a closing bracket that belongs to it. */
function trimLink(link: string): string {
	let trimmed = link.replace(TRAILING_PUNCTUATION, '')
	while (/[)\]}]$/.test(trimmed)) {
		const closer = trimmed.slice(-1)
		const opener = closer === ')' ? '(' : closer === ']' ? '[' : '{'
		if (trimmed.split(closer).length <= trimmed.split(opener).length) break
		trimmed = trimmed.slice(0, -1).replace(TRAILING_PUNCTUATION, '')
	}
	return trimmed
}

function asWebUrl(candidate: string): string {
	try {
		const parsed = new URL(candidate)
		return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.hostname !== ''
			? parsed.toString()
			: ''
	} catch {
		return ''
	}
}

/** The first web address in some text, or ''. */
export function findLinkInText(text: string | null | undefined): string {
	const match = LINK_IN_TEXT.exec(text ?? '')
	return match ? asWebUrl(trimLink(match[0])) : ''
}

/** A bare domain such as "example.com/story" counts as a link; anything with spaces does not. */
function asTypedLink(value: string): string {
	const trimmed = value.trim()
	if (trimmed === '' || /\s/.test(trimmed)) return ''
	if (/^https?:\/\//i.test(trimmed)) return asWebUrl(trimmed)
	return /^[^/]+\.[a-z]{2,}(\/|$)/i.test(trimmed) ? asWebUrl(`https://${trimmed}`) : ''
}

export function extractSharedLink(input: SharedInput): SharedLink {
	const url = asTypedLink(input.url ?? '') || findLinkInText(input.text) || findLinkInText(input.title)
	if (url === '') {
		return {url: '', title: ''}
	}

	// A title that is only the address again (some apps do this) says nothing.
	const title = (input.title ?? '').trim()
	let host = ''
	try {
		host = new URL(url).hostname.replace(/^www\./, '')
	} catch {
		// url was validated above
	}
	const isJustTheLink = title === '' || LINK_IN_TEXT.test(title) || title === url || title.replace(/^www\./, '') === host
	return {url, title: isJustTheLink ? '' : title.slice(0, 250)}
}

/** Reads a share from the query string the web share target and the bookmarklet send. */
export function sharedInputFromQuery(query: Record<string, unknown>): SharedInput {
	const first = (value: unknown): string | null => {
		const v = Array.isArray(value) ? value[0] : value
		return typeof v === 'string' ? v : null
	}
	return {url: first(query.url), text: first(query.text), title: first(query.title)}
}
