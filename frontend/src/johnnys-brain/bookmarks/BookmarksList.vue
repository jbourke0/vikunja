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
				:disabled="createMutation.isPending.value"
			>
			<XButton
				type="submit"
				:loading="createMutation.isPending.value"
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
					:to="{name: 'bookmarks.index', query: tab.status ? {status: tab.status} : {}}"
					class="jb-bookmarks__tab"
					:class="{'is-active': (status ?? 'all') === tab.key}"
					active-class=""
					exact-active-class=""
				>
					{{ tab.label }}
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
				v-for="bookmark in bookmarks"
				:key="bookmark.id"
				class="jb-bookmark-row"
			>
				<span class="jb-bookmark-row__icon">
					<img
						v-if="bookmark.favicon_url && !brokenIcons.has(bookmark.id)"
						:src="bookmark.favicon_url"
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
				</div>
			</li>
		</ul>
	</div>
</template>

<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useQuery} from '@tanstack/vue-query'
import {refDebounced} from '@vueuse/core'

import BaseButton from '@/components/base/BaseButton.vue'
import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {useDelayedLoading} from '@/composables/useDelayedLoading'
import {
	type BookmarkResponse,
	type BookmarkStatus,
	bookmarksQuery,
	hostOf,
	useCreateBookmarkMutation,
	useUpdateBookmarkMutation,
} from '@/client/queries/bookmarks'

const props = defineProps<{
	status?: BookmarkStatus
}>()

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('johnnysBrain.bookmarks.title'))

const tabs = computed(() => [
	{key: 'to_read', status: 'to_read' as const, label: t('johnnysBrain.bookmarks.status.to_read')},
	{key: 'reference', status: 'reference' as const, label: t('johnnysBrain.bookmarks.status.reference')},
	{key: 'archived', status: 'archived' as const, label: t('johnnysBrain.bookmarks.status.archived')},
	{key: 'all', status: undefined, label: t('johnnysBrain.bookmarks.all')},
])

function statusLabel(status: BookmarkStatus) {
	return t(`johnnysBrain.bookmarks.status.${status}`)
}

const search = ref('')
const debouncedSearch = refDebounced(search, 250)

const query = useQuery(computed(() => bookmarksQuery({status: props.status, q: debouncedSearch.value})))
const bookmarks = computed(() => query.data.value ?? [])
const isPending = computed(() => query.isPending.value)
const showLoading = useDelayedLoading(computed(() => query.isFetching.value))

const brokenIcons = reactive(new Set<number>())

const newUrl = ref('')
const createMutation = useCreateBookmarkMutation()

function withScheme(raw: string): string {
	const trimmed = raw.trim()
	return /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
}

async function save() {
	if (newUrl.value.trim() === '') return
	try {
		await createMutation.mutateAsync({url: withScheme(newUrl.value), status: props.status ?? 'to_read'})
	} catch {
		return
	}
	newUrl.value = ''
}

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
