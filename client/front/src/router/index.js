import { createRouter, createWebHashHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import LogViewer from '../views/LogViewer.vue'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: DashboardView,
    meta: { taskMode: 'backup' }
  },
  {
    path: '/sync',
    name: 'SyncDashboard',
    component: DashboardView,
    meta: { taskMode: 'sync' }
  },
  {
    path: '/logs',
    name: 'Logs',
    component: LogViewer
  },
  {
    path: '/platform',
    name: 'Platform',
    component: () => import('../views/PlatformView.vue')
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/SettingsView.vue')
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
