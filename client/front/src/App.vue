<script setup>
import { computed, inject, ref, onMounted, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import axios from 'axios'
import {
  Sun, Moon, Monitor,
  HardDrive, Repeat, FileText,
  Settings, Plus, Server,
  Cpu, LogOut
} from 'lucide-vue-next'

const zhCn = inject('element-locale')

const router = useRouter()
const route = useRoute()

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

const isLoggedIn = ref(false)
const createTaskTrigger = ref(0)
const createTaskPreset = ref('backup')

const serverList = ref([])
const currentServerId = ref(localStorage.getItem('zcopy_current_server') || '')

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'
const serverApi = axios.create({ baseURL: apiBaseURL })

function setUserLoggedIn(val) {
  isLoggedIn.value = val
}

function triggerCreateTask(mode = 'backup') {
  if (!isLoggedIn.value) {
    const query = currentServerId.value ? { serverId: currentServerId.value } : {}
    router.push({ path: '/connect', query })
    return
  }
  createTaskPreset.value = mode
  createTaskTrigger.value++
  const target = mode === 'sync' ? '/sync' : '/tasks'
  if (route.path !== target) {
    router.push(target)
  }
}

async function fetchServers() {
  try {
    const { data } = await serverApi.get('/servers')
    serverList.value = data.items || []
  } catch {
    serverList.value = []
  }
}

async function logout() {
  const tokenKey = currentServerId.value ? `zcopy_token_${currentServerId.value}` : 'zcopy_token'
  const token = localStorage.getItem(tokenKey)
  try {
    if (token) {
      const api = axios.create({ baseURL: apiBaseURL })
      api.defaults.headers.common.Authorization = `Bearer ${token}`
      await api.post('/auth/logout', { serverId: currentServerId.value })
    }
  } catch {
    // ignore logout transport failures
  }
  localStorage.removeItem(tokenKey)
  isLoggedIn.value = false
  router.push('/servers')
}

provide('setUserLoggedIn', setUserLoggedIn)
provide('createTaskTrigger', createTaskTrigger)
provide('createTaskPreset', createTaskPreset)
provide('themeMode', themeMode)
provide('currentServerId', currentServerId)
provide('serverList', serverList)

const navItems = [
  { icon: HardDrive, label: '备份任务', path: '/tasks' },
  { icon: Repeat, label: '同步任务', path: '/sync' },
  { icon: FileText, label: '日志', path: '/logs' }
]

const bottomNavItems = [
  { icon: Cpu, label: '平台能力', path: '/platform' },
  { icon: Server, label: '服务器', path: '/servers' },
  { icon: Settings, label: '设置', path: '/settings' }
]

const createButtonLabel = computed(() => route.path === '/sync' ? '创建同步任务' : '创建备份任务')
const createButtonMode = computed(() => route.path === '/sync' ? 'sync' : 'backup')

function isNavActive(path) {
  return route.path === path
}

onMounted(() => {
  applyTheme()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (themeMode.value === 'auto') applyTheme()
  })
  fetchServers()
  if (currentServerId.value) {
    const tokenKey = `zcopy_token_${currentServerId.value}`
    if (localStorage.getItem(tokenKey)) {
      isLoggedIn.value = true
    }
  }
})
</script>

<template>
  <el-config-provider :locale="zhCn">
    <div class="app-layout">
      <aside class="app-sidebar">
        <div class="sidebar-brand">
          <span class="sidebar-brand-text">ZCopy</span>
        </div>

        <div class="sidebar-nav-main">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-nav-item"
            :class="{ active: isNavActive(item.path) }"
          >
            <component :is="item.icon" class="sidebar-nav-icon" />
            <span class="sidebar-nav-label">{{ item.label }}</span>
          </router-link>

          <div class="sidebar-divider" />

          <router-link
            v-for="item in bottomNavItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-nav-item"
            :class="{ active: route.path === item.path }"
          >
            <component :is="item.icon" class="sidebar-nav-icon" />
            <span class="sidebar-nav-label">{{ item.label }}</span>
          </router-link>
        </div>

        <!-- Create task button -->
        <div class="sidebar-create">
          <el-button
            size="small"
            type="primary"
            :icon="Plus"
            @click.stop="triggerCreateTask(createButtonMode)"
            class="sidebar-create-btn"
          >
            <span class="sidebar-nav-label">{{ createButtonLabel }}</span>
          </el-button>
        </div>

        <!-- Spacer -->
        <div class="sidebar-spacer" />

        <!-- Theme toggle + Logout -->
        <div class="sidebar-bottom">
          <button v-if="isLoggedIn" class="sidebar-theme-btn" @click.stop="logout" title="退出登录">
            <LogOut class="sidebar-nav-icon" />
          </button>
          <button class="sidebar-theme-btn" @click.stop="cycleTheme" :title="`主题: ${themeMode}`">
            <component :is="themeIcon" class="sidebar-nav-icon" />
          </button>
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
  --z-window-top-space: 38px;
}

