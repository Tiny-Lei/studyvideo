<script setup>
import { computed } from 'vue'
import { gradientFor, plainText, tagList } from '../utils'

const props = defineProps({
  video: { type: Object, required: true },
  index: { type: Number, default: 0 },
  showTopic: { type: Boolean, default: false }
})

const summary = computed(() => plainText(props.video.description))
</script>

<template>
  <router-link class="video-item" :to="`/videos/${video.id}`">
    <div class="video-thumb" :style="{ background: gradientFor(video.topic_id) }">
      <span v-if="index" class="idx">{{ String(index).padStart(2, '0') }}</span>
      <span class="play"><el-icon><VideoPlay /></el-icon></span>
    </div>
    <div class="video-info">
      <div class="video-title">{{ video.title }}</div>
      <div v-if="summary" class="video-desc">{{ summary }}</div>
      <div class="video-meta">
        <el-tag v-if="showTopic && video.topic_name" size="small" effect="light" round>
          {{ video.topic_name }}
        </el-tag>
        <el-tag v-for="tag in tagList(video.tags)" :key="tag" size="small" type="info" effect="plain" round>
          {{ tag }}
        </el-tag>
        <el-tag v-if="video.status === 'fail'" size="small" type="danger" effect="light" round>
          链接失效
        </el-tag>
      </div>
    </div>
    <el-icon class="row-arrow"><ArrowRight /></el-icon>
  </router-link>
</template>

<style scoped>
.row-arrow {
  color: #c3c7d4;
  flex-shrink: 0;
  transition: transform 0.16s, color 0.16s;
}
.video-item:hover .row-arrow {
  color: var(--el-color-primary);
  transform: translateX(3px);
}
@media (max-width: 720px) {
  .row-arrow {
    display: none;
  }
}
</style>
