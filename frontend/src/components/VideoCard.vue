<script setup>
import { computed } from 'vue'
import { gradientFor, tagList } from '../utils'

const props = defineProps({
  video: { type: Object, required: true },
  index: { type: Number, default: 0 },
  showTopic: { type: Boolean, default: false }
})

const tags = computed(() => tagList(props.video.tags).slice(0, 3))
</script>

<template>
  <router-link class="video-card" :to="`/videos/${video.id}`">
    <div class="video-card-cover" :style="{ background: gradientFor(video.topic_id) }">
      <span v-if="index" class="idx">{{ String(index).padStart(2, '0') }}</span>
      <span class="play"><el-icon><VideoPlay /></el-icon></span>
      <el-tag v-if="video.status === 'fail'" class="fail-tag" type="danger" size="small" effect="dark" round>
        无法播放
      </el-tag>
    </div>
    <div class="video-card-body">
      <div class="video-card-title">{{ video.title }}</div>
      <div class="video-card-tags">
        <el-tag v-if="showTopic && video.topic_name" size="small" effect="light" round>
          {{ video.topic_name }}
        </el-tag>
        <el-tag v-for="tag in tags" :key="tag" size="small" type="info" effect="plain" round>
          {{ tag }}
        </el-tag>
      </div>
    </div>
  </router-link>
</template>

<style scoped>
.video-card {
  display: flex;
  flex-direction: column;
  background: var(--sv-card);
  border: 1px solid var(--sv-line);
  border-radius: 14px;
  overflow: hidden;
  transition: transform 0.16s ease, box-shadow 0.16s ease, border-color 0.16s;
}

.video-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--sv-shadow);
  border-color: var(--el-color-primary-light-7);
}

.video-card-cover {
  position: relative;
  aspect-ratio: 16 / 9;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.95);
}

.video-card-cover .idx {
  position: absolute;
  left: 10px;
  top: 8px;
  font-size: 12px;
  font-weight: 700;
  opacity: 0.85;
}

.video-card-cover .play {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.22);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  transition: transform 0.16s, background 0.16s;
}

.video-card:hover .play {
  transform: scale(1.08);
  background: rgba(255, 255, 255, 0.32);
}

.fail-tag {
  position: absolute;
  right: 8px;
  bottom: 8px;
}

.video-card-body {
  padding: 10px 12px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
}

.video-card-title {
  font-size: 14px;
  font-weight: 650;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 2.9em;
  word-break: break-word;
}

.video-card-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: auto;
}

.video-card-tags :deep(.el-tag) {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
