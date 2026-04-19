<script setup>
import { computed } from 'vue'
import { Play, Pause, MoreVertical, Folder, Cloud, ArrowUpRight, Edit, Trash2, RefreshCw } from 'lucide-vue-next'
import { formatBytes, formatSpeed } from '../../utils/format.js'
import { getTaskStatusText, calcSyncProgress } from '../../utils/task.js'

const props = defineProps({
  task: {
    type: Object,
    required: true
  },
  loading: {
    type: Boolean,
    default: false
  },
  onDemandStatus: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['edit', 'sync', 'toggle-auto', 'delete', 'open-location'])

const taskStatusText = computed(() => getTaskStatusText(props.task))

const syncProgress = computed(() => calcSyncProgress(props.task))

const isSyncing = computed(() => props.task?.syncReport?.state === 'syncing')

const statusType = computed(() => {
  const state = props.task?.syncReport?.state || props.task?.status || 'idle'
  if (state === 'syncing') return 'success'
  if (state === 'failed' || state === 'error') return 'danger'
  return 'info'
})

const progressLabel = computed(() => {
  if (!isSyncing.value) return ''
  const r = props.task.syncReport
  const totalItems = r?.totalFiles || 0
  const totalSize = formatBytes(r?.totalBytes || 0)
  const doneItems = r?.uploadedFiles || 0
  const doneSize = formatBytes(r?.transferredBytes || 0)
  return `共 ${totalItems} 项 (${totalSize}) | 传输完成 ${doneItems} 项 (${doneSize})`
})
</script>

<template>
  <div class="task-card">
    <!-- Header: name + status + actions -->
    <div class="task-header">
      <div class="task-title">
        <h3>{{ task.name }}</h3>
        <el-tag :type="statusType" size="small" round>
          {{ isSyncing ? `正在同步 ${syncProgress}%` : taskStatusText }}
        </el-tag>
      </div>
      <div class="task-header-actions">
        <el-button
          v-if="!isSyncing"
          circle
          size="small"
          :disabled="loading"
          @click="emit('sync', task.id)"
          aria-label="开始同步"
        >
          <component :is="Play" style="width:16px;height:16px" />
        </el-button>
        <el-dropdown trigger="click" @command="(cmd) => {
          if (cmd === 'edit') emit('edit', task)
          else if (cmd === 'toggle') emit('toggle-auto', task)
          else if (cmd === 'delete') emit('delete', task.id)
        }">
          <el-button circle size="small" aria-label="更多操作">
            <component :is="MoreVertical" style="width:16px;height:16px" />
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="edit">
                <component :is="Edit" style="width:14px;height:14px;margin-right:6px" />
                编辑
              </el-dropdown-item>
              <el-dropdown-item command="toggle">
                <component :is="RefreshCw" style="width:14px;height:14px;margin-right:6px" />
                {{ task.autoBackup ? '停止自动' : '开启自动' }}
              </el-dropdown-item>
              <el-dropdown-item command="delete" divided>
                <component :is="Trash2" style="width:14px;height:14px;margin-right:6px" />
                删除
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- Progress section (only when syncing) -->
    <div v-if="isSyncing" class="task-progress">
      <div class="progress-stats">{{ progressLabel }}</div>
      <el-progress :percentage="syncProgress" :stroke-width="12" :show-text="true" />
    </div>

    <!-- Paths -->
    <div class="task-paths">
      <div class="path-row">
        <component :is="Folder" style="width:14px;height:14px;flex-shrink:0" />
        <span class="path-label">本地路径</span>
        <span class="path-value">{{ task.localPath }}</span>
      </div>
      <div class="path-row">
        <component :is="Cloud" style="width:14px;height:14px;flex-shrink:0" />
        <span class="path-label">远程路径</span>
        <span class="path-value">{{ task.remotePath || '根目录' }}</span>
      </div>
    </div>

    <!-- Meta -->
    <div class="task-meta">
      <span>自动：{{ task.autoBackup ? '开启' : '关闭' }}</span>
      <span>上次同步：{{ task.lastSyncAt ? new Date(task.lastSyncAt).toLocaleString() : '无' }}</span>
    </div>

    <!-- Error Alert -->
    <el-alert
      v-if="task.syncReport?.state === 'failed' && task.syncReport.failedFilePaths?.length"
      title="失败文件"
      type="error"
      :closable="false"
      style="margin-top: 12px"
    >
      <div class="failed-files">
        <div v-for="item in task.syncReport.failedFilePaths.slice(0, 8)" :key="item" class="failed-item">
          {{ item }}
        </div>
      </div>
    </el-alert>

    <el-alert
      v-if="task.lastError"
      :title="task.lastError"
      type="error"
      :closable="false"
      style="margin-top: 12px"
    />

    <!-- On-Demand Panel -->
    <div v-if="task.onDemandSync" class="ondemand-panel">
      <el-descriptions :column="2" border size="small" style="margin-top: 16px">
        <el-descriptions-item label="按需同步">
          {{ onDemandStatus?.supported ? '支持' : '不支持' }}
        </el-descriptions-item>
        <el-descriptions-item label="同步根">
          {{ onDemandStatus?.registered ? '已注册' : '未注册' }}
        </el-descriptions-item>
        <el-descriptions-item v-if="onDemandStatus?.mountPath" label="Location" :span="2">
          {{ onDemandStatus.mountPath }}
        </el-descriptions-item>
        <el-descriptions-item v-else-if="onDemandStatus?.reason" label="原因" :span="2">
          {{ onDemandStatus.reason }}
        </el-descriptions-item>
      </el-descriptions>
      <el-button
        v-if="onDemandStatus?.mountPath"
        size="small"
        style="margin-top: 12px"
        :disabled="loading"
        @click="emit('open-location', onDemandStatus.mountPath)"
      >
        打开 location
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.task-card {
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 12px;
  padding: 20px;
  transition: box-shadow 0.15s;
}

.task-card:hover {
  box-shadow: var(--z-shadow-lg);
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.task-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.task-title h3 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
}

.task-header-actions {
  display: flex;
  gap: 6px;
}

.task-progress {
  margin-bottom: 12px;
}

.progress-stats {
  font-size: 0.9rem;
  color: var(--z-text-secondary);
  margin-bottom: 8px;
}

.task-paths {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 12px;
  padding: 12px;
  background: var(--z-bg-sunken);
  border-radius: 8px;
}

.path-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--z-text-secondary);
  font-size: 0.9rem;
}

.path-label {
  color: var(--z-text-muted);
  min-width: 56px;
}

.path-value {
  font-family: monospace;
  color: var(--z-text-primary);
  word-break: break-all;
}

.task-meta {
  display: flex;
  gap: 24px;
  color: var(--z-text-muted);
  font-size: 0.85rem;
}

.failed-files {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 160px;
  overflow-y: auto;
}

.failed-item {
  font-family: monospace;
  font-size: 0.85rem;
}

.ondemand-panel {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--z-border);
}

@media (max-width: 640px) {
  .task-title {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .task-meta {
    flex-direction: column;
    gap: 4px;
  }

  .task-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
}
</style>
