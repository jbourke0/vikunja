<!-- eslint-disable vue/no-v-html -->
<template>
	<div class="jb-bookmark is-max-width-desktop">
		<RouterLink
			:to="{name: 'bookmarks.index'}"
			class="jb-bookmark__back"
		>
			<Icon icon="arrow-left" />
			{{ $t('johnnysBrain.bookmarks.title') }}
		</RouterLink>

		<p
			v-if="bookmarkQueryResult.isError.value"
			class="jb-bookmark__missing"
		>
			{{ $t('johnnysBrain.bookmarks.notFound') }}
		</p>

		<template v-else-if="bookmark">
			<header class="jb-bookmark__header">
				<span class="jb-bookmark__icon">
					<img
						v-if="bookmark.favicon_url && !iconBroken"
						:src="bookmark.favicon_url"
						referrerpolicy="no-referrer"
						alt=""
						width="28"
						height="28"
						@error="iconBroken = true"
					>
					<Icon
						v-else
						icon="globe"
					/>
				</span>
				<div class="jb-bookmark__heading">
					<textarea
						v-if="canEdit"
						v-model="draft.title"
						class="jb-bookmark__title-input"
						rows="1"
						:aria-label="$t('johnnysBrain.bookmarks.titleLabel')"
						@keydown.enter.prevent="($event.target as HTMLTextAreaElement).blur()"
						@blur="saveIfChanged"
					/>
					<h1
						v-else
						class="title"
					>
						{{ bookmark.title }}
					</h1>
					<a
						:href="bookmark.url"
						target="_blank"
						rel="noopener noreferrer"
						class="jb-bookmark__url"
					>
						{{ hostOf(bookmark.url) }}
						<Icon icon="arrow-up-right-from-square" />
					</a>
				</div>
			</header>

			<img
				v-if="bookmark.image_url"
				:src="bookmark.image_url"
				referrerpolicy="no-referrer"
				alt=""
				class="jb-bookmark__image"
				loading="lazy"
			>

			<div
				class="jb-bookmark__page"
				:class="{'is-problem': pageProblem}"
			>
				<Icon :icon="pageProblem ? 'triangle-exclamation' : 'circle-info'" />
				<span class="jb-bookmark__page-text">{{ pageStatusText }}</span>
				<XButton
					v-if="canEdit"
					variant="tertiary"
					icon="rotate"
					:loading="refreshing"
					@click="refreshFromPage"
				>
					{{ $t('johnnysBrain.page.refresh') }}
				</XButton>
			</div>

			<div
				v-if="bookmark.has_snapshot"
				class="jb-bookmark__saved"
			>
				<button
					type="button"
					class="jb-bookmark__saved-toggle"
					:aria-expanded="showSaved"
					@click="showSaved = !showSaved"
				>
					<Icon :icon="showSaved ? 'chevron-up' : 'chevron-down'" />
					{{ showSaved ? $t('johnnysBrain.page.savedTextHide') : $t('johnnysBrain.page.savedTextShow') }}
				</button>
				<div
					v-if="showSaved"
					class="jb-bookmark__saved-body"
				>
					<p
						v-if="snapshotQueryResult.isPending.value"
						class="jb-muted"
					>
						{{ $t('johnnysBrain.page.savedTextLoading') }}
					</p>
					<template v-else-if="snapshot">
						<p class="jb-muted">
							{{ $t('johnnysBrain.page.savedTextWhen', {when: formatDisplayDate(snapshot.fetched_at)}) }}
						</p>
						<pre class="jb-bookmark__saved-text">{{ snapshot.text }}</pre>
					</template>
				</div>
			</div>

			<section
				v-if="duplicates.length"
				class="jb-bookmark__section jb-duplicates"
				:aria-label="$t('johnnysBrain.duplicates.title')"
			>
				<h3 class="jb-bookmark__label jb-duplicates__heading">
					<Icon icon="clone" />
					{{ $t('johnnysBrain.duplicates.title', {count: duplicates.length}, duplicates.length) }}
				</h3>
				<ul class="jb-duplicates__list">
					<li
						v-for="suggestion in duplicates"
						:key="suggestion.bookmark.id"
						class="jb-duplicates__item"
					>
						<div class="jb-duplicates__body">
							<RouterLink
								:to="{name: 'bookmarks.detail', params: {bookmarkId: suggestion.bookmark.id}}"
								class="jb-duplicates__title"
							>
								{{ suggestion.bookmark.title }}
							</RouterLink>
							<span class="jb-duplicates__meta">
								{{ hostOf(suggestion.bookmark.url) }}
								<span class="jb-duplicates__reason">{{ suggestion.reason === 'ai' ? $t('johnnysBrain.duplicates.reasonAi') : $t('johnnysBrain.duplicates.reasonSimilar') }}</span>
							</span>
							<span
								v-if="suggestion.explanation"
								class="jb-duplicates__explanation"
							>{{ suggestion.explanation }}</span>
						</div>
						<div
							v-if="canEdit"
							class="jb-duplicates__actions"
						>
							<XButton
								variant="secondary"
								:loading="mergeMutation.isPending.value && mergingId === suggestion.bookmark.id"
								@click="merge(suggestion.bookmark.id, suggestion.bookmark.title)"
							>
								{{ $t('johnnysBrain.duplicates.merge') }}
							</XButton>
							<XButton
								variant="tertiary"
								@click="dismiss(suggestion.bookmark.id)"
							>
								{{ $t('johnnysBrain.duplicates.dismiss') }}
							</XButton>
						</div>
					</li>
				</ul>
			</section>

			<section class="jb-bookmark__section">
				<div
					class="jb-segmented"
					role="radiogroup"
					:aria-label="$t('johnnysBrain.bookmarks.statusFilter')"
				>
					<button
						v-for="status in BOOKMARK_STATUSES"
						:key="status"
						type="button"
						role="radio"
						class="jb-segmented__option"
						:class="{'is-active': bookmark.status === status}"
						:aria-checked="bookmark.status === status"
						:disabled="!canEdit"
						@click="setStatus(status)"
					>
						{{ $t(`johnnysBrain.bookmarks.status.${status}`) }}
					</button>
				</div>
			</section>

			<section class="jb-bookmark__section">
				<h3 class="jb-bookmark__label">
					{{ $t('johnnysBrain.bookmarks.labels') }}
				</h3>
				<div class="jb-bookmark__labels">
					<span
						v-for="label in bookmark.labels"
						:key="label.id"
						class="jb-chip"
					>
						<span
							class="jb-dot"
							:style="{background: label.hex_color ? `#${label.hex_color}` : undefined}"
						/>
						{{ label.title }}
						<BaseButton
							v-if="canEdit"
							class="jb-chip__remove"
							:aria-label="$t('johnnysBrain.bookmarks.removeLabel', {label: label.title})"
							@click="removeLabel(label.id ?? 0)"
						>
							<Icon icon="times" />
						</BaseButton>
					</span>
					<select
						v-if="canEdit && availableLabels.length"
						class="jb-bookmark__add-label"
						:aria-label="$t('johnnysBrain.bookmarks.addLabel')"
						value=""
						@change="addLabel(($event.target as HTMLSelectElement))"
					>
						<option
							value=""
							disabled
						>
							+ {{ $t('johnnysBrain.bookmarks.addLabel') }}
						</option>
						<option
							v-for="label in availableLabels"
							:key="label.id"
							:value="label.id"
						>
							{{ label.title }}
						</option>
					</select>
					<span
						v-if="!bookmark.labels.length && !(canEdit && availableLabels.length)"
						class="jb-muted"
					>{{ $t('johnnysBrain.bookmarks.noLabels') }}</span>
				</div>
			</section>

			<section class="jb-bookmark__section">
				<h3 class="jb-bookmark__label">
					{{ $t('johnnysBrain.sources.linkedTo') }}
				</h3>
				<div class="jb-bookmark__labels">
					<span
						v-for="project in bookmark.projects"
						:key="`p${project.id}`"
						class="jb-chip"
					>
						<span
							class="jb-dot"
							:style="{background: project.hex_color ? `#${project.hex_color}` : undefined}"
						/>
						<RouterLink :to="{name: 'project.index', params: {projectId: project.id}}">
							{{ project.title }}
						</RouterLink>
						<BaseButton
							v-if="canEdit"
							class="jb-chip__remove"
							:aria-label="$t('johnnysBrain.sources.unlink', {title: project.title})"
							@click="unlinkProject.mutate({bookmarkId, projectId: project.id ?? 0})"
						>
							<Icon icon="times" />
						</BaseButton>
					</span>
					<span
						v-for="task in bookmark.tasks"
						:key="`t${task.id}`"
						class="jb-chip"
						:class="{'is-done': task.done}"
					>
						<Icon
							icon="check"
							class="jb-chip__task-icon"
						/>
						<RouterLink :to="{name: 'task.detail', params: {id: task.id}}">
							{{ task.title }}
						</RouterLink>
						<BaseButton
							v-if="canEdit"
							class="jb-chip__remove"
							:aria-label="$t('johnnysBrain.sources.unlink', {title: task.title})"
							@click="unlinkTask.mutate({bookmarkId, taskId: task.id ?? 0})"
						>
							<Icon icon="times" />
						</BaseButton>
					</span>
					<select
						v-if="canEdit && linkableProjects.length"
						class="jb-bookmark__add-label"
						:aria-label="$t('johnnysBrain.sources.linkProject')"
						value=""
						@change="linkToProject(($event.target as HTMLSelectElement))"
					>
						<option
							value=""
							disabled
						>
							+ {{ $t('johnnysBrain.sources.linkProject') }}
						</option>
						<option
							v-for="project in linkableProjects"
							:key="project.id"
							:value="project.id"
						>
							{{ project.title }}
						</option>
					</select>
					<span
						v-if="!bookmark.projects.length && !bookmark.tasks.length"
						class="jb-muted"
					>{{ $t('johnnysBrain.sources.notLinked') }}</span>
				</div>
			</section>

			<section class="jb-bookmark__section">
				<label
					class="jb-bookmark__label"
					for="jb-bookmark-description"
				>{{ $t('johnnysBrain.bookmarks.description') }}</label>
				<textarea
					v-if="canEdit"
					id="jb-bookmark-description"
					v-model="draft.description"
					class="textarea"
					rows="3"
					:placeholder="$t('johnnysBrain.bookmarks.descriptionPlaceholder')"
					@blur="saveIfChanged"
				/>
				<p
					v-else
					class="jb-bookmark__description"
				>
					{{ bookmark.description || $t('johnnysBrain.bookmarks.noDescription') }}
				</p>
				<label
					v-if="canEdit"
					class="jb-bookmark__label jb-bookmark__label--spaced"
					for="jb-bookmark-url-edit"
				>{{ $t('johnnysBrain.bookmarks.urlLabel') }}</label>
				<input
					v-if="canEdit"
					id="jb-bookmark-url-edit"
					v-model="draft.url"
					class="input"
					type="url"
					@blur="saveIfChanged"
				>
			</section>

			<section class="jb-bookmark__section">
				<h3 class="jb-bookmark__label">
					{{ $t('johnnysBrain.bookmarks.comments') }}
				</h3>
				<ul class="jb-comments">
					<li
						v-for="comment in comments"
						:key="comment.id"
						class="jb-comment"
					>
						<div class="jb-comment__meta">
							<strong>{{ comment.author?.name || comment.author?.username }}</strong>
							<time
								v-if="comment.created"
								:datetime="comment.created"
								:title="formatDateLong(comment.created)"
							>{{ formatDisplayDate(comment.created) }}</time>
							<BaseButton
								v-if="comment.author?.id === currentUserId"
								class="jb-comment__delete"
								@click="deleteComment(comment.id ?? 0)"
							>
								{{ $t('misc.delete') }}
							</BaseButton>
						</div>
						<!-- Comment HTML is sanitised with DOMPurify before rendering. -->
						<div
							class="jb-comment__body content"
							v-html="sanitize(comment.comment ?? '')"
						/>
					</li>
				</ul>
				<form
					class="jb-comment-form"
					@submit.prevent="addComment"
				>
					<label
						class="is-sr-only"
						for="jb-new-comment"
					>{{ $t('johnnysBrain.bookmarks.addComment') }}</label>
					<textarea
						id="jb-new-comment"
						v-model="newComment"
						class="textarea"
						rows="2"
						:placeholder="$t('johnnysBrain.bookmarks.commentPlaceholder')"
						@keydown.meta.enter="addComment"
						@keydown.ctrl.enter="addComment"
					/>
					<XButton
						type="submit"
						:loading="createCommentMutation.isPending.value"
						:disabled="newComment.trim() === ''"
					>
						{{ $t('johnnysBrain.bookmarks.addComment') }}
					</XButton>
				</form>
			</section>

			<footer
				v-if="canEdit"
				class="jb-bookmark__footer"
			>
				<span class="jb-muted">{{ $t('johnnysBrain.bookmarks.savedOn', {date: formatDateLong(bookmark.created)}) }}</span>
				<XButton
					variant="tertiary"
					danger
					icon="trash-alt"
					@click="remove"
				>
					{{ $t('johnnysBrain.bookmarks.delete') }}
				</XButton>
			</footer>
		</template>
	</div>
