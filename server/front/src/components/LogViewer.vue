<script setup>
import axios from 'axios'
import { ref, computed, onMounted } from 'vue'

const apiBaseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8890/api/v1'
const token = ref(localStorage.getItem('zcopy_token') || '')

const api = axios.create({ baseURL: apiBaseURL })
api.interceptors.request.use((config) => {
  if (token.value) {
    config.headers.Authorization = `Bearer ${token.value}`
  }
  return config
})

const filterLevel = ref('all')
const filterKeyword = ref('')
const filterStartDate = ref('')
const filterEndDate = ref('')
const currentPage = ref(1)
const pageSize = 20
const totalCount = ref(0)
const logItems = ref([])
const loading = ref(false)
const errorMsg = ref('')

const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / pageSize)))

function buildParams() {
  const params = {}
  params.page = currentPage.value
  params.pageSize = pageSize
  if (filterLevel.value && filterLevel.value !== 'all') params.level = filterLevel.value
  if (filterKeyword.value) params.keyword = filterKeyword.value
  if (filterStartDate.value) params.startTime = new Date(filterStartDate.value).toISOString()
  if (filterEndDate.value) params.endTime = new Date(filterEndDate.value).toISOString()
  return params
}

async function fetchLogs() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get('/logs', { params: buildParams() })
    logItems.value = data.items || []
    totalCount.value = data.total || 0
    currentPage.value = data.page || currentPage.value
  } catch (e) {
    errorMsg.value = e?.response?.data?.message || '查询日志失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  currentPage.value = 1
  fetchLogs()
}

function handleReset() {
  filterLevel.value = 'all'
  filterKeyword.value = ''
  filterStartDate.value = ''
  filterEndDate.value = ''
  currentPage.value = 1
  fetchLogs()
}

async function handleExport() {
  const params = buildParams()
  delete params.page
  delete params.pageSize
  try {
    const resp = await api.get('/logs/export', { params, responseType: 'blob' })
    const blobUrl = window.URL.createObjectURL(new Blob([resp.data]))
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = 'zcopy-server-logs-' + new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19) + '.jsonl'
    document.body.appendChild(a)
    a.click()
    a.remove()
    window.URL.revokeObjectURL(blobUrl)
  } catch (e) {
    errorMsg.value = '导出日志失败'
  }
}

function goToPrevPage() {
  if (currentPage.value > 1) {
    currentPage.value--
    fetchLogs()
  }
}

function goToNextPage() {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
    fetchLogs()
  }
}

function getLevelBadgeClass(item) {
  const level = extractField(item, 'level')
  switch ((level || '').toUpperCase()) {
    case 'DEBUG': return 'badge-debug'
    case 'INFO': return 'badge-info'
    case 'WARN': return 'badge-warn'
    case 'ERROR': return 'badge-error'
    default: return ''
  }
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN')
}

function extractField(item, ...fields) {
  for (const f of fields) {
    // Check both snake_case and camelCase versions of each field
    const camelCaseF = f.replace(/_([a-z])/g, (_, c) => c.toUpperCase())
    if (item[f] !== undefined && item[f] !== null && item[f] !== '') return item[f]
    if (item[camelCaseF] !== undefined && item[camelCaseF] !== null && item[camelCaseF] !== '') return item[camelCaseF]
  }
  return '-'
}

onMounted(() => {
  token.value = localStorage.getItem('zcopy_token') || ''
  if (token.value) fetchLogs()
})
</script>

