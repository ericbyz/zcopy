<script setup>
import { computed, ref } from 'vue'
import { Folder, FileText, RefreshCw } from 'lucide-vue-next'
import { buildBreadcrumbList } from '../utils/breadcrumb.js'

const props = defineProps({
  currentPath: { type: String, default: '' },
  fileItems: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false }
})

const emit = defineEmits(['navigate', 'create-folder', 'upload', 'delete', 'refresh'])

const folderName = ref('')
const uploadFile = ref(null)

const breadcrumbList = computed(() => buildBreadcrumbList(props.currentPath))

function handleFileChange(file) {
  uploadFile.value = file.raw || null
}

function handleCreateFolder() {
  emit('create-folder', folderName.value)
  folderName.value = ''
}

function handleSubmitUpload() {
  emit('upload', uploadFile.value)
  uploadFile.value = null
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
  <div>
    <div class="toolbar">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item v-for="item in breadcrumbList" :key="item.path || 'root'" @click="emit('navigate', { path: item.path, isDirectory: true })">
          {{ item.label }}
        </el-breadcrumb-item>
      </el-breadcrumb>
      <div class="actions">
        <el-input v-model="folderName" placeholder="新建文件夹名称" style="width: 200px" />
        <el-button type="primary" :loading="loading" @click="handleCreateFolder">新建文件夹</el-button>
      </div>
    </div>
    <div class="upload-bar">
      <el-upload :auto-upload="false" :on-change="handleFileChange" :show-file-list="false">
        <el-button>选择文件</el-button>
      </el-upload>
      <el-button type="primary" :loading="loading" @click="handleSubmitUpload">上传文件</el-button>
      <el-button :icon="RefreshCw" :loading="loading" @click="handleRefresh">刷新</el-button>
    </div>
    <el-table :data="fileItems" v-loading="loading" stripe style="width: 100%">
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
  </div>
</template>

<style scoped>
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

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.upload-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
