<script setup>
const props = defineProps({
  capabilities: {
    type: Object,
    required: true
  }
})
</script>

<template>
  <el-card class="info-card">
    <template #header>
      <span>平台能力</span>
    </template>
    <el-descriptions :column="1" border>
      <el-descriptions-item label="当前系统">
        {{ capabilities.os }}
      </el-descriptions-item>
      <el-descriptions-item label="按需同步支持">
        {{ capabilities.onDemandSupport ? '支持' : '不支持' }}
      </el-descriptions-item>
      <el-descriptions-item label="模式">
        {{ capabilities.onDemandMode }}
      </el-descriptions-item>
      <el-descriptions-item label="说明">
        启用按需同步后会自动初始化，同步目录请直接从 location 中按需打开文件。
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<style scoped>
.info-card {
  border-radius: 12px;
}
</style>
