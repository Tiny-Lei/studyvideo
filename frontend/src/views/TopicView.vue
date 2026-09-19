<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { gradientFor } from '../utils'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const topic = ref(null)
const categories = ref([])

async function load() {
  loading.value = true
  try {
    const res = await api.topic(route.params.id)
    topic.value = res.topic
    categories.value = res.categories || []
  } catch (e) {
    ElMessage.error(e.message)
    if (e.status === 404) router.replace('/')
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div class="container" style="padding-top: 24px">
    <el-skeleton v-if="loading" :rows="6" animated />

    <template v-else-if="topic">
      <el-breadcrumb class="watch-breadcrumb" separator="/">
        <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
        <el-breadcrumb-item>{{ topic.name }}</el-breadcrumb-item>
      </el-breadcrumb>

      <div class="topic-head">
        <div class="topic-icon" :style="{ background: gradientFor(topic.id) }">
          <el-icon><Collection /></el-icon>
        </div>
        <div style="min-width: 0">
          <h2 style="margin: 0 0 6px">{{ topic.name }}</h2>
          <div style="color: var(--sv-muted); font-size: 14px; line-height: 1.6">
            {{ topic.description || '选择一个分类查看讲解视频' }}
          </div>
        </div>
        <el-tag class="topic-count" effect="light" round>{{ categories.length }} 个分类</el-tag>
      </div>

      <div v-if="categories.length" class="topic-grid">
        <router-link v-for="c in categories" :key="c.id" class="topic-card" :to="`/categories/${c.id}`">
          <div class="topic-icon" :style="{ background: gradientFor(c.id + 3) }">
            <el-icon><FolderOpened /></el-icon>
          </div>
          <div style="min-width: 0">
            <div class="name">{{ c.name }}</div>
            <div class="desc">{{ c.description || '点击查看该分类下的讲解视频' }}</div>
          </div>
          <div class="count">{{ c.video_count }} 个</div>
        </router-link>
      </div>
      <div v-else class="empty-box">
        <div class="icon"><el-icon><FolderOpened /></el-icon></div>
        <p>该主题下暂时没有分类，请稍后再来看看。</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.topic-count {
  margin-left: auto;
  flex-shrink: 0;
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
