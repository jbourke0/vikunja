// Todoist-style due date labels: "Today", "Tomorrow", "Fri", "5 Nov", plus the time when one is set.

export type DueTone = 'overdue' | 'today' | 'tomorrow' | 'later'

export interface DueLabel {
	text: string
	tone: DueTone
}

const DAY = 24 * 60 * 60 * 1000

function startOfDay(d: Date): Date {
	return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

function capitalise(s: string): string {
	return s.charAt(0).toUpperCase() + s.slice(1)
}

export function dueLabel(due: Date | string, now: Date = new Date(), locale?: string): DueLabel {
	const date = typeof due === 'string' ? new Date(due) : due
	const dayDiff = Math.round((startOfDay(date).getTime() - startOfDay(now).getTime()) / DAY)

	let day: string
	if (dayDiff >= -1 && dayDiff <= 1) {
		day = capitalise(new Intl.RelativeTimeFormat(locale, {numeric: 'auto'}).format(dayDiff, 'day'))
	} else if (dayDiff > 1 && dayDiff < 7) {
		day = new Intl.DateTimeFormat(locale, {weekday: 'short'}).format(date)
	} else {
		day = new Intl.DateTimeFormat(locale, {
			day: 'numeric',
			month: 'short',
			...(date.getFullYear() !== now.getFullYear() ? {year: 'numeric'} : {}),
		}).format(date)
	}

	const hasTime = date.getHours() !== 0 || date.getMinutes() !== 0
	const text = hasTime
		? `${day} ${new Intl.DateTimeFormat(locale, {hour: 'numeric', minute: '2-digit'}).format(date)}`
		: day

	let tone: DueTone = 'later'
	if (date.getTime() < now.getTime() && dayDiff < 0) tone = 'overdue'
	else if (dayDiff === 0) tone = date.getTime() < now.getTime() ? 'overdue' : 'today'
	else if (dayDiff === 1) tone = 'tomorrow'

	return {text, tone}
}
