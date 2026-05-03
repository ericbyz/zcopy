<script setup>
import { computed, ref } from 'vue'
import { Folder, FileText, FolderPlus, RefreshCw, UploadCloud } from 'lucide-vue-next'
import { buildBreadcrumbList } from '../utils/breadcrumb.js'

const props = defineProps({
  currentPath: { type: String, default: '' },
  fileItems: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false }
})

const emit = defineEmits(['navigate', 'create-folder', 'upload', 'delete', 'refresh'])

const folderName = ref('')
const uploadRef = ref(null)

const breadcrumbList = computed(() => buildBreadcrumbList(props.currentPath))

function handleFileChange(file) {
  if (file?.raw) {
    emit('upload', file.raw)
    uploadRef.value?.clearFiles()
  }
}

function handleOpenItem(item) {
  emit('navigate', item)
}

function handleRemoveItem(item) {
  emit('delete', item)
}

function handleRefresh() {
  emit('refresh')
}
</script>

<template>
  <section class="file-browser">
    <header class="file-browser-toolbar">
      <div class="path-panel">
        <span class="path-label">当前位置</span>
        <el-breadcrumb separator="/">
          <el-breadcrumb-item v-for="item in breadcrumbList" :key="item.path || 'root'" @click="emit('navigate', { path: item.path, isDirectory: true })">
            {{ item.label }}
          </el-breadcrumb-item>
        </el-breadcrumb>
      </div>

      <div class="primary-actions">
        <el-upload
          ref="uploadRef"
          :auto-upload="false"
          :on-change="handleFileChange"
          :show-file-list="false"
        >
          <el-button type="primary" :icon="UploadCloud" :loading="loading">上传文件</el-button>
        </el-upload>
        <el-button :icon="RefreshCw" :loading="loading" @click="handleRefresh">刷新</el-button>
      </div>
    </header>

    <section class="folder-tools">
      <el-input v-model="folderName" placeholder="新建文件夹名称" @keyup.enter="emit('create-folder', folderName); folderName = ''" />
      <el-button :icon="FolderPlus" :disabled="!folderName.trim()" @click="emit('create-folder', folderName); folderName = ''">
        新建文件夹
      </el-button>
    </section>

    <el-table :data="fileItems" v-loading="loading" stripe class="file-table">
      <el-table-column label="名称" min-width="220">
        <template #default="{ row }">
          <span class="file-name" @click="handleOpenItem(row)">
            <component :is="row.isDirectory ? Folder : FileText" class="file-icon" />
            {{ row.name }}
            <el-tag v-if="row.isDirectory && row.occupied" size="small" type="warning">
              任务：{{ row.taskName }}
            </el-tag>
          </span>
        </template>
      </el-table-column>
      <el-table-column label="任务占用" width="180">
        <template #default="{ row }">
          <span v-if="row.isDirectory && row.occupied">{{ row.taskName }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="大小" width="120">
        <template #default="{ row }">{{ row.isDirectory ? '-' : row.size + ' B' }}</template>
      </el-table-column>
      <el-table-column label="更新时间" width="200">
        <template #default="{ row }">{{ new Date(row.updatedAt).toLocaleString() }}</template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="handleOpenItem(row)">{{ row.isDirectory ? '进入' : '下载' }}</el-button>
          <el-button size="small" type="danger" @click="handleRemoveItem(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="fileItems.length === 0" description="当前目录暂无文件" />
  </section>
</template>

<style scoped>
.file-browser {
  display: grid;
  gap: 14px;
}

.file-browser-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 14px;
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
}

.path-panel {
  min-width: 0;
  display: grid;
  gap: 7px;
}

.path-label {
  color: var(--z-text-muted);
  font-size: 0.78rem;
  font-weight: 700;
}

.primary-actions {
  flex: 0 0 auto;
  display: flex;
  gap: 8px;
  align-items: center;
}

.folder-tools {
  display: grid;
  grid-template-columns: minmax(180px, 260px) auto;
  gap: 8px;
  justify-content: end;
  align-items: center;
}

.file-table {
  width: 100%;
  border: 1px solid var(--z-border);
  border-radius: 8px;
  overflow: hidden;
}

.file-name {
  cursor: pointer;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.file-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

@media (max-width: 640px) {
  .file-browser-toolbar,
  .primary-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .folder-tools {
    grid-template-columns: 1fr;
  }
}
</style>
