import {describe, expect, it} from 'vitest'

import {extractSharedLink, findLinkInText, sharedInputFromQuery} from './sharedLink'

describe('extractSharedLink', () => {
	it('uses the url and title Chrome sends', () => {
		expect(extractSharedLink({url: 'https://example.com/story', title: 'A good story'}))
			.toEqual({url: 'https://example.com/story', title: 'A good story'})
	})

	it('finds a link inside the text other apps send', () => {
		expect(extractSharedLink({text: 'Have a look at this https://example.com/story?id=7&x=1 it is great'}))
			.toEqual({url: 'https://example.com/story?id=7&x=1', title: ''})
	})

	it('finds a link in the title when nothing else has one', () => {
		expect(extractSharedLink({title: 'https://example.com/a'}).url).toBe('https://example.com/a')
	})

	it('prefers the url field over a link in the text', () => {
		expect(extractSharedLink({url: 'https://example.com/real', text: 'see https://other.example/x'}).url)
			.toBe('https://example.com/real')
	})

	it('adds https to a bare domain typed or pasted', () => {
		expect(extractSharedLink({url: 'example.com/story'}).url).toBe('https://example.com/story')
	})

	it('drops a title that is only the address again', () => {
		expect(extractSharedLink({url: 'https://www.example.com/x', title: 'https://www.example.com/x'}).title).toBe('')
		expect(extractSharedLink({url: 'https://www.example.com/x', title: 'www.example.com'}).title).toBe('')
		expect(extractSharedLink({url: 'https://www.example.com/x', title: 'Visit https://www.example.com/x now'}).title).toBe('')
	})

	it('caps a very long title', () => {
		expect(extractSharedLink({url: 'https://example.com', title: 'x'.repeat(400)}).title).toHaveLength(250)
	})

	it('returns nothing when the share holds no link', () => {
		expect(extractSharedLink({text: 'just some words', title: 'a title'})).toEqual({url: '', title: ''})
		expect(extractSharedLink({})).toEqual({url: '', title: ''})
		expect(extractSharedLink({url: '', text: null, title: null})).toEqual({url: '', title: ''})
	})

	it('refuses anything that is not a web address', () => {
		for (const url of ['javascript:alert(1)', 'file:///etc/passwd', 'ftp://example.com/x', 'data:text/html,<b>x</b>', 'intent://scan/#Intent;end']) {
			expect(extractSharedLink({url}).url, url).toBe('')
			expect(extractSharedLink({text: `open ${url}`}).url, url).toBe('')
		}
	})

	it('does not treat words with spaces as a domain', () => {
		expect(extractSharedLink({url: 'a b.com'}).url).toBe('')
	})
})

describe('findLinkInText', () => {
	it('strips punctuation that belongs to the sentence', () => {
		expect(findLinkInText('Read https://example.com/story.')).toBe('https://example.com/story')
		expect(findLinkInText('Is it https://example.com/story?')).toBe('https://example.com/story')
		expect(findLinkInText('(see https://example.com/story)')).toBe('https://example.com/story')
		expect(findLinkInText('"https://example.com/story",')).toBe('https://example.com/story')
	})

	it('keeps a bracket that is part of the address', () => {
		expect(findLinkInText('https://en.wikipedia.org/wiki/Cheese_(disambiguation)')).toBe('https://en.wikipedia.org/wiki/Cheese_(disambiguation)')
		expect(findLinkInText('see https://en.wikipedia.org/wiki/Cheese_(disambiguation).')).toBe('https://en.wikipedia.org/wiki/Cheese_(disambiguation)')
	})

	it('takes the first of several links', () => {
		expect(findLinkInText('a https://one.example/x b https://two.example/y')).toBe('https://one.example/x')
	})

	it('handles empty input', () => {
		expect(findLinkInText('')).toBe('')
		expect(findLinkInText(null)).toBe('')
		expect(findLinkInText(undefined)).toBe('')
	})
})

describe('sharedInputFromQuery', () => {
	it('reads the three share target fields', () => {
		expect(sharedInputFromQuery({url: 'u', text: 't', title: 'x'})).toEqual({url: 'u', text: 't', title: 'x'})
	})

	it('takes the first of a repeated parameter and ignores other types', () => {
		expect(sharedInputFromQuery({url: ['first', 'second'], text: 5, title: undefined}))
			.toEqual({url: 'first', text: null, title: null})
	})
})
