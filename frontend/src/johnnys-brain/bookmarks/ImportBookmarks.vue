<template>
	<div class="jb-import is-max-width-desktop">
		<RouterLink
			:to="{name: 'bookmarks.index'}"
			class="jb-import__back"
		>
			<Icon icon="arrow-left" />
			{{ $t('johnnysBrain.bookmarks.title') }}
		</RouterLink>
		<h2 class="title">
			{{ $t('johnnysBrain.import.title') }}
		</h2>

		<!-- Finished -->
		<section
			v-if="finished"
			class="jb-import__card jb-import__done"
			role="status"
		>
			<h3>{{ $t('johnnysBrain.import.doneHeading') }}</h3>
			<ul class="jb-import__totals">
				<li>{{ $t('johnnysBrain.import.doneSaved', {count: totals.created}) }}</li>
				<li v-if="totals.duplicates">
					{{ $t('johnnysBrain.import.doneDuplicates', {count: totals.duplicates}) }}
				</li>
				<li v-if="totals.invalid + parsed.skipped">
					{{ $t('johnnysBrain.import.doneInvalid', {count: totals.invalid + parsed.skipped}) }}
				</li>
				<li v-if="totals.labels">
					{{ $t('johnnysBrain.import.doneLabels', {count: totals.labels}) }}
				</li>
			</ul>
			<p class="jb-import__note">
				{{ $t('johnnysBrain.import.doneBackground') }}
			</p>
			<div class="jb-import__actions">
				<XButton :to="{name: 'bookmarks.index'}">
					{{ $t('johnnysBrain.import.viewBookmarks') }}
				</XButton>
				<XButton
					variant="secondary"
					@click="reset"
				>
					{{ $t('johnnysBrain.import.importAnother') }}
				</XButton>
			</div>
		</section>

		<!-- Importing, or stopped partway -->
		<section
			v-else-if="running || failed"
			class="jb-import__card"
		>
			<p
				class="jb-import__progress-text"
				role="status"
			>
				{{ $t('johnnysBrain.import.progress', {done: sentCount, total: parsed.items.length}) }}
			</p>
			<progress
				class="jb-import__progress"
				:value="sentCount"
				:max="parsed.items.length"
			/>
			<template v-if="failed">
				<p class="jb-import__warning">
					{{ $t('johnnysBrain.import.failed') }}
				</p>
				<XButton @click="run">
					{{ $t('johnnysBrain.import.resume') }}
				</XButton>
			</template>
		</section>

		<!-- Choose a file, then review -->
		<template v-else>
			<p class="jb-import__intro">
				{{ $t('johnnysBrain.import.intro') }}
			</p>

			<section class="jb-import__card">
				<label
					class="jb-import__file"
					:class="{'is-busy': reading}"
				>
					<input
						type="file"
						accept=".html,.htm,text/html"
						class="is-sr-only"
						:disabled="reading"
						@change="onFile"
					>
					<span class="jb-import__file-button">{{ reading ? $t('johnnysBrain.import.reading') : $t('johnnysBrain.import.choose') }}</span>
				</label>
				<p
					v-if="fileError"
					class="jb-import__warning"
					role="alert"
				>
					{{ fileError }}
				</p>
			</section>

			<section
				v-if="parsed.items.length"
				class="jb-import__card"
			>
				<p class="jb-import__found">
					{{ $t('johnnysBrain.import.found', {count: parsed.items.length, folders: parsed.folderCount}) }}
				</p>
				<p
					v-if="parsed.skipped"
					class="jb-import__note"
				>
					{{ $t('johnnysBrain.import.foundSkipped', {count: parsed.skipped}) }}
				</p>

				<div class="jb-import__field">
					<span
						id="jb-import-status"
						class="jb-import__label"
					>{{ $t('johnnysBrain.import.statusLabel') }}</span>
					<div
						class="jb-segmented"
						role="radiogroup"
						aria-labelledby="jb-import-status"
					>
						<button
							v-for="option in BOOKMARK_STATUSES"
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

				<label class="jb-import__check">
					<input
						v-model="foldersAsLabels"
						type="checkbox"
					>
					<span>
						{{ $t('johnnysBrain.import.foldersAsLabels') }}
						<small>{{ $t('johnnysBrain.import.foldersAsLabelsHelp') }}</small>
					</span>
				</label>

				<XButton @click="run">
					{{ $t('johnnysBrain.import.start', {count: parsed.items.length}) }}
				</XButton>
			</section>

			<section
				v-else
				class="jb-import__card jb-import__how"
			>
				<h3>{{ $t('johnnysBrain.import.howTitle') }}</h3>
				<ul>
					<li>{{ $t('johnnysBrain.import.howChrome') }}</li>
					<li>{{ $t('johnnysBrain.import.howFirefox') }}</li>
					<li>{{ $t('johnnysBrain.import.howSafari') }}</li>
				</ul>
			</section>
		</template>
	</div>
</template>

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'

import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {BOOKMARK_STATUSES, type BookmarkStatus, useImportBookmarksMutation} from '@/client/queries/bookmarks'
import {
	inBatches,
	MAX_IMPORT_FILE_BYTES,
	type ParsedBookmarks,
	parseBookmarksHtml,
} from './bookmarkImport'

