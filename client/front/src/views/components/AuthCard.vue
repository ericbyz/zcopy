<script setup>
const props = defineProps({
  loading: {
    type: Boolean,
    default: false
  },
  authForm: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['submit', 'update:authForm'])
</script>

<template>
  <section class="auth-section">
    <el-card class="auth-card">
      <div class="auth-title">登录</div>
      <el-form>
        <el-form-item>
          <el-input :model-value="authForm.account" @update:model-value="(v) => emit('update:authForm', { ...authForm, account: v })" placeholder="用户名或邮箱" />
        </el-form-item>
        <el-form-item>
          <el-input :model-value="authForm.password" @update:model-value="(v) => emit('update:authForm', { ...authForm, password: v })" type="password" placeholder="密码" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" style="width: 100%" @click="emit('submit')">
            {{ loading ? '处理中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </section>
</template>

<style scoped>
.auth-section {
  display: flex;
  justify-content: center;
  width: 100%;
}

.auth-card {
  width: min(420px, 100%);
}

.auth-card :deep(.el-card__body) {
  padding: 22px 26px;
}

.auth-title {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 18px;
  color: var(--z-text-primary);
}

.auth-card :deep(.el-form-item) {
  margin-bottom: 18px;
}

.auth-card :deep(.el-input__wrapper) {
  padding: 7px 12px;
  font-size: 0.92rem;
}

.auth-card :deep(.el-input__inner) {
  font-size: 0.92rem;
}

.auth-card :deep(.el-button) {
  padding: 10px 18px;
  font-size: 0.95rem;
  height: auto;
}
</style>
