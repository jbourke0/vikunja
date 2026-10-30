import {describe, expect, it} from 'vitest'

import {inBatches, parseAddDate, parseBookmarksHtml} from './bookmarkImport'

// Shaped like Chrome's export: tags are left open, exactly as browsers write them.
const CHROME = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<!-- This is an automatically generated file. -->
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 ADD_DATE="1551000000" LAST_MODIFIED="1700000000" PERSONAL_TOOLBAR_FOLDER="true">Bookmarks bar</H3>
    <DL><p>
        <DT><A HREF="https://www.carbongym.com.au/" ADD_DATE="1551000100" ICON="data:image/png;base64,AAAA">Carbon Gym Geelong</A>
        <DT><H3 ADD_DATE="1551000200">Work</H3>
        <DL><p>
            <DT><A HREF="https://example.com/tools?a=1&amp;b=2" ADD_DATE="1551000300">Tools &amp; things</A>
            <DT><H3>Deep</H3>
            <DL><p>
                <DT><A HREF="https://example.org/deep">Deep link</A>
            </DL><p>
            <DT><A HREF="https://example.net/after-deep">After the deep folder</A>
        </DL><p>
        <DT><A HREF="javascript:alert(1)">A bookmarklet</A>
        <DT><A HREF="chrome://settings">Settings</A>
    </DL><p>
    <DT><H3>Other bookmarks</H3>
    <DL><p>
        <DT><A HREF="http://old.example.com/page">Old page</A>
    </DL><p>
</DL><p>`

const WITH_NOTES = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
 <DT><H3>SEO</H3>
 <DL><p>
   <DT><A HREF="https://www.instagram.com/reel/AAA">First &#x27;one&#x27;</A>
   <DD>Document notes: Go to Search Console &gt; Performance.
   <DT><A HREF="https://www.instagram.com/reel/BBB">No note here</A>
   <DT><A HREF="https://www.instagram.com/reel/CCC">Third</A>
   <DD>Saved source for this task:   fix   the sitemap
   <DD>A second note line is ignored
 </DL><p>
 <DT><H3>Claude</H3>
 <DD>A note on a folder belongs to no link
 <DL><p>
   <DT><A HREF="javascript:alert(1)">Bookmarklet</A>
   <DD>Note on a skipped link
   <DT><A HREF="https://example.com/d">Fourth</A>
   <DD>${'x'.repeat(2500)}
 </DL><p>
</DL><p>`

describe('parseBookmarksHtml notes', () => {
	it('attaches each DD note to the link before it', () => {
		const {items} = parseBookmarksHtml(WITH_NOTES)
		const byUrl = Object.fromEntries(items.map(i => [i.url.split('/').pop(), i]))

		expect(byUrl.AAA.description).toBe('Document notes: Go to Search Console > Performance.')
		expect(byUrl.AAA.title).toBe('First \'one\'')
		expect(byUrl.BBB.description).toBeUndefined()
		expect(byUrl.CCC.description).toBe('Saved source for this task: fix the sitemap')
	})

	it('does not hand a folder note or a skipped link\'s note to another link', () => {
		const {items, skipped} = parseBookmarksHtml(WITH_NOTES)
		expect(skipped).toBe(1)
		expect(items.some(i => i.description?.includes('folder'))).toBe(false)
		expect(items.some(i => i.description?.includes('skipped link'))).toBe(false)
	})

	it('cuts very long notes to what the server keeps', () => {
		const {items} = parseBookmarksHtml(WITH_NOTES)
		expect(items.find(i => i.url === 'https://example.com/d')?.description).toHaveLength(2000)
	})
})

