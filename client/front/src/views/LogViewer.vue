<script setup>
import axios from 'axios'
import { ref, computed, onMounted, watch } from 'vue'
import { Search, Download, RefreshCw } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'

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

function handlePageChange(page) {
  currentPage.value = page
  fetchLogs()
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN')
}

onMounted(() => {
  token.value = localStorage.getItem('zcopy_token') || ''
  if (token.value) {
    fetchTasks()
    fetchLogs()
  }
})
</script>

<template>
  <div class="page">
    <div class="shell">
      <header class="hero">
        <div>
          <h1>日志查看器</h1>
          <p>查看和管理传输日志</p>
        </div>
      </header>

      <el-alert v-if="errorMsg" type="error" :closable="false" show-icon style="margin-bottom: 16px">
        {{ errorMsg }}
      </el-alert>

      <div class="filter-bar">
        <div class="filter-row">
          <el-select v-model="filterLevel" placeholder="日志级别" style="min-width: 120px">
            <el-option label="全部" value="all" />
            <el-option label="DEBUG" value="debug" />
            <el-option label="INFO" value="info" />
            <el-option label="WARN" value="warn" />
            <el-option label="ERROR" value="error" />
          </el-select>

          <el-select v-model="filterTaskId" placeholder="任务" style="min-width: 180px">
            <el-option label="全部任务" value="all" />
            <el-option v-for="t in tasks" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>

          <el-input v-model="filterKeyword" placeholder="搜索日志..." style="min-width: 220px" clearable>
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
            style="min-width: 360px"
          />
        </div>

        <div class="filter-actions">
          <el-button type="primary" :loading="loading" @click="handleSearch">
            {{ loading ? '查询中...' : '查询' }}
          </el-button>
          <el-button :icon="RefreshCw" @click="handleReset">重置</el-button>
          <el-button :icon="Download" @click="handleExport">导出</el-button>
        </div>
      </div>

      <div class="table-container" v-loading="loading">
        <el-table :data="logItems" stripe style="width: 100%">
          <el-table-column prop="createdAt" label="时间" min-width="180">
            <template #default="{ row }">
              {{ formatTime(row.createdAt) }}
            </template>
          </el-table-column>
          <el-table-column prop="level" label="级别" width="100">
            <template #default="{ row }">
              <el-tag :type="getLevelTagType(row)" size="small">
                {{ (row.level || '').toUpperCase() }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="taskName" label="任务名" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.taskName || '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="filePath" label="文件路径" min-width="240" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.filePath || '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="message" label="消息" min-width="300" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.message || '-' }}
            </template>
          </el-table-column>
        </el-table>

        <el-empty v-if="!loading && logItems.length === 0" description="暂无日志" />

        <div v-if="totalCount > 0" class="pagination">
          <el-pagination
            :total="totalCount"
            v-model:current-page="currentPage"
            :page-size="pageSize"
            layout="total, prev, pager, next"
            @current-change="handlePageChange"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.filter-bar {
  background: var(--z-bg-card);
  border: 1px solid var(--z-border);
  border-radius: var(--z-radius-lg);
  padding: 16px;
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
}

.filter-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: center;
}

.filter-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.table-container {
  background: var(--z-bg-card);
  border: 1px solid var(--z-border);
  border-radius: var(--z-radius-lg);
  padding: 16px;
  overflow: visible;
}

.table-container :deep(.el-table__body-wrapper) {
  overflow: visible;
}

.table-container :deep(.el-table__body-wrapper tr:hover > td) {
  overflow: visible;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 1024px) {
  .filter-row,
  .filter-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-row > * {
    width: 100% !important;
  }
}
</style>
