import {computed} from 'vue'
import {useRoute} from 'vue-router'
import {TODAY_QUERY} from '@/johnnys-brain/brand'

// Vue Router ignores the query when deciding which link is active, so "Today" and
// "Upcoming" (same route, different query) need their active state worked out here.
export function useTodayView() {
	const route = useRoute()

	const isRangeView = computed(() => route.name === 'tasks.range')
	const isToday = computed(() => isRangeView.value
		&& route.query.from === TODAY_QUERY.from
		&& route.query.to === TODAY_QUERY.to)

	return {isRangeView, isToday}
}