const BATCH_SIZE = 200
const EMPTY: ParsedBookmarks = {items: [], skipped: 0, folderCount: 0}

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('johnnysBrain.import.title'))

const reading = ref(false)
const fileError = ref('')
const parsed = ref<ParsedBookmarks>(EMPTY)
const status = ref<BookmarkStatus>('to_read')
const foldersAsLabels = ref(true)

async function onFile(event: Event) {
	const input = event.target as HTMLInputElement
	const file = input.files?.[0]
	input.value = '' // so choosing the same file again still fires
	if (!file) return

	fileError.value = ''
	parsed.value = EMPTY
	if (file.size > MAX_IMPORT_FILE_BYTES) {
		fileError.value = t('johnnysBrain.import.tooBig')
		return
	}
	reading.value = true
	try {
		const result = parseBookmarksHtml(await file.text())
		if (result.items.length === 0) {
			fileError.value = t('johnnysBrain.import.none')
		} else {
			parsed.value = result
		}
	} catch {
		fileError.value = t('johnnysBrain.import.readError')
	} finally {
		reading.value = false
	}
}

// Importing: one batch at a time, so a big file is a series of small requests and a failure
// partway can be resumed without saving anything twice (the server skips links it already has).
const importMutation = useImportBookmarksMutation()
const batches = computed(() => inBatches(parsed.value.items, BATCH_SIZE))
const nextBatch = ref(0)
const running = ref(false)
const failed = ref(false)
const finished = ref(false)
const totals = ref({created: 0, duplicates: 0, invalid: 0, labels: 0})
const sentCount = computed(() => Math.min(nextBatch.value * BATCH_SIZE, parsed.value.items.length))

async function run() {
	running.value = true
	failed.value = false
	try {
		while (nextBatch.value < batches.value.length) {
			const result = await importMutation.mutateAsync({
				items: batches.value[nextBatch.value],
				status: status.value,
				foldersAsLabels: foldersAsLabels.value,
			})
			totals.value = {
				created: totals.value.created + (result.created ?? 0),
				duplicates: totals.value.duplicates + (result.duplicates ?? 0),
				invalid: totals.value.invalid + (result.invalid ?? 0),
				labels: totals.value.labels + (result.labels_created ?? 0),
			}
			nextBatch.value++
		}
		finished.value = true
	} catch {
		failed.value = true // the mutation has already said why
	} finally {
		running.value = false
	}
}

function reset() {
	parsed.value = EMPTY
	nextBatch.value = 0
	finished.value = false
	failed.value = false
	totals.value = {created: 0, duplicates: 0, invalid: 0, labels: 0}
	fileError.value = ''
}
</script>

<style lang="scss" scoped>
.jb-import__back {
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

.jb-import__intro {
	color: var(--grey-500);
	margin-block-end: 1.25rem;
	max-inline-size: 40rem;
}

.jb-import__card {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	gap: .85rem;
	max-inline-size: 40rem;
	margin-block-end: 1.25rem;
	padding: 1.25rem 1.4rem;
	border-radius: var(--jb-radius-card, 20px);
	border: 1px solid var(--grey-200);
	background: var(--grey-50);

	h3 {
		margin: 0;
	}
}

.jb-import__file {
	cursor: pointer;

	&.is-busy {
		opacity: .6;
		cursor: progress;
	}

	&:focus-within .jb-import__file-button {
		box-shadow: 0 0 0 2px hsla(var(--primary-hsl), .5);
	}
}

.jb-import__file-button {
	display: inline-block;
	padding: .65rem 1.2rem;
	border-radius: var(--jb-radius-button, 12px);
	background: var(--jb-green);
	color: var(--jb-green-ink);
	font-weight: 600;
}

.jb-import__found {
	font-weight: 600;
	font-size: 1.05rem;
}

.jb-import__note {
	color: var(--grey-500);
	font-size: .9rem;
}

.jb-import__warning {
	color: var(--jb-prio-high, #ff9a14);
	font-size: .9rem;
}

.jb-import__field {
	display: flex;
	flex-direction: column;
	gap: .4rem;
}

.jb-import__label {
	font-size: .8rem;
	font-weight: 600;
	text-transform: uppercase;
	letter-spacing: .06em;
	color: var(--grey-500);
}

.jb-import__check {
	display: flex;
	align-items: flex-start;
	gap: .6rem;
	cursor: pointer;

	input {
		margin-block-start: .3rem;
		accent-color: var(--jb-green);
	}

	small {
		display: block;
		color: var(--grey-500);
		margin-block-start: .15rem;
	}
}

.jb-import__how ul {
	margin: 0;
	padding-inline-start: 1.2rem;

	li {
		margin-block-end: .4rem;
	}
}

.jb-import__progress {
	inline-size: 100%;
	block-size: .6rem;
	accent-color: var(--jb-green);
}

.jb-import__progress-text {
	font-weight: 500;
}

.jb-import__totals {
	margin: 0;
	padding-inline-start: 1.2rem;
}

.jb-import__actions {
	display: flex;
	flex-wrap: wrap;
	gap: .5rem;
}

.jb-segmented {
	display: inline-flex;
	padding: .2rem;
	border-radius: 999px;
	background: var(--grey-100);
	gap: .15rem;
	flex-wrap: wrap;
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
</style>
