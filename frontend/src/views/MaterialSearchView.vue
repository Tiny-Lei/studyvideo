<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import MaterialRow from '../components/MaterialRow.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const materials = ref([])
const kw = ref('')
const searched = ref(false)

async function search(q) {
  if (!q) {
    materials.value = []
    searched.value = false
    return
  }
  loading.value = true
  searched.value = true
  try {
    const res = await api.materialSearch(q)
    materials.value = res.materials || []
  } catch {
    materials.value = []
  } finally {
    loading.value = false
  }
}

function submit() {
  const q = kw.value.trim()
  if (!q) return
  router.push({ name: 'material-search', query: { q } })
}

watch(
  () => route.query.q,
  (q) => {
    kw.value = String(q || '')
    search(kw.value.trim())
  },
  { immediate: true }
)
</script>

<template>
  <div class="container search-head">
    <el-breadcrumb class="watch-breadcrumb" separator="/">
      <el-breadcrumb-item :to="{ path: '/materials' }">资料</el-breadcrumb-item>
      <el-breadcrumb-item>搜索</el-breadcrumb-item>
    </el-breadcrumb>
    <h2 style="margin: 8px 0 14px">搜索学习资料</h2>
    <div class="search-input">
      <el-input v-model="kw" size="large" placeholder="输入资料标题、标签、套题或分类名称" clearable @keyup.enter="submit">
        <template #prefix><el-icon><Search /></el-icon></template>
        <template #append>
          <el-button type="primary" @click="submit">搜索</el-button>
        </template>
      </el-input>
    </div>
    <div class="search-scope">只搜索资料库，不包含讲解视频</div>
  </div>

  <div class="container" style="margin-top: 20px">
    <div v-if="loading" class="empty-box">正在搜索…</div>
    <template v-else-if="searched">
      <div v-if="materials.length" class="video-list">
        <MaterialRow v-for="m in materials" :key="m.id" :material="m" show-category />
      </div>
      <div v-else class="empty-box">
        <div class="icon"><el-icon><Search /></el-icon></div>
        <p>没有找到与「{{ kw }}」相关的资料</p>
      </div>
    </template>
    <div v-else class="empty-box">
      <div class="icon"><el-icon><Search /></el-icon></div>
      <p>输入关键词开始搜索资料</p>
    </div>
  </div>
</template>

<style scoped>
.video-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.search-scope {
  margin-top: 8px;
  color: var(--sv-muted);
  font-size: 12.5px;
}
</style>
