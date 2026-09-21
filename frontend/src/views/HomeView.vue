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
const materialCategories = ref([])
const materialStats = ref({ materials: 0, categories: 0 })
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
  // 资料区入口（独立加载，互不影响）
  try {
    const m = await api.materialHome()
    materialCategories.value = (m.categories || []).slice(0, 4)
    materialStats.value = m.stats || { materials: 0, categories: 0 }
  } catch {
    /* 资料区不可用时不影响首页视频展示 */
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

      <section v-if="materialCategories.length" class="section">
        <div class="section-head">
          <div class="section-title"><span class="bar"></span>学习资料</div>
          <router-link class="section-more" to="/materials">
            全部 {{ materialStats.materials }} 份资料
            <el-icon><ArrowRight /></el-icon>
          </router-link>
        </div>
        <div class="topic-grid">
          <router-link
            v-for="c in materialCategories"
            :key="c.id"
            class="topic-card"
            :to="`/materials/categories/${c.id}`"
          >
            <div class="topic-icon" :style="{ background: gradientFor(c.id + 5) }">
              <el-icon><Files /></el-icon>
            </div>
            <div style="min-width: 0">
              <div class="name">{{ c.name }}</div>
              <div class="desc">{{ c.description || '点击查看该分类下的资料' }}</div>
            </div>
          </router-link>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.section-more {
  color: var(--el-color-primary);
  font-size: 13.5px;
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
</style>
