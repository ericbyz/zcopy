import { createRouter, createWebHashHistory } from 'vue-router'
import ServerConnectView from '../views/ServerConnectView.vue'
import DashboardView from '../views/DashboardView.vue'
import LogViewer from '../views/LogViewer.vue'

const routes = [
  {
    path: '/',
    name: 'Home',
    redirect: '/servers'
  },
  {
    path: '/connect',
    name: 'ServerConnect',
    component: ServerConnectView
  },
  {
    path: '/tasks',
    name: 'BackupTasks',
    component: DashboardView,
    meta: { taskMode: 'backup' }
  },
  {
    path: '/sync',
    name: 'SyncTasks',
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
  },
  {
    path: '/servers',
    name: 'Servers',
    component: () => import('../views/ServersView.vue')
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
