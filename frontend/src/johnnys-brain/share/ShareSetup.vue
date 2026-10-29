<template>
	<div class="jb-share is-max-width-desktop">
		<h2 class="title">
			{{ $t('johnnysBrain.share.setupTitle') }}
		</h2>
		<p class="jb-share__intro">
			{{ $t('johnnysBrain.share.setupIntro') }}
		</p>

		<section class="jb-share__card">
			<h3>{{ $t('johnnysBrain.share.androidHeading') }}</h3>
			<ol class="jb-share__steps">
				<li>{{ $t('johnnysBrain.share.androidStep1') }}</li>
				<li>{{ $t('johnnysBrain.share.androidStep2') }}</li>
				<li>{{ $t('johnnysBrain.share.androidStep3') }}</li>
			</ol>
			<p class="jb-share__note">
				{{ $t('johnnysBrain.share.androidNote') }}
			</p>
		</section>

		<section class="jb-share__card">
			<h3>{{ $t('johnnysBrain.share.iphoneHeading') }}</h3>
			<p>{{ $t('johnnysBrain.share.iphoneIntro') }}</p>

			<div class="jb-share__token">
				<h4>{{ $t('johnnysBrain.share.tokenHeading') }}</h4>
				<p class="jb-share__note">
					{{ $t('johnnysBrain.share.tokenIntro') }}
				</p>
				<XButton
					v-if="!token"
					:loading="createMutation.isPending.value"
					@click="createToken"
				>
					{{ $t('johnnysBrain.share.tokenCreate') }}
				</XButton>
				<div
					v-else
					class="jb-share__token-value"
				>
					<p class="jb-share__warning">
						{{ $t('johnnysBrain.share.tokenWarning') }}
					</p>
					<div class="jb-share__copyrow">
						<code class="jb-share__code">{{ authorization }}</code>
						<XButton
							variant="secondary"
							@click="copy('token', authorization)"
						>
							{{ copied === 'token' ? $t('johnnysBrain.share.copied') : $t('johnnysBrain.share.copy') }}
						</XButton>
					</div>
				</div>
			</div>

			<ol class="jb-share__steps">
				<li>{{ $t('johnnysBrain.share.iphoneStep1') }}</li>
				<li>{{ $t('johnnysBrain.share.iphoneStep2') }}</li>
				<li>{{ $t('johnnysBrain.share.iphoneStep3') }}</li>
				<li>
					{{ $t('johnnysBrain.share.iphoneStep4') }}
					<div class="jb-share__copyrow">
						<code class="jb-share__code">{{ endpoint }}</code>
						<XButton
							variant="secondary"
							@click="copy('endpoint', endpoint)"
						>
							{{ copied === 'endpoint' ? $t('johnnysBrain.share.copied') : $t('johnnysBrain.share.copy') }}
						</XButton>
					</div>
				</li>
				<li>{{ $t('johnnysBrain.share.iphoneStep5') }}</li>
				<li>{{ $t('johnnysBrain.share.iphoneStep6') }}</li>
			</ol>
			<p class="jb-share__note">
				{{ $t('johnnysBrain.share.iphoneNote') }}
			</p>
		</section>

		<section class="jb-share__card">
			<h3>{{ $t('johnnysBrain.share.desktopHeading') }}</h3>
			<p>{{ $t('johnnysBrain.share.desktopIntro') }}</p>
			<div class="jb-share__copyrow">
				<a
					class="jb-share__bookmarklet"
					:href="bookmarklet"
					draggable="true"
					@click.prevent
				>{{ $t('johnnysBrain.share.bookmarkletName') }}</a>
				<XButton
					variant="secondary"
					@click="copy('bookmarklet', bookmarklet)"
				>
					{{ copied === 'bookmarklet' ? $t('johnnysBrain.share.copied') : $t('johnnysBrain.share.copyBookmarklet') }}
				</XButton>
			</div>
			<p class="jb-share__note">
				{{ $t('johnnysBrain.share.desktopNote') }}
			</p>
		</section>
	</div>
</template>

<script setup lang="ts">
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'

import XButton from '@/components/input/Button.vue'
import {useTitle} from '@/composables/useTitle'
import {useCreateApiTokenMutation} from '@/client/queries/apiTokens'
import {APP_NAME} from '@/johnnys-brain/brand'
import {bookmarkletHref, createBookmarkEndpoint} from './shareLinks'

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('johnnysBrain.share.setupTitle'))

const origin = window.location.origin
const endpoint = computed(() => createBookmarkEndpoint(origin))
const bookmarklet = computed(() => bookmarkletHref(origin))

// The shortcut's token can only save bookmarks, and lasts a year.
const createMutation = useCreateApiTokenMutation()
const token = ref('')
const authorization = computed(() => `Bearer ${token.value}`)

async function createToken() {
	const expires = new Date()
	expires.setFullYear(expires.getFullYear() + 1)
	try {
		const created = await createMutation.mutateAsync({
			title: `${APP_NAME} iPhone shortcut`,
			permissions: {bookmarks: ['create']},
			expires_at: expires.toISOString(),
		})
		token.value = created.token ?? ''
	} catch {
		// The mutation has already shown why it failed.
	}
}

const copied = ref('')
async function copy(what: string, value: string) {
	try {
		await navigator.clipboard.writeText(value)
		copied.value = what
		setTimeout(() => {
			if (copied.value === what) copied.value = ''
		}, 2000)
	} catch {
		// Clipboard access can be blocked; the text is on screen to select by hand.
	}
}
</script>

<style lang="scss" scoped>
.jb-share__intro {
	color: var(--grey-500);
	margin-block-end: 1.25rem;
}

.jb-share__card {
	margin-block-end: 1.25rem;
	padding: 1.25rem 1.4rem;
	border-radius: var(--jb-radius-card, 20px);
	border: 1px solid var(--grey-200);
	background: var(--grey-50);

	h3 {
		margin-block-end: .6rem;
	}

	h4 {
		font-family: var(--jb-font-body);
		font-weight: 600 !important;
		margin-block-end: .3rem;
	}
}

.jb-share__steps {
	margin: .75rem 0 .5rem;
	padding-inline-start: 1.3rem;

	li {
		margin-block-end: .55rem;
		line-height: 1.45;
	}
}

.jb-share__note {
	color: var(--grey-500);
	font-size: .9rem;
}

.jb-share__token {
	margin-block: .9rem;
	padding: .9rem 1rem;
	border-radius: 14px;
	background: var(--grey-100);
}

.jb-share__warning {
	color: var(--jb-prio-high, #ff9a14);
	font-size: .9rem;
	margin-block-end: .5rem;
}

.jb-share__copyrow {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .5rem;
	margin-block-start: .5rem;
}

.jb-share__code {
	flex: 1 1 14rem;
	min-inline-size: 0;
	padding: .55rem .75rem;
	border-radius: 10px;
	background: var(--grey-200);
	color: var(--text);
	font-size: .85rem;
	overflow-wrap: anywhere;
}

.jb-share__bookmarklet {
	display: inline-block;
	padding: .6rem 1.1rem;
	border-radius: var(--jb-radius-button, 12px);
	background: var(--jb-green);
	color: var(--jb-green-ink);
	font-weight: 600;
	cursor: grab;

	&:hover {
		color: var(--jb-green-ink);
	}
}
</style>
