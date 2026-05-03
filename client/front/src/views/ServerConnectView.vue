<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Server, Wifi, WifiOff, Plus, ArrowRight, ArrowLeft } from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()

const setUserLoggedIn = inject('setUserLoggedIn')
const currentServerId = inject('currentServerId', ref(''))
const serverList = inject('serverList', ref([]))

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'
const api = axios.create({ baseURL: apiBaseURL })

// State
const loading = ref(false)
const addForm = ref({ name: '', address: '', webUrl: '' })
const showAddForm = ref(false)

// Login state
const loginServerId = ref('')
const loginForm = ref({ account: '', password: '' })
const rememberLogin = ref(true)
const loginLoading = ref(false)

// View: 'select' | 'login'
const view = ref('select')

const loginServer = computed(() =>
  serverList.value.find(s => s.id === loginServerId.value)
)

function getTokenKey(serverId) {
  return serverId ? `zcopy_token_${serverId}` : 'zcopy_token'
}

function getCredentialKey(serverId) {
  return serverId ? `zcopy_credentials_${serverId}` : 'zcopy_credentials'
}

function loadSavedCredentials(serverId) {
  try {
    const raw = localStorage.getItem(getCredentialKey(serverId))
    if (!raw) return { account: '', password: '' }
    const parsed = JSON.parse(raw)
    return {
      account: typeof parsed.account === 'string' ? parsed.account : '',
      password: typeof parsed.password === 'string' ? parsed.password : ''
    }
  } catch {
    return { account: '', password: '' }
  }
}

function saveCredentials(serverId) {
  if (!rememberLogin.value) {
    localStorage.removeItem(getCredentialKey(serverId))
    return
  }
  localStorage.setItem(getCredentialKey(serverId), JSON.stringify({
    account: loginForm.value.account,
    password: loginForm.value.password
  }))
}

function selectServerAndLogin(serverId) {
  loginServerId.value = serverId
  currentServerId.value = serverId
  localStorage.setItem('zcopy_current_server', serverId)
  loginForm.value = loadSavedCredentials(serverId)
  view.value = 'login'
}

function backToSelect() {
  view.value = 'select'
  loginServerId.value = ''
  loginForm.value = { account: '', password: '' }
}

function switchServer() {
  backToSelect()
}

async function submitLogin() {
  if (!loginForm.value.account || !loginForm.value.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loginLoading.value = true
  try {
    const { data } = await api.post('/auth/login', {
      serverId: loginServerId.value,
      account: loginForm.value.account,
      password: loginForm.value.password
    })
    // Save token per server
    const key = getTokenKey(loginServerId.value)
    localStorage.setItem(key, data.token)
    saveCredentials(loginServerId.value)
    setUserLoggedIn(true)
    ElMessage.success(data.message || '登录成功')
    router.push('/tasks')
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '登录失败')
  } finally {
    loginLoading.value = false
  }
}

async function addServer() {
  if (!addForm.value.name || !addForm.value.address) {
    ElMessage.warning('请输入服务器名称和地址')
    return
  }
  loading.value = true
  try {
    await api.post('/servers', addForm.value)
    addForm.value = { name: '', address: '', webUrl: '' }
    showAddForm.value = false
    await fetchServers()
    ElMessage.success('服务器已添加')
  } catch (err) {
    ElMessage.error(err?.response?.data?.message || '添加失败')
  } finally {
    loading.value = false
  }
}

async function fetchServers() {
  try {
    const { data } = await api.get('/servers')
    serverList.value = data.items || []
  } catch {
    serverList.value = []
  }
}

function statusText(status) {
  return status === 'online' ? '在线' : status === 'offline' ? '离线' : '未知'
}

onMounted(async () => {
  await fetchServers()
  const queryServerId = typeof route.query.serverId === 'string' ? route.query.serverId : ''
  if (queryServerId && serverList.value.some(server => server.id === queryServerId)) {
    selectServerAndLogin(queryServerId)
  }
})
</script>

<template>
  <div class="server-connect">
    <!-- SELECT SERVER VIEW -->
    <template v-if="view === 'select'">
      <div class="connect-hero">
        <h1 class="connect-title">ZCopy Desktop</h1>
        <p class="connect-desc">云文件同步工具 — 选择服务器并登录，配置备份任务，支持自动备份与按需同步。</p>
      </div>

      <!-- Server List -->
      <div class="connect-section">
        <div class="section-header-row">
          <h2>选择服务器</h2>
          <div class="section-actions">
            <el-button size="small" :icon="Plus" @click="showAddForm = !showAddForm">
              {{ showAddForm ? '取消' : '手动添加' }}
            </el-button>
          </div>
        </div>

        <!-- Add form -->
        <el-card v-if="showAddForm" class="add-server-card">
          <div class="add-form-row">
            <el-input v-model="addForm.name" placeholder="服务器名称" style="width: 140px" />
            <el-input v-model="addForm.address" placeholder="地址 (host:port)" style="flex:1" />
            <el-input v-model="addForm.webUrl" placeholder="前端地址（可选）" style="width: 180px" />
            <el-button type="primary" :loading="loading" @click="addServer">添加</el-button>
          </div>
        </el-card>

        <!-- Configured servers -->
        <div v-if="serverList.length === 0" class="empty-servers">
          <Server class="empty-icon" />
          <p>尚未添加服务器</p>
          <p class="empty-hint">点击「手动添加」来添加服务器地址</p>
        </div>

        <div v-else class="server-list">
          <div
            v-for="s in serverList"
            :key="s.id"
            class="server-card"
            :class="{ offline: s.status === 'offline' }"
            @click="s.status !== 'offline' && selectServerAndLogin(s.id)"
          >
            <div class="server-card-info">
              <div class="server-card-name">
                <span class="server-status-dot" :class="s.status || 'unknown'" />
                {{ s.name }}
              </div>
              <div class="server-card-addr">{{ s.address }}</div>
              <div class="server-card-status">{{ statusText(s.status) }}</div>
            </div>
            <div class="server-card-action">
              <ArrowRight class="server-card-arrow" />
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- LOGIN VIEW -->
    <template v-if="view === 'login'">
      <div class="login-view">
        <div class="login-back" @click="backToSelect">
          <ArrowLeft style="width:16px;height:16px" />
          <span>切换服务器</span>
        </div>

        <div class="login-hero">
          <h2>{{ loginServer?.name || '服务器' }}</h2>
          <p class="login-addr">{{ loginServer?.address }}</p>
        </div>

        <el-card class="login-card">
          <div class="login-card-title">登录</div>
          <el-form @submit.prevent="submitLogin">
            <el-form-item>
              <el-input
                v-model="loginForm.account"
                placeholder="用户名或邮箱"
                @keyup.enter="submitLogin"
              />
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="密码"
                show-password
                @keyup.enter="submitLogin"
              />
            </el-form-item>
            <el-form-item>
              <el-checkbox v-model="rememberLogin">记住账号和密码</el-checkbox>
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="loginLoading"
                style="width: 100%"
                @click="submitLogin"
              >
                {{ loginLoading ? '登录中...' : '登录' }}
              </el-button>
            </el-form-item>
          </el-form>
          <div class="login-switch">
            <span @click="switchServer">切换其他服务器</span>
          </div>
        </el-card>
      </div>
    </template>
  </div>