</template>

<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import {useQuery} from '@tanstack/vue-query'
import {useTimeoutFn} from '@vueuse/core'
import DOMPurify from 'dompurify'

import BaseButton from '@/components/base/BaseButton.vue'
import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {useLabels} from '@/composables/useLabels'
import {useAuthStore} from '@/stores/auth'
import {formatDateLong, formatDisplayDate} from '@/helpers/time/formatDate'
import {
	BOOKMARK_STATUSES,
	type BookmarkStatus,
	bookmarkCommentsQuery,
	bookmarkDuplicatesQuery,
	bookmarkQuery,
	bookmarkSnapshotQuery,
	hostOf,
	useAddBookmarkLabelMutation,
	useCreateBookmarkCommentMutation,
	useDeleteBookmarkCommentMutation,
	useDeleteBookmarkMutation,
	useDismissBookmarkDuplicateMutation,
	useLinkBookmarkProjectMutation,
	useMergeBookmarksMutation,
	useRefreshBookmarkMutation,
	useRemoveBookmarkLabelMutation,
	useUnlinkBookmarkProjectMutation,
	useUnlinkBookmarkTaskMutation,
	useUpdateBookmarkMutation,
} from '@/client/queries/bookmarks'
import {useProjects} from '@/composables/useProjects'

