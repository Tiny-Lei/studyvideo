<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { gradientFor, formatSize } from '../utils'
import MaterialRow from '../components/MaterialRow.vue'

const router = useRouter()
const loading = ref(true)
const categories = ref([])
const latest = ref([])
const stats = ref({ materials: 0, categories: 0 })
const kw = ref('')

onMounted(async () => {
  try {
    const res = await api.materialHome()
    categories.value = res.categories || []
    latest.value = res.latest || []
    stats.value = res.stats || { materials: 0, categories: 0 }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
})

function goSearch() {
  const q = kw.value.trim()
  if (!q) return
  router.push({ name: 'material-search', query: { q } })
}
</script>

<template>
  <section class="hero hero-materials">
    <div class="container hero-inner">
      <h1>学习资料库</h1>
      <p>课件、试卷、历年真题与解析，支持在线查看与下载（Markdown 资料可直接在线阅读）</p>
      <div class="hero-search">
        <el-input v-model="kw" size="large" placeholder="搜索资料标题 / 标签 / 分类，例如：GESP 一级" clearable @keyup.enter="goSearch">
          <template #prefix><el-icon><Search /></el-icon></template>
          <template #append>
            <el-button type="primary" @click="goSearch">搜索</el-button>
          </template>
        </el-input>
      </div>
      <div class="hero-stats">
        <div>
          <div class="num">{{ stats.categories }}</div>
          <div class="label">资料分类</div>
        </div>
        <div>
          <div class="num">{{ stats.materials }}</div>
          <div class="label">份资料</div>
        </div>
      </div>
    </div>
  </section>

  <div class="container">
    <el-skeleton v-if="loading" :rows="6" animated />

    <template v-else>
      <section class="section">
        <div class="section-head">
          <div class="section-title"><span class="bar"></span>资料分类</div>
        </div>
        <div v-if="categories.length" class="topic-grid">
          <router-link v-for="c in categories" :key="c.id" class="topic-card" :to="`/materials/categories/${c.id}`">
            <div class="topic-icon" :style="{ background: gradientFor(c.id + 5) }">
              <el-icon><FolderOpened /></el-icon>
            </div>
            <div style="min-width: 0">
              <div class="name">{{ c.name }}</div>
              <div class="desc">{{ c.description || '点击查看该分类下的资料' }}</div>
            </div>
          </router-link>
        </div>
        <div v-else class="empty-box">
          <div class="icon"><el-icon><FolderOpened /></el-icon></div>
          <p>还没有资料，敬请期待。</p>
        </div>
      </section>

      <section v-if="latest.length" class="section">
        <div class="section-head">
          <div class="section-title"><span class="bar"></span>最新资料</div>
        </div>
        <div class="video-list">
          <MaterialRow v-for="m in latest" :key="m.id" :material="m" show-category />
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.hero-materials {
  background: linear-gradient(135deg, #0f766e 0%, #0d9488 55%, #14b8a6 100%);
}

.video-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.topic-card .name {
  font-weight: 700;
  font-size: 16px;
  margin-bottom: 4px;
}

.topic-card .desc {
  color: var(--sv-muted);
  font-size: 13px;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
