<script setup>
import { computed, inject, ref, onMounted, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Sun, Moon, Monitor, HardDrive, Repeat, FileText, Info, Settings, Plus } from 'lucide-vue-next'

const zhCn = inject('element-locale')

const router = useRouter()
const route = useRoute()

// Theme logic
const themeMode = ref(localStorage.getItem('zcopy-theme') || 'auto')

function cycleTheme() {
  const modes = ['light', 'dark', 'auto']
  const current = modes.indexOf(themeMode.value)
  themeMode.value = modes[(current + 1) % modes.length]
  localStorage.setItem('zcopy-theme', themeMode.value)
  applyTheme()
}

function applyTheme() {
  const isDark = themeMode.value === 'dark' ||
    (themeMode.value === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', isDark)
}

const themeIcon = computed(() => {
  if (themeMode.value === 'light') return Sun
  if (themeMode.value === 'dark') return Moon
  return Monitor
})

const themeLabel = computed(() => {
  if (themeMode.value === 'light') return '浅色模式'
  if (themeMode.value === 'dark') return '深色模式'
  return '跟随系统'
})

// Shared state with child views via provide/inject
const isLoggedIn = ref(false)
const createTaskTrigger = ref(0)
const createTaskPreset = ref('backup')

function setUserLoggedIn(val) {
  isLoggedIn.value = val
}

function triggerCreateTask(mode = 'backup') {
  createTaskPreset.value = mode
  createTaskTrigger.value++
  const target = mode === 'sync' ? '/sync' : '/'
  if (route.path !== target) {
    router.push(target)
  }
}

provide('setUserLoggedIn', setUserLoggedIn)
provide('createTaskTrigger', createTaskTrigger)
provide('createTaskPreset', createTaskPreset)
provide('themeMode', themeMode)

const navItems = [
  { icon: HardDrive, label: '备份任务', path: '/' },
  { icon: Repeat, label: '同步任务', path: '/sync' },
  { icon: FileText, label: '日志', path: '/logs' }
]

const bottomNavItems = [
  { icon: Info, label: '平台能力', path: '/platform' },
  { icon: Settings, label: '设置', path: '/settings' }
]

const createButtonLabel = computed(() => route.path === '/sync' ? '创建同步任务' : '创建备份任务')
const createButtonMode = computed(() => route.path === '/sync' ? 'sync' : 'backup')

onMounted(() => {
  applyTheme()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (themeMode.value === 'auto') applyTheme()
  })
})
</script>

<template>
  <el-config-provider :locale="zhCn">
    <div class="app-layout">
      <!-- Sidebar -->
      <aside v-if="isLoggedIn" class="app-sidebar">
        <nav class="sidebar-nav">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-nav-item"
            :class="{ active: route.path === item.path }"
          >
            <component :is="item.icon" style="width:18px;height:18px" />
            <span>{{ item.label }}</span>
          </router-link>
        </nav>

        <div class="sidebar-bottom">
          <el-button
            type="primary"
            :icon="Plus"
            class="sidebar-create-btn"
            @click="triggerCreateTask(createButtonMode)"
          >
            {{ createButtonLabel }}
          </el-button>

          <nav class="sidebar-nav sidebar-nav-bottom">
            <router-link
              v-for="item in bottomNavItems"
              :key="item.path"
              :to="item.path"
              class="sidebar-nav-item"
              :class="{ active: route.path === item.path }"
            >
              <component :is="item.icon" style="width:18px;height:18px" />
              <span>{{ item.label }}</span>
            </router-link>
          </nav>

          <div class="sidebar-theme" @click="cycleTheme">
            <component :is="themeIcon" style="width:16px;height:16px" />
            <span>{{ themeLabel }}</span>
          </div>
        </div>
      </aside>

      <!-- Main Content -->
      <main class="app-main">
        <router-view />
      </main>
    </div>
  </el-config-provider>
</template>

<style scoped>
.app-layout {
  display: flex;
  min-height: 100vh;
  background: var(--z-bg-base);
}

.app-sidebar {
  width: 164px;
  min-width: 164px;
  background: var(--z-bg-elevated);
  border-right: 1px solid var(--z-border);
  display: flex;
  flex-direction: column;
  padding: 36px 0 10px;
  position: sticky;
  top: 0;
  height: 100vh;
}

.sidebar-nav {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 0 8px;
}

.sidebar-nav-bottom {
  flex: 0 0 auto;
  padding: 0;
}

.sidebar-nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 8px;
  border-radius: 8px;
  text-decoration: none;
  color: var(--z-text-secondary);
  font-size: 0.82rem;
  font-weight: 500;
  transition: background 0.15s, color 0.15s;
  cursor: pointer;
}

.sidebar-nav-item:hover {
  background: var(--z-bg-sunken);
  color: var(--z-text-primary);
}

.sidebar-nav-item.active {
  background: var(--z-accent-bg);
  color: var(--z-accent);
}

.sidebar-bottom {
  padding: 10px 8px 0;
  border-top: 1px solid var(--z-border);
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.sidebar-create-btn {
  width: 100%;
  border-radius: 8px;
  font-size: 0.86rem;
}

.sidebar-theme {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 8px;
  border-radius: 8px;
  color: var(--z-text-muted);
  font-size: 0.76rem;
  cursor: pointer;
  transition: background 0.15s;
}

.sidebar-theme:hover {
  background: var(--z-bg-sunken);
  color: var(--z-text-primary);
}

.app-main {
  flex: 1;
  min-width: 0;
  padding: 36px 16px 16px;
  overflow-y: auto;
  background: var(--z-bg-base);
}

@media (max-width: 760px) {
  .app-sidebar {
    width: 64px;
    min-width: 64px;
  }

  .sidebar-nav-item,
  .sidebar-theme {
    justify-content: center;
  }

  .sidebar-nav-item span,
  .sidebar-theme span,
  .sidebar-create-btn span {
    display: none;
  }

  .sidebar-create-btn {
    aspect-ratio: 1;
    padding: 8px;
  }
}
</style>