const props = defineProps<{
	bookmarkId: number
}>()

const {t} = useI18n({useScope: 'global'})
const router = useRouter()
const authStore = useAuthStore()
const currentUserId = computed(() => authStore.info?.id)

// While a refresh runs, ask again every moment: the server fetches the page in the background.
const refreshing = ref(false)
const bookmarkQueryResult = useQuery(computed(() => ({
	...bookmarkQuery(props.bookmarkId),
	meta: {handlesError: true},
	refetchInterval: refreshing.value ? 1500 : false,
})))
const bookmark = computed(() => bookmarkQueryResult.data.value)
const canEdit = computed(() => (bookmark.value?.max_permission ?? 0) >= 2)
const commentsQueryResult = useQuery(computed(() => bookmarkCommentsQuery(props.bookmarkId)))
const comments = computed(() => commentsQueryResult.data.value ?? [])

useTitle(() => bookmark.value?.title ?? t('johnnysBrain.bookmarks.title'))

const iconBroken = ref(false)

// Page status and refresh
const refreshMutation = useRefreshBookmarkMutation()
const checkedAtBeforeRefresh = ref<string | null | undefined>(undefined)
const {start: startRefreshTimeout, stop: stopRefreshTimeout} = useTimeoutFn(() => {
	refreshing.value = false
}, 20_000, {immediate: false})

