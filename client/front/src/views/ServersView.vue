<script setup>
import { inject, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const setUserLoggedIn = inject('setUserLoggedIn', () => {})
const currentServerId = inject('currentServerId', ref(''))
const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'
const api = axios.create({ baseURL: apiBaseURL })

const servers = ref([])
const loading = ref(false)
const addForm = ref({ name: '', address: '' })
const testingId = ref('')

async function fetchServers() {
  loading.value = true
  try {
    const { data } = await api.get('/servers')
    servers.value = data.items || []
  } catch {
    servers.value = []
  } finally {
    loading.value = false
  }
}

async function addServer() {
  if (!addForm.value.name || !addForm.value.address) return
  loading.value = true
  try {
    await api.post('/servers', addForm.value)
    addForm.value = { name: '', address: '' }
    await fetchServers()
  } catch (err) {
    alert(err?.response?.data?.message || '添加失败')
  } finally {
    loading.value = false
  }
}

async function deleteServer(id) {
  if (!confirm('确定要删除这个服务器吗？')) return
  loading.value = true
  try {
    await api.delete(`/servers/${id}`)
    await fetchServers()
  } catch (err) {
    alert(err?.response?.data?.message || '删除失败')
  } finally {
    loading.value = false
  }
}

async function testConnection(id) {
  testingId.value = id
  try {
    const { data } = await api.post(`/servers/${id}/test`)
    alert(data.message || '连接成功')
    await fetchServers()
  } catch (err) {
    alert(err?.response?.data?.message || '连接失败')
    await fetchServers()
  } finally {
    testingId.value = ''
  }
}

function statusClass(status) {
  return status || 'unknown'
}

function tokenKey(serverId) {
  return serverId ? `zcopy_token_${serverId}` : 'zcopy_token'
}

function hasSavedToken(serverId) {
  return !!localStorage.getItem(tokenKey(serverId))
}

function openServer(server) {
  currentServerId.value = server.id
  localStorage.setItem('zcopy_current_server', server.id)
  if (hasSavedToken(server.id)) {
    setUserLoggedIn(true)
    router.push('/tasks')
    return
  }
  setUserLoggedIn(false)
  router.push({ path: '/connect', query: { serverId: server.id } })
}

onMounted(fetchServers)
</script>

<template>
  <div class="servers-page">
    <h2>服务器管理</h2>

    <!-- Add Server Form -->
    <el-card class="section-card">
      <template #header><span>添加服务器</span></template>
      <div class="add-form">
        <el-input v-model="addForm.name" placeholder="名称" style="width: 160px" />
        <el-input v-model="addForm.address" placeholder="地址 (host:port)" style="width: 220px" />
        <el-button type="primary" :loading="loading" @click="addServer">添加</el-button>
      </div>
    </el-card>

    <!-- Server List -->
    <el-card class="section-card">
      <template #header><span>已配置服务器</span></template>

      <div v-if="servers.length === 0" class="empty-state">
        尚未添加任何服务器
      </div>

      <table v-else class="server-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>地址</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in servers" :key="s.id">
            <td>{{ s.name }}</td>
            <td><code>{{ s.address }}</code></td>
            <td>
              <span class="status-dot" :class="statusClass(s.status)" />
              {{ s.status === 'online' ? '在线' : s.status === 'offline' ? '离线' : '未知' }}
            </td>
            <td class="actions">
              <el-button size="small" type="primary" @click="openServer(s)">
                {{ hasSavedToken(s.id) ? '进入面板' : '登录' }}
              </el-button>
              <el-button size="small" :loading="testingId === s.id" @click="testConnection(s.id)">
                测试连接
              </el-button>
              <el-button size="small" type="danger" @click="deleteServer(s.id)">删除</el-button>
            </td>
          </tr>
        </tbody>
      </table>
    </el-card>
  </div>
</template>

<style scoped>
.servers-page {
  max-width: 800px;
}

.servers-page h2 {
  margin: 0 0 20px;
}

.section-card {
  margin-bottom: 16px;
  border-radius: 12px;
}

.add-form {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.empty-state {
  text-align: center;
  padding: 32px 0;
  color: var(--z-text-muted);
}

.server-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}

.server-table th {
  text-align: left;
  padding: 8px 12px;
  border-bottom: 2px solid var(--z-border);
  color: var(--z-text-secondary);
  font-weight: 600;
}

.server-table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--z-border);
}

.server-table code {
  background: var(--z-bg-sunken);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.85rem;
}

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}

.status-dot.online { background: #67c23a; }
.status-dot.offline { background: #f56c6c; }
.status-dot.unknown { background: #909399; }

.actions {
  display: flex;
  gap: 6px;
}

@media (max-width: 640px) {
  .add-form {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
