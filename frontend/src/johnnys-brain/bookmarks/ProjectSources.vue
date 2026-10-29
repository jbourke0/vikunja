<template>
	<CreateEdit
		:title="$t('johnnysBrain.sources.projectTitle', {project: project?.title ?? ''})"
		:has-primary-action="false"
	>
		<p class="jb-project-sources__intro">
			{{ $t('johnnysBrain.sources.projectIntro') }}
		</p>
		<SourcesPanel
			:project-id="projectId"
			:can-write="canWrite"
			:show-heading="false"
		/>
	</CreateEdit>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useQuery} from '@tanstack/vue-query'

import CreateEdit from '@/components/misc/CreateEdit.vue'
import {projectQuery} from '@/client/queries/projects'
import {useTitle} from '@/composables/useTitle'
import {useI18n} from 'vue-i18n'
import SourcesPanel from './SourcesPanel.vue'

const props = defineProps<{
	projectId: number
}>()

const {t} = useI18n({useScope: 'global'})

const query = useQuery(computed(() => projectQuery(props.projectId)))
const project = computed(() => query.data.value)
// Permission 1 is write; linking a source needs it.
const canWrite = computed(() => (project.value?.max_permission ?? 0) >= 1)

useTitle(() => t('johnnysBrain.sources.projectTitle', {project: project.value?.title ?? ''}))
</script>

<style lang="scss" scoped>
.jb-project-sources__intro {
	color: var(--grey-500);
	font-size: .9rem;
	margin-block-end: 1rem;
}
</style>