watch(() => bookmark.value?.checked_at, checkedAt => {
	if (refreshing.value && checkedAt !== checkedAtBeforeRefresh.value) {
		refreshing.value = false
		stopRefreshTimeout()
	}
})

async function refreshFromPage() {
	checkedAtBeforeRefresh.value = bookmark.value?.checked_at
	refreshing.value = true
	try {
		await refreshMutation.mutateAsync(props.bookmarkId)
		startRefreshTimeout()
	} catch {
		refreshing.value = false // the mutation has already shown why
	}
}

// Saved page text, fetched only once it is opened
const showSaved = ref(false)
const snapshotQueryResult = useQuery(computed(() => bookmarkSnapshotQuery(props.bookmarkId, showSaved.value && (bookmark.value?.has_snapshot ?? false))))
const snapshot = computed(() => snapshotQueryResult.data.value)
watch(() => bookmark.value?.checked_at, () => {
	// A refresh may have saved newer text.
	if (showSaved.value) void snapshotQueryResult.refetch()
})

const pageProblem = computed(() => {
	const status = bookmark.value?.link_status ?? 0
	return status >= 400 || status === -1
})

const pageStatusText = computed(() => {
	const b = bookmark.value
	if (!b) return ''
	if (refreshing.value) return t('johnnysBrain.page.fetching')
	if (!b.checked_at) return t('johnnysBrain.page.notChecked')
	if (b.link_status === -2) return t('johnnysBrain.page.private')

	const when = formatDisplayDate(b.checked_at)
	let text: string
	if (b.link_status === -1) {
		text = t('johnnysBrain.page.unreachable', {when})
	} else if (b.link_status >= 400) {
		text = t('johnnysBrain.page.error', {status: b.link_status, when})
	} else {
		text = t('johnnysBrain.page.checkedOk', {when})
	}
	if (b.has_snapshot) {
		text += ' ' + t(pageProblem.value ? 'johnnysBrain.page.copyKept' : 'johnnysBrain.page.textSaved')
	}
	return text
})

