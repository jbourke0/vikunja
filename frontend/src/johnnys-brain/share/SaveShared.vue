<template>
	<div class="jb-save is-max-width-desktop">
		<h2 class="title">
			{{ $t('johnnysBrain.share.saveTitle') }}
		</h2>

		<section
			v-if="result"
			class="jb-save__done"
			role="status"
		>
			<span class="jb-save__done-icon">
				<Icon icon="check" />
			</span>
			<h3 class="jb-save__done-heading">
				{{ result.alreadySaved ? $t('johnnysBrain.share.alreadySavedHeading') : $t('johnnysBrain.share.savedHeading') }}
			</h3>
			<p class="jb-save__done-title">
				{{ result.bookmark.title }}
			</p>
			<p
				v-if="result.linkedTo.length"
				class="jb-save__done-linked"
			>
				{{ $t('johnnysBrain.share.linkedTo', {where: result.linkedTo.join(', ')}) }}
			</p>
			<p
				v-if="closing"
				class="jb-save__hint"
			>
				{{ $t('johnnysBrain.share.closing') }}
			</p>
			<div class="jb-save__done-actions">
				<XButton :to="{name: 'bookmarks.detail', params: {bookmarkId: result.bookmark.id}}">
					{{ $t('johnnysBrain.share.open') }}
				</XButton>
				<XButton
					variant="secondary"
					@click="another"
				>
					{{ $t('johnnysBrain.share.another') }}
				</XButton>
				<XButton
					variant="tertiary"
					:to="{name: 'bookmarks.index'}"
				>
					{{ $t('johnnysBrain.share.done') }}
				</XButton>
			</div>
		</section>

		<form
			v-else
			class="jb-save__form"
			@submit.prevent="submit"
		>
			<p
				v-if="noLinkInShare"
				class="jb-save__notice"
			>
				{{ $t('johnnysBrain.share.noLink') }}
			</p>

			<div class="jb-save__field">
				<label
					class="jb-save__label"
					for="jb-save-link"
				>{{ $t('johnnysBrain.share.linkLabel') }}</label>
				<input
					id="jb-save-link"
					v-model="link"
					class="input"
					type="text"
					inputmode="url"
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
					:placeholder="$t('johnnysBrain.share.linkPlaceholder')"
				>
			</div>

			<p
				v-if="existing"
				class="jb-save__notice"
			>
				{{ $t('johnnysBrain.share.alreadySavedNotice', {title: existing.title}) }}
				<RouterLink :to="{name: 'bookmarks.detail', params: {bookmarkId: existing.id}}">
					{{ $t('johnnysBrain.share.openExisting') }}
				</RouterLink>
			</p>

			<div
				v-if="!existing"
				class="jb-save__field"
			>
				<label
					class="jb-save__label"
					for="jb-save-title"
				>{{ $t('johnnysBrain.share.titleLabel') }}</label>
				<input
					id="jb-save-title"
					v-model="title"
					class="input"
					type="text"
					:placeholder="$t('johnnysBrain.share.titlePlaceholder')"
				>
			</div>

			<div
				v-if="!existing"
				class="jb-save__field"
			>
				<span
					id="jb-save-status-label"
					class="jb-save__label"
				>{{ $t('johnnysBrain.share.statusLabel') }}</span>
				<div
					class="jb-segmented"
					role="radiogroup"
					aria-labelledby="jb-save-status-label"
				>
					<button
						v-for="option in STATUS_CHOICES"
						:key="option"
						type="button"
						role="radio"
						class="jb-segmented__option"
						:class="{'is-active': status === option}"
						:aria-checked="status === option"
						@click="status = option"
					>
						{{ $t(`johnnysBrain.bookmarks.status.${option}`) }}
					</button>
				</div>
			</div>

			<div
				v-if="writableProjects.length"
				class="jb-save__field"
			>
				<label
					class="jb-save__label"
					for="jb-save-project"
				>{{ $t('johnnysBrain.share.projectLabel') }}</label>
				<select
					id="jb-save-project"
					v-model.number="projectId"
					class="jb-save__select"
				>
					<option :value="0">
						{{ $t('johnnysBrain.share.noProject') }}
					</option>
					<option
						v-for="project in writableProjects"
						:key="project.id"
						:value="project.id"
					>
						{{ project.title }}
					</option>
				</select>
			</div>

			<div class="jb-save__field jb-save__task">
				<label
					class="jb-save__label"
					for="jb-save-task"
				>{{ $t('johnnysBrain.share.taskLabel') }}</label>
				<div
					v-if="task"
					class="jb-chip"
				>
					<Icon
						icon="check"
						class="jb-chip__task-icon"
					/>
					{{ task.title }}
					<BaseButton
						class="jb-chip__remove"
						:aria-label="$t('johnnysBrain.share.removeTask')"
						@click="task = null"
					>
						<Icon icon="times" />
					</BaseButton>
				</div>
				<template v-else>
					<input
						id="jb-save-task"
						v-model="taskSearch"
						class="input"
						type="text"
						autocomplete="off"
						:placeholder="$t('johnnysBrain.share.taskPlaceholder')"
					>
					<ul
						v-if="taskMatches.length"
						class="jb-save__matches"
					>
						<li
							v-for="match in taskMatches"
							:key="match.id"
						>
							<BaseButton
								class="jb-save__match"
								@click="pickTask(match)"
							>
								{{ match.title }}
							</BaseButton>
						</li>
					</ul>
				</template>
			</div>

			<XButton
				type="submit"
				class="jb-save__submit"
				:loading="isSaving || linking"
				:disabled="!looksLikeLink(link)"
			>
				{{ existing ? $t('johnnysBrain.share.linkExisting') : $t('johnnysBrain.share.save') }}
			</XButton>
		</form>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute} from 'vue-router'
