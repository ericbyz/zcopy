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
  },
  emptyText: {
    type: String,
    default: '暂无备份任务'
  },
  emptyHint: {
    type: String,
    default: '点击左侧「创建备份任务」开始'
  }
})

const emit = defineEmits(['edit', 'sync', 'toggle-auto', 'delete', 'open-location', 'open-local', 'open-remote', 'refresh'])
</script>

<template>
  <div>
    <div v-if="tasks.length === 0" class="empty-state">
      <p>{{ emptyText }}</p>
      <p class="empty-hint">{{ emptyHint }}</p>
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
        @open-local="emit('open-local', $event)"
        @open-remote="emit('open-remote', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.empty-state {
  text-align: center;
  padding: 48px 16px;
  color: var(--z-text-muted);
  background: var(--z-bg-elevated);
  border: 1px solid var(--z-border);
  border-radius: 8px;
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
  gap: 10px;
}
</style>
