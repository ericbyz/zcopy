<script setup>
import axios from 'axios'
import { ref, computed, onMounted, watch } from 'vue'
import { Search, Download, RefreshCw } from 'lucide-vue-next'

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
const dateRange = ref([])
const currentPage = ref(1)
const pageSize = 20
const totalCount = ref(0)
const logItems = ref([])
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
  const level = extractField(item, 'level')
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
    a.download = 'zcopy-server-logs-' + new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19) + '.jsonl'
    document.body.appendChild(a)
    a.click()
    a.remove()
    window.URL.revokeObjectURL(blobUrl)
  } catch (e) {
    errorMsg.value = '导出日志失败'
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
    <el-alert v-if="errorMsg" type="error" :closable="false" show-icon style="margin-bottom: 16px">
      {{ errorMsg }}
    </el-alert>

    <div class="filter-bar">
      <div class="filter-row">
        <el-select v-model="filterLevel" placeholder="日志级别" style="min-width: 120px">
          <el-option label="全部" value="all" />
          <el-option label="DEBUG" value="DEBUG" />
          <el-option label="INFO" value="INFO" />
          <el-option label="WARN" value="WARN" />
          <el-option label="ERROR" value="ERROR" />
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
        <el-table-column prop="time" label="时间" min-width="180">
          <template #default="{ row }">
            {{ formatTime(extractField(row, 'time')) }}
          </template>
        </el-table-column>
        <el-table-column prop="level" label="级别" width="100">
          <template #default="{ row }">
            <el-tag :type="getLevelTagType(row)" size="small">
              {{ (extractField(row, 'level') || '').toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="user_id" label="用户" width="100">
          <template #default="{ row }">
            {{ extractField(row, 'user_id', 'userId') }}
          </template>
        </el-table-column>
        <el-table-column prop="method" label="操作" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            {{ extractField(row, 'method') ? extractField(row, 'method') + ' ' + (extractField(row, 'path') || '') : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="path" label="路径" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            {{ extractField(row, 'path') || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="msg" label="消息" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            {{ extractField(row, 'msg') || '-' }}
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
</template>

<style scoped>
.log-viewer {
  display: grid;
  gap: 16px;
}

.filter-bar {
  background: var(--z-bg-card);
  border: 1px solid var(--z-border);
  border-radius: var(--z-radius-lg);
  padding: 16px;
  display: grid;
  gap: 12px;
  backdrop-filter: var(--z-blur);
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
  backdrop-filter: var(--z-blur);
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
