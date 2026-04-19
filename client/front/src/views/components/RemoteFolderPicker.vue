<script setup>
import { computed } from 'vue'
import { Folder } from 'lucide-vue-next'

const props = defineProps({
  open: {
    type: Boolean,
    default: false
  },
  path: {
    type: String,
    default: ''
  },
  folders: {
    type: Array,
    default: () => []
  },
  breadcrumbs: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  },
  error: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['navigate', 'navigate-parent', 'choose', 'close'])
</script>

<template>
  <el-dialog :model-value="open" @update:model-value="(v) => emit('close')" title="选择远程目录" width="720px" append-to-body>
    <div class="picker-toolbar">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item
          v-for="item in breadcrumbs"
          :key="item.path || 'root'"
          @click="emit('navigate', item.path)"
          class="breadcrumb-item"
        >
          {{ item.label }}
        </el-breadcrumb-item>
      </el-breadcrumb>
      <div class="picker-actions">
        <el-button :disabled="loading || !path" @click="emit('navigate-parent')">
          返回上级
        </el-button>
      </div>
    </div>

    <el-alert v-if="error" :title="error" type="error" :closable="false" style="margin: 16px 0" />

    <div v-loading="loading" class="folder-list">
      <el-empty v-if="folders.length === 0 && !loading" description="当前目录暂无子目录" />
      
      <div v-else class="folder-items">
        <div
          v-for="folder in folders"
          :key="folder.path"
          class="folder-item"
          @click="emit('navigate', folder.path)"
        >
          <div class="folder-info">
            <Folder class="folder-icon" />
            <span class="folder-name">{{ folder.name }}</span>
          </div>
          <span class="folder-path">{{ folder.path || '根目录' }}</span>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('close')">关闭</el-button>
      <el-button type="primary" :disabled="loading" @click="emit('choose')">
        选择当前目录
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.picker-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.breadcrumb-item {
  cursor: pointer;
  transition: color 0.2s;
}

.breadcrumb-item:hover {
  color: var(--z-accent);
}

.picker-actions {
  display: flex;
  gap: 8px;
}

.folder-list {
  min-height: 200px;
}

.folder-items {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.folder-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-radius: 8px;
  border: 1px solid var(--z-border);
  cursor: pointer;
  transition: all 0.2s;
}

.folder-item:hover {
  background: var(--z-bg-sunken);
  border-color: var(--z-accent);
}

.folder-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.folder-icon {
  width: 20px;
  height: 20px;
  color: var(--z-accent);
  flex-shrink: 0;
}

.folder-name {
  font-weight: 500;
}

.folder-path {
  color: var(--z-text-muted);
  font-size: 0.85rem;
  font-family: monospace;
}
</style>
