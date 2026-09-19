<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { gradientFor } from '../utils'
import VideoRow from '../components/VideoRow.vue'

const router = useRouter()
const loading = ref(true)
const topics = ref([])
const recent = ref([])
const stats = ref({ topics: 0, videos: 0 })
const kw = ref('')

onMounted(async () => {
  try {
    const res = await api.home()
    topics.value = res.topics || []
    recent.value = res.recent || []
    stats.value = res.stats || { topics: 0, videos: 0 }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
})

function goSearch() {
  const q = kw.value.trim()
  if (!q) return
  router.push({ name: 'search', query: { q } })
}
</script>

<template>
  <section class="hero">
    <div class="container hero-inner">
      <h1>一站式观看</h1>
      <p>按主题浏览真题讲解与考点视频，支持搜索、在线播放、随时回看。无需下载，点开即学。</p>
      <div class="hero-search">
        <el-input v-model="kw" size="large" placeholder="搜索标题 / 标签 / 主题，例如：GESP 一级" clearable @keyup.enter="goSearch">
          <template #prefix><el-icon><Search /></el-icon></template>
          <template #append>
            <el-button type="primary" @click="goSearch">搜索</el-button>
          </template>
        </el-input>
      </div>
      <div class="hero-stats">
        <div>
          <div class="num">{{ stats.topics }}</div>
          <div class="label">视频主题</div>
        </div>
        <div>
          <div class="num">{{ stats.videos }}</div>
          <div class="label">讲解视频</div>
        </div>
      </div>
    </div>
  </section>

  <div class="container">
    <el-skeleton v-if="loading" :rows="6" animated />

    <template v-else>
      <section class="section">
        <div class="section-head">
          <div class="section-title"><span class="bar"></span>视频主题</div>
        </div>
        <div v-if="topics.length" class="topic-grid">
          <router-link v-for="t in topics" :key="t.id" class="topic-card" :to="`/topics/${t.id}`">
            <div class="topic-icon" :style="{ background: gradientFor(t.id) }">
              <el-icon><Collection /></el-icon>
            </div>
            <div style="min-width: 0">
              <div class="name">{{ t.name }}</div>
              <div class="desc">{{ t.description || '点击查看该主题下的讲解视频' }}</div>
            </div>
          </router-link>
        </div>
        <div v-else class="empty-box">
          <div class="icon"><el-icon><FolderOpened /></el-icon></div>
          <p>暂时还没有内容，请稍后再来看看。</p>
        </div>
      </section>

      <section v-if="recent.length" class="section">
        <div class="section-head">
          <div class="section-title"><span class="bar"></span>最新更新</div>
        </div>
        <div class="video-list">
          <VideoRow v-for="(v, i) in recent" :key="v.id" :video="v" :index="i + 1" show-topic />
        </div>
      </section>
    </template>
  </div>
</template>
