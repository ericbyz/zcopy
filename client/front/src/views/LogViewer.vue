<script setup>
import axios from 'axios'
import { inject, ref, computed, onMounted, watch } from 'vue'
import { Search, Download, RefreshCw, Trash2 } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'

const currentServerId = inject('currentServerId', ref(''))
const token = ref('')
const api = axios.create({ baseURL: apiBaseURL })

api.interceptors.request.use((config) => {
  if (token.value) {
    config.headers.Authorization = `Bearer ${token.value}`
  }
  return config
})

const filterLevel = ref('all')
const filterKeyword = ref('')
const filterTaskId = ref('all')
const filterStartDate = ref('')
const filterEndDate = ref('')
const dateRange = ref([])
const currentPage = ref(1)
const pageSize = 20
const totalCount = ref(0)
const logItems = ref([])
const tasks = ref([])
const loading = ref(false)
const errorMsg = ref('')

// Sync dateRange with filterStartDate/filterEndDate
watch(dateRange, (newRange) => {
  if (newRange && newRange.length === 2) {
    filterStartDate.value = newRange[0] ? new Date(newRange[0]).toISOString().slice(0, 16) : ''
    filterEndDate.value = newRange[1] ? new Date(newRange[1]).toISOString().slice(0, 16) : ''
  } else {
    filterStartDate.value = ''
    filterEndDate.value = ''
  }
})

const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / pageSize)))

const visibleRange = computed(() => {
  if (totalCount.value === 0) return '0'
  const start = (currentPage.value - 1) * pageSize + 1
  const end = Math.min(currentPage.value * pageSize, totalCount.value)
  return `${start}-${end}`
})

function getLevelTagType(item) {
  const level = item.level
  switch ((level || '').toUpperCase()) {
    case 'DEBUG': return 'info'
    case 'INFO': return 'primary'
    case 'WARN': return 'warning'
    case 'ERROR': return 'danger'
    default: return 'info'
  }
}

function buildParams() {
  const params = {}
  params.page = currentPage.value
  params.pageSize = pageSize
  if (filterLevel.value && filterLevel.value !== 'all') params.level = filterLevel.value
  if (filterTaskId.value && filterTaskId.value !== 'all') params.taskId = filterTaskId.value
  if (filterKeyword.value) params.keyword = filterKeyword.value
  if (filterStartDate.value) params.startTime = new Date(filterStartDate.value).toISOString()
  if (filterEndDate.value) params.endTime = new Date(filterEndDate.value).toISOString()
  return params
}

async function fetchTasks() {
  try {
    const { data } = await api.get('/tasks')
    tasks.value = data.items || []
  } catch {
    // silent — tasks dropdown is optional
  }
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
    ElMessage.error(errorMsg.value)
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
  filterTaskId.value = 'all'
  dateRange.value = []
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
    a.download = 'zcopy-logs-' + new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19) + '.jsonl'
    document.body.appendChild(a)
    a.click()
    a.remove()
    window.URL.revokeObjectURL(blobUrl)
  } catch (e) {
    errorMsg.value = '导出日志失败'
    ElMessage.error(errorMsg.value)
  }
}

async function handleClearLogs() {
  try {
    await ElMessageBox.confirm('确定要清空所有客户端日志吗？', '清空日志', {
      type: 'warning',
      confirmButtonText: '清空',
      cancelButtonText: '取消'
    })
    loading.value = true
    const { data } = await api.delete('/logs')
    ElMessage.success(data.message || '日志已清空')
    currentPage.value = 1
    await fetchLogs()
  } catch (error) {
    if (error !== 'cancel') {
      const text = error?.response?.data?.message || '清空日志失败'
      errorMsg.value = text
      ElMessage.error(text)
    }
  } finally {
    loading.value = false
  }
}

function handlePageChange(page) {
  currentPage.value = page
  fetchLogs()
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN')
}

onMounted(() => {
  const tokenKey = currentServerId.value ? `zcopy_token_${currentServerId.value}` : 'zcopy_token'
  token.value = localStorage.getItem(tokenKey) || ''
  if (token.value) {
    fetchTasks()
    fetchLogs()
  }
})
</script>