const draft = reactive({title: '', description: '', url: ''})
watch(bookmark, value => {
	if (!value) return
	draft.title = value.title
	draft.description = value.description ?? ''
	draft.url = value.url
}, {immediate: true})

const updateMutation = useUpdateBookmarkMutation()

function saveIfChanged() {
	const current = bookmark.value
	if (!current) return
	if (draft.title.trim() === current.title && draft.description === (current.description ?? '') && draft.url.trim() === current.url) {
		return
	}
	updateMutation.mutate({...current, title: draft.title.trim(), description: draft.description, url: draft.url.trim()})
}

function setStatus(status: BookmarkStatus) {
	const current = bookmark.value
	if (!current || current.status === status) return
	updateMutation.mutate({...current, status})
}

const {labels: allLabels} = useLabels()
const availableLabels = computed(() => {
	const onBookmark = new Set((bookmark.value?.labels ?? []).map(l => l.id))
	return allLabels.value.filter(l => !onBookmark.has(l.id))
})
const addLabelMutation = useAddBookmarkLabelMutation()
const removeLabelMutation = useRemoveBookmarkLabelMutation()

function addLabel(select: HTMLSelectElement) {
	const labelId = Number(select.value)
	select.value = ''
	if (labelId) addLabelMutation.mutate({bookmarkId: props.bookmarkId, labelId})
}