/* ---- Sidebar ---- */
.app-sidebar {
  width: 164px;
  min-width: 164px;
  display: flex;
  flex-direction: column;
  background: var(--z-bg-elevated);
  border-right: 1px solid var(--z-border);
  padding: var(--z-window-top-space) 0 0;
  position: sticky;
  top: 0;
  height: 100vh;
  box-sizing: border-box;
  overflow-y: auto;
  z-index: 100;
  transition: width 0.2s ease, min-width 0.2s ease;
}

.sidebar-brand {
  display: flex;
  align-items: center;
  padding: 16px 14px 12px;
  gap: 8px;
}

.sidebar-brand-text {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--z-text-primary);
  white-space: nowrap;
}

.sidebar-nav-main {
  display: flex;
  flex-direction: column;
  padding: 0 8px;
  gap: 2px;
}

.sidebar-nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 6px;
  text-decoration: none;
  color: var(--z-text-secondary);
  font-size: 0.82rem;
  font-weight: 500;
  transition: background 0.15s, color 0.15s;
  cursor: pointer;
  white-space: nowrap;
}

.sidebar-nav-item:hover {
  background: var(--z-bg-sunken);
  color: var(--z-text-primary);
}

.sidebar-nav-item.active {
  background: var(--z-accent-bg);
  color: var(--z-accent);
}

.sidebar-nav-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.sidebar-nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: opacity 0.15s;
}

.sidebar-divider {
  height: 1px;
  background: var(--z-border);
  margin: 6px 10px;
}

.sidebar-create {
  padding: 8px 10px 4px;
}

.sidebar-create-btn {
  width: 100%;
  font-size: 0.8rem !important;
}

.sidebar-spacer {
  flex: 1;
}

/* Bottom buttons */
.sidebar-bottom {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 8px 10px;
  gap: 4px;
  border-top: 1px solid var(--z-border);
}

.sidebar-theme-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 6px;
  background: transparent;
  color: var(--z-text-muted);
  cursor: pointer;
  border: none;
  transition: background 0.15s, color 0.15s;
}

.sidebar-theme-btn:hover {
  background: var(--z-bg-sunken);
  color: var(--z-text-primary);
}

/* ---- Main content ---- */
.app-main {
  flex: 1;
  padding: var(--z-window-top-space) 16px 16px;
  overflow-y: auto;
  background: var(--z-bg-base);
  min-width: 0;
}

.app-main--full {
  padding: var(--z-window-top-space) 16px 16px;
  max-width: none;
}

/* ---- Responsive: collapse sidebar on narrow screens ---- */
@media (max-width: 760px) {
  .app-sidebar {
    width: 56px;
    min-width: 56px;
  }

  .sidebar-brand {
    justify-content: center;
    padding: 16px 0 12px;
  }

  .sidebar-brand-text {
    display: none;
  }

  .sidebar-nav-item {
    justify-content: center;
    padding: 9px 0;
  }

  .sidebar-nav-label {
    display: none;
  }

  .sidebar-divider {
    margin: 6px 6px;
  }

  .sidebar-create {
    padding: 8px 4px 4px;
  }

  .sidebar-create-btn {
    padding: 9px 0 !important;
  }

  .sidebar-create-btn .sidebar-nav-label {
    display: none;
  }

  .sidebar-bottom {
    justify-content: center;
  }
}
</style>
