import {describe, expect, it} from 'vitest'

import {type DatedTask, groupTasksByDay} from './groupByDay'

type Task = DatedTask & {id: number}

const labels = {overdue: 'Overdue', today: 'Today', tomorrow: 'Tomorrow', noDate: 'No date'}
// Thursday 30 October 2026, mid-afternoon, local time.
const now = new Date(2026, 9, 30, 15, 0, 0)
const at = (y: number, m: number, d: number, h = 9) => ({due_date: new Date(y, m - 1, d, h).toISOString()})

describe('groupTasksByDay', () => {
	it('groups into overdue, today, tomorrow and later days in order', () => {
		const tasks: Task[] = [
			{id: 1, ...at(2026, 11, 3)},
			{id: 2, ...at(2026, 10, 30, 17)},
			{id: 3, ...at(2026, 10, 29)},
			{id: 4, ...at(2026, 10, 31)},
			{id: 5, ...at(2026, 11, 3, 12)},
			{id: 6},
			{id: 7, ...at(2026, 10, 30, 8)},
		]
		const groups = groupTasksByDay(tasks, now, labels, 'en-AU')

		expect(groups.map(g => [g.title, g.tone, g.tasks.map(t => t.id)])).toEqual([
			['Overdue', 'overdue', [3]],
			['Today', 'today', [2, 7]],
			['Tomorrow', 'tomorrow', [4]],
			['Tuesday', 'later', [1, 5]],
			['No date', 'none', [6]],
		])
		expect(groups[1].subtitle).toBe('30 Oct')
		expect(groups[3].subtitle).toBe('3 Nov')
	})

	it('treats earlier today as today, not overdue', () => {
		const groups = groupTasksByDay([{id: 1, ...at(2026, 10, 30, 6)}], now, labels, 'en-AU')
		expect(groups.map(g => g.title)).toEqual(['Today'])
	})

	it('treats the API zero date as no date', () => {
		const groups = groupTasksByDay([{id: 1, due_date: '0001-01-01T00:00:00Z'}, {id: 2, due_date: null}], now, labels, 'en-AU')
		expect(groups.map(g => [g.title, g.tasks.length])).toEqual([['No date', 2]])
	})

	it('keeps the order tasks arrived in within a day', () => {
		const tasks: Task[] = [{id: 3, ...at(2026, 11, 5, 18)}, {id: 1, ...at(2026, 11, 5, 8)}, {id: 2, ...at(2026, 11, 5, 12)}]
		expect(groupTasksByDay(tasks, now, labels, 'en-AU')[0].tasks.map(t => t.id)).toEqual([3, 1, 2])
	})

	it('returns nothing for no tasks', () => {
		expect(groupTasksByDay([], now, labels)).toEqual([])
	})

	it('crosses a month and year end', () => {
		const groups = groupTasksByDay([{id: 1, ...at(2027, 1, 2)}], new Date(2026, 11, 31, 10), labels, 'en-AU')
		expect(groups[0].title).toBe('Saturday')
		expect(groups[0].subtitle).toBe('2 Jan')
	})
})
