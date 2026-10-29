<template>
	<section class="jb-sources">
		<h2
			v-if="showHeading"
			class="task-section-title"
		>
			<span class="icon is-grey">
				<Icon icon="bookmark" />
			</span>
			{{ $t('johnnysBrain.sources.title') }}
			<span
				v-if="sources.length"
				class="jb-sources__count"
			>{{ sources.length }}</span>
		</h2>

		<ul
			v-if="sources.length"
			class="jb-sources__list"
		>
			<li
				v-for="source in sources"
				:key="source.id"
				class="jb-sources__item"
			>
				<span class="jb-sources__icon">
					<img
						v-if="source.favicon_url && !brokenIcons.has(source.id)"
						:src="source.favicon_url"
						referrerpolicy="no-referrer"
						alt=""
						width="18"
						height="18"
						loading="lazy"
						@error="brokenIcons.add(source.id)"
					>
					<Icon
						v-else
						icon="globe"
					/>
				</span>
				<div class="jb-sources__body">
					<RouterLink
						:to="{name: 'bookmarks.detail', params: {bookmarkId: source.id}}"
						class="jb-sources__title"
					>
						{{ source.title }}
					</RouterLink>
					<a
						:href="source.url"
						target="_blank"
						rel="noopener noreferrer"
						class="jb-sources__host"
					>
						{{ hostOf(source.url) }}
						<Icon icon="arrow-up-right-from-square" />
					</a>
				</div>
				<BaseButton
					v-if="canWrite"
					class="jb-sources__remove"
					:aria-label="$t('johnnysBrain.sources.remove', {title: source.title})"
					:title="$t('johnnysBrain.sources.remove', {title: source.title})"
					@click="unlink(source.id)"
				>
					<Icon icon="times" />
				</BaseButton>
			</li>
		</ul>
		<p
			v-else-if="!isPending"
			class="jb-sources__empty"
		>
			{{ $t('johnnysBrain.sources.empty') }}
		</p>

		<form
			v-if="canWrite"
			class="jb-sources__add"
			@submit.prevent="submit"
		>
			<label
				class="is-sr-only"
				:for="inputId"
			>{{ $t('johnnysBrain.sources.add') }}</label>
			<input
				:id="inputId"
				v-model="input"
				class="input"
				type="text"
				inputmode="url"
				autocomplete="off"
				autocapitalize="off"
				spellcheck="false"
				:placeholder="$t('johnnysBrain.sources.addPlaceholder')"
				:disabled="busy"
			>
			<XButton
				type="submit"
				:loading="busy"
				:disabled="!looksLikeLink(input)"
			>
				{{ $t('johnnysBrain.sources.addLink') }}
			</XButton>
			<ul
				v-if="matches.length"
				class="jb-sources__matches"
				role="listbox"
			>
				<li
					v-for="match in matches"
					:key="match.id"
				>
					<BaseButton
						class="jb-sources__match"
						@click="linkExisting(match.id)"
					>
						<span class="jb-sources__match-title">{{ match.title }}</span>
						<span class="jb-sources__match-host">{{ hostOf(match.url) }}</span>
					</BaseButton>
				</li>
			</ul>
		</form>
	</section>
</template>

<script setup lang="ts">
import {computed, reactive, ref, useId} from 'vue'
import {useI18n} from 'vue-i18n'
import {useQuery} from '@tanstack/vue-query'
import {refDebounced} from '@vueuse/core'

import BaseButton from '@/components/base/BaseButton.vue'
import XButton from '@/components/input/Button.vue'
import {useAuthStore} from '@/stores/auth'
import {success} from '@/message'
import {
	bookmarksQuery,
	hostOf,
	useLinkBookmarkProjectMutation,
	useLinkBookmarkTaskMutation,
	useUnlinkBookmarkProjectMutation,
	useUnlinkBookmarkTaskMutation,
} from '@/client/queries/bookmarks'
import {looksLikeLink, useSaveLink} from './useSaveLink'

const props = withDefaults(defineProps<{
	projectId?: number
	taskId?: number
	canWrite?: boolean
	showHeading?: boolean
}>(), {
	projectId: undefined,
	taskId: undefined,
	canWrite: false,
	showHeading: true,
})

const {t} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const inputId = useId()

const sourcesQuery = useQuery(computed(() => bookmarksQuery(
	props.taskId ? {task_id: props.taskId} : {project_id: props.projectId},
)))
const sources = computed(() => sourcesQuery.data.value ?? [])
const isPending = computed(() => sourcesQuery.isPending.value)
const brokenIcons = reactive(new Set<number>())

