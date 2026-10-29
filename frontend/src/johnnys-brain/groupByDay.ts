// Groups tasks under day headings the way Todoist's Upcoming does: Overdue first, then Today,
// Tomorrow and each later day in order, with undated tasks last.

export type DayGroupTone = 'overdue' | 'today' | 'tomorrow' | 'later' | 'none'

export type DayGroup<T> = {
	key: string
	/** "Overdue", "Today", "Tomorrow", or the weekday for a later day. */
	title: string
	/** The date, such as "31 Oct", shown beside the title. Empty for Overdue and No date. */
	subtitle: string
	tone: DayGroupTone
	tasks: T[]
}

export type DatedTask = {due_date?: string | null}

export type DayGroupLabels = {overdue: string, today: string, tomorrow: string, noDate: string}

const DAY = 24 * 60 * 60 * 1000

function startOfDay(d: Date): number {
	return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

function dueDate(task: DatedTask): Date | null {
	if (!task.due_date) return null
	const date = new Date(task.due_date)
	// The API sends 0001-01-01 for "no date".
	return Number.isNaN(date.getTime()) || date.getFullYear() < 1970 ? null : date
}

/**
 * Tasks keep the order they came in within a group. Overdue means due on an earlier day than
 * today, so a task due earlier today still sits under Today.
 */
export function groupTasksByDay<T extends DatedTask>(
	tasks: T[],
	now: Date,
	labels: DayGroupLabels,
	locale?: string,
): DayGroup<T>[] {
	const today = startOfDay(now)
	const byDay = new Map<number, T[]>()
	const overdue: T[] = []
	const undated: T[] = []

	for (const task of tasks) {
		const due = dueDate(task)
		if (!due) {
			undated.push(task)
			continue
		}
		const day = startOfDay(due)
		if (day < today) {
			overdue.push(task)
			continue
		}
		const list = byDay.get(day) ?? []
		list.push(task)
		byDay.set(day, list)
	}

	const groups: DayGroup<T>[] = []
	if (overdue.length) {
		groups.push({key: 'overdue', title: labels.overdue, subtitle: '', tone: 'overdue', tasks: overdue})
	}

	const dateFormat = new Intl.DateTimeFormat(locale, {day: 'numeric', month: 'short'})
	const weekdayFormat = new Intl.DateTimeFormat(locale, {weekday: 'long'})
	for (const day of [...byDay.keys()].sort((a, b) => a - b)) {
		const date = new Date(day)
		const diff = Math.round((day - today) / DAY)
		const tone: DayGroupTone = diff === 0 ? 'today' : diff === 1 ? 'tomorrow' : 'later'
		const title = diff === 0 ? labels.today : diff === 1 ? labels.tomorrow : weekdayFormat.format(date)
		groups.push({
			key: `day-${day}`,
			title,
			subtitle: dateFormat.format(date),
			tone,
			tasks: byDay.get(day) ?? [],
		})
	}

	if (undated.length) {
		groups.push({key: 'none', title: labels.noDate, subtitle: '', tone: 'none', tasks: undated})
	}
	return groups
}
