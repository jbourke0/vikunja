<template>
	<BaseButton
		v-show="!baseStore.quickActionsActive && !baseStore.menuActive"
		class="jb-fab d-print-none"
		:aria-label="$t('johnnysBrain.nav.addTask')"
		@click="baseStore.setQuickActionsActive(true)"
	>
		<Icon icon="plus" />
	</BaseButton>
</template>

<script setup lang="ts">
// App-shell extras for Johnny's Brain: the mobile add-task button, plus a data attribute
// on <html> so the theme can adjust specific views without editing upstream components.
import {watchEffect, onBeforeUnmount} from 'vue'
import BaseButton from '@/components/base/BaseButton.vue'
import {useBaseStore} from '@/stores/base'
import {useTodayView} from '@/johnnys-brain/useTodayView'
import {useGlobalNow} from '@/composables/useGlobalNow'

const baseStore = useBaseStore()
const {isToday} = useTodayView()

// Create the shared clock during setup. Left to the first date formatter that runs, it can be
// created mid-render, where its route hook cannot register (opening a task by URL did this).
useGlobalNow()

watchEffect(() => {
	if (isToday.value) {
		document.documentElement.dataset.jbView = 'today'
	} else {
		delete document.documentElement.dataset.jbView
	}
})

onBeforeUnmount(() => {
	delete document.documentElement.dataset.jbView
})
</script>
