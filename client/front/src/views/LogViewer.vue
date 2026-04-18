<script setup>
import { ref, computed, onMounted } from 'vue'

const API_BASE =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'

const token = ref(localStorage.getItem('zcopy_token') || '')

const logs = ref([])
const tasks = ref([])
const loading = ref(false)
const errorMsg = ref('')

const filters = ref({
  level: '',
  taskId: '',
  keyword: '',
  startDate: '',
  endDate: ''
})

const pagination = ref({
  currentPage: 1,
  pageSize: 20,
  total: 0
})

const totalPages = computed(() => Math.max(1, Math.ceil(pagination.value.total / pagination.value.pageSize)))

function buildQueryParams() {
  const params = new URLSearchParams()
  params.set('page', pagination.value.currentPage)
  params.set('pageSize', pagination.value.pageSize)
  if (filters.value.level) params.set('level', filters.value.level)
  if (filters.value.taskId) params.set('taskId', filters.value.taskId)
  if (filters.value.keyword) params.set('keyword', filters.value.keyword)
  if (filters.value.startDate) {
    params.set('startTime', new Date(filters.value.startDate).toISOString())
  }
  if (filters.value.endDate) {
    params.set('endTime', new Date(filters.value.endDate).toISOString())
  }
  return params
}

async function fetchTasks() {
  try {
    const resp = await fetch(`${API_BASE}/tasks`, {
      headers: token.value ? { Authorization: `Bearer ${token.value}` } : {}
    })
    if (resp.ok) {
      const data = await resp.json()
      tasks.value = data.items || []
    }
  } catch {
    // silent — tasks dropdown is optional
  }
}

async function fetchLogs() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = buildQueryParams()
    const resp = await fetch(`${API_BASE}/logs?${params}`, {
      headers: token.value ? { Authorization: `Bearer ${token.value}` } : {}
    })
    if (!resp.ok) {
      const data = await resp.json().catch(() => null)
      errorMsg.value = data?.message || '查询日志失败'
      return
    }
    const data = await resp.json()
    logs.value = data.items || []
    pagination.value.total = data.total || 0
    pagination.value.currentPage = data.page || pagination.value.currentPage
    pagination.value.pageSize = data.pageSize || pagination.value.pageSize
  } catch (e) {
    errorMsg.value = '网络错误：无法连接到本地服务'
  } finally {
    loading.value = false
  }
}

function handleQuery() {
  pagination.value.currentPage = 1
  fetchLogs()
}

function handleReset() {
  filters.value = { level: '', taskId: '', keyword: '', startDate: '', endDate: '' }
  pagination.value.currentPage = 1
  fetchLogs()
}

function handleExport() {
  const params = buildQueryParams()
  params.delete('page')
  params.delete('pageSize')
  const url = `${API_BASE}/logs/export?${params}`
  const a = document.createElement('a')
  a.href = url
  if (token.value) {
    fetch(url, {
      headers: {
        Authorization: `Bearer ${token.value}`
      }
    })
      .then(async (resp) => {
        if (!resp.ok) {
          const data = await resp.json().catch(() => null)
          throw new Error(data?.message || '导出失败')
        }
        return resp.blob()
      })
      .then((blob) => {
        const objectURL = URL.createObjectURL(blob)
        a.href = objectURL
        a.download = 'zcopy-logs-' + new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19) + '.jsonl'
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(objectURL)
      })
      .catch((error) => {
        errorMsg.value = error.message || '导出失败'
      })
    return
  }
  a.download = 'zcopy-logs-' + new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19) + '.jsonl'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function prevPage() {
  if (pagination.value.currentPage > 1) {
    pagination.value.currentPage--
    fetchLogs()
  }
}

function nextPage() {
  if (pagination.value.currentPage < totalPages.value) {
    pagination.value.currentPage++
    fetchLogs()
  }
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN')
}

function levelClass(level) {
  const l = (level || '').toLowerCase()
  return { 'level-debug': l === 'debug', 'level-info': l === 'info', 'level-warn': l === 'warn', 'level-error': l === 'error' }
}

onMounted(() => {
  fetchTasks()
  fetchLogs()
})
</script>