function removeLabel(labelId: number) {
	removeLabelMutation.mutate({bookmarkId: props.bookmarkId, labelId})
}

function sanitize(html: string): string {
	return DOMPurify.sanitize(html)
}

function escapeHtml(text: string): string {
	const div = document.createElement('div')
	div.textContent = text
	return div.innerHTML
}

const newComment = ref('')
const createCommentMutation = useCreateBookmarkCommentMutation()
const deleteCommentMutation = useDeleteBookmarkCommentMutation()

async function addComment() {
	const text = newComment.value.trim()
	if (text === '') return
	const html = text
		.split(/\n{2,}/)
		.map(paragraph => `<p>${escapeHtml(paragraph).replace(/\n/g, '<br>')}</p>`)
		.join('')
	try {
		await createCommentMutation.mutateAsync({bookmarkId: props.bookmarkId, comment: html})
	} catch {
		return
	}
	newComment.value = ''
}

function deleteComment(commentId: number) {
	if (!window.confirm(t('johnnysBrain.bookmarks.confirmDeleteComment'))) return
	deleteCommentMutation.mutate({bookmarkId: props.bookmarkId, commentId})
}

// Sources
const bookmarkId = computed(() => props.bookmarkId)
const projectList = useProjects()
const linkableProjects = computed(() => {
	const linked = new Set((bookmark.value?.projects ?? []).map(p => p.id))
	return projectList.projectsArray
		.filter(p => p.id > 0 && !p.is_archived && !linked.has(p.id))
		.sort((a, b) => (a.title ?? '').localeCompare(b.title ?? ''))
})
const linkProject = useLinkBookmarkProjectMutation()
const unlinkProject = useUnlinkBookmarkProjectMutation()
const unlinkTask = useUnlinkBookmarkTaskMutation()

function linkToProject(select: HTMLSelectElement) {
	const projectId = Number(select.value)
	select.value = ''
	if (projectId) linkProject.mutate({bookmarkId: props.bookmarkId, projectId})
}

// Duplicates
const duplicatesQueryResult = useQuery(computed(() => bookmarkDuplicatesQuery(props.bookmarkId)))
const duplicates = computed(() => duplicatesQueryResult.data.value ?? [])
const dismissMutation = useDismissBookmarkDuplicateMutation()
const mergeMutation = useMergeBookmarksMutation()
const mergingId = ref<number | null>(null)

function dismiss(otherId: number) {
	dismissMutation.mutate({bookmarkId: props.bookmarkId, otherId})
}

async function merge(otherId: number, otherTitle: string) {
	if (!window.confirm(t('johnnysBrain.duplicates.confirmMerge', {title: otherTitle}))) return
	mergingId.value = otherId
	try {
		await mergeMutation.mutateAsync({bookmarkId: props.bookmarkId, otherId})
	} catch {
		// toasted by the mutation
	} finally {
		mergingId.value = null
	}
}

const deleteMutation = useDeleteBookmarkMutation()

async function remove() {
	if (!window.confirm(t('johnnysBrain.bookmarks.confirmDelete'))) return
	try {
		await deleteMutation.mutateAsync(props.bookmarkId)
	} catch {
		return
	}
	router.push({name: 'bookmarks.index'})
}
</script>

<style lang="scss" scoped>
.jb-bookmark__back {
	display: inline-flex;
	align-items: center;
	gap: .4rem;
	color: var(--grey-500);
	font-size: .9rem;
	margin-block-end: 1rem;

	&:hover {
		color: var(--jb-green);
	}
}

.jb-bookmark__missing {
	color: var(--grey-500);
	padding-block: 2rem;
}

.jb-bookmark__header {
	display: flex;
	align-items: flex-start;
	gap: .85rem;
	margin-block-end: 1rem;
}

