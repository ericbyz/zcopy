<script setup>
import { inject } from 'vue'
import { Sun, Moon, Monitor } from 'lucide-vue-next'

const themeMode = inject('themeMode')
</script>

<template>
  <div class="settings-page">
    <h2>设置</h2>

    <el-card class="settings-card">
      <template #header>
        <span>外观</span>
      </template>

      <div class="theme-options">
        <div
          v-for="opt in [
            { value: 'light', icon: Sun, label: '浅色' },
            { value: 'dark', icon: Moon, label: '深色' },
            { value: 'auto', icon: Monitor, label: '跟随系统' }
          ]"
          :key="opt.value"
          class="theme-option"
          :class="{ active: themeMode === opt.value }"
          @click="themeMode = opt.value"
        >
          <component :is="opt.icon" style="width:24px;height:24px" />
          <span>{{ opt.label }}</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.settings-page {
  max-width: 640px;
}

.settings-page h2 {
  margin: 0 0 20px;
}

.settings-card {
  border-radius: 12px;
}

.theme-options {
  display: flex;
  gap: 12px;
}

.theme-option {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 20px 16px;
  border-radius: 12px;
  border: 2px solid var(--z-border);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  color: var(--z-text-secondary);
}

.theme-option:hover {
  border-color: var(--z-accent-hover);
  background: var(--z-accent-bg);
}

.theme-option.active {
  border-color: var(--z-accent);
  background: var(--z-accent-bg);
  color: var(--z-accent);
}
</style>
