<script setup>
import { computed } from 'vue'
import { Play, MoreVertical, FolderCheck, Edit, Trash2, RefreshCw, Cloud } from 'lucide-vue-next'
import { formatBytes } from '../../utils/format.js'
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

const emit = defineEmits(['edit', 'sync', 'toggle-auto', 'delete', 'open-location', 'open-local', 'open-remote'])

const taskStatusText = computed(() => getTaskStatusText(props.task))

const syncProgress = computed(() => calcSyncProgress(props.task))

const isSyncing = computed(() => props.task?.syncReport?.state === 'syncing')

const statusType = computed(() => {
  const state = props.task?.syncReport?.state || props.task?.status || 'idle'
  if (state === 'syncing') return 'success'
  if (state === 'failed' || state === 'error') return 'danger'
  return 'info'
})

const taskKindLabel = computed(() => props.task.taskMode === 'sync' ? '同步任务' : '备份任务')

const actionLabel = computed(() => props.task.taskMode === 'sync' ? '开始同步' : '开始备份')

const remoteLabel = computed(() => props.task.taskMode === 'sync' ? '文件服务器路径' : '备份目标路径')

const ruleLabel = computed(() => {
  if (props.task.taskMode === 'sync') {
    const mode = props.task.conflictMode
    if (mode === 'local') return '本地优先'
    if (mode === 'remote') return '文件服务器优先'
    return '最新优先'
  }
  return props.task.autoBackup ? '自动备份' : '手动备份'
})

const lastSyncText = computed(() => {
  if (!props.task.lastSyncAt) return '无'
  return new Date(props.task.lastSyncAt).toLocaleString()
})

const progressStats = computed(() => {
  const r = props.task.syncReport
  const totalItems = r?.totalFiles || 0
  const totalSize = formatBytes(r?.totalBytes || 0)
  const doneItems = r?.uploadedFiles || 0
  const doneSize = formatBytes(r?.transferredBytes || 0)
  return `共 ${totalItems} 项 (${totalSize}) | 传输完成 ${doneItems} 项 (${doneSize})`
})

const idleStats = computed(() => {
  const r = props.task.syncReport
  if (!r?.totalFiles && !r?.transferredBytes) return `上次${props.task.taskMode === 'sync' ? '同步' : '备份'} ${lastSyncText.value}`
  return `共 ${r.totalFiles || 0} 项 | 已传输 ${formatBytes(r.transferredBytes || 0)}`
})
</script>

<template>
  <div class="task-card">
    <div class="task-header">
      <div class="task-main">
        <div class="task-icon" :class="{ syncing: isSyncing }">
          <component :is="FolderCheck" />
        </div>
        <div class="task-summary">
          <div class="task-title">
            <h3>{{ task.name || taskKindLabel }}</h3>
            <span v-if="task.onDemandSync" class="ondemand-badge">
              <component :is="Cloud" />
              按需同步
            </span>
          </div>
          <div class="task-status-line">
            <span class="status-text" :class="statusType">
              {{ isSyncing ? `${task.taskMode === 'sync' ? '正在同步' : '正在备份'} ${syncProgress}%` : taskStatusText }}
            </span>
            <span class="divider"></span>
            <span>{{ isSyncing ? progressStats : idleStats }}</span>
          </div>
        </div>
      </div>

      <div class="task-header-actions">
        <el-button
          v-if="!isSyncing && !task.cloudOnly"
          text
          size="small"
          :disabled="loading"
          :aria-label="actionLabel"
          @click="emit('sync', task.id)"
        >
          <component :is="Play" style="width:18px;height:18px" />
        </el-button>
        <el-dropdown trigger="click" @command="(cmd) => {
          if (cmd === 'edit') emit('edit', task)
          else if (cmd === 'toggle') emit('toggle-auto', task)
          else if (cmd === 'delete') emit('delete', task.id)
        }">
          <el-button text size="small" aria-label="更多操作">
            <component :is="MoreVertical" style="width:18px;height:18px" />
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="edit">
                <component :is="Edit" style="width:14px;height:14px;margin-right:6px" />
                编辑
              </el-dropdown-item>
              <el-dropdown-item v-if="!task.cloudOnly" command="toggle">
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

    <div v-if="isSyncing" class="task-progress" :aria-label="`任务进度 ${syncProgress}%`">
      <span :style="{ width: `${syncProgress}%` }"></span>
    </div>

    <div class="task-columns">
      <div class="task-column">
        <span class="column-label">本地路径</span>
        <button class="column-value path-button" type="button" @click="emit('open-local', task)">
          {{ task.onDemandSync && onDemandStatus?.mountPath ? onDemandStatus.mountPath : (task.localPath || '云端按需管理') }}
        </button>
      </div>
      <div class="task-column">
        <span class="column-label">{{ remoteLabel }}</span>
        <button class="column-value path-button" type="button" @click="emit('open-remote', task)">
          {{ task.remotePath || '根目录' }}
        </button>
      </div>
      <div class="task-column rule-column">
        <span class="column-label">{{ task.taskMode === 'sync' ? '同步规则' : '备份规则' }}</span>
        <span class="column-value">{{ ruleLabel }}</span>
      </div>
    </div>

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

  </div>
