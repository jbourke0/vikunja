// Johnny's Brain customisations live under src/johnnys-brain/ to keep upstream merges simple.

export const APP_NAME = 'Johnny\'s Brain'

// Todoist-style "Today": everything due today plus anything overdue.
export const TODAY_QUERY = {
	from: 'now/d',
	to: 'now/d+1d',
	showOverdue: 'true',
	showNulls: 'false',
} as const

export function pageTitle(title: string | null | undefined): string {
	return (typeof title === 'undefined' || title === null || title === '')
		? APP_NAME
		: `${title} | ${APP_NAME}`
}
