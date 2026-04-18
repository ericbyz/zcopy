<script setup>
import axios from 'axios'
import { computed, onMounted, onUnmounted, ref } from 'vue'

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'

const api = axios.create({ baseURL: apiBaseURL })

const token = ref(localStorage.getItem('zcopy_token') || '')
const currentUser = ref(null)
const authMode = ref('login')
const loading = ref(false)
const message = ref('')

const tasks = ref([])
const capabilities = ref(null)
const onDemandStatuses = ref({})

const remotePickerOpen = ref(false)
const remotePickerPath = ref('')
const remotePickerFolders = ref([])
const remotePickerLoading = ref(false)
const remotePickerError = ref('')

const localFolderInput = ref(null)
const autoInitPending = new Set()
let refreshTimer = null

const authForm = ref({
  username: '',
  email: '',
  nickname: '',
  account: '',
  password: ''
})

const taskForm = ref(emptyTask())

api.interceptors.request.use((config) => {
  if (token.value) {
    config.headers.Authorization = `Bearer ${token.value}`
  }
  return config
})

function emptyTask() {
  return {
    id: '',
    name: '',
    localPath: '',
    remotePath: '',
    autoBackup: false,
    onDemandSync: false
  }
}

const formTitle = computed(() => (taskForm.value.id ? '编辑任务' : '新建任务'))

const remotePickerBreadcrumbs = computed(() => {
  const parts = remotePickerPath.value.split('/').filter(Boolean)
  const result = [{ label: '根目录', path: '' }]
  let current = ''
  for (const part of parts) {
    current = current ? `${current}/${part}` : part
    result.push({ label: part, path: current })
  }
  return result
})

function setMessage(text) {
  message.value = text || ''
}

function saveToken(value) {
  token.value = value
  if (value) {
    localStorage.setItem('zcopy_token', value)
  } else {
    localStorage.removeItem('zcopy_token')
  }
}