<template>
  <div class="log-view">
    <header class="log-toolbar">
      <div>
        <h1>日志</h1>
        <p>共 {{ totalCount }} 条，当前 {{ visibleRange }}</p>
      </div>

      <div class="toolbar-actions">
        <el-button :icon="RefreshCw" :loading="loading" @click="handleReset">刷新</el-button>
        <el-button :icon="Download" @click="handleExport">导出</el-button>
        <el-button :icon="Trash2" type="danger" plain @click="handleClearLogs">清空</el-button>
      </div>
    </header>

    <el-alert v-if="errorMsg" type="error" :closable="false" show-icon class="log-alert">
        {{ errorMsg }}
    </el-alert>

    <section class="filter-bar">
      <el-select v-model="filterLevel" placeholder="级别" class="level-filter" @change="handleSearch">
        <el-option label="全部级别" value="all" />
        <el-option label="DEBUG" value="debug" />
        <el-option label="INFO" value="info" />
        <el-option label="WARN" value="warn" />
        <el-option label="ERROR" value="error" />
      </el-select>

      <el-select v-model="filterTaskId" placeholder="任务" class="task-filter" @change="handleSearch">
        <el-option label="全部任务" value="all" />
        <el-option v-for="t in tasks" :key="t.id" :label="t.name" :value="t.id" />
      </el-select>

      <el-input v-model="filterKeyword" placeholder="搜索消息或路径" class="keyword-filter" clearable @keyup.enter="handleSearch">
        <template #prefix>
          <Search class="el-input__icon" />
        </template>
      </el-input>

      <el-date-picker
        v-model="dateRange"
        type="datetimerange"
        range-separator="至"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        class="date-filter"
      />

      <el-button type="primary" :loading="loading" @click="handleSearch">
        {{ loading ? '查询中' : '查询' }}
      </el-button>
    </section>

    <section class="log-table" v-loading="loading">
      <el-table :data="logItems" stripe height="100%" style="width: 100%">
        <el-table-column prop="createdAt" label="时间" width="156">
          <template #default="{ row }">
            <span class="time-cell">{{ formatTime(row.createdAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="level" label="级别" width="84">
          <template #default="{ row }">
            <el-tag :type="getLevelTagType(row)" size="small" effect="light">
              {{ (row.level || '').toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="taskName" label="任务" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.taskName || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="filePath" label="路径" min-width="170" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="path-cell">{{ row.filePath || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="message" label="消息" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.message || '-' }}
          </template>
        </el-table-column>
      </el-table>

      <div v-if="!loading && logItems.length === 0" class="empty-logs">
        <el-empty description="暂无日志" :image-size="86" />
      </div>
    </section>

    <footer v-if="totalCount > 0" class="pagination">
      <el-pagination
        :total="totalCount"
        v-model:current-page="currentPage"
        :page-size="pageSize"
        small
        layout="prev, pager, next"
        @current-change="handlePageChange"
      />
      <span>{{ currentPage }} / {{ totalPages }}</span>
    </footer>
    </div>
</template>

<style scoped>
.log-view {
  height: calc(100vh - 52px);
  min-height: 0;
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  gap: 10px;
}

.log-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.log-toolbar h1 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
}

.log-toolbar p {
  margin: 4px 0 0;
  color: var(--z-text-muted);
  font-size: 0.8rem;
}

.toolbar-actions {
  display: flex;
  gap: 8px;
  flex: 0 0 auto;
}

.log-alert {
  margin: 0;
}

.filter-bar {
  min-width: 0;
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
  padding: 10px;
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.level-filter {
  width: 112px;
}

.task-filter {
  width: 150px;
}

.keyword-filter {
  width: 190px;
  flex: 1 1 170px;
}

.date-filter {
  width: 290px;
}

.log-table {
  position: relative;
  min-height: 0;
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
  overflow: hidden;
}

.log-table :deep(.el-table) {
  --el-table-header-bg-color: var(--z-bg-elevated);
  --el-table-tr-bg-color: var(--z-bg-elevated);
  --el-table-row-hover-bg-color: var(--z-accent-bg);
  font-size: 0.82rem;
}

.log-table :deep(.el-table th.el-table__cell) {
  color: var(--z-text-muted);
  font-size: 0.78rem;
  font-weight: 600;
}

.log-table :deep(.el-table .cell) {
  line-height: 1.35;
}

.time-cell,
.path-cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.path-cell {
  color: var(--z-text-secondary);
}

.empty-logs {
  position: absolute;
  inset: 44px 0 0;
  display: grid;
  place-items: center;
  background: var(--z-bg-elevated);
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  color: var(--z-text-muted);
  font-size: 0.8rem;
}

@media (max-width: 760px) {
  .log-toolbar {
    align-items: flex-start;
  }

  .toolbar-actions {
    gap: 6px;
  }

  .toolbar-actions :deep(.el-button) {
    padding-left: 8px;
    padding-right: 8px;
  }

  .level-filter,
  .task-filter,
  .keyword-filter,
  .date-filter,
  .filter-bar > :deep(.el-button) {
    width: 100% !important;
  }
}
</style>
