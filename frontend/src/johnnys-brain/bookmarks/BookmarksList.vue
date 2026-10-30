<template>
	<div class="jb-bookmarks is-max-width-desktop">
		<header class="jb-bookmarks__header">
			<h2 class="title">
				{{ $t('johnnysBrain.bookmarks.title') }}
			</h2>
			<span
				v-if="!isPending"
				class="jb-bookmarks__count"
			>{{ $t('johnnysBrain.bookmarks.count', {count: bookmarks.length}, bookmarks.length) }}</span>
			<span class="jb-bookmarks__links">
				<RouterLink
					:to="{name: 'bookmarks.import'}"
					class="jb-bookmarks__share"
				>
					{{ $t('johnnysBrain.bookmarks.importLink') }}
				</RouterLink>
				<RouterLink
					:to="{name: 'bookmarks.share'}"
					class="jb-bookmarks__share"
				>
					{{ $t('johnnysBrain.share.setupLink') }}
				</RouterLink>
			</span>
		</header>

		<form
			class="jb-bookmarks__save"
			@submit.prevent="save"
		>
			<label
				class="is-sr-only"
				for="jb-bookmark-url"
			>{{ $t('johnnysBrain.bookmarks.urlLabel') }}</label>
			<span class="jb-bookmarks__save-icon">
				<Icon icon="link" />
			</span>
			<input
				id="jb-bookmark-url"
				v-model="newUrl"
				class="input"
				type="text"
				inputmode="url"
				autocomplete="off"
				autocapitalize="off"
				spellcheck="false"
				:placeholder="$t('johnnysBrain.bookmarks.urlPlaceholder')"
				:disabled="isSaving"
			>
			<XButton
				type="submit"
				:loading="isSaving"
				:disabled="newUrl.trim() === ''"
			>
				{{ $t('johnnysBrain.bookmarks.save') }}
			</XButton>
		</form>

		<div class="jb-bookmarks__toolbar">
			<nav
				class="jb-bookmarks__tabs"
				:aria-label="$t('johnnysBrain.bookmarks.statusFilter')"
			>
				<RouterLink
					v-for="tab in tabs"
					:key="tab.key"
					:to="{name: 'bookmarks.index', query: tab.query}"
					class="jb-bookmarks__tab"
					:class="{'is-active': activeTab === tab.key}"
					active-class=""
					exact-active-class=""
				>
					{{ tab.label }}
					<span
						v-if="tab.key === 'broken'"
						class="jb-bookmarks__tab-count"
					>{{ brokenCount }}</span>
				</RouterLink>
			</nav>
			<label class="jb-bookmarks__search">
				<span class="is-sr-only">{{ $t('johnnysBrain.bookmarks.search') }}</span>
				<Icon icon="search" />
				<input
					v-model="search"
					class="input"
					type="search"
					:placeholder="$t('johnnysBrain.bookmarks.search')"
				>
			</label>
		</div>

		<p
			v-if="!isPending && bookmarks.length === 0"
			class="jb-bookmarks__empty"
		>
			{{ search.trim() ? $t('johnnysBrain.bookmarks.noMatches') : $t('johnnysBrain.bookmarks.empty') }}
		</p>

		<ul
			v-else
			class="jb-bookmarks__list"
			:class="{'is-loading': showLoading}"
		>
			<li
				v-for="bookmark in visibleBookmarks"
				:key="bookmark.id"
				class="jb-bookmark-row"
			>
				<span class="jb-bookmark-row__icon">
					<img
						v-if="bookmark.favicon_url && !brokenIcons.has(bookmark.id)"
						:src="bookmark.favicon_url"
						referrerpolicy="no-referrer"
						alt=""
						width="20"
						height="20"
						loading="lazy"
						@error="brokenIcons.add(bookmark.id)"
					>
					<Icon
						v-else
						icon="globe"
					/>
				</span>
				<div class="jb-bookmark-row__body">
					<RouterLink
						:to="{name: 'bookmarks.detail', params: {bookmarkId: bookmark.id}}"
						class="jb-bookmark-row__title"
					>
						{{ bookmark.title }}
					</RouterLink>
					<p
						v-if="bookmark.description"
						class="jb-bookmark-row__note"
					>
						{{ bookmark.description }}
					</p>
					<div class="jb-bookmark-row__meta">
						<span>{{ hostOf(bookmark.url) }}</span>
						<span
							v-for="label in bookmark.labels"
							:key="label.id"
							class="jb-bookmark-row__label"
						>
							<span
								class="jb-dot"
								:style="{background: label.hex_color ? `#${label.hex_color}` : undefined}"
							/>
							{{ label.title }}
						</span>
						<span
							v-if="!status"
							class="jb-bookmark-row__status"
						>{{ statusLabel(bookmark.status) }}</span>
						<span
							v-if="pageProblem(bookmark.link_status)"
							class="jb-bookmark-row__gone"
						>
							<Icon icon="triangle-exclamation" />
							{{ $t(`johnnysBrain.page.problem.${pageProblem(bookmark.link_status)}`) }}
						</span>
						<span
							v-if="bookmark.duplicate_count"
							class="jb-bookmark-row__duplicate"
						>
							<Icon icon="clone" />
							{{ $t('johnnysBrain.duplicates.badge') }}
						</span>
					</div>
				</div>
				<div class="jb-bookmark-row__actions">
					<a
						:href="bookmark.url"
						target="_blank"
						rel="noopener noreferrer"
						class="jb-icon-button"
						:title="$t('johnnysBrain.bookmarks.open')"
					>
						<Icon icon="arrow-up-right-from-square" />
						<span class="is-sr-only">{{ $t('johnnysBrain.bookmarks.open') }}</span>
					</a>
					<BaseButton
						v-if="bookmark.status !== 'archived'"
						class="jb-icon-button"
						:title="$t('johnnysBrain.bookmarks.archive')"
						@click="setStatus(bookmark, 'archived')"
					>
						<Icon icon="archive" />
						<span class="is-sr-only">{{ $t('johnnysBrain.bookmarks.archive') }}</span>
					</BaseButton>
					<BaseButton
						class="jb-icon-button jb-icon-button--danger"
						:title="$t('johnnysBrain.bookmarks.delete')"
						@click="removeBookmark(bookmark)"
					>
						<Icon icon="trash-alt" />
						<span class="is-sr-only">{{ $t('johnnysBrain.bookmarks.delete') }}</span>
					</BaseButton>
				</div>
			</li>
		</ul>
	</div>