<template>
  <div class="page">
    <div class="shell">
      <header class="hero card">
        <div>
          <h1>日志查看器</h1>
          <p>查看和管理传输日志</p>
        </div>
      </header>

      <div v-if="errorMsg" class="message card" style="padding: 14px 18px; margin-bottom: 16px; color: #fca5a5">{{ errorMsg }}</div>

      <!-- Filter Card -->
      <div class="card form-card">
        <div class="section-header">
          <h2>筛选条件</h2>
          <button class="btn primary" @click="handleExport">导出</button>
        </div>

        <div class="form-grid" style="grid-template-columns: repeat(auto-fit, minmax(200px, 1fr))">
          <select v-model="filters.level">
            <option value="">全部级别</option>
            <option value="debug">DEBUG</option>
            <option value="info">INFO</option>
            <option value="warn">WARN</option>
            <option value="error">ERROR</option>
          </select>

          <select v-model="filters.taskId">
            <option value="">全部任务</option>
            <option v-for="t in tasks" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>

          <input v-model="filters.keyword" placeholder="关键词搜索" />

          <input v-model="filters.startDate" type="datetime-local" />
          <input v-model="filters.endDate" type="datetime-local" />
        </div>

        <div class="row-actions" style="margin-top: 16px">
          <button class="btn primary" :disabled="loading" @click="handleQuery">{{ loading ? '查询中...' : '查询' }}</button>
          <button class="btn ghost" @click="handleReset">重置</button>
        </div>
      </div>

      <!-- Log Table Card -->
      <div class="card task-card">
        <div class="section-header">
          <h2>日志列表</h2>
          <span style="color: #94a3b8">共 {{ pagination.total }} 条</span>
        </div>

        <div v-if="loading" class="empty">加载中...</div>

        <table v-else style="width: 100%; border-collapse: collapse">
          <thead>
            <tr style="text-align: left; border-bottom: 1px solid rgba(148, 163, 184, 0.16)">
              <th style="padding: 12px 8px; color: #cbd5e1">时间</th>
              <th style="padding: 12px 8px; color: #cbd5e1">级别</th>
              <th style="padding: 12px 8px; color: #cbd5e1">任务名</th>
              <th style="padding: 12px 8px; color: #cbd5e1">文件路径</th>
              <th style="padding: 12px 8px; color: #cbd5e1">消息</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="logs.length === 0">
              <td colspan="5">
                <div class="empty">暂无日志</div>
              </td>
            </tr>
            <tr v-for="log in logs" :key="log.id" style="border-bottom: 1px solid rgba(148, 163, 184, 0.08)">
              <td style="padding: 10px 8px; white-space: nowrap; font-size: 13px">{{ formatTime(log.createdAt) }}</td>
              <td style="padding: 10px 8px"><span :class="['level-badge', levelClass(log.level)]">{{ (log.level || '').toUpperCase() }}</span></td>
              <td style="padding: 10px 8px; max-width: 140px; overflow: hidden; text-overflow: ellipsis">{{ log.taskName || '-' }}</td>
              <td style="padding: 10px 8px; max-width: 200px; overflow: hidden; text-overflow: ellipsis" :title="log.filePath">{{ log.filePath || '-' }}</td>
              <td style="padding: 10px 8px; max-width: 300px; overflow: hidden; text-overflow: ellipsis" :title="log.message">{{ log.message || '-' }}</td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="pagination.total > 0" style="display: flex; justify-content: space-between; align-items: center; margin-top: 16px">
          <span style="color: #94a3b8">
            第 {{ pagination.currentPage }} / {{ totalPages }} 页
          </span>
          <div class="row-actions">
            <button class="btn ghost" :disabled="pagination.currentPage <= 1" @click="prevPage">上一页</button>
            <button class="btn ghost" :disabled="pagination.currentPage >= totalPages" @click="nextPage">下一页</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
select {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.92);
  color: #e2e8f0;
  padding: 12px 14px;
  width: 100%;
  font: inherit;
}

table {
  color: #e2e8f0;
}

.level-badge {
  display: inline-block;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.level-debug {
  background: rgba(148, 163, 184, 0.18);
  color: #94a3b8;
}

.level-info {
  background: rgba(59, 130, 246, 0.18);
  color: #93c5fd;
}

.level-warn {
  background: rgba(245, 158, 11, 0.18);
  color: #fcd34d;
}

.level-error {
  background: rgba(220, 38, 38, 0.18);
  color: #fca5a5;
}
</style>