.jb-bookmark__icon {
	flex: 0 0 2.25rem;
	block-size: 2.25rem;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: 10px;
	background: var(--grey-100);
	color: var(--grey-500);
	margin-block-start: .2rem;

	img {
		border-radius: 6px;
	}
}

.jb-bookmark__heading {
	flex: 1;
	min-inline-size: 0;

	.title {
		margin: 0;
		font-size: 1.5rem;
		overflow-wrap: anywhere;
	}
}

.jb-bookmark__title-input {
	display: block;
	inline-size: 100%;
	resize: none;
	overflow: hidden;
	field-sizing: content;
	line-height: 1.25;
	background: transparent;
	border: 0;
	border-block-end: 1px solid transparent;
	color: var(--text);
	font-family: var(--jb-font-heading);
	font-weight: 700;
	font-size: 1.5rem;
	letter-spacing: -.015em;
	padding: 0 0 .1rem;

	&:hover {
		border-block-end-color: var(--grey-300);
	}

	&:focus {
		border-block-end-color: var(--jb-green);
		outline: none;
	}
}

.jb-bookmark__url {
	display: inline-flex;
	align-items: center;
	gap: .35rem;
	color: var(--jb-green);
	font-size: .9rem;
	overflow-wrap: anywhere;

	.icon,
	svg {
		font-size: .75rem;
	}
}

.jb-bookmark__image {
	display: block;
	inline-size: 100%;
	max-block-size: 260px;
	object-fit: cover;
	border-radius: var(--jb-radius-card);
	margin-block-end: 1.25rem;
	background: var(--grey-100);
}

.jb-bookmark__section {
	margin-block-end: 1.5rem;
}

.jb-bookmark__label {
	display: block;
	font-family: var(--jb-font-body);
	font-size: .8rem !important;
	font-weight: 600 !important;
	text-transform: uppercase;
	letter-spacing: .06em;
	color: var(--grey-500);
	margin-block-end: .5rem;
}

.jb-bookmark__label--spaced {
	margin-block-start: 1rem;
}

.jb-segmented {
	display: inline-flex;
	padding: .2rem;
	border-radius: 999px;
	background: var(--grey-100);
	gap: .15rem;
}

.jb-segmented__option {
	border: 0;
	background: transparent;
	color: var(--grey-500);
	padding: .4rem .9rem;
	border-radius: 999px;
	font: inherit;
	font-size: .9rem;
	font-weight: 500;
	cursor: pointer;

	&:hover:not(:disabled) {
		color: var(--text);
	}

	&.is-active {
		background: var(--jb-green);
		color: var(--jb-green-ink);
		font-weight: 600;
	}

	&:disabled {
		cursor: default;
	}
}

.jb-bookmark__labels {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .4rem;
}

.jb-chip {
	display: inline-flex;
	align-items: center;
	gap: .4rem;
	padding: .25rem .4rem .25rem .7rem;
	border-radius: 999px;
	background: var(--grey-100);
	font-size: .85rem;
}

.jb-chip__remove {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 1.4rem;
	block-size: 1.4rem;
	border-radius: 50%;
	color: var(--grey-500);
	font-size: .7rem;

	&:hover {
		color: var(--text);
		background: var(--grey-200);
	}
}

.jb-dot {
	inline-size: .5rem;
	block-size: .5rem;
	border-radius: 50%;
	background: var(--grey-400);
}

.jb-bookmark__add-label {
	appearance: none;
	border: 1px dashed var(--grey-300);
	background: transparent;
	color: var(--grey-500);
	border-radius: 999px;
	padding: .3rem .8rem;
	font: inherit;
	font-size: .85rem;
	cursor: pointer;

	&:hover,
	&:focus {
		border-color: var(--jb-green);
		color: var(--jb-green);
	}
}

.jb-bookmark__description {
	white-space: pre-wrap;
	color: var(--text);
}

.jb-muted {
	color: var(--grey-500);
	font-size: .85rem;
}

.jb-comments {
	list-style: none;
	margin: 0 0 1rem;
	padding: 0;
}

