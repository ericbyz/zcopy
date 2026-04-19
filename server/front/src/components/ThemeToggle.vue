<script setup>
import { computed, ref, onMounted } from 'vue'
import { Sun, Moon, Monitor } from 'lucide-vue-next'
import { cycleThemeMode } from '../utils/theme.js'

const themeMode = ref(localStorage.getItem('zcopy-theme') || 'auto')

function cycleTheme() {
  themeMode.value = cycleThemeMode(themeMode.value)
  localStorage.setItem('zcopy-theme', themeMode.value)
  applyTheme()
}

function applyTheme() {
  const isDark = themeMode.value === 'dark' || 
    (themeMode.value === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', isDark)
}

const themeIcon = computed(() => {
  if (themeMode.value === 'light') return Sun
  if (themeMode.value === 'dark') return Moon
  return Monitor
})

onMounted(() => {
  applyTheme()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (themeMode.value === 'auto') applyTheme()
  })
})
</script>

<template>
  <el-button circle @click="cycleTheme">
    <component :is="themeIcon" style="width: 18px; height: 18px" />
  </el-button>
</template>
