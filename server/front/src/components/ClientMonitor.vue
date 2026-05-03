<script setup>
import axios from 'axios'
import { ref, onMounted, onUnmounted } from 'vue'

const apiBaseURL = import.meta.env.VITE_API_BASE_URL || '/api/v1'
const token = ref(localStorage.getItem('zcopy_token') || '')

const api = axios.create({ baseURL: apiBaseURL })
api.interceptors.request.use((config) => {
  if (token.value) {
    config.headers.Authorization = `Bearer ${token.value}`
  }
  return config
})

const clients = ref([])
const loading = ref(false)
let refreshTimer = null

async function fetchClients() {
  loading.value = true
  try {
    const { data } = await api.get('/admin/clients')
    clients.value = data.items || []
  } catch {
    clients.value = []
  } finally {
    loading.value = false
  }
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function statusDot(status) {
  if (status === 'online') return 'online'
  return 'offline'
}

onMounted(() => {
  fetchClients()
  refreshTimer = setInterval(fetchClients, 10000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <div class="client-monitor">
    <div class="monitor-header">
      <div class="monitor-stat">
        <span class="stat-number">{{ clients.length }}</span>
        <span class="stat-label">在线客户端</span>
      </div>
    </div>

    <div v-if="loading && clients.length === 0" class="monitor-empty">
      加载中...
    </div>

    <div v-else-if="clients.length === 0" class="monitor-empty">
      暂无在线客户端
    </div>

    <table v-else class="monitor-table">
      <thead>
        <tr>
          <th>用户名</th>
          <th>客户端 IP</th>
          <th>活跃任务</th>
          <th>最后心跳</th>
          <th>状态</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="client in clients" :key="client.sessionId">
          <td>{{ client.username }}</td>
          <td><code>{{ client.clientIp }}</code></td>
          <td>{{ client.activeTasks }}</td>
          <td>{{ formatTime(client.lastHeartbeat) }}</td>
          <td>
            <span class="status-indicator" :class="statusDot(client.status)" />
            {{ client.status === 'online' ? '在线' : '离线' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.client-monitor {
  padding: 4px 0;
}

.monitor-header {
  display: flex;
  gap: 16px;
  margin-bottom: 20px;
}

.monitor-stat {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.stat-number {
  font-size: 2rem;
  font-weight: 700;
  color: var(--el-color-primary);
}

.stat-label {
  color: var(--el-text-color-secondary);
  font-size: 0.9rem;
}

.monitor-empty {
  text-align: center;
  padding: 40px 0;
  color: var(--el-text-color-secondary);
}

.monitor-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}

.monitor-table th {
  text-align: left;
  padding: 10px 12px;
  border-bottom: 2px solid var(--el-border-color);
  color: var(--el-text-color-secondary);
  font-weight: 600;
}

.monitor-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.monitor-table code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.85rem;
}

.status-indicator {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}

.status-indicator.online {
  background: #67c23a;
}

.status-indicator.offline {
  background: #909399;
}
</style>