function formatBytes(value) {
  const size = Number(value || 0)
  if (size >= 1024 * 1024 * 1024) return `${(size / 1024 / 1024 / 1024).toFixed(2)} GB`
  if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(2)} MB`
  if (size >= 1024) return `${(size / 1024).toFixed(2)} KB`
  return `${size.toFixed(0)} B`
}

function formatSpeed(value) {
  const speed = Number(value || 0)
  if (speed <= 0) return '0 B/s'
  if (speed >= 1024 * 1024) return `${(speed / 1024 / 1024).toFixed(2)} MB/s`
  if (speed >= 1024) return `${(speed / 1024).toFixed(2)} KB/s`
  return `${speed.toFixed(0)} B/s`
}

function taskStatusText(task) {
  const state = task?.syncReport?.state || task?.status || 'idle'
  if (state === 'syncing') return '同步中'
  if (state === 'completed' || state === 'idle') return '空闲'
  if (state === 'failed' || state === 'error') return '失败'
  return state
}

function taskStatusClass(task) {
  const state = task?.syncReport?.state || task?.status || 'idle'
  if (state === 'syncing') return 'task-status syncing'
  if (state === 'failed' || state === 'error') return 'task-status failed'
  return 'task-status idle'
}

function syncProgress(task) {
  const total = Number(task?.syncReport?.totalFiles || 0)
  if (total <= 0) return 0
  const finished =
    Number(task?.syncReport?.uploadedFiles || 0) +
    Number(task?.syncReport?.failedFiles || 0)
  return Math.min(100, Math.round((finished / total) * 100))
}

async function submitAuth() {
  loading.value = true
  setMessage('')
  try {
    if (authMode.value === 'register') {
      const { data } = await api.post('/auth/register', {
        username: authForm.value.username,
        email: authForm.value.email,
        nickname: authForm.value.nickname,
        password: authForm.value.password
      })
      saveToken(data.token)
      currentUser.value = data.user
      await refreshDashboard(true)
      startRefreshTimer()
      setMessage(data.message || '注册成功')
      return
    }

    const { data } = await api.post('/auth/login', {
      account: authForm.value.account,
      password: authForm.value.password
    })
    saveToken(data.token)
    currentUser.value = data.user
    await refreshDashboard(true)
    startRefreshTimer()
    setMessage(data.message || '登录成功')
  } catch (error) {
    setMessage(error?.response?.data?.message || '操作失败')
  } finally {
    loading.value = false
  }
}

async function fetchCurrentUser() {
  if (!token.value) return
  try {
    const { data } = await api.get('/auth/me')
    currentUser.value = data.user
  } catch {
    logout(false)
  }
}

async function fetchTasks() {
  const { data } = await api.get('/tasks')
  tasks.value = data.items || []
}

async function fetchCapabilities() {
  const { data } = await api.get('/system/capabilities')
  capabilities.value = data
}

async function initTaskFileProvider(taskId, silent = false) {
  if (autoInitPending.has(taskId)) return
  autoInitPending.add(taskId)
  try {
    const { data } = await api.post(`/tasks/${taskId}/on-demand/cfapi/init`)
    if (!silent) {
      setMessage(data.message || '按需同步初始化完成')
    }
  } catch (error) {
    if (!silent) {
      setMessage(error?.response?.data?.message || '按需同步初始化失败')
    }
  } finally {
    autoInitPending.delete(taskId)
  }
}

async function fetchOnDemandStatuses(autoInitMissing = false) {
  const next = {}
  for (const task of tasks.value.filter((item) => item.onDemandSync)) {
    try {
      let { data } = await api.get(`/tasks/${task.id}/on-demand/cfapi/status`)
      if (autoInitMissing && data.supported && !data.registered && !autoInitPending.has(task.id)) {
        await initTaskFileProvider(task.id, true)
        data = (await api.get(`/tasks/${task.id}/on-demand/cfapi/status`)).data
      }
      next[task.id] = data
    } catch (error) {
      next[task.id] = {
        supported: false,
        registered: false,
        reason: error?.response?.data?.message || '查询失败'
      }
    }
  }
  onDemandStatuses.value = next
}

async function refreshDashboard(autoInitMissing = false) {
  if (!currentUser.value) return
  await Promise.all([fetchTasks(), fetchCapabilities()])
  await fetchOnDemandStatuses(autoInitMissing)
}

function resetTaskForm() {
  taskForm.value = emptyTask()
  remotePickerPath.value = ''
  remotePickerFolders.value = []
  remotePickerError.value = ''
}

function editTask(task) {
  taskForm.value = {
    id: task.id,
    name: task.name,
    localPath: task.localPath,
    remotePath: task.remotePath,
    autoBackup: task.autoBackup,
    onDemandSync: task.onDemandSync
  }
}

async function pickLocalFolder() {
  try {
    if (window.desktopApi?.pickFolder) {
      const value = await window.desktopApi.pickFolder()
      if (value) {
        taskForm.value.localPath = value
      }
      return
    }
  } catch {
    setMessage('系统目录选择失败，已切换备用方式')
  }
  localFolderInput.value?.click()
}

function handleLocalFolderInput(event) {
  const file = event.target.files?.[0]
  if (!file) return
  const rawPath = file.path || ''
  if (!rawPath) {
    setMessage('当前环境不支持读取目录绝对路径，请使用桌面客户端')
    return
  }
  const normalized = rawPath.replace(/\\/g, '/')
  const separatorIndex = normalized.lastIndexOf('/')
  taskForm.value.localPath = separatorIndex > 0 ? normalized.slice(0, separatorIndex) : normalized
  event.target.value = ''
}

async function saveTask() {
  loading.value = true
  try {
    let task
    if (taskForm.value.id) {
      const { data } = await api.put(`/tasks/${taskForm.value.id}`, taskForm.value)
      task = data.task
      setMessage(data.message || '任务已更新')
    } else {
      const { data } = await api.post('/tasks', taskForm.value)
      task = data.task
      setMessage(data.message || '任务已创建')
    }
    resetTaskForm()
    await refreshDashboard(true)
    if (task?.id && task.onDemandSync) {
      await fetchOnDemandStatuses(true)
    }
  } catch (error) {
    setMessage(error?.response?.data?.message || '保存任务失败')
  } finally {
    loading.value = false
  }
}

async function loadRemoteFolders(path = '') {
  remotePickerLoading.value = true
  remotePickerError.value = ''
  try {
    const { data } = await api.get('/remote/folders', {
      params: path ? { path } : {}
    })
    remotePickerPath.value = data.path || ''
    remotePickerFolders.value = data.folders || []
  } catch (error) {
    const text = error?.response?.data?.message || '加载远程目录失败'
    remotePickerError.value = text
    setMessage(text)
  } finally {
    remotePickerLoading.value = false
  }
}

async function openRemotePicker() {
  remotePickerOpen.value = true
  await loadRemoteFolders(taskForm.value.remotePath || '')
}

function closeRemotePicker() {
  remotePickerOpen.value = false
}

function chooseRemotePath() {
  taskForm.value.remotePath = remotePickerPath.value
  remotePickerOpen.value = false
}

async function navigateRemote(path) {
  await loadRemoteFolders(path)
}

async function navigateRemoteParent() {
  if (!remotePickerPath.value) return
  const parts = remotePickerPath.value.split('/').filter(Boolean)
  parts.pop()
  await loadRemoteFolders(parts.join('/'))
}

async function deleteTask(taskId) {
  loading.value = true
  try {
    await api.delete(`/tasks/${taskId}`)
    if (taskForm.value.id === taskId) {
      resetTaskForm()
    }
    setMessage('任务已删除')
    await refreshDashboard()
  } catch (error) {
    setMessage(error?.response?.data?.message || '删除失败')
  } finally {
    loading.value = false
  }
}

async function syncTask(taskId) {
  loading.value = true
  try {
    await api.post(`/tasks/${taskId}/sync`)
    setMessage('手动备份完成')
    await refreshDashboard()
  } catch (error) {
    setMessage(error?.response?.data?.message || '手动备份失败')
  } finally {
    loading.value = false
  }
}

async function toggleAutoBackup(task) {
  loading.value = true
  try {
    if (task.autoBackup) {
      await api.post(`/tasks/${task.id}/auto/stop`)
      setMessage('自动备份已停止')
    } else {
      await api.post(`/tasks/${task.id}/auto/start`)
      setMessage('自动备份已启动')
    }
    await refreshDashboard()
  } catch (error) {
    setMessage(error?.response?.data?.message || '操作失败')
  } finally {
    loading.value = false
  }
}

async function openLocation(path) {
  if (!path) return
  try {
    if (window.desktopApi?.openPath) {
      const failure = await window.desktopApi.openPath(path)
      if (failure) {
        setMessage(failure)
      } else {
        setMessage(`已打开 location：${path}`)
      }
      return
    }
    await navigator.clipboard.writeText(path)
    setMessage(`location 已复制：${path}`)
  } catch {
    setMessage(path)
  }
}

async function logout(showMessage = true) {
  try {
    if (token.value) {
      await api.post('/auth/logout')
    }
  } catch {
    // ignore logout transport failures
  }
  saveToken('')
  currentUser.value = null
  tasks.value = []
  capabilities.value = null
  onDemandStatuses.value = {}
  resetTaskForm()
  stopRefreshTimer()
  if (showMessage) {
    setMessage('已退出')
  }
}

function startRefreshTimer() {
  stopRefreshTimer()
  refreshTimer = setInterval(async () => {
    if (!currentUser.value || loading.value) return
    try {
      await refreshDashboard(true)
    } catch {
      // ignore polling failures
    }
  }, 2500)
}

function stopRefreshTimer() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

onMounted(async () => {
  await fetchCurrentUser()
  if (token.value && currentUser.value) {
    await refreshDashboard(true)
    startRefreshTimer()
  }
})

onUnmounted(() => {
  stopRefreshTimer()
})
</script>

<template>
  <div class="page">
    <div class="shell">
      <header class="hero card">
        <div>
          <h1>ZCopy Desktop</h1>
          <p>登录后配置任务。勾选按需同步后会自动完成初始化，不再需要手动点击。</p>
        </div>
        <div v-if="currentUser" class="user-card">
          <strong>{{ currentUser.nickname || currentUser.username }}</strong>
          <span>{{ currentUser.email }}</span>
          <button class="btn ghost" :disabled="loading" @click="logout()">退出</button>
        </div>
      </header>

      <div v-if="message" class="message card">{{ message }}</div>

      <section v-if="!currentUser" class="auth card">
        <div class="tabs">
          <button class="btn" :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
          <button class="btn" :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
        </div>

        <div v-if="authMode === 'register'" class="form-grid">
          <input v-model="authForm.username" placeholder="用户名" />
          <input v-model="authForm.email" placeholder="邮箱" />
          <input v-model="authForm.nickname" placeholder="昵称" />
          <input v-model="authForm.password" type="password" placeholder="密码（至少 6 位）" />
        </div>

        <div v-else class="form-grid">
          <input v-model="authForm.account" placeholder="用户名或邮箱" />
          <input v-model="authForm.password" type="password" placeholder="密码" />
        </div>

        <button class="btn primary block" :disabled="loading" @click="submitAuth">
          {{ loading ? '处理中...' : authMode === 'register' ? '注册并进入' : '登录' }}
        </button>
      </section>

      <section v-else class="workspace">
        <div class="card form-card">
          <div class="section-header">
            <h2>{{ formTitle }}</h2>
          </div>

          <input
            ref="localFolderInput"
            type="file"
            webkitdirectory
            directory
            multiple
            style="display: none"
            @change="handleLocalFolderInput"
          />

          <div class="form-grid">
            <input v-model="taskForm.name" placeholder="备份任务名称" />
            <div class="input-with-button">
              <input v-model="taskForm.localPath" placeholder="本地目录" />
              <button class="btn ghost" @click="pickLocalFolder">选择目录</button>
            </div>
            <div class="input-with-button">
              <input :value="taskForm.remotePath || '根目录'" readonly />
              <button class="btn ghost" @click="openRemotePicker">选择远程目录</button>
            </div>
            <div class="checkbox-row">
              <label><input v-model="taskForm.autoBackup" type="checkbox" /> 自动备份（fsnotify）</label>
              <label><input v-model="taskForm.onDemandSync" type="checkbox" /> 按需同步</label>
            </div>
          </div>

          <div class="row-actions">
            <button class="btn primary" :disabled="loading" @click="saveTask">保存任务</button>
            <button class="btn ghost" :disabled="loading" @click="resetTaskForm">重置</button>
          </div>
        </div>

        <div class="card task-card">
          <div class="section-header">
            <h2>备份任务</h2>
            <button class="btn ghost" :disabled="loading" @click="refreshDashboard(true)">刷新</button>
          </div>

          <div v-if="tasks.length === 0" class="empty">暂无任务</div>

          <div v-for="task in tasks" :key="task.id" class="task-row">
            <div class="task-main">
              <h3>{{ task.name }}</h3>
              <p>{{ task.localPath }} → {{ task.remotePath || '根目录' }}</p>
              <div class="task-meta">
                <span :class="taskStatusClass(task)">{{ taskStatusText(task) }}</span>
                <span>自动：{{ task.autoBackup ? '开启' : '关闭' }}</span>
                <span>上次同步：{{ task.lastSyncAt ? new Date(task.lastSyncAt).toLocaleString() : '无' }}</span>
              </div>

              <div v-if="task.syncReport?.state === 'syncing'" class="sync-box">
                <div class="sync-meta">
                  <span>{{ task.syncReport.message || taskStatusText(task) }}</span>
                  <span>{{ task.syncReport.uploadedFiles || 0 }}/{{ task.syncReport.totalFiles || 0 }} 文件</span>
                  <span>{{ formatSpeed(task.syncReport.speedBytesPerSec) }}</span>
                </div>
                <div class="sync-meta">
                  <span>传输：{{ formatBytes(task.syncReport.transferredBytes) }}</span>
                  <span>总量：{{ formatBytes(task.syncReport.totalBytes) }}</span>
                  <span>失败：{{ task.syncReport.failedFiles || 0 }}</span>
                </div>
                <div class="progress-bar">
                  <div class="progress-value" :style="{ width: `${syncProgress(task)}%` }"></div>
                </div>
              </div>

              <div v-if="task.syncReport?.state === 'failed' && task.syncReport.failedFilePaths?.length" class="error-box">
                <div class="error-title">失败文件</div>
                <div v-for="item in task.syncReport.failedFilePaths.slice(0, 8)" :key="item" class="failed-item">
                  {{ item }}
                </div>
              </div>

              <div v-if="task.lastError" class="error-box">{{ task.lastError }}</div>
            </div>

            <div class="task-actions">
              <button class="btn ghost" @click="editTask(task)">编辑</button>
              <button class="btn primary" :disabled="loading" @click="syncTask(task.id)">手动备份</button>
              <button class="btn ghost" :disabled="loading" @click="toggleAutoBackup(task)">
                {{ task.autoBackup ? '停止自动' : '开启自动' }}
              </button>
              <button class="btn danger" :disabled="loading" @click="deleteTask(task.id)">删除</button>
            </div>

            <div v-if="task.onDemandSync" class="ondemand-panel">
              <span>按需同步：{{ onDemandStatuses[task.id]?.supported ? '支持' : '不支持' }}</span>
              <span>同步根：{{ onDemandStatuses[task.id]?.registered ? '已注册' : '未注册' }}</span>
              <span v-if="onDemandStatuses[task.id]?.mountPath">
                location：{{ onDemandStatuses[task.id].mountPath }}
              </span>
              <span v-else-if="onDemandStatuses[task.id]?.reason">
                原因：{{ onDemandStatuses[task.id].reason }}
              </span>
              <button
                v-if="onDemandStatuses[task.id]?.mountPath"
                class="btn ghost"
                :disabled="loading"
                @click="openLocation(onDemandStatuses[task.id].mountPath)"
              >
                打开 location
              </button>
            </div>
          </div>
        </div>

        <div v-if="capabilities" class="card info-card">
          <h2>平台能力</h2>
          <p>当前系统：{{ capabilities.os }}</p>
          <p>按需同步支持：{{ capabilities.onDemandSupport ? '支持' : '不支持' }}</p>
          <p>模式：{{ capabilities.onDemandMode }}</p>
          <p>说明：启用按需同步后会自动初始化，同步目录请直接从 location 中按需打开文件。</p>
        </div>

      </section>

      <div v-if="remotePickerOpen" class="picker-mask" @click.self="closeRemotePicker">
        <div class="picker card">
          <div class="section-header">
            <h3>选择远程目录</h3>
            <button class="btn ghost" @click="closeRemotePicker">关闭</button>
          </div>
          <div class="picker-toolbar">
            <div class="breadcrumbs">
              <button
                v-for="item in remotePickerBreadcrumbs"
                :key="item.path || 'root'"
                class="btn link"
                @click="navigateRemote(item.path)"
              >
                {{ item.label }}
              </button>
            </div>
            <div class="row-actions">
              <button class="btn ghost" :disabled="remotePickerLoading || !remotePickerPath" @click="navigateRemoteParent">
                返回上级
              </button>
              <button class="btn primary" :disabled="remotePickerLoading" @click="chooseRemotePath">选择当前目录</button>
            </div>
          </div>

          <div v-if="remotePickerError" class="picker-error">{{ remotePickerError }}</div>
          <div v-if="remotePickerFolders.length === 0" class="empty">
            {{ remotePickerLoading ? '加载中...' : '当前目录暂无子目录' }}
          </div>
          <button
            v-for="folder in remotePickerFolders"
            :key="folder.path"
            class="folder-row"
            :disabled="remotePickerLoading"
            @click="navigateRemote(folder.path)"
          >
            <span>📁 {{ folder.name }}</span>
            <span>{{ folder.path || '根目录' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