import {useQuery} from '@tanstack/vue-query'
import {refDebounced, watchDebounced} from '@vueuse/core'

import BaseButton from '@/components/base/BaseButton.vue'
import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {useProjects} from '@/composables/useProjects'
import {tasksList} from '@/client/generated'
import {
	type BookmarkResponse,
	type BookmarkStatus,
	fetchBookmark,
	findBookmarkByUrl,
	useLinkBookmarkProjectMutation,
	useLinkBookmarkTaskMutation,
} from '@/client/queries/bookmarks'
import {looksLikeLink, useSaveLink, withScheme} from '@/johnnys-brain/bookmarks/useSaveLink'
import {extractSharedLink, sharedInputFromQuery} from './sharedLink'

type TaskChoice = {id: number, title: string}
type Result = {bookmark: BookmarkResponse, alreadySaved: boolean, linkedTo: string[]}

const STATUS_CHOICES: BookmarkStatus[] = ['to_read', 'reference']
const LAST_PROJECT_KEY = 'jb-share-last-project'

const {t} = useI18n({useScope: 'global'})
const route = useRoute()
useTitle(() => t('johnnysBrain.share.saveTitle'))

// What the sharing app sent. A share with no link in it still opens the screen, with an empty field.
const shared = extractSharedLink(sharedInputFromQuery(route.query))
const noLinkInShare = shared.url === '' && Object.keys(route.query).length > 0
const link = ref(shared.url)
const title = ref(shared.title)
const status = ref<BookmarkStatus>('to_read')

// Is this link already saved? Asked as the link settles, so typing and pasting both work.
const existing = ref<BookmarkResponse | undefined>(undefined)
watchDebounced(link, async value => {
	existing.value = undefined
	if (!looksLikeLink(value)) return
	try {
		const found = await findBookmarkByUrl(withScheme(value))
		if (found && withScheme(link.value) === withScheme(value)) {
			existing.value = found
		}
	} catch {
		// Not being able to look is not a reason to stop the save; the server refuses a copy anyway.
	}
}, {debounce: 300, immediate: true})

// Projects to offer. The list doesn't say which ones can be written to, so a project that turns
// out to be read-only is refused by the server when linking, and the bookmark is still saved.
const projectList = useProjects() // read through the object: destructuring would freeze an empty list
const writableProjects = computed(() => projectList.projectsArray
	.filter(p => p.id > 0 && !p.is_archived)
	.sort((a, b) => (a.title ?? '').localeCompare(b.title ?? '')))

function readLastProject(): number {
	try {
		return Number(localStorage.getItem(LAST_PROJECT_KEY)) || 0
	} catch {
		return 0
	}
}

const projectId = ref(0)
watch(writableProjects, projects => {
	// Offer the project used last time, once the list has loaded and only if it is still there.
	if (projectId.value === 0) {
		const last = readLastProject()
		if (projects.some(p => p.id === last)) projectId.value = last
	}
}, {immediate: true})

// Task search
const task = ref<TaskChoice | null>(null)
const taskSearch = ref('')
const debouncedTaskSearch = refDebounced(taskSearch, 250)
const taskQuery = useQuery(computed(() => ({
	queryKey: ['johnnysBrain', 'share', 'tasks', debouncedTaskSearch.value] as const,
	queryFn: async () => (await tasksList({query: {q: debouncedTaskSearch.value, per_page: 6}})).data,
	enabled: debouncedTaskSearch.value.trim().length >= 2,
})))
const taskMatches = computed<TaskChoice[]>(() => (taskQuery.data.value?.items ?? [])
	.filter(item => !item.done)
	.map(item => ({id: item.id ?? 0, title: item.title ?? ''})))

function pickTask(choice: TaskChoice) {
	task.value = choice
	taskSearch.value = ''
}

// Saving
const {save, isSaving} = useSaveLink()
const linkProject = useLinkBookmarkProjectMutation()
const linkTask = useLinkBookmarkTaskMutation()
const linking = computed(() => linkProject.isPending.value || linkTask.isPending.value)
const result = ref<Result | null>(null)
const closing = ref(false)

