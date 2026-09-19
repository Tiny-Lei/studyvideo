<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import VideoRow from '../components/VideoRow.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const videos = ref([])
const kw = ref('')
const searched = ref(false)

async function search(q) {
  if (!q) {
    videos.value = []
    searched.value = false
    return
  }
  loading.value = true
  searched.value = true
  try {
    const res = await api.search(q)
    videos.value = res.videos || []
  } catch {
    videos.value = []
  } finally {
    loading.value = false
  }
}

function submit() {
  const q = kw.value.trim()
  if (!q) return
  router.push({ name: 'search', query: { q } })
}

watch(
  () => route.query.q,
  (q) => {
    kw.value = String(q || '')
    search(kw.value.trim())
  },
  { immediate: true }
)

onMounted(() => {
  if (!route.query.q) searched.value = false
})
</script>

<template>
  <div class="container search-head">
    <h2 style="margin: 0 0 14px">搜索讲解视频</h2>
    <div class="search-input">
      <el-input v-model="kw" size="large" placeholder="输入标题、标签或主题名称" clearable @keyup.enter="submit">
        <template #prefix><el-icon><Search /></el-icon></template>
        <template #append>
          <el-button type="primary" @click="submit">搜索</el-button>
        </template>
      </el-input>
    </div>
  </div>

  <div class="container" style="margin-top: 20px">
    <div v-if="loading" class="empty-box">正在搜索…</div>
    <template v-else-if="searched">
      <div v-if="videos.length" class="video-list">
        <VideoRow v-for="(v, i) in videos" :key="v.id" :video="v" :index="i + 1" show-topic />
      </div>
      <div v-else class="empty-box">
        <div class="icon"><el-icon><Search /></el-icon></div>
        <p>没有找到与「{{ kw }}」相关的视频</p>
      </div>
    </template>
    <div v-else class="empty-box">
      <div class="icon"><el-icon><Search /></el-icon></div>
      <p>输入关键词开始搜索</p>
    </div>
  </div>
</template>
