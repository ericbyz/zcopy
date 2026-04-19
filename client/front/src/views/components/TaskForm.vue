<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  taskForm: {
    type: Object,
    required: true
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

const nameCount = computed(() => props.taskForm.name.length)

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
  { title: '备份路径' }
]

const showLocalPath = computed(() => !props.taskForm.cloudOnly)
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
        <el-form-item>
          <el-checkbox v-model="taskForm.autoBackup">自动备份（文件监听）</el-checkbox>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="taskForm.onDemandSync">按需同步</el-checkbox>
        </el-form-item>
        <el-form-item v-if="taskForm.onDemandSync">
          <el-checkbox v-model="taskForm.cloudOnly">全新模式（无本地目录，直接创建云端文件）</el-checkbox>
        </el-form-item>
      </template>

      <!-- Step 2 (only for new tasks) -->
      <template v-if="step === 1 && !isEdit">
        <el-form-item v-if="showLocalPath" label="本地目录">
          <div class="input-row">
            <el-input :model-value="taskForm.localPath" placeholder="选择本地目录" readonly />
            <el-button @click="emit('pick-local')">选择目录</el-button>
          </div>
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
