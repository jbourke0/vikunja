<template>
	<div class="jb-dupes is-max-width-desktop">
		<RouterLink
			:to="{name: 'bookmarks.index'}"
			class="jb-dupes__back"
		>
			<Icon icon="arrow-left" />
			{{ $t('johnnysBrain.bookmarks.title') }}
		</RouterLink>
		<div class="jb-dupes__header">
			<h2 class="title">
				{{ $t('johnnysBrain.duplicates.cleanupTitle') }}
			</h2>
			<XButton
				variant="secondary"
				:loading="scanMutation.isPending.value"
				@click="scanMutation.mutate()"
			>
				{{ $t('johnnysBrain.duplicates.scan') }}
			</XButton>
		</div>
		<p class="jb-dupes__intro">
			{{ $t('johnnysBrain.duplicates.cleanupIntro') }}
		</p>

		<section
			v-if="!isPending && groups.length === 0"
			class="jb-dupes__empty"
		>
			<h3>{{ $t('johnnysBrain.duplicates.emptyTitle') }}</h3>
			<p>{{ $t('johnnysBrain.duplicates.emptyBody') }}</p>
		</section>

		<section
			v-for="(group, index) in groups"
			:key="group.bookmarks.map(b => b.id).join('-')"
			class="jb-dupes__group"
		>
			<header class="jb-dupes__group-header">
				<h3>{{ $t('johnnysBrain.duplicates.groupHeading', {count: group.bookmarks.length}) }}</h3>
				<BaseButton
					class="jb-dupes__dismiss"
					:disabled="busyGroup === index"
					@click="notDuplicates(group, index)"
				>
					{{ $t('johnnysBrain.duplicates.notDuplicates') }}
				</BaseButton>
			</header>
			<ul class="jb-dupes__list">
				<li
					v-for="bookmark in group.bookmarks"
					:key="bookmark.id"
					class="jb-dupes__row"
				>
					<span class="jb-dupes__icon">
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
					<div class="jb-dupes__body">
						<RouterLink
							:to="{name: 'bookmarks.detail', params: {bookmarkId: bookmark.id}}"
							class="jb-dupes__title"
						>
							{{ bookmark.title }}
						</RouterLink>
						<a
							:href="bookmark.url"
							target="_blank"
							rel="noopener noreferrer"
							class="jb-dupes__url"
						>{{ shortUrl(bookmark.url) }}</a>
						<p
							v-if="bookmark.description"
							class="jb-dupes__note"
						>
							{{ bookmark.description }}
						</p>
					</div>
					<XButton
						variant="secondary"
						size="small"
						:disabled="busyGroup === index"
						@click="keep(group, bookmark, index)"
					>
						{{ $t('johnnysBrain.duplicates.keep') }}
					</XButton>
				</li>
			</ul>
		</section>
	</div>
</template>

<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {useQuery} from '@tanstack/vue-query'

import BaseButton from '@/components/base/BaseButton.vue'
import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {
	type BookmarkResponse,
	type DuplicateGroup,
	duplicateGroupsQuery,
	useDismissGroupMutation,
	useKeepOneOfGroupMutation,
	useScanDuplicatesMutation,
} from '@/client/queries/bookmarks'

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('johnnysBrain.duplicates.cleanupTitle'))

const query = useQuery(duplicateGroupsQuery())
const groups = computed(() => query.data.value ?? [])
const isPending = computed(() => query.isPending.value)

const brokenIcons = reactive(new Set<number>())
const busyGroup = ref<number | null>(null)

const keepMutation = useKeepOneOfGroupMutation()
const dismissMutation = useDismissGroupMutation()
const scanMutation = useScanDuplicatesMutation()

function shortUrl(url: string): string {
	try {
		const u = new URL(url)
		return (u.host + u.pathname).replace(/\/$/, '')
	} catch {
		return url
	}
}

async function keep(group: DuplicateGroup, keeper: BookmarkResponse, index: number) {
	const others = group.bookmarks.filter(b => b.id !== keeper.id)
	if (!window.confirm(t('johnnysBrain.duplicates.confirmKeep', {title: keeper.title, count: others.length}))) return
	busyGroup.value = index
	try {
		await keepMutation.mutateAsync({keepId: keeper.id, otherIds: others.map(b => b.id)})
	} catch {
		// toasted by the mutation
	} finally {
		busyGroup.value = null
	}
}

async function notDuplicates(group: DuplicateGroup, index: number) {
	busyGroup.value = index
	try {
		await dismissMutation.mutateAsync(group.pairs)
	} catch {
		// toasted by the mutation
	} finally {
		busyGroup.value = null
	}
}
</script>

<style lang="scss" scoped>
.jb-dupes__back {
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

.jb-dupes__header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	flex-wrap: wrap;

	.title {
		margin: 0;
	}
}

.jb-dupes__intro {
	color: var(--grey-500);
	margin: .5rem 0 1.25rem;
	max-inline-size: 42rem;
}

.jb-dupes__empty {
	padding: 1.25rem 1.4rem;
	border-radius: var(--jb-radius-card, 20px);
	border: 1px solid var(--grey-200);
	background: var(--grey-50);
	max-inline-size: 36rem;

	p {
		color: var(--grey-500);
	}
}

.jb-dupes__group {
	margin-block-end: 1.25rem;
	padding: 1rem 1.25rem;
	border-radius: var(--jb-radius-card, 20px);
	border: 1px solid var(--grey-200);
	background: var(--grey-50);
}

.jb-dupes__group-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	margin-block-end: .5rem;

	h3 {
		margin: 0;
		font-size: 1rem;
	}
}

.jb-dupes__dismiss {
	color: var(--grey-500);
	font-size: .85rem;

	&:hover {
		color: var(--text);
	}
}

.jb-dupes__list {
	list-style: none;
	margin: 0;
	padding: 0;
}

.jb-dupes__row {
	display: flex;
	align-items: flex-start;
	gap: .75rem;
	padding: .6rem 0;
	border-block-start: 1px solid var(--jb-divider, var(--grey-200));
}

.jb-dupes__icon {
	flex: 0 0 1.25rem;
	display: flex;
	justify-content: center;
	padding-block-start: .15rem;
	color: var(--grey-500);

	img {
		border-radius: 4px;
	}
}

.jb-dupes__body {
	flex: 1;
	min-inline-size: 0;
	display: flex;
	flex-direction: column;
	gap: .1rem;
}

.jb-dupes__title {
	color: var(--text);
	font-weight: 500;
	overflow-wrap: anywhere;

	&:hover {
		color: var(--jb-green);
	}
}

.jb-dupes__url {
	color: var(--grey-500);
	font-size: .8rem;
	overflow-wrap: anywhere;
}

.jb-dupes__note {
	margin: .15rem 0 0;
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
</style>