const input = ref('')
const search = refDebounced(computed(() => looksLikeLink(input.value) ? '' : input.value.trim()), 250)
const searchQuery = useQuery(computed(() => ({
	...bookmarksQuery({q: search.value}),
	enabled: props.canWrite && search.value.length >= 2,
})))
const matches = computed(() => {
	const linked = new Set(sources.value.map(s => s.id))
	const me = authStore.info?.id
	// Only your own bookmarks can be linked.
	return (searchQuery.data.value ?? [])
		.filter(b => !linked.has(b.id) && b.created_by?.id === me)
		.slice(0, 6)
})

const {save, isSaving} = useSaveLink()
const linkProject = useLinkBookmarkProjectMutation()
const linkTask = useLinkBookmarkTaskMutation()
const unlinkProject = useUnlinkBookmarkProjectMutation()
const unlinkTask = useUnlinkBookmarkTaskMutation()
const busy = computed(() => isSaving.value || linkProject.isPending.value || linkTask.isPending.value)

async function link(bookmarkId: number) {
	if (sources.value.some(s => s.id === bookmarkId)) return
	if (props.taskId) {
		await linkTask.mutateAsync({bookmarkId, taskId: props.taskId})
	} else if (props.projectId) {
		await linkProject.mutateAsync({bookmarkId, projectId: props.projectId})
	}
}

async function submit() {
	if (!looksLikeLink(input.value)) return
	const saved = await save(input.value)
	if (!saved) return
	try {
		await link(saved.bookmark.id)
	} catch {
		return
	}
	if (saved.alreadySaved) {
		success({message: t('johnnysBrain.sources.linkedExisting', {title: saved.bookmark.title})})
	}
	input.value = ''
}

async function linkExisting(bookmarkId: number) {
	try {
		await link(bookmarkId)
	} catch {
		return
	}
	input.value = ''
}

function unlink(bookmarkId: number) {
	if (props.taskId) {
		unlinkTask.mutate({bookmarkId, taskId: props.taskId})
	} else if (props.projectId) {
		unlinkProject.mutate({bookmarkId, projectId: props.projectId})
	}
}
</script>

<style lang="scss" scoped>
.jb-sources__count {
	margin-inline-start: .4rem;
	color: var(--grey-500);
	font-weight: 500;
	font-size: .85em;
}

.jb-sources__list {
	list-style: none;
	margin: 0 0 .75rem;
	padding: 0;
}

.jb-sources__item {
	display: flex;
	align-items: flex-start;
	gap: .65rem;
	padding: .55rem .25rem;
	border-block-end: 1px solid var(--jb-divider, var(--grey-200));
}

.jb-sources__icon {
	flex: 0 0 1.25rem;
	display: flex;
	align-items: center;
	justify-content: center;
	block-size: 1.4rem;
	color: var(--grey-500);

	img {
		border-radius: 3px;
	}
}

.jb-sources__body {
	flex: 1;
	min-inline-size: 0;
	display: flex;
	flex-direction: column;
}

.jb-sources__title {
	color: var(--text);
	font-weight: 500;
	overflow-wrap: anywhere;

	&:hover {
		color: var(--jb-green, var(--primary));
	}
}

.jb-sources__host {
	display: inline-flex;
	align-items: center;
	gap: .3rem;
	color: var(--grey-500);
	font-size: .8rem;

	svg {
		font-size: .65rem;
	}
}

.jb-sources__remove {
	flex-shrink: 0;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 1.9rem;
	block-size: 1.9rem;
	border-radius: 8px;
	color: var(--grey-500);

	&:hover {
		color: var(--danger);
		background: var(--grey-100);
	}
}

.jb-sources__empty {
	color: var(--grey-500);
	font-size: .9rem;
	margin-block-end: .75rem;
}

.jb-sources__add {
	position: relative;
	display: flex;
	gap: .5rem;

	.input {
		flex: 1;
	}
}

.jb-sources__matches {
	position: absolute;
	inset-block-start: calc(100% + .25rem);
	inset-inline: 0;
	z-index: 5;
	list-style: none;
	margin: 0;
	padding: .3rem;
	border-radius: 12px;
	border: 1px solid var(--grey-200);
	background: var(--scheme-main, var(--white));
	box-shadow: 0 10px 30px rgba(0, 0, 0, .35);
}

.jb-sources__match {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	inline-size: 100%;
	padding: .45rem .6rem;
	border-radius: 8px;
	text-align: start;

	&:hover,
	&:focus-visible {
		background: var(--grey-100);
	}
}

.jb-sources__match-title {
	color: var(--text);
	font-weight: 500;
}

.jb-sources__match-host {
	color: var(--grey-500);
	font-size: .8rem;
}
</style>
