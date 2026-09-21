<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const kw = ref('')

// 资料区独立搜索：资料页面里的搜索只搜资料，其它页面只搜视频
const inMaterials = computed(() => route.path.startsWith('/materials'))

const placeholder = computed(() =>
  inMaterials.value ? '搜索资料标题 / 标签 / 套题' : '搜索视频标题 / 标签 / 主题'
)

watch(
  () => route.query.q,
  (q) => {
    if (route.name === 'search' || route.name === 'material-search') kw.value = String(q || '')
  },
  { immediate: true }
)

function goSearch() {
  const q = kw.value.trim()
  if (!q) return
  if (inMaterials.value) {
    router.push({ name: 'material-search', query: { q } })
  } else {
    router.push({ name: 'search', query: { q } })
  }
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
        <el-input v-model="kw" :placeholder="placeholder" clearable @keyup.enter="goSearch">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>

      <nav class="header-nav">
        <router-link to="/">首页</router-link>
        <router-link to="/materials">资料</router-link>
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
