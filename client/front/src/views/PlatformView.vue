<script setup>
import axios from 'axios'
import { computed, onMounted, ref } from 'vue'
import { CheckCircle2, Cloud, Cpu, RefreshCw } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'

const apiBaseURL =
  import.meta.env.VITE_CLIENT_BACKEND ||
  import.meta.env.VITE_API_BASE_URL ||
  'http://localhost:8090/api/v1'

const token = ref(localStorage.getItem('zcopy_token') || '')
const loading = ref(false)
const capabilities = ref(null)
const errorMsg = ref('')

const api = axios.create({ baseURL: apiBaseURL })

api.interceptors.request.use((config) => {
  if (token.value) {
    config.headers.Authorization = `Bearer ${token.value}`
  }
  return config
})

const modeText = computed(() => {
  const mode = capabilities.value?.onDemandMode
  if (mode === 'file_provider') return 'macOS File Provider'
  if (mode === 'incremental-upload') return '增量上传'
  return mode || '-'
})

async function fetchCapabilities() {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get('/system/capabilities')
    capabilities.value = data
  } catch (error) {
    errorMsg.value = error?.response?.data?.message || '读取平台能力失败'
    ElMessage.error(errorMsg.value)
  } finally {
    loading.value = false
  }
}

onMounted(fetchCapabilities)
</script>

<template>
  <section class="platform-page">
    <header class="platform-header">
      <div>
        <h1>平台能力</h1>
        <p>查看当前客户端可用的系统集成能力。</p>
      </div>
      <el-button :icon="RefreshCw" :loading="loading" @click="fetchCapabilities">刷新</el-button>
    </header>

    <el-alert v-if="errorMsg" type="error" :closable="false" show-icon>
      {{ errorMsg }}
    </el-alert>

    <div class="capability-grid" v-if="capabilities">
      <article class="capability-card">
        <div class="capability-icon">
          <component :is="Cpu" />
        </div>
        <div>
          <span>当前系统</span>
          <strong>{{ capabilities.os }}</strong>
        </div>
      </article>

      <article class="capability-card" :class="{ supported: capabilities.onDemandSupport }">
        <div class="capability-icon">
          <component :is="Cloud" />
        </div>
        <div>
          <span>按需同步</span>
          <strong>{{ capabilities.onDemandSupport ? '支持' : '不支持' }}</strong>
        </div>
      </article>

      <article class="capability-card">
        <div class="capability-icon">
          <component :is="CheckCircle2" />
        </div>
        <div>
          <span>当前模式</span>
          <strong>{{ modeText }}</strong>
        </div>
      </article>
    </div>

    <section class="platform-note">
      <h2>说明</h2>
      <p>
        启用按需同步后，客户端会为任务初始化系统级同步入口。支持按需同步的任务会在任务列表中显示醒目标识，点击本地路径时会优先打开对应的云文件夹。
      </p>
    </section>
  </section>
</template>

<style scoped>
.platform-page {
  display: grid;
  gap: 16px;
}

.platform-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.platform-header h1 {
  margin: 0;
  font-size: 1.35rem;
}

.platform-header p {
  margin: 6px 0 0;
  color: var(--z-text-muted);
  font-size: 0.9rem;
}

.capability-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.capability-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px;
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
}

.capability-card.supported {
  border-color: rgba(52, 199, 89, 0.35);
  background: linear-gradient(0deg, var(--z-success-bg), var(--z-bg-elevated));
}

.capability-icon {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  color: var(--z-accent);
  background: var(--z-accent-bg);
  border-radius: 8px;
}

.capability-icon :deep(svg) {
  width: 20px;
  height: 20px;
}

.capability-card span {
  display: block;
  color: var(--z-text-muted);
  font-size: 0.82rem;
  margin-bottom: 5px;
}

.capability-card strong {
  font-size: 1rem;
}

.platform-note {
  padding: 18px;
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
}

.platform-note h2 {
  margin: 0 0 8px;
  font-size: 1rem;
}

.platform-note p {
  margin: 0;
  color: var(--z-text-secondary);
  line-height: 1.7;
}

@media (max-width: 760px) {
  .capability-grid {
    grid-template-columns: 1fr;
  }
}
</style>