</template>

<style scoped>
.task-card {
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
  overflow: hidden;
  transition: box-shadow 0.15s;
}

.task-card:hover {
  box-shadow: var(--z-shadow);
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 14px;
  padding: 18px 20px 16px;
}

.task-main {
  min-width: 0;
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.task-icon {
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  display: grid;
  place-items: center;
  color: #5b8def;
  background: rgba(91, 141, 239, 0.16);
  border-radius: 7px;
}

.task-icon :deep(svg) {
  width: 25px;
  height: 25px;
}

.task-icon.syncing {
  color: var(--z-success);
  background: var(--z-success-bg);
}

.task-summary {
  min-width: 0;
  display: grid;
  gap: 7px;
}

.task-title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.task-title h3 {
  margin: 0;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1rem;
  font-weight: 600;
}

.ondemand-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: 0 0 auto;
  padding: 3px 7px;
  border-radius: 999px;
  color: var(--z-success);
  background: var(--z-success-bg);
  font-size: 0.76rem;
  font-weight: 700;
}

.ondemand-badge :deep(svg) {
  width: 12px;
  height: 12px;
}

.task-status-line {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--z-text-secondary);
  font-size: 0.86rem;
  line-height: 1.25;
  white-space: nowrap;
}

.task-status-line span:last-child {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-text {
  flex: 0 0 auto;
  font-weight: 600;
}

.status-text.success {
  color: var(--z-accent);
}

.status-text.danger {
  color: var(--z-danger);
}

.status-text.info {
  color: var(--z-text-secondary);
}

.divider {
  width: 1px;
  height: 14px;
  background: var(--z-border);
}

.task-header-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: 0 0 auto;
}

.task-header-actions :deep(.el-button) {
  color: var(--z-text-primary);
  padding: 4px;
}

.task-progress {
  height: 2px;
  background: var(--z-bg-sunken);
}

.task-progress span {
  display: block;
  height: 100%;
  background: var(--z-accent);
  transition: width 0.2s ease;
}

.task-columns {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.2fr) minmax(110px, 0.7fr);
  border-top: 1px solid var(--z-border);
  background: var(--z-bg-elevated);
}

.task-column {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 14px 20px 15px;
  border-right: 1px solid var(--z-border);
}

.task-column:last-child {
  border-right: 0;
}

.column-label {
  color: var(--z-text-muted);
  font-size: 0.8rem;
  font-weight: 600;
}

.column-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--z-text-primary);
  font-size: 0.85rem;
}

.path-button {
  display: block;
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.path-button:hover {
  color: var(--z-accent);
  text-decoration: underline;
  text-underline-offset: 3px;
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

@media (max-width: 640px) {
  .task-header {
    padding: 14px;
  }

  .task-icon {
    width: 30px;
    height: 30px;
    flex-basis: 30px;
  }

  .task-status-line {
    flex-wrap: wrap;
    gap: 4px 8px;
    white-space: normal;
  }

  .divider {
    display: none;
  }

  .task-columns {
    grid-template-columns: 1fr;
  }

  .task-column {
    border-right: 0;
    border-bottom: 1px solid var(--z-border);
    padding: 12px 14px;
  }

  .task-column:last-child {
    border-bottom: 0;
  }
}
</style>
