<script setup>
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

const isActive = (path) => {
  return route.path === path
}
</script>

<template>
  <div class="page">
    <div class="shell">
      <!-- Navigation Bar -->
      <div class="card" style="padding: 16px 24px; margin-bottom: 16px; display: flex; gap: 12px; align-items: center">
        <div style="display: flex; gap: 8px">
          <router-link to="/" class="btn" :class="{ active: isActive('/') }">
            Dashboard
          </router-link>
          <router-link to="/logs" class="btn" :class="{ active: isActive('/logs') }">
            日志
          </router-link>
        </div>
      </div>

      <!-- Router View -->
      <router-view />
    </div>
  </div>
</template>

<style>
div .card .btn {
  background: rgba(148, 163, 184, 0.12);
  color: #cbd5e1;
  text-decoration: none;
}

div .card .btn.active {
  background: linear-gradient(135deg, #2563eb, #4f46e5);
  color: #fff;
}
</style>
