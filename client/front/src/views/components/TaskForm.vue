<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  taskForm: {
    type: Object,
    required: true
  },
  modeLock: {
    type: String,
    default: ''
  },
  loading: {
    type: Boolean,
    default: false
  },
  open: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['save', 'reset', 'pick-local', 'open-remote-picker', 'close'])

const step = ref(0)

const isEdit = computed(() => !!props.taskForm.id)

const formTitle = computed(() => isEdit.value ? '编辑任务' : '新建任务')

function handleClose() {
  step.value = 0
  emit('close')
}

function handleNext() {
  if (step.value < 1) {
    step.value++
  } else {
    emit('save')
  }
}

function handlePrev() {
  if (step.value > 0) step.value--
}

const steps = [
  { title: '基本设置' },
  { title: '任务路径' }
]

const isSyncMode = computed(() => props.taskForm.taskMode === 'sync')
const showCloudOnly = computed(() => !isSyncMode.value && props.taskForm.onDemandSync)
const autoLocalPathHint = computed(() => {
  if (!isSyncMode.value || !props.taskForm.onDemandSync) return ''
  return props.taskForm.name ? `将自动创建任务目录：${props.taskForm.name}` : '将自动创建以任务名命名的本地目录'
})
const showLocalPath = computed(() => (isSyncMode.value || !props.taskForm.cloudOnly) && !(isSyncMode.value && props.taskForm.onDemandSync))

watch(() => props.taskForm.taskMode, (value) => {
  if (value === 'sync') {
    props.taskForm.cloudOnly = false
  }
})
</script>

<template>
  <el-dialog
    :model-value="open"
    :title="formTitle"
    width="520px"
    :close-on-click-modal="false"
    append-to-body
    @close="handleClose"
  >
    <!-- Step indicator (only for new tasks) -->
    <el-steps v-if="!isEdit" :active="step" finish-status="success" simple style="margin-bottom: 24px">
      <el-step v-for="(s, i) in steps" :key="i" :title="s.title" />
    </el-steps>

    <el-form label-position="top">
      <!-- Step 1 or Edit: all fields -->
      <template v-if="step === 0 || isEdit">
        <el-form-item label="任务名称">
          <el-input
            v-model="taskForm.name"
            placeholder="请输入任务名称"
            maxlength="32"
            show-word-limit
          />
        </el-form-item>
        <el-form-item v-if="!modeLock" label="任务模式">
          <el-radio-group v-model="taskForm.taskMode">
            <el-radio label="backup">备份模式</el-radio>
            <el-radio label="sync">同步模式</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-else label="任务模式">
          <el-tag type="primary">{{ modeLock === 'sync' ? '同步模式' : '备份模式' }}</el-tag>
        </el-form-item>
        <el-form-item v-if="isSyncMode" label="冲突策略">
          <el-select v-model="taskForm.conflictMode" style="width: 100%">
            <el-option label="最新优先" value="latest" />
            <el-option label="本地优先" value="local" />
            <el-option label="文件服务器优先" value="remote" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="taskForm.autoBackup">
            {{ isSyncMode ? '自动同步（文件监听）' : '自动备份（文件监听）' }}
          </el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="taskForm.onDemandSync">按需同步</el-checkbox>
        </el-form-item>
        <el-form-item v-if="showCloudOnly">
          <el-checkbox v-model="taskForm.cloudOnly">全新模式（无本地目录，直接创建云端文件）</el-checkbox>
        </el-form-item>
        <el-alert
          v-if="isSyncMode && taskForm.onDemandSync"
          type="info"
          :closable="false"
          :title="autoLocalPathHint"
        />
      </template>

      <!-- Step 2 (only for new tasks) -->
      <template v-if="step === 1 && !isEdit">
        <el-form-item v-if="showLocalPath" label="本地目录">
          <div class="input-row">
            <el-input :model-value="taskForm.localPath" placeholder="选择本地目录" readonly />
            <el-button @click="emit('pick-local')">选择目录</el-button>
          </div>
        </el-form-item>
        <el-form-item v-else-if="isSyncMode && taskForm.onDemandSync" label="本地目录">
          <el-input :model-value="autoLocalPathHint" readonly disabled />
        </el-form-item>
        <el-form-item label="远程目录">
          <div class="input-row">
            <el-input :model-value="taskForm.remotePath || '根目录'" readonly />
            <el-button @click="emit('open-remote-picker')">选择远程目录</el-button>
          </div>
        </el-form-item>
      </template>

      <!-- Edit mode: show paths too -->
      <template v-if="isEdit">
        <el-form-item v-if="showLocalPath" label="本地目录">
          <div class="input-row">
            <el-input v-model="taskForm.localPath" placeholder="选择本地目录" readonly />
            <el-button @click="emit('pick-local')">选择目录</el-button>
          </div>
        </el-form-item>
        <el-form-item v-else-if="isSyncMode && taskForm.onDemandSync" label="本地目录">
          <el-input :model-value="taskForm.localPath || autoLocalPathHint" readonly disabled />
        </el-form-item>
        <el-form-item label="远程目录">
          <div class="input-row">
            <el-input :model-value="taskForm.remotePath || '根目录'" readonly />
            <el-button @click="emit('open-remote-picker')">选择远程目录</el-button>
          </div>
        </el-form-item>
      </template>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleClose">取消</el-button>
        <el-button v-if="!isEdit && step > 0" @click="handlePrev">上一步</el-button>
        <el-button
          v-if="!isEdit && step < 1"
          type="primary"
          :disabled="loading"
          @click="handleNext"
        >
          下一步
        </el-button>
        <el-button
          v-if="isEdit || step === 1"
          type="primary"
          :disabled="loading"
          @click="emit('save')"
        >
          {{ loading ? '保存中...' : '保存任务' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.input-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.input-row .el-input {
  flex: 1;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