</template>

<script setup lang="ts">
import {computed, onBeforeUnmount, reactive, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useQuery, useQueryClient} from '@tanstack/vue-query'
import {notify} from '@kyvg/vue3-notification'
import {refDebounced} from '@vueuse/core'

import BaseButton from '@/components/base/BaseButton.vue'
import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {useDelayedLoading} from '@/composables/useDelayedLoading'
import {
	type BookmarkResponse,
	type BookmarkStatus,
	bookmarksQuery,
	bookmarkKeys,
	brokenCountQuery,
	hostOf,
	pageProblem,
	useUpdateBookmarkMutation,
} from '@/client/queries/bookmarks'
import {useRouter} from 'vue-router'
import {error, success} from '@/message'
import {bookmarksDelete} from '@/client/generated'
import {useSaveLink} from './useSaveLink'

const props = defineProps<{
	status?: BookmarkStatus
	broken?: boolean
}>()

const activeTab = computed(() => props.broken ? 'broken' : (props.status ?? 'all'))

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('johnnysBrain.bookmarks.title'))

const tabs = computed(() => [
	{key: 'to_read', query: {status: 'to_read'}, label: t('johnnysBrain.bookmarks.status.to_read')},
	{key: 'reference', query: {status: 'reference'}, label: t('johnnysBrain.bookmarks.status.reference')},
	{key: 'archived', query: {status: 'archived'}, label: t('johnnysBrain.bookmarks.status.archived')},
	{key: 'all', query: {}, label: t('johnnysBrain.bookmarks.all')},
	// Only worth a tab once a link check has found something.
	...(brokenCount.value > 0 ? [{key: 'broken', query: {broken: 'true'}, label: t('johnnysBrain.bookmarks.broken')}] : []),
])

function statusLabel(status: BookmarkStatus) {
	return t(`johnnysBrain.bookmarks.status.${status}`)
}

const search = ref('')
const debouncedSearch = refDebounced(search, 250)

