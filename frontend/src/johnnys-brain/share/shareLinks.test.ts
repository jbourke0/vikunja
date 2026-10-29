import {describe, expect, it} from 'vitest'

import {bookmarkletHref, createBookmarkEndpoint} from './shareLinks'

describe('shareLinks', () => {
	it('points the shortcut at the v2 bookmarks endpoint of this server', () => {
		expect(createBookmarkEndpoint('https://tasks.jbourke.com')).toBe('https://tasks.jbourke.com/api/v2/bookmarks')
	})

	it('builds a bookmarklet that opens the save screen with the page address and title', () => {
		const href = bookmarkletHref('https://tasks.jbourke.com')
		expect(href.startsWith('javascript:')).toBe(true)

		const opened: unknown[][] = []
		const fakeWindow = {open: (...args: unknown[]) => { opened.push(args) }}
		const run = new Function('location', 'document', 'window', 'URL', `return ${href.slice('javascript:'.length)}`)
		run({href: 'https://example.com/a?x=1&y=2'}, {title: 'Tom & Jerry "quoted"'}, fakeWindow, URL)

		expect(opened).toHaveLength(1)
		const [url, name, features] = opened[0] as [URL, string, string]
		expect(url.origin + url.pathname).toBe('https://tasks.jbourke.com/save')
		expect(url.searchParams.get('url')).toBe('https://example.com/a?x=1&y=2')
		expect(url.searchParams.get('title')).toBe('Tom & Jerry "quoted"')
		expect(name).toBe('jbsave')
		expect(features).toContain('popup')
	})

	it('survives an origin with characters that need escaping', () => {
		expect(() => bookmarkletHref("https://example.com/it's")).not.toThrow()
	})
})
