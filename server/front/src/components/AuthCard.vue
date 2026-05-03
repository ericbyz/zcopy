<script setup>
const props = defineProps({
  loading: { type: Boolean, default: false },
  authMode: { type: String, default: 'login' },
  authForm: { type: Object, required: true }
})

const emit = defineEmits(['update:authMode', 'submit'])
</script>

<template>
  <el-card class="auth-card">
    <el-tabs :model-value="authMode" @update:model-value="(val) => emit('update:authMode', val)">
      <el-tab-pane label="登录" name="login">
        <el-form>
          <el-form-item>
            <el-input v-model="authForm.account" placeholder="用户名或邮箱" />
          </el-form-item>
          <el-form-item>
            <el-input v-model="authForm.password" type="password" placeholder="密码" show-password />
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
            <el-input v-model="authForm.username" placeholder="用户名" />
          </el-form-item>
          <el-form-item>
            <el-input v-model="authForm.email" placeholder="邮箱" />
          </el-form-item>
          <el-form-item>
            <el-input v-model="authForm.nickname" placeholder="昵称" />
          </el-form-item>
          <el-form-item>
            <el-input v-model="authForm.password" type="password" placeholder="密码（至少 6 位）" show-password />
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
</template>

<style scoped>
.auth-card {
  width: min(480px, 100%);
}
</style>
