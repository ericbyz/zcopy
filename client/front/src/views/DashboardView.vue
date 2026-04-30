<script setup>
import axios from 'axios'
import { computed, inject, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'

import TaskForm from './components/TaskForm.vue'
import TaskList from './components/TaskList.vue'
import RemoteFolderPicker from './components/RemoteFolderPicker.vue'
import { buildFileServerPathURL } from '../utils/server.js'

// Inject from App.vue
const createTaskTrigger = inject('createTaskTrigger')
const createTaskPreset = inject('createTaskPreset', ref('backup'))
const currentServerId = inject('currentServerId', ref(''))
const serverList = inject('serverList', ref([]))
const route = useRoute()

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'

const api = axios.create({ baseURL: apiBaseURL })

const token = ref('')
const loading = ref(false)

const tasks = ref([])
const onDemandStatuses = ref({})

// Wizard state
const wizardOpen = ref(false)

const remotePickerOpen = ref(false)
const remotePickerPath = ref('')
const remotePickerFolders = ref([])
const remotePickerLoading = ref(false)
const remotePickerError = ref('')
const remotePickerCurrentOwner = ref({ occupied: false, taskId: '', taskName: '' })

const localFolderInput = ref(null)
const autoInitPending = new Set()
let refreshTimer = null

const taskForm = ref(emptyTask())
const currentPageMode = computed(() => route.meta?.taskMode === 'sync' ? 'sync' : 'backup')
const isSyncPage = computed(() => currentPageMode.value === 'sync')
const serverFilter = ref('')
const filteredTasks = computed(() => tasks.value.filter((task) => {
  const mode = task.taskMode || 'backup'
  if (currentPageMode.value === 'sync' ? mode !== 'sync' : mode === 'sync') return false
  if (serverFilter.value && task.serverId !== serverFilter.value) return false
  return true
}))
const emptyText = computed(() => isSyncPage.value ? '暂无同步任务' : '暂无备份任务')
const emptyHint = computed(() => isSyncPage.value ? '点击左侧「创建同步任务」开始' : '点击左侧「创建备份任务」开始')

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
    serverId: currentServerId.value || '',
    taskMode: 'backup',
    conflictMode: 'latest',
    localPath: '',
    remotePath: '',
    autoBackup: false,
    onDemandSync: false,
    cloudOnly: false
  }
}

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

// Watch createTaskTrigger from App.vue sidebar
watch(createTaskTrigger, () => {
  openCreateWizard(createTaskPreset.value || currentPageMode.value)
})

function openCreateWizard(mode = currentPageMode.value) {
  resetTaskForm()
  taskForm.value.taskMode = mode
  wizardOpen.value = true
}

function setMessage(text) {
  if (!text) return
  if (
    text.includes('成功') ||
    text.includes('完成') ||
    text.includes('已打开') ||
    text.includes('已删除') ||
    text.includes('已停止') ||
    text.includes('已启动') ||
    text.includes('已更新') ||
    text.includes('已创建')
  ) {
    ElMessage.success(text)
  } else {
    ElMessage.error(text)
  }
}

function getTokenKey() {
  return currentServerId.value ? `zcopy_token_${currentServerId.value}` : 'zcopy_token'
}

function loadToken() {
  token.value = localStorage.getItem(getTokenKey()) || ''
}

async function fetchTasks() {
  const { data } = await api.get('/tasks')
  tasks.value = data.items || []
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
  await fetchTasks()
  await fetchOnDemandStatuses(autoInitMissing)
}

function resetTaskForm() {
  taskForm.value = emptyTask()
  remotePickerPath.value = ''
  remotePickerFolders.value = []
  remotePickerError.value = ''
  remotePickerCurrentOwner.value = { occupied: false, taskId: '', taskName: '' }
}

function editTask(task) {
  taskForm.value = {
    id: task.id,
    name: task.name,
    serverId: task.serverId || '',
    taskMode: task.taskMode || 'backup',
    conflictMode: task.conflictMode || 'latest',
    localPath: task.localPath,
    remotePath: task.remotePath,
    autoBackup: task.autoBackup,
    onDemandSync: task.onDemandSync,
    cloudOnly: task.cloudOnly || false
  }
  wizardOpen.value = true
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
    wizardOpen.value = false
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
      params: {
        ...(path ? { path } : {}),
        ...(taskForm.value.id ? { excludeTaskId: taskForm.value.id } : {}),
        ...(taskForm.value.serverId ? { serverId: taskForm.value.serverId } : {})
      }
    })
    remotePickerPath.value = data.path || ''
    remotePickerFolders.value = data.folders || []
    remotePickerCurrentOwner.value = data.currentOwner || { occupied: false, taskId: '', taskName: '' }
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
  if (remotePickerCurrentOwner.value?.occupied) {
    setMessage(`当前目录已被任务「${remotePickerCurrentOwner.value.taskName}」占用`)
    return
  }
  taskForm.value.remotePath = remotePickerPath.value
  remotePickerOpen.value = false
}

