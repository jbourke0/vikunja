import {computed, toValue, type MaybeRef, type MaybeRefOrGetter} from 'vue'

import {useTitle as useTitleVueUse, type UseTitleOptions, type ReadonlyRefOrGetter} from '@vueuse/core'
import {pageTitle as brandedPageTitle} from '@/johnnys-brain/brand'

export function useTitle(
	newTitle:
		| ReadonlyRefOrGetter<string | null | undefined>
		| MaybeRef<string | null | undefined>
		| MaybeRefOrGetter<string | null | undefined> = null,
	options?: UseTitleOptions,
) {
	const pageTitle = computed(() => toValue(newTitle))

	const completeTitle = computed(() => brandedPageTitle(pageTitle.value as string | null | undefined))

	return useTitleVueUse(completeTitle, options)
}
