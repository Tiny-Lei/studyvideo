<script setup>
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const kw = ref('')

watch(
  () => route.query.q,
  (q) => {
    if (route.name === 'search') kw.value = String(q || '')
  },
  { immediate: true }
)

function goSearch() {
  const q = kw.value.trim()
  if (!q) return
  router.push({ name: 'search', query: { q } })
}
</script>

<template>
  <header class="site-header">
    <div class="container header-inner">
      <router-link to="/" class="brand">
        <span class="brand-logo"><el-icon><VideoPlay /></el-icon></span>
        <span class="brand-text">StudyVideo</span>
      </router-link>

      <div class="header-search">
        <el-input v-model="kw" placeholder="搜索标题 / 标签 / 主题" clearable @keyup.enter="goSearch">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>

      <nav class="header-nav">
        <router-link to="/">首页</router-link>
      </nav>
    </div>
  </header>

  <main class="site-main">
    <router-view />
  </main>

  <footer class="site-footer">
    <div class="container">StudyVideo</div>
  </footer>
</template>