function handleBlockedRemoteFolder(folder) {
  setMessage(`文件服务器文件夹「${folder.path || folder.name}」已被任务「${folder.taskName}」占用`)
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
  try {
    await ElMessageBox.confirm('确定要删除这个任务吗？', '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
    
    loading.value = true
    await api.delete(`/tasks/${taskId}`)
    if (taskForm.value.id === taskId) {
      resetTaskForm()
      wizardOpen.value = false
    }
    setMessage('任务已删除')
    await refreshDashboard()
  } catch (error) {
    if (error !== 'cancel') {
      setMessage(error?.response?.data?.message || '删除失败')
    }
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
        setMessage(`打开文件夹失败：${failure}`)
      } else {
        setMessage(`已打开文件夹：${path}`)
      }
      return
    }
    setMessage('当前环境无法直接打开文件夹，请在桌面客户端中操作')
  } catch (error) {
    setMessage(`打开文件夹失败：${error instanceof Error ? error.message : String(error)}`)
  }
}

async function openTaskLocalFolder(task) {
  const status = onDemandStatuses.value[task.id]
  const targetPath = task.onDemandSync && status?.mountPath ? status.mountPath : task.localPath
  if (!targetPath) {
    setMessage(task.onDemandSync ? '按需同步位置尚未就绪' : '本地路径为空')
    return
  }
  await openLocation(targetPath)
}

async function openRemoteFolder(task) {
  const url = buildFileServerPathURL({
    task,
    servers: serverList.value,
    fallbackURL: import.meta.env.VITE_FILE_SERVER_WEB_URL || 'http://localhost:5176'
  })
  try {
    if (window.desktopApi?.openExternal) {
      const failure = await window.desktopApi.openExternal(url)
      if (failure) {
        setMessage(failure)
      }
      return
    }
    window.open(url, '_blank', 'noopener,noreferrer')
  } catch (error) {
    setMessage(error instanceof Error ? error.message : String(error))
  }
}

function startRefreshTimer() {
  stopRefreshTimer()
  refreshTimer = setInterval(async () => {
    if (loading.value) return
    try {
      await refreshDashboard(false)
    } catch {
      // ignore polling failures
    }
  }, 15000)
}

function stopRefreshTimer() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

onMounted(async () => {
  loadToken()
  await refreshDashboard(true)
  startRefreshTimer()
})

onUnmounted(() => {
  stopRefreshTimer()
})
</script>

<template>
  <div class="dashboard">
    <input
      ref="localFolderInput"
      type="file"
      webkitdirectory
      directory
      multiple
      style="display: none"
      @change="handleLocalFolderInput"
    />

    <div v-if="serverList && serverList.length > 1" class="task-toolbar">
      <el-select v-model="serverFilter" placeholder="全部服务器" clearable size="small" style="width: 180px">
        <el-option label="全部服务器" value="" />
        <el-option
          v-for="server in serverList"
          :key="server.id"
          :label="server.name"
          :value="server.id"
        />
      </el-select>
    </div>

    <TaskList
      :tasks="filteredTasks"
      :loading="loading"
      :on-demand-statuses="onDemandStatuses"
      :empty-text="emptyText"
      :empty-hint="emptyHint"
      :server-list="serverList"
      @edit="editTask($event)"
      @sync="syncTask($event)"
      @toggle-auto="toggleAutoBackup($event)"
      @delete="deleteTask($event)"
      @open-location="openLocation($event)"
      @open-local="openTaskLocalFolder($event)"
      @open-remote="openRemoteFolder($event)"
      @refresh="refreshDashboard(true)"
    />

    <!-- Task Wizard Dialog -->
    <TaskForm
      :open="wizardOpen"
      :task-form="taskForm"
      :mode-lock="currentPageMode"
      :loading="loading"
      :server-list="serverList"
      @save="saveTask"
      @reset="resetTaskForm"
      @pick-local="pickLocalFolder"
      @open-remote-picker="openRemotePicker"
      @close="wizardOpen = false"
    />

    <!-- Remote Folder Picker Dialog -->
    <RemoteFolderPicker
      :open="remotePickerOpen"
      :path="remotePickerPath"
      :folders="remotePickerFolders"
      :current-owner="remotePickerCurrentOwner"
      :breadcrumbs="remotePickerBreadcrumbs"
      :loading="remotePickerLoading"
      :error="remotePickerError"
      @navigate="navigateRemote($event)"
      @blocked="handleBlockedRemoteFolder"
      @navigate-parent="navigateRemoteParent"
      @choose="chooseRemotePath"
      @close="closeRemotePicker"
    />
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 900px;
}

.task-toolbar {
  margin-bottom: 12px;
}
</style>
