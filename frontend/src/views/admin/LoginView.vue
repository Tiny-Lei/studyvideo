<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../../api'
import { invalidateSessionCache } from '../../router'

const route = useRoute()
const router = useRouter()
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!password.value) {
    ElMessage.warning('请输入管理密码')
    return
  }
  loading.value = true
  try {
    await api.login(password.value)
    invalidateSessionCache()
    ElMessage.success('登录成功')
    router.replace(String(route.query.redirect || '/admin'))
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-brand">
        <span class="brand-logo"><el-icon><VideoPlay /></el-icon></span>
        <h2 style="margin: 0">StudyVideo 管理后台</h2>
        <p style="color: var(--sv-muted); font-size: 13px; margin-top: 8px">仅运营管理员使用</p>
      </div>
      <el-form @submit.prevent="submit">
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            size="large"
            placeholder="管理密码"
            show-password
            @keyup.enter="submit"
          >
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="submit">
          登录
        </el-button>
      </el-form>
    </div>
  </div>
</template>