.jb-comment {
	padding-block: .75rem;
	border-block-end: 1px solid var(--jb-divider);
}

.jb-comment__meta {
	display: flex;
	align-items: baseline;
	gap: .6rem;
	font-size: .85rem;

	time {
		color: var(--grey-500);
	}
}

.jb-comment__delete {
	margin-inline-start: auto;
	color: var(--grey-500);
	font-size: .8rem;

	&:hover {
		color: var(--danger);
	}
}

.jb-comment__body {
	margin-block-start: .3rem;
	font-size: .95rem;

	:deep(p:last-child) {
		margin-block-end: 0;
	}
}

.jb-comment-form {
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	gap: .5rem;

	.textarea {
		inline-size: 100%;
	}
}

.jb-bookmark__footer {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	padding-block-start: 1rem;
	border-block-start: 1px solid var(--jb-divider);
}
.jb-duplicates {
	padding: .9rem 1rem;
	border-radius: var(--jb-radius-card, 16px);
	border: 1px solid color-mix(in srgb, var(--jb-prio-high, #ff9a14) 45%, transparent);
	background: color-mix(in srgb, var(--jb-prio-high, #ff9a14) 7%, transparent);
}

.jb-duplicates__heading {
	display: flex;
	align-items: center;
	gap: .45rem;
	color: var(--jb-prio-high, #ff9a14);
}

.jb-duplicates__list {
	list-style: none;
	margin: 0;
	padding: 0;
}

.jb-duplicates__item {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .5rem 1rem;
	padding-block: .55rem;

	& + & {
		border-block-start: 1px solid var(--jb-divider, var(--grey-200));
	}
}

.jb-duplicates__body {
	flex: 1 1 16rem;
	min-inline-size: 0;
	display: flex;
	flex-direction: column;
	gap: .1rem;
}

.jb-duplicates__title {
	color: var(--text);
	font-weight: 600;
	overflow-wrap: anywhere;

	&:hover {
		color: var(--jb-green);
	}
}

.jb-duplicates__meta {
	display: flex;
	gap: .6rem;
	color: var(--grey-500);
	font-size: .8rem;
}

.jb-duplicates__reason {
	color: var(--jb-prio-high, #ff9a14);
}

.jb-duplicates__explanation {
	color: var(--text);
	font-size: .85rem;
}

.jb-duplicates__actions {
	display: flex;
	gap: .4rem;
}

.jb-chip.is-done a {
	text-decoration: line-through;
	color: var(--grey-500);
}

.jb-chip a {
	color: var(--text);

	&:hover {
		color: var(--jb-green);
	}
}

.jb-chip__task-icon {
	color: var(--grey-500);
	font-size: .7rem;
}
.jb-bookmark__page {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .5rem .75rem;
	margin-block-end: 1.25rem;
	color: var(--grey-500);
	font-size: .85rem;

	&.is-problem {
		color: var(--jb-prio-high, #ff9a14);
	}
}

.jb-bookmark__page-text {
	flex: 1 1 14rem;
}
.jb-bookmark__saved {
	margin-block: -.5rem 1.25rem;
}

.jb-bookmark__saved-toggle {
	display: inline-flex;
	align-items: center;
	gap: .4rem;
	padding: 0;
	border: 0;
	background: transparent;
	color: var(--jb-green);
	font: inherit;
	font-size: .85rem;
	font-weight: 500;
	cursor: pointer;
}

.jb-bookmark__saved-body {
	margin-block-start: .6rem;
	padding: .9rem 1rem;
	border-radius: 14px;
	background: var(--grey-100);
}

.jb-bookmark__saved-text {
	margin: .4rem 0 0;
	max-block-size: 24rem;
	overflow: auto;
	padding: 0;
	background: transparent;
	color: var(--text);
	font: inherit;
	font-size: .9rem;
	line-height: 1.5;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
}
</style>
