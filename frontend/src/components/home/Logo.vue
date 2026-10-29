<script setup lang="ts">
import {computed} from 'vue'
import {useColorScheme} from '@/composables/useColorScheme'
import {APP_NAME} from '@/johnnys-brain/brand'
import avatarUrl from '@/assets/johnnys-brain/avatar.png'

const {isDark} = useColorScheme()

const CustomLogo = computed(() => {
	const lightLogo = window.CUSTOM_LOGO_URL
	const darkLogo = window.CUSTOM_LOGO_URL_DARK

	if (!lightLogo && !darkLogo) return ''
	if (!darkLogo) return lightLogo
	if (!lightLogo) return darkLogo

	return isDark.value ? darkLogo : lightLogo
})
</script>

<template>
	<div>
		<span
			v-if="!CustomLogo"
			class="logo jb-logo"
		>
			<img
				:src="avatarUrl"
				alt=""
				class="jb-logo__avatar"
				width="36"
				height="36"
			>
			<span class="jb-logo__name">{{ APP_NAME }}</span>
		</span>
		<img
			v-else
			:src="CustomLogo"
			:alt="APP_NAME"
			class="logo"
		>
	</div>
</template>

<style lang="scss" scoped>
.logo {
	color: var(--logo-text-color);
	max-inline-size: 220px;
	max-block-size: 48px;
}

.jb-logo {
	display: inline-flex;
	align-items: center;
	gap: .6rem;
	white-space: nowrap;
}

.jb-logo__avatar {
	inline-size: 36px;
	block-size: 36px;
	border-radius: 50%;
	flex-shrink: 0;
}

.jb-logo__name {
	font-family: var(--jb-font-heading);
	font-weight: 700;
	font-size: 1.15rem;
	letter-spacing: -.01em;
	color: var(--text-strong);
}
</style>