const query = useQuery(computed(() => bookmarksQuery({status: props.status, q: debouncedSearch.value, broken: props.broken})))
const bookmarks = computed(() => query.data.value ?? [])
const brokenCountQueryResult = useQuery(brokenCountQuery())
const brokenCount = computed(() => brokenCountQueryResult.data.value ?? 0)
const isPending = computed(() => query.isPending.value)
const showLoading = useDelayedLoading(computed(() => query.isFetching.value))

const brokenIcons = reactive(new Set<number>())

const newUrl = ref('')
const router = useRouter()
const {save: saveLink, isSaving} = useSaveLink()

async function save() {
	if (newUrl.value.trim() === '') return
	const saved = await saveLink(newUrl.value, props.status ?? 'to_read')
	if (!saved) return
	newUrl.value = ''
	if (saved.alreadySaved) {
		success({message: t('johnnysBrain.bookmarks.alreadySaved')})
		router.push({name: 'bookmarks.detail', params: {bookmarkId: saved.bookmark.id}})
	}
}

// Quick delete: the row goes at once and the delete itself waits a few seconds, so Undo is a
// real undo (labels, notes and links are still there) rather than a re-create.
const UNDO_MS = 5000
const queryClient = useQueryClient()
const pendingDeletes = reactive(new Map<number, ReturnType<typeof setTimeout>>())
const visibleBookmarks = computed(() => bookmarks.value.filter(b => !pendingDeletes.has(b.id as number)))

async function commitDelete(id: number) {
	const timer = pendingDeletes.get(id)
	if (timer === undefined) return
	clearTimeout(timer)
	try {
		await bookmarksDelete({path: {id}})
		queryClient.removeQueries({queryKey: bookmarkKeys.detail(id), exact: true})
	} catch (e) {
		error(e)
	} finally {
		pendingDeletes.delete(id)
		await Promise.all([
			queryClient.invalidateQueries({queryKey: bookmarkKeys.lists()}),
			queryClient.invalidateQueries({queryKey: bookmarkKeys.brokenCount()}),
		])
	}
}

function removeBookmark(bookmark: BookmarkResponse) {
	const id = bookmark.id as number
	if (pendingDeletes.has(id)) return
	pendingDeletes.set(id, setTimeout(() => commitDelete(id), UNDO_MS))
	notify({
		type: 'success',
		title: t('johnnysBrain.bookmarks.deletedTitle'),
		text: t('johnnysBrain.bookmarks.deletedUndo', {title: bookmark.title}),
		duration: UNDO_MS,
		data: {actions: [{title: t('johnnysBrain.bookmarks.undo'), callback: () => {
			clearTimeout(pendingDeletes.get(id))
			pendingDeletes.delete(id)
		}}]},
	})
}

// Leaving the page keeps the delete the person asked for.
onBeforeUnmount(() => {
	for (const id of Array.from(pendingDeletes.keys())) commitDelete(id)
})

const updateMutation = useUpdateBookmarkMutation()

function setStatus(bookmark: BookmarkResponse, status: BookmarkStatus) {
	updateMutation.mutate({...bookmark, status})
}
</script>

<style lang="scss" scoped>
.jb-bookmarks__header {
	display: flex;
	align-items: baseline;
	gap: .75rem;
	margin-block-end: 1rem;

	.title {
		margin: 0;
	}
}

.jb-bookmarks__count {
	color: var(--grey-500);
	font-size: .9rem;
}

.jb-bookmarks__links {
	display: flex;
	flex-wrap: wrap;
	gap: .25rem 1.1rem;
	margin-inline-start: auto;
}

.jb-bookmarks__share {
	color: var(--jb-green);
	font-size: .9rem;
	font-weight: 500;
}

.jb-bookmarks__save {
	display: flex;
	align-items: center;
	gap: .5rem;
	position: relative;
	margin-block-end: 1rem;

	.input {
		flex: 1;
		padding-inline-start: 2.25rem;
		block-size: 2.75rem;
	}
}

.jb-bookmarks__save-icon {
	position: absolute;
	inset-inline-start: .85rem;
	z-index: 1;
	color: var(--grey-500);
	pointer-events: none;
}

.jb-bookmarks__toolbar {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: space-between;
	gap: .75rem;
	margin-block-end: .5rem;
}