describe('parseBookmarksHtml', () => {
	it('reads links with their folder paths, titles and dates', () => {
		const {items, skipped} = parseBookmarksHtml(CHROME)

		expect(items.map(i => i.url)).toEqual([
			'https://www.carbongym.com.au/',
			'https://example.com/tools?a=1&b=2',
			'https://example.org/deep',
			'https://example.net/after-deep',
			'http://old.example.com/page',
		])
		expect(items[0]).toEqual({
			url: 'https://www.carbongym.com.au/',
			title: 'Carbon Gym Geelong',
			added: '2019-02-24T09:21:40.000Z',
			folders: ['Bookmarks bar'],
		})
		expect(items[1].title).toBe('Tools & things')
		expect(items[1].folders).toEqual(['Bookmarks bar', 'Work'])
		expect(items[2].folders).toEqual(['Bookmarks bar', 'Work', 'Deep'])
		expect(items[3].folders).toEqual(['Bookmarks bar', 'Work'])
		expect(items[4].folders).toEqual(['Other bookmarks'])
		expect(skipped).toBe(2)
	})

	it('counts the folders', () => {
		expect(parseBookmarksHtml(CHROME).folderCount).toBe(4)
	})

	it('handles links outside any folder', () => {
		const {items} = parseBookmarksHtml('<DL><p><DT><A HREF="https://example.com/x">X</A></DL>')
		expect(items).toEqual([{url: 'https://example.com/x', title: 'X', folders: []}])
	})

	it('copes with a folder whose list is a sibling rather than a child', () => {
		const {items} = parseBookmarksHtml(`<DL><DT><H3>Recipes</H3></DT><DL><DT><A HREF="https://example.com/soup">Soup</A></DT></DL></DL>`)
		expect(items[0].folders).toEqual(['Recipes'])
	})

	it('copes with properly closed tags too', () => {
		const {items} = parseBookmarksHtml(`<DL><DT><H3>Folder</H3><DL><DT><A HREF="https://example.com/a">A</A></DT></DL></DT><DT><A HREF="https://example.com/b">B</A></DT></DL>`)
		expect(items.map(i => [i.url, i.folders])).toEqual([
			['https://example.com/a', ['Folder']],
			['https://example.com/b', []],
		])
	})

	it('skips links with no address and never trusts scripts or other schemes', () => {
		const {items, skipped} = parseBookmarksHtml(`<DL><DT><A HREF="">Empty</A><DT><A>No href</A><DT><A HREF="data:text/html,hi">Data</A><DT><A HREF="place:sort=8">Smart</A><DT><A HREF="file:///etc/passwd">File</A><DT><A HREF="ftp://example.com/x">FTP</A></DL>`)
		expect(items).toEqual([])
		expect(skipped).toBe(4)
	})

	it('keeps a link whose title is empty', () => {
		const {items} = parseBookmarksHtml('<DL><DT><A HREF="https://example.com/x"></A></DL>')
		expect(items[0].title).toBe('')
	})

	it('returns nothing for a file that is not a bookmarks export', () => {
		expect(parseBookmarksHtml('')).toEqual({items: [], skipped: 0, folderCount: 0})
		expect(parseBookmarksHtml('<html><body><p>Hello</p></body></html>').items).toEqual([])
	})

	it('does not execute anything in the file', () => {
		const spy = (globalThis as {__jbRan?: boolean})
		spy.__jbRan = false
		parseBookmarksHtml('<script>globalThis.__jbRan = true</script><DL><DT><A HREF="https://example.com/x" onclick="globalThis.__jbRan = true">X</A></DL>')
		expect(spy.__jbRan).toBe(false)
	})
})

describe('parseAddDate', () => {
	it('reads seconds, milliseconds and microseconds', () => {
		expect(parseAddDate('1551000100')).toBe('2019-02-24T09:21:40.000Z')
		expect(parseAddDate('1551000100000')).toBe('2019-02-24T09:21:40.000Z')
		expect(parseAddDate('1551000100000000')).toBe('2019-02-24T09:21:40.000Z')
	})

	it('ignores anything else', () => {
		for (const bad of [null, '', 'abc', '-5', '0', '12.5', '99999999999999999999999']) {
			expect(parseAddDate(bad), String(bad)).toBeUndefined()
		}
	})
})

describe('inBatches', () => {
	it('splits evenly and keeps the remainder', () => {
		expect(inBatches([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]])
		expect(inBatches([], 3)).toEqual([])
		expect(inBatches([1, 2], 5)).toEqual([[1, 2]])
	})
})