async function submit() {
	if (!looksLikeLink(link.value)) return

	const saved = await save(link.value, status.value, title.value)
	if (!saved) return

	// Link only to what is not linked already, so a repeat share never raises "already linked".
	const full = await fetchBookmark(saved.bookmark.id).catch(() => saved.bookmark)
	const linkedTo: string[] = []
	try {
		if (projectId.value > 0) {
			const project = writableProjects.value.find(p => p.id === projectId.value)
			if (!full.projects.some(p => p.id === projectId.value)) {
				await linkProject.mutateAsync({bookmarkId: full.id, projectId: projectId.value})
			}
			if (project) linkedTo.push(project.title ?? '')
		}
		if (task.value) {
			if (!full.tasks.some(x => x.id === task.value?.id)) {
				await linkTask.mutateAsync({bookmarkId: full.id, taskId: task.value.id})
			}
			linkedTo.push(task.value.title)
		}
	} catch {
		// The mutation has already shown why a link failed; the bookmark itself is saved.
	}

	try {
		if (projectId.value > 0) localStorage.setItem(LAST_PROJECT_KEY, String(projectId.value))
	} catch {
		// Remembering the project is a convenience.
	}

	result.value = {bookmark: saved.bookmark, alreadySaved: saved.alreadySaved, linkedTo}

	// A popup opened by the bookmarklet closes itself; a full window stays for the buttons.
	if (window.opener) {
		closing.value = true
		setTimeout(() => window.close(), 1500)
	}
}

function another() {
	result.value = null
	link.value = ''
	title.value = ''
	task.value = null
	status.value = 'to_read'
}
</script>

<style lang="scss" scoped>
.jb-save__form {
	display: flex;
	flex-direction: column;
	gap: 1.1rem;
	max-inline-size: 36rem;
}

.jb-save__field {
	display: flex;
	flex-direction: column;
	gap: .4rem;
	position: relative;
}

.jb-save__label {
	font-size: .8rem;
	font-weight: 600;
	text-transform: uppercase;
	letter-spacing: .06em;
	color: var(--grey-500);
}

.jb-save__select {
	appearance: none;
	min-block-size: 2.75rem;
	padding: .5rem .9rem;
	border-radius: var(--jb-radius-button, 12px);
	border: 1px solid var(--input-border-color, var(--grey-300));
	background: var(--input-background-color, var(--grey-100));
	color: var(--text);
	font: inherit;
}

.jb-save__notice {
	padding: .75rem .9rem;
	border-radius: 12px;
	border: 1px solid color-mix(in srgb, var(--jb-prio-high, #ff9a14) 45%, transparent);
	background: color-mix(in srgb, var(--jb-prio-high, #ff9a14) 7%, transparent);
	font-size: .9rem;

	a {
		margin-inline-start: .35rem;
		color: var(--jb-green);
		font-weight: 600;
	}
}

.jb-save__matches {
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

.jb-save__match {
	inline-size: 100%;
	padding: .55rem .7rem;
	border-radius: 8px;
	text-align: start;
	color: var(--text);

	&:hover,
	&:focus-visible {
		background: var(--grey-100);
	}
}

.jb-save__submit {
	align-self: flex-start;
	min-block-size: 2.75rem;
	padding-inline: 1.4rem;
}

.jb-save__done {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	gap: .5rem;
	padding: 1.5rem;
	max-inline-size: 36rem;
	border-radius: var(--jb-radius-card, 20px);
	border: 1px solid var(--grey-200);
	background: var(--grey-50);
}

.jb-save__done-icon {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 2.6rem;
	block-size: 2.6rem;
	border-radius: 50%;
	background: var(--jb-green);
	color: var(--jb-green-ink);
	font-size: 1.1rem;
}

.jb-save__done-heading {
	margin: .25rem 0 0;
}

.jb-save__done-title {
	font-weight: 500;
	overflow-wrap: anywhere;
}

.jb-save__done-linked,
.jb-save__hint {
	color: var(--grey-500);
	font-size: .9rem;
}

.jb-save__done-actions {
	display: flex;
	flex-wrap: wrap;
	gap: .5rem;
	margin-block-start: .75rem;
}

.jb-segmented {
	display: inline-flex;
	align-self: flex-start;
	padding: .2rem;
	border-radius: 999px;
	background: var(--grey-100);
	gap: .15rem;
}

.jb-segmented__option {
	border: 0;
	background: transparent;
	color: var(--grey-500);
	padding: .45rem 1rem;
	border-radius: 999px;
	font: inherit;
	font-size: .9rem;
	font-weight: 500;
	cursor: pointer;

	&.is-active {
		background: var(--jb-green);
		color: var(--jb-green-ink);
		font-weight: 600;
	}
}

.jb-chip {
	display: inline-flex;
	align-self: flex-start;
	align-items: center;
	gap: .4rem;
	padding: .3rem .45rem .3rem .75rem;
	border-radius: 999px;
	background: var(--grey-100);
	font-size: .9rem;
}

.jb-chip__task-icon {
	color: var(--grey-500);
	font-size: .7rem;
}

.jb-chip__remove {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 1.5rem;
	block-size: 1.5rem;
	border-radius: 50%;
	color: var(--grey-500);
	font-size: .7rem;

	&:hover {
		color: var(--text);
		background: var(--grey-200);
	}
}
</style>
