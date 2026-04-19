<script setup>
const props = defineProps({
  loading: {
    type: Boolean,
    default: false
  },
  authMode: {
    type: String,
    required: true
  },
  authForm: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['submit', 'update:authMode', 'update:authForm'])
</script>

<template>
  <section class="auth-section">
    <el-card class="auth-card">
      <el-tabs :model-value="authMode" @update:model-value="(v) => emit('update:authMode', v)">
        <el-tab-pane label="登录" name="login">
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
        </el-tab-pane>
        <el-tab-pane label="注册" name="register">
          <el-form>
            <el-form-item>
              <el-input :model-value="authForm.username" @update:model-value="(v) => emit('update:authForm', { ...authForm, username: v })" placeholder="用户名" />
            </el-form-item>
            <el-form-item>
              <el-input :model-value="authForm.email" @update:model-value="(v) => emit('update:authForm', { ...authForm, email: v })" placeholder="邮箱" />
            </el-form-item>
            <el-form-item>
              <el-input :model-value="authForm.nickname" @update:model-value="(v) => emit('update:authForm', { ...authForm, nickname: v })" placeholder="昵称" />
            </el-form-item>
            <el-form-item>
              <el-input :model-value="authForm.password" @update:model-value="(v) => emit('update:authForm', { ...authForm, password: v })" type="password" placeholder="密码（至少 6 位）" show-password />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="loading" style="width: 100%" @click="emit('submit')">
                {{ loading ? '处理中...' : '注册并进入' }}
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
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
  width: min(560px, 100%);
}

.auth-card :deep(.el-card__body) {
  padding: 32px 36px;
}

.auth-card :deep(.el-tabs__header) {
  margin-bottom: 24px;
}

.auth-card :deep(.el-tabs__item) {
  font-size: 1.05rem;
  padding: 0 24px;
}

.auth-card :deep(.el-form-item) {
  margin-bottom: 24px;
}

.auth-card :deep(.el-input__wrapper) {
  padding: 10px 14px;
  font-size: 1rem;
}

.auth-card :deep(.el-input__inner) {
  font-size: 1rem;
}

.auth-card :deep(.el-button) {
  padding: 14px 24px;
  font-size: 1.05rem;
  height: auto;
}
</style>