.jb-bookmarks__tabs {
	display: flex;
	gap: .25rem;
	overflow-x: auto;
}

.jb-bookmarks__tab-count {
	margin-inline-start: .3rem;
	padding: .05rem .45rem;
	border-radius: 999px;
	background: color-mix(in srgb, var(--jb-prio-high, #ff9a14) 25%, transparent);
	color: var(--jb-prio-high, #ff9a14);
	font-size: .75rem;
	font-weight: 600;
}

.jb-bookmarks__tab {
	padding: .35rem .8rem;
	border-radius: 999px;
	color: var(--grey-500);
	font-size: .9rem;
	font-weight: 500;
	white-space: nowrap;

	&:hover {
		color: var(--text);
		background: var(--grey-100);
	}

	&.is-active {
		color: var(--jb-green);
		background: rgba(var(--jb-green-rgb), .14);
		font-weight: 600;
	}
}

.jb-bookmarks__search {
	display: flex;
	align-items: center;
	gap: .5rem;
	color: var(--grey-500);
	flex: 0 1 16rem;

	.input {
		block-size: 2.25rem;
	}
}

.jb-bookmarks__empty {
	color: var(--grey-500);
	padding-block: 2rem;
	text-align: center;
}

.jb-bookmarks__list {
	list-style: none;
	margin: 0;
	padding: 0;
	transition: opacity 120ms ease;

	&.is-loading {
		opacity: .6;
	}
}

.jb-bookmark-row {
	display: flex;
	align-items: flex-start;
	gap: .75rem;
	padding: .75rem .4rem;
	border-block-end: 1px solid var(--jb-divider);

	&:hover {
		background: var(--jb-row-hover);
	}
}

.jb-bookmark-row__icon {
	flex: 0 0 1.5rem;
	display: flex;
	align-items: center;
	justify-content: center;
	block-size: 1.5rem;
	color: var(--grey-500);

	img {
		border-radius: 4px;
	}
}

.jb-bookmark-row__body {
	flex: 1;
	min-inline-size: 0;
}

.jb-bookmark-row__title {
	display: block;
	color: var(--text);
	font-size: .95rem;
	font-weight: 500;
	overflow-wrap: anywhere;

	&:hover {
		color: var(--jb-green);
	}
}

.jb-bookmark-row__note {
	margin: .15rem 0 .1rem;
	color: var(--grey-500);
	font-size: .85rem;
	line-height: 1.4;
	display: -webkit-box;
	-webkit-line-clamp: 2;
	line-clamp: 2;
	-webkit-box-orient: vertical;
	overflow: hidden;
	overflow-wrap: anywhere;
}

.jb-bookmark-row__meta {
	display: flex;
	flex-wrap: wrap;
	gap: .2rem .75rem;
	margin-block-start: .15rem;
	color: var(--grey-500);
	font-size: .8rem;
}

.jb-bookmark-row__label {
	display: inline-flex;
	align-items: center;
	gap: .3rem;
}

.jb-dot {
	inline-size: .5rem;
	block-size: .5rem;
	border-radius: 50%;
	background: var(--grey-400);
}

.jb-bookmark-row__status {
	color: var(--jb-date-tomorrow);
}

.jb-bookmark-row__gone {
	display: inline-flex;
	align-items: center;
	gap: .3rem;
	color: var(--jb-prio-high, #ff9a14);
}

.jb-bookmark-row__duplicate {
	display: inline-flex;
	align-items: center;
	gap: .3rem;
	color: var(--jb-prio-high, #ff9a14);
}

.jb-bookmark-row__actions {
	display: flex;
	gap: .25rem;
	flex-shrink: 0;
}

.jb-icon-button {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 2.25rem;
	block-size: 2.25rem;
	border-radius: var(--jb-radius-row);
	color: var(--grey-500);

	&:hover {
		color: var(--jb-green);
		background: var(--grey-100);
	}

	&--danger:hover {
		color: var(--danger);
	}
}

@media (hover: hover) and (pointer: fine) {
	.jb-bookmark-row__actions {
		opacity: 0;
		transition: opacity 120ms ease;
	}

	.jb-bookmark-row:hover .jb-bookmark-row__actions,
	.jb-bookmark-row:focus-within .jb-bookmark-row__actions {
		opacity: 1;
	}
}
</style>
