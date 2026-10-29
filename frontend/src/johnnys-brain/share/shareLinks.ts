// The addresses and snippets the "Save from other apps" page hands out.

/** Where the iPhone shortcut posts new bookmarks. */
export function createBookmarkEndpoint(origin: string): string {
	return `${origin}/api/v2/bookmarks`
}

/**
 * A bookmarklet for desktop browsers: opens the Save screen in a small window with the page's
 * address and title filled in. It does nothing else and never sends the page's content anywhere.
 */
export function bookmarkletHref(origin: string): string {
	const target = JSON.stringify(`${origin}/save`)
	const code = `(()=>{const u=new URL(${target});u.searchParams.set('url',location.href);u.searchParams.set('title',document.title);window.open(u,'jbsave','popup,width=480,height=720')})()`
	return `javascript:${code}`
}
