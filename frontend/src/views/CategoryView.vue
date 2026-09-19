<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { gradientFor } from '../utils'
import VideoCard from '../components/VideoCard.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const category = ref(null)
const topic = ref(null)
const videos = ref([])
const order = ref('newest')

const ORDER_OPTIONS = [
  { value: 'newest', label: '最新发布' },
  { value: 'oldest', label: '最早发布' },
  { value: 'default', label: '推荐排序' }
]

async function load() {
  loading.value = true
  try {
    const res = await api.category(route.params.id, order.value === 'default' ? '' : order.value)
    category.value = res.category
    topic.value = res.topic
    videos.value = res.videos || []
  } catch (e) {
    ElMessage.error(e.message)
    if (e.status === 404) router.replace('/')
  } finally {
    loading.value = false
  }
}

function changeOrder(value) {
  order.value = value
  load()
}

onMounted(load)
watch(
  () => route.params.id,
  () => {
    order.value = 'newest'
    load()
  }
)
</script>

<template>
  <div class="container" style="padding-top: 24px">
    <el-skeleton v-if="loading" :rows="6" animated />

    <template v-else-if="category">
      <el-breadcrumb class="watch-breadcrumb" separator="/">
        <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
        <el-breadcrumb-item v-if="topic" :to="`/topics/${topic.id}`">{{ topic.name }}</el-breadcrumb-item>
        <el-breadcrumb-item>{{ category.name }}</el-breadcrumb-item>
      </el-breadcrumb>

      <div class="topic-head">
        <div class="topic-icon" :style="{ background: gradientFor(category.id + 3) }">
          <el-icon><FolderOpened /></el-icon>
        </div>
        <div style="min-width: 0">
          <h2 style="margin: 0 0 6px">{{ category.name }}</h2>
          <div style="color: var(--sv-muted); font-size: 14px; line-height: 1.6">
            {{ category.description || '该分类下的讲解视频列表' }}
          </div>
        </div>
        <el-tag class="topic-count" effect="light" round>{{ videos.length }} 个视频</el-tag>
      </div>

      <div class="topic-toolbar">
        <span class="toolbar-label">排序</span>
        <el-radio-group :model-value="order" size="small" @change="changeOrder">
          <el-radio-button v-for="opt in ORDER_OPTIONS" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </el-radio-button>
        </el-radio-group>
      </div>

      <div v-if="videos.length" class="video-grid">
        <VideoCard v-for="(v, i) in videos" :key="v.id" :video="v" :index="i + 1" />
      </div>
      <div v-else class="empty-box">
        <div class="icon"><el-icon><VideoCamera /></el-icon></div>
        <p>该分类下暂时没有视频</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.topic-count {
  margin-left: auto;
  flex-shrink: 0;
}

.topic-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.toolbar-label {
  color: var(--sv-muted);
  font-size: 13px;
}

@media (max-width: 720px) {
  .topic-head {
    flex-wrap: wrap;
  }
  .topic-count {
    margin-left: 0;
  }
}
</style>
