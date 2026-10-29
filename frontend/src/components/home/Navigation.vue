<template>
	<aside
		:class="{'is-active': baseStore.menuActive, 'is-resizing': isResizing}"
		class="menu-container"
		:style="{'--sidebar-width': sidebarWidthStyle}"
	>
		<nav
			class="menu top-menu"
			:aria-label="$t('navigation.main')"
		>
			<RouterLink
				:to="{name: 'home'}"
				class="logo"
				:aria-label="$t('navigation.home')"
			>
				<Logo
					width="164"
					height="48"
				/>
			</RouterLink>
			<menu class="menu-list other-menu-items">
				<li>
					<BaseButton
						class="jb-nav-button jb-add-task"
						@click="openQuickAdd"
					>
						<span class="menu-item-icon icon jb-add-task__icon">
							<Icon icon="plus" />
						</span>
						{{ $t('johnnysBrain.nav.addTask') }}
					</BaseButton>
				</li>
				<li v-if="inboxProject">
					<RouterLink :to="{ name: 'project.index', params: { projectId: inboxProject.id } }">
						<span class="menu-item-icon icon">
							<Icon icon="inbox" />
						</span>
						{{ $t('johnnysBrain.nav.inbox') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						:to="{ name: 'tasks.range', query: TODAY_QUERY }"
						active-class=""
						exact-active-class=""
						:class="{'router-link-exact-active': isToday}"
					>
						<span class="menu-item-icon icon">
							<Icon :icon="['far', 'calendar']" />
						</span>
						{{ $t('johnnysBrain.nav.today') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.upcoming"
						:to="{ name: 'tasks.range'}"
						active-class=""
						exact-active-class=""
						:class="{'router-link-exact-active': isRangeView && !isToday}"
					>
						<span class="menu-item-icon icon">
							<Icon :icon="['far', 'calendar-alt']" />
						</span>
						{{ $t('navigation.upcoming') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.overview"
						:to="{ name: 'home'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="calendar" />
						</span>
						{{ $t('navigation.overview') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.labels"
						:to="{ name: 'labels.index'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="tags" />
						</span>
						{{ $t('label.title') }}
					</RouterLink>
				</li>
				<li>
					<RouterLink
						v-shortcut="SHORTCUTS.navigation.projects"
						:to="{ name: 'projects.index'}"
					>
						<span class="menu-item-icon icon">
							<Icon icon="layer-group" />
						</span>
						{{ $t('project.projects') }}
					</RouterLink>
				</li>
				<li v-if="timeTrackingEnabled">
					<RouterLink :to="{ name: 'time-tracking'}">
						<span class="menu-item-icon icon">
							<Icon :icon="['far', 'clock']" />
						</span>
						{{ $t('timeTracking.title') }}
					</RouterLink>
				</li>
			</menu>
		</nav>

		<Loading
			v-if="projectList.isLoading"
			variant="small"
		/>
		<template v-else>
			<nav
				v-if="favoriteProjects.length"
				class="menu"
				:aria-label="$t('project.pseudo.favorites.title')"
			>
				<ProjectsNavigation
					:model-value="favoriteProjects"
					:can-edit-order="false"
					:can-collapse="false"
				/>
			</nav>
			
			<nav
				v-if="savedFilterProjects.length"
				class="menu"
				:aria-label="$t('navigation.savedFilters')"
			>
				<ProjectsNavigation
					:model-value="savedFilterProjects"
					:can-edit-order="false"
					:can-collapse="false"
				/>
			</nav>

			<nav
				class="menu"
				:aria-label="$t('project.projects')"
			>
				<ProjectsNavigation
					:model-value="projects"
					:can-edit-order="true"
					:can-collapse="true"
				/>
			</nav>
		</template>


		<div
			v-if="!isMobile"
			class="resize-handle"
			@mousedown="startResize"
			@touchstart="startResize"
		/>
	</aside>
</template>

<script setup lang="ts">
import {computed} from 'vue'

import {SHORTCUTS} from '@/constants/shortcuts'
import Logo from '@/components/home/Logo.vue'
import Loading from '@/components/misc/Loading.vue'
import BaseButton from '@/components/base/BaseButton.vue'

import {useBaseStore} from '@/stores/base'
import {useProjects} from '@/composables/useProjects'
import {useConfigStore} from '@/stores/config'
import {PRO_FEATURE} from '@/constants/proFeatures'
import ProjectsNavigation from '@/components/home/ProjectsNavigation.vue'
import {useSidebarResize} from '@/composables/useSidebarResize'
import {useAuthStore} from '@/stores/auth'
import {TODAY_QUERY} from '@/johnnys-brain/brand'
import {useTodayView} from '@/johnnys-brain/useTodayView'

const baseStore = useBaseStore()
const projectList = useProjects()
const configStore = useConfigStore()

const timeTrackingEnabled = computed(() => configStore.isProFeatureEnabled(PRO_FEATURE.TIME_TRACKING))

const {sidebarWidthStyle, isResizing, startResize, isMobile} = useSidebarResize()

const projects = computed(() => projectList.notArchivedRootProjects)
const favoriteProjects = computed(() => projectList.favoriteProjects)
const savedFilterProjects = computed(() => projectList.savedFilterProjects)

const authStore = useAuthStore()
const {isToday, isRangeView} = useTodayView()
const inboxProject = computed(() => {
	const all = projectList.notArchivedRootProjects
	const defaultId = authStore.settings.default_project_id
	return all.find(p => p.id === defaultId) ?? all.find(p => p.title === 'Inbox')
})

function openQuickAdd() {
	if (isMobile.value) {
		baseStore.setMenuActive(false)
	}
	baseStore.setQuickActionsActive(true)
}
</script>

<style lang="scss" scoped>
.logo {
	display: block;

	padding-inline-start: 1rem;
	margin-inline-end: 1rem;
	margin-block-end: 1rem;

	@media screen and (min-width: $tablet) {
		display: none;
	}
}

.menu-container {
	--sidebar-width: #{$navbar-width};

	display: flex;
	flex-direction: column;
	background: var(--site-background);
	color: $vikunja-nav-color;
	padding: 1rem 0;
	transition: transform $transition-duration ease-in;
	position: fixed;
	inset-block-start: $navbar-height;
	inset-block-end: 0;
	inset-inline-start: 0;
	transform: translateX(-100%);
	inline-size: var(--sidebar-width);
	overflow-y: auto;

	[dir="rtl"] & {
		transform: translateX(100%);
	}

	@media screen and (max-width: $tablet) {
		inset-block-start: 0;
		inline-size: 70vw;
		z-index: 20;
	}

	&.is-active {
		transform: translateX(0);
		transition: transform $transition-duration ease-out;
	}

	&.is-resizing {
		transition: none;
	}
}

.resize-handle {
	position: absolute;
	inset-block-start: 0;
	inset-block-end: 0;
	inset-inline-end: 0;
	inline-size: 4px;
	cursor: ew-resize;
	background: transparent;
	transition: background-color $transition-duration ease;
	touch-action: none;

	&:hover,
	&:active {
		background-color: var(--primary);
	}
}

.top-menu .menu-list {
	li {
		font-weight: 600;
		font-family: $vikunja-font;
	}

	.list-menu-link,
	li > a {
		padding-inline-start: 2rem;
		display: inline-block;

		.icon {
			padding-block-end: .25rem;
		}
	}
}

.menu + .menu {
	padding-block-start: math.div($navbar-padding, 2);
}

.jb-add-task {
	color: var(--jb-green);
	font-weight: 600;
}

.jb-add-task__icon {
	inline-size: 1.25rem;
	block-size: 1.25rem;
	border-radius: 50%;
	background: var(--jb-green);
	color: var(--jb-green-ink) !important;
	font-size: .7rem;
}
</style>