<template>
  <div class="log-viewer">
    <div v-if="errorMsg" style="padding: 12px 16px; color: #fca5a5; background: rgba(220,38,38,0.1); border-radius: 12px; margin-bottom: 12px">{{ errorMsg }}</div>

    <div class="filter-bar">
      <div class="filter-row">
        <select v-model="filterLevel" class="small-input">
          <option value="all">全部</option>
          <option value="DEBUG">DEBUG</option>
          <option value="INFO">INFO</option>
          <option value="WARN">WARN</option>
          <option value="ERROR">ERROR</option>
        </select>

        <input v-model="filterKeyword" class="small-input" placeholder="搜索日志..." />

        <input v-model="filterStartDate" type="datetime-local" class="small-input" />

        <input v-model="filterEndDate" type="datetime-local" class="small-input" />
      </div>

      <div class="filter-actions">
        <button class="primary-btn" :disabled="loading" @click="handleSearch">{{ loading ? '查询中...' : '查询' }}</button>
        <button class="ghost-btn" @click="handleReset">重置</button>
        <button class="ghost-btn" @click="handleExport">导出</button>
      </div>
    </div>

    <div v-if="loading" class="empty">加载中...</div>

    <div v-else class="log-table">
      <div class="log-header">
        <span>时间</span>
        <span>级别</span>
        <span>用户</span>
        <span>操作</span>
        <span>路径</span>
        <span>消息</span>
      </div>

      <div v-if="logItems.length === 0" class="empty">暂无日志</div>

      <div v-for="(item, idx) in logItems" :key="idx" class="log-row">
        <span style="white-space: nowrap; font-size: 13px">{{ formatTime(extractField(item, 'time')) }}</span>
        <span><span :class="['badge', getLevelBadgeClass(item)]">{{ (extractField(item, 'level') || '').toUpperCase() }}</span></span>
        <span>{{ extractField(item, 'user_id', 'userId') }}</span>
        <span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ extractField(item, 'method') ? extractField(item, 'method') + ' ' + (extractField(item, 'path') || '') : '-' }}</span>
        <span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ extractField(item, 'path') || '-' }}</span>
        <span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap" :title="extractField(item, 'msg')">{{ extractField(item, 'msg') || '-' }}</span>
      </div>
    </div>

    <div v-if="totalCount > 0" class="pagination">
      <div class="pagination-info">共 {{ totalCount }} 条 · 第 {{ currentPage }} / {{ totalPages }} 页</div>
      <div class="pagination-buttons">
        <button class="ghost-btn" :disabled="currentPage <= 1" @click="goToPrevPage">上一页</button>
        <span class="page-number">{{ currentPage }}</span>
        <button class="ghost-btn" :disabled="currentPage >= totalPages" @click="goToNextPage">下一页</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-viewer {
  display: grid;
  gap: 16px;
}

.filter-bar {
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(148, 163, 184, 0.08);
  border-radius: 14px;
  padding: 16px;
  display: grid;
  gap: 12px;
}

.filter-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.small-input {
  width: auto;
  min-width: 160px;
}

.log-table {
  display: grid;
  gap: 8px;
}

.log-header,
.log-row {
  display: grid;
  grid-template-columns: minmax(140px, 1.2fr) minmax(80px, 0.8fr) minmax(80px, 0.8fr) minmax(120px, 1.2fr) minmax(140px, 1.3fr) minmax(180px, 2fr);
  gap: 12px;
  align-items: center;
  padding: 14px 16px;
  border-radius: 14px;
}

.log-header {
  background: rgba(148, 163, 184, 0.08);
  color: #94a3b8;
}

.log-row {
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(148, 163, 184, 0.08);
  color: #e2e8f0;
}

.badge {
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
}

.badge-debug {
  background: rgba(148, 163, 184, 0.18);
  color: #94a3b8;
}

.badge-info {
  background: rgba(59, 130, 246, 0.18);
  color: #93c5fd;
}

.badge-warn {
  background: rgba(245, 158, 11, 0.18);
  color: #fcd34d;
}

.badge-error {
  background: rgba(220, 38, 38, 0.18);
  color: #fca5a5;
}

.empty {
  padding: 28px;
  text-align: center;
  color: #94a3b8;
  background: rgba(148, 163, 184, 0.06);
  border-radius: 14px;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  color: #94a3b8;
}

.pagination-buttons {
  display: flex;
  gap: 10px;
  align-items: center;
}

.page-number {
  padding: 8px 16px;
  background: rgba(148, 163, 184, 0.12);
  border-radius: 12px;
  min-width: 60px;
  text-align: center;
}

@media (max-width: 1024px) {
  .log-header {
    display: none;
  }

  .log-row {
    grid-template-columns: 1fr;
  }

  .filter-row,
  .filter-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .small-input {
    width: 100%;
  }
}
</style>
