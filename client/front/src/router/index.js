import { createRouter, createWebHashHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import LogViewer from '../views/LogViewer.vue'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: DashboardView
  },
  {
    path: '/logs',
    name: 'Logs',
    component: LogViewer
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
