<script setup>
import { computed, inject, ref, onMounted, provide, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Sun, Moon, Monitor, HardDrive, FileText, Settings, Plus } from 'lucide-vue-next'

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

function setUserLoggedIn(val) {
  isLoggedIn.value = val
}

function triggerCreateTask() {
  createTaskTrigger.value++
  if (route.path !== '/') {
    router.push('/')
  }
}

provide('setUserLoggedIn', setUserLoggedIn)
provide('createTaskTrigger', createTaskTrigger)
provide('themeMode', themeMode)

const navItems = [
  { icon: HardDrive, label: '备份任务', path: '/' },
  { icon: FileText, label: '日志', path: '/logs' },
  { icon: Settings, label: '设置', path: '/settings' }
]

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
        <div class="sidebar-brand">
          <span class="brand-text">ZCopy</span>
        </div>

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
            @click="triggerCreateTask"
          >
            创建新任务
          </el-button>

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
}

.app-sidebar {
  width: 220px;
  min-width: 220px;
  background: var(--z-bg-elevated);
  border-right: 1px solid var(--z-border);
  display: flex;
  flex-direction: column;
  padding: 20px 0;
  position: sticky;
  top: 0;
  height: 100vh;
}

.sidebar-brand {
  padding: 0 20px 24px;
  border-bottom: 1px solid var(--z-border);
  margin-bottom: 12px;
}

.brand-text {
  font-size: 1.4rem;
  font-weight: 700;
  background: linear-gradient(135deg, var(--z-success), var(--z-accent));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.sidebar-nav {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 0 12px;
}

.sidebar-nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  text-decoration: none;
  color: var(--z-text-secondary);
  font-size: 0.95rem;
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
  padding: 12px 12px 0;
  border-top: 1px solid var(--z-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sidebar-create-btn {
  width: 100%;
  border-radius: 8px;
}

.sidebar-theme {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  color: var(--z-text-muted);
  font-size: 0.85rem;
  cursor: pointer;
  transition: background 0.15s;
}

.sidebar-theme:hover {
  background: var(--z-bg-sunken);
  color: var(--z-text-primary);
}

.app-main {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  background:
    radial-gradient(circle at top right, var(--z-accent-bg), transparent 28%),
    radial-gradient(circle at bottom left, var(--z-success-bg), transparent 26%),
    var(--z-bg-base);
}

@media (max-width: 768px) {
  .app-sidebar {
    display: none;
  }
}
</style>