</template>

<style scoped>
.server-connect {
  max-width: 580px;
  margin: 0 auto;
  padding: 32px 0;
}

/* Hero */
.connect-hero {
  text-align: center;
  margin-bottom: 28px;
}

.connect-title {
  margin: 0 0 10px;
  font-size: 1.8rem;
  font-weight: 700;
  background: linear-gradient(135deg, var(--z-success), var(--z-accent));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.connect-desc {
  margin: 0;
  color: var(--z-text-muted);
  font-size: 0.96rem;
  max-width: 420px;
  margin-inline: auto;
}

/* Section header */
.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.section-header-row h2 {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 600;
}

.section-actions {
  display: flex;
  gap: 6px;
}

/* Add server card */
.add-server-card {
  margin-bottom: 14px;
  border-radius: var(--z-radius-sm);
}

.add-server-card :deep(.el-card__body) {
  padding: 14px 16px;
}

.add-form-row {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

/* Empty state */
.empty-servers {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  color: var(--z-text-muted);
  text-align: center;
}

.empty-icon {
  width: 40px;
  height: 40px;
  opacity: 0.35;
  margin-bottom: 12px;
}

.empty-servers p {
  margin: 0 0 4px;
  font-size: 0.92rem;
}

.empty-hint {
  font-size: 0.82rem !important;
  opacity: 0.7;
}

/* Server list */
.server-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.server-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-radius: var(--z-radius);
  background: var(--z-bg-card);
  border: 1px solid var(--z-border);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
}

.server-card:hover {
  background: var(--z-bg-elevated);
  border-color: var(--z-border-strong);
  box-shadow: var(--z-shadow);
}

.server-card.offline {
  opacity: 0.6;
  cursor: not-allowed;
}

.server-card-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.server-card-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 0.95rem;
  color: var(--z-text-primary);
}

.server-card-addr {
  font-size: 0.82rem;
  color: var(--z-text-muted);
  font-family: monospace;
}

.server-card-status {
  font-size: 0.78rem;
  color: var(--z-text-secondary);
}

.server-status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.server-status-dot.online { background: #67c23a; }
.server-status-dot.offline { background: #f56c6c; }
.server-status-dot.unknown { background: #909399; }

.server-card-action {
  color: var(--z-text-muted);
}

.server-card-arrow {
  width: 18px;
  height: 18px;
}

/* Login view */
.login-view {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 16px;
}

.login-back {
  display: flex;
  align-items: center;
  gap: 4px;
  align-self: flex-start;
  font-size: 0.84rem;
  color: var(--z-text-secondary);
  cursor: pointer;
  margin-bottom: 20px;
  transition: color 0.15s;
}

.login-back:hover {
  color: var(--z-accent);
}

.login-hero {
  text-align: center;
  margin-bottom: 20px;
}

.login-hero h2 {
  margin: 0 0 4px;
  font-size: 1.3rem;
  font-weight: 700;
}

.login-addr {
  margin: 0;
  font-size: 0.85rem;
  color: var(--z-text-muted);
  font-family: monospace;
}

.login-card {
  width: min(420px, 100%);
  border-radius: var(--z-radius-sm);
}

.login-card :deep(.el-card__body) {
  padding: 22px 26px;
}

.login-card-title {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 18px;
  color: var(--z-text-primary);
}

.login-card :deep(.el-form-item) {
  margin-bottom: 18px;
}

.login-card :deep(.el-input__wrapper) {
  padding: 7px 12px;
  font-size: 0.92rem;
}

.login-card :deep(.el-button) {
  padding: 10px 18px;
  font-size: 0.95rem;
  height: auto;
}

.login-switch {
  text-align: center;
  margin-top: 4px;
}

.login-switch span {
  font-size: 0.82rem;
  color: var(--z-accent);
  cursor: pointer;
}

.login-switch span:hover {
  text-decoration: underline;
}

@media (max-width: 768px) {
  .server-connect {
    padding: 20px 0;
  }

  .connect-title {
    font-size: 1.5rem;
  }

  .add-form-row {
    flex-direction: column;
    align-items: stretch;
  }

  .add-form-row .el-input {
    width: 100% !important;
  }
}
</style>
