<script setup>
import TaskCard from './TaskCard.vue'

const props = defineProps({
  tasks: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  },
  onDemandStatuses: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['edit', 'sync', 'toggle-auto', 'delete', 'open-location', 'refresh'])
</script>

<template>
  <div>
    <div v-if="tasks.length === 0" class="empty-state">
      <p>暂无备份任务</p>
      <p class="empty-hint">点击左侧「创建新任务」开始</p>
    </div>

    <div v-else class="tasks-list">
      <TaskCard
        v-for="task in tasks"
        :key="task.id"
        :task="task"
        :loading="loading"
        :on-demand-status="onDemandStatuses[task.id]"
        @edit="emit('edit', $event)"
        @sync="emit('sync', $event)"
        @toggle-auto="emit('toggle-auto', $event)"
        @delete="emit('delete', $event)"
        @open-location="emit('open-location', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--z-text-muted);
}

.empty-state p {
  margin: 0 0 8px;
}

.empty-state p:first-child {
  font-size: 1.1rem;
  font-weight: 500;
  color: var(--z-text-secondary);
}

.empty-hint {
  font-size: 0.9rem;
}

.tasks-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
</style>
