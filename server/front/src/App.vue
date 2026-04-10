<script setup>
import axios from 'axios'
import { computed, onMounted, ref } from 'vue'

const apiBaseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1'
const token = ref(localStorage.getItem('zcopy_token') || '')
const currentUser = ref(null)
const authMode = ref('login')
const loading = ref(false)
const message = ref('')
const currentPath = ref('')
const fileItems = ref([])
const uploadFile = ref(null)
const folderName = ref('')

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

const breadcrumbList = computed(() => {
  if (!currentPath.value) {
    return [{ label: '根目录', path: '' }]
  }

  const parts = currentPath.value.split('/').filter(Boolean)
  return [{ label: '根目录', path: '' }].concat(
    parts.map((part, index) => ({
      label: part,
      path: parts.slice(0, index + 1).join('/')
    }))
  )
})

function setMessage(text) {
  message.value = text
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

async function createFolder() {
  if (!folderName.value.trim()) {
    setMessage('请输入文件夹名称')
    return
  }

  loading.value = true
  try {
    const { data } = await api.post('/files/folder', {
      path: currentPath.value,
      name: folderName.value
    })
    folderName.value = ''
    setMessage(data.message || '创建成功')
    await fetchFiles(currentPath.value)
  } catch (error) {
    setMessage(error?.response?.data?.message || '创建文件夹失败')
  } finally {
    loading.value = false
  }
}

function handleFileChange(event) {
  uploadFile.value = event.target.files?.[0] || null
}

function parseDownloadFilename(contentDisposition, fallbackName) {
  if (!contentDisposition) {
    return fallbackName
  }

  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }

  const basicMatch = contentDisposition.match(/filename="?([^";]+)"?/i)
  if (basicMatch?.[1]) {
    return basicMatch[1]
  }

  return fallbackName
}

async function submitUpload() {
  if (!uploadFile.value) {
    setMessage('请选择文件')
    return
  }

  const formData = new FormData()
  formData.append('file', uploadFile.value)
  formData.append('path', currentPath.value)

  loading.value = true
  try {
    const { data } = await api.post('/files/upload', formData)
    setMessage(data.message || '上传成功')
    uploadFile.value = null
    const input = document.getElementById('file-input')
    if (input) {
      input.value = ''
    }
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

function openItem(item) {
  if (item.isDirectory) {
    fetchFiles(item.path)
  } else {
    downloadItem(item)
  }
}

async function removeItem(item) {
  loading.value = true
  try {
    const { data } = await api.delete('/files', {
      params: { path: item.path }
    })
    setMessage(data.message || '删除成功')
    await fetchFiles(currentPath.value)
  } catch (error) {
    setMessage(error?.response?.data?.message || '删除失败')
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
  <div class="page">
    <div class="shell">
      <div class="hero">
        <div>
          <h1>ZCopy 文件服务器</h1>
          <p>支持用户注册登录、独立文件空间、上传下载与基础文件管理。</p>
        </div>
        <div v-if="currentUser" class="user-card">
          <div>{{ currentUser.nickname || currentUser.username }}</div>
          <div>{{ currentUser.email }}</div>
          <button class="ghost-btn" @click="logout">退出登录</button>
        </div>
      </div>

      <div v-if="message" class="message">{{ message }}</div>

      <div v-if="!token || !currentUser" class="auth-layout">
        <div class="auth-card">
          <div class="tabs">
            <button :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
            <button :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
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

          <button class="primary-btn" :disabled="loading" @click="submitAuth">
            {{ loading ? '处理中...' : authMode === 'register' ? '注册并进入' : '登录' }}
          </button>
        </div>
      </div>

      <div v-else class="workspace">
        <div class="toolbar">
          <div class="breadcrumb">
            <button
              v-for="item in breadcrumbList"
              :key="item.path || 'root'"
              class="link-btn"
              @click="fetchFiles(item.path)"
            >
              {{ item.label }}
            </button>
          </div>

          <div class="actions">
            <input v-model="folderName" class="small-input" placeholder="新建文件夹名称" />
            <button class="primary-btn" :disabled="loading" @click="createFolder">新建文件夹</button>
          </div>
        </div>

        <div class="upload-bar">
          <input id="file-input" type="file" @change="handleFileChange" />
          <button class="primary-btn" :disabled="loading" @click="submitUpload">上传文件</button>
          <button class="ghost-btn" :disabled="loading" @click="fetchFiles(currentPath)">刷新</button>
        </div>

        <div class="file-list">
          <div class="file-header">
            <span>名称</span>
            <span>大小</span>
            <span>更新时间</span>
            <span>操作</span>
          </div>

          <div v-if="fileItems.length === 0" class="empty">当前目录暂无文件</div>

          <div v-for="item in fileItems" :key="item.path" class="file-row">
            <span class="file-name" @click="openItem(item)">
              {{ item.isDirectory ? '📁' : '📄' }} {{ item.name }}
            </span>
            <span>{{ item.isDirectory ? '-' : `${item.size} B` }}</span>
            <span>{{ new Date(item.updatedAt).toLocaleString() }}</span>
            <span class="row-actions">
              <button class="ghost-btn" @click="openItem(item)">
                {{ item.isDirectory ? '进入' : '下载' }}
              </button>
              <button class="danger-btn" @click="removeItem(item)">删除</button>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
