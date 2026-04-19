<script setup>
import axios from 'axios'
import { ref, inject, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import LogViewer from './components/LogViewer.vue'
import ThemeToggle from './components/ThemeToggle.vue'
import AuthCard from './components/AuthCard.vue'
import FileBrowser from './components/FileBrowser.vue'
import { parseDownloadFilename } from './utils/format.js'

const zhCn = inject('element-locale')

const apiBaseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8890/api/v1'
const token = ref(localStorage.getItem('zcopy_token') || '')
const currentUser = ref(null)
const authMode = ref('login')
const activeTab = ref('files')
const loading = ref(false)
const message = ref('')
const currentPath = ref('')
const fileItems = ref([])

const authForm = ref({
  username: '',
  email: '',
  nickname: '',
  account: '',
  password: ''
})

const api = axios.create({
  baseURL: apiBaseURL
})

api.interceptors.request.use((config) => {
  if (token.value) {
    config.headers.Authorization = `Bearer ${token.value}`
  }
  return config
})

function setMessage(text) {
  message.value = text
  if (text) {
    if (text.includes('成功') || text.includes('已退出')) {
      ElMessage.success(text)
    } else {
      ElMessage.error(text)
    }
  }
}

function saveToken(value) {
  token.value = value
  if (value) {
    localStorage.setItem('zcopy_token', value)
  } else {
    localStorage.removeItem('zcopy_token')
  }
}

async function submitAuth() {
  loading.value = true
  setMessage('')

  try {
    if (authMode.value === 'register') {
      const { data } = await api.post('/auth/register', {
        username: authForm.value.username,
        email: authForm.value.email,
        password: authForm.value.password,
        nickname: authForm.value.nickname
      })
      saveToken(data.token)
      currentUser.value = data.user
      await fetchFiles('')
      setMessage(data.message || '注册成功')
    } else {
      const { data } = await api.post('/auth/login', {
        account: authForm.value.account,
        password: authForm.value.password
      })
      saveToken(data.token)
      currentUser.value = data.user
      await fetchFiles('')
      setMessage(data.message || '登录成功')
    }
  } catch (error) {
    setMessage(error?.response?.data?.message || '操作失败')
  } finally {
    loading.value = false
  }
}

async function fetchCurrentUser() {
  if (!token.value) {
    return
  }

  try {
    const { data } = await api.get('/auth/me')
    currentUser.value = data.user
  } catch (error) {
    logout()
  }
}

async function fetchFiles(path = currentPath.value) {
  if (!token.value) {
    return
  }

  loading.value = true
  try {
    const { data } = await api.get('/files', {
      params: { path }
    })
    currentPath.value = data.path || ''
    fileItems.value = data.items || []
  } catch (error) {
    setMessage(error?.response?.data?.message || '读取文件失败')
  } finally {
    loading.value = false
  }
}

async function createFolder(name) {
  if (!name.trim()) {
    setMessage('请输入文件夹名称')
    return
  }

  loading.value = true
  try {
    const { data } = await api.post('/files/folder', {
      path: currentPath.value,
      name
    })
    setMessage(data.message || '创建成功')
    await fetchFiles(currentPath.value)
  } catch (error) {
    setMessage(error?.response?.data?.message || '创建文件夹失败')
  } finally {
    loading.value = false
  }
}



async function submitUpload(file) {
  if (!file) {
    setMessage('请选择文件')
    return
  }

  const formData = new FormData()
  formData.append('file', file)
  formData.append('path', currentPath.value)

  loading.value = true
  try {
    const { data } = await api.post('/files/upload', formData)
    setMessage(data.message || '上传成功')
    await fetchFiles(currentPath.value)
  } catch (error) {
    setMessage(error?.response?.data?.message || '上传失败')
  } finally {
    loading.value = false
  }
}

async function downloadItem(item) {
  if (!token.value) {
    setMessage('请先登录')
    return
  }

  loading.value = true
  try {
    const response = await api.get('/files/download', {
      params: { path: item.path },
      responseType: 'blob'
    })
    const blobUrl = window.URL.createObjectURL(new Blob([response.data]))
    const anchor = document.createElement('a')
    anchor.href = blobUrl
    anchor.download = parseDownloadFilename(response.headers['content-disposition'], item.name)
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    window.URL.revokeObjectURL(blobUrl)
  } catch (error) {
    setMessage(error?.response?.data?.message || '下载失败')
  } finally {
    loading.value = false
  }
}

function handleNavigate(item) {
  if (item.isDirectory) {
    fetchFiles(item.path)
  } else {
    downloadItem(item)
  }
}

async function removeItem(item) {
  try {
    await ElMessageBox.confirm('确定要删除吗？', '确认', { type: 'warning' })
    loading.value = true
    const { data } = await api.delete('/files', {
      params: { path: item.path }
    })
    setMessage(data.message || '删除成功')
    await fetchFiles(currentPath.value)
  } catch (error) {
    if (error !== 'cancel') {
      setMessage(error?.response?.data?.message || '删除失败')
    }
  } finally {
    loading.value = false
  }
}

function logout() {
  saveToken('')
  currentUser.value = null
  currentPath.value = ''
  fileItems.value = []
  authForm.value.password = ''
  setMessage('已退出登录')
}

onMounted(async () => {
  await fetchCurrentUser()
  if (token.value && currentUser.value) {
    await fetchFiles('')
  }
})
</script>

<template>
  <el-config-provider :locale="zhCn">
    <div class="page">
      <div class="shell">
        <div class="hero">
          <div>
            <h1>ZCopy 文件服务器</h1>
            <p>支持用户注册登录、独立文件空间、上传下载与基础文件管理。</p>
          </div>
          <div class="hero-actions">
            <ThemeToggle />
            <el-card v-if="currentUser" class="user-card">
              <div>{{ currentUser.nickname || currentUser.username }}</div>
              <div class="user-email">{{ currentUser.email }}</div>
              <el-button size="small" @click="logout">退出登录</el-button>
            </el-card>
          </div>
        </div>

        <div v-if="!token || !currentUser" class="auth-layout">
          <AuthCard
            :loading="loading"
            :auth-mode="authMode"
            :auth-form="authForm"
            @update:auth-mode="authMode = $event"
            @submit="submitAuth"
          />
        </div>

        <div v-else class="workspace">
          <el-tabs v-model="activeTab">
            <el-tab-pane label="文件浏览" name="files">
              <FileBrowser
                :current-path="currentPath"
                :file-items="fileItems"
                :loading="loading"
                @navigate="handleNavigate"
                @create-folder="createFolder"
                @upload="submitUpload"
                @delete="removeItem"
                @refresh="fetchFiles(currentPath)"
              />
            </el-tab-pane>
            <el-tab-pane label="日志查看" name="logs">
              <LogViewer />
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>
  </el-config-provider>
</template>

<style scoped>
.hero-actions {
  display: flex;
  gap: 16px;
  align-items: center;
}

.user-card {
  min-width: 220px;
}

.user-email {
  color: var(--z-text-muted);
  font-size: 0.9em;
}

@media (max-width: 640px) {
  .hero {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
