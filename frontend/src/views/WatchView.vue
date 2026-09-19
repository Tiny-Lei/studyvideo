<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { formatDate, formatSize, tagList } from '../utils'
import MarkdownPreview from '../components/MarkdownPreview.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const video = ref(null)
const topic = ref(null)
const related = ref([])
const pdfs = ref([])
const playUrl = ref('')
const playError = ref('')
const videoError = ref(false)

const isWeChat = computed(() => /MicroMessenger/i.test(navigator.userAgent))
const watchReported = ref(false)

function onVideoPlay() {
  // 只上报本页第一次播放，暂停/继续不重复计数
  if (watchReported.value) return
  watchReported.value = true
  api.reportWatch(route.params.id).catch(() => {})
}

async function load() {
  loading.value = true
  videoError.value = false
  playError.value = ''
  playUrl.value = ''
  pdfs.value = []
  watchReported.value = false
  try {
    const res = await api.video(route.params.id)
    video.value = res.video
    topic.value = res.topic
    related.value = res.related || []
    pdfs.value = res.pdfs || []
  } catch (e) {
    video.value = null
    if (e.status === 404) router.replace('/')
    return
  } finally {
    loading.value = false
  }
  if (video.value && video.value.status !== 'fail') {
    try {
      const res = await api.play(route.params.id)
      playUrl.value = res.url
    } catch (e) {
      playError.value = e.status === 429 ? e.message : '视频地址获取失败，请稍后重试'
    }
  }
}

function openPdf(pdf) {
  window.open(api.pdfFileUrl(pdf.id), '_blank', 'noopener')
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div class="watch-wrap container">
    <el-skeleton v-if="loading" :rows="8" animated />

    <template v-else-if="video">
      <el-breadcrumb class="watch-breadcrumb" separator="/">
        <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
        <el-breadcrumb-item :to="`/topics/${video.topic_id}`">{{ video.topic_name }}</el-breadcrumb-item>
        <el-breadcrumb-item v-if="video.category_id" :to="`/categories/${video.category_id}`">
          {{ video.category_name || '分类' }}
        </el-breadcrumb-item>
        <el-breadcrumb-item>播放</el-breadcrumb-item>
      </el-breadcrumb>

      <div class="watch-layout">
        <div>
          <div class="player-shell">
            <div class="player-box">
              <video
                v-if="playUrl"
                :src="playUrl"
                controls
                playsinline
                webkit-playsinline
                x5-playsinline
                x5-video-player-type="h5-page"
                preload="metadata"
                @play="onVideoPlay"
                @error="videoError = true"
              ></video>
              <div v-else class="player-state">
                <el-icon size="42"><VideoPlay /></el-icon>
                <div v-if="playError">{{ playError }}</div>
                <div v-else-if="video.status === 'fail'">该视频暂时无法播放，请稍后再试</div>
                <div v-else>加载中…</div>
              </div>
            </div>
          </div>

          <div class="watch-panel">
            <h1 class="watch-title">{{ video.title }}</h1>
            <div class="watch-meta">
              <router-link :to="`/topics/${video.topic_id}`">
                <el-tag effect="light" round>{{ video.topic_name }}</el-tag>
              </router-link>
              <router-link v-if="video.category_id" :to="`/categories/${video.category_id}`">
                <el-tag effect="plain" round>{{ video.category_name }}</el-tag>
              </router-link>
              <span>更新于 {{ formatDate(video.updated_at) }}</span>
              <el-tag v-if="video.status === 'fail'" type="danger" effect="light" round>链接失效</el-tag>
              <el-tag v-for="tag in tagList(video.tags)" :key="tag" type="info" effect="plain" round>
                {{ tag }}
              </el-tag>
            </div>

            <el-alert
              v-if="videoError"
              class="wechat-tip"
              type="error"
              show-icon
              :closable="false"
              title="视频加载失败"
              description="可能是网络波动，请刷新重试；如持续失败请稍后再来。"
            />
            <el-alert
              v-else-if="isWeChat"
              class="wechat-tip"
              type="info"
              show-icon
              :closable="false"
              title="微信中需点击播放器才能开始播放"
              description="受微信浏览器限制，视频不会自动播放；点击播放按钮后即可正常拖动进度条。"
            />

            <div v-if="video.description" class="watch-desc">
              <MarkdownPreview :content="video.description" />
            </div>
          </div>

          <div v-if="pdfs.length" class="watch-panel pdf-panel">
            <h3 class="pdf-heading">
              <el-icon><Document /></el-icon>
              配套资料（{{ pdfs.length }}）
            </h3>
            <div class="pdf-list">
              <div v-for="pdf in pdfs" :key="pdf.id" class="pdf-item">
                <span class="pdf-icon"><el-icon><Document /></el-icon></span>
                <span class="pdf-main">
                  <span class="pdf-title">{{ pdf.title }}</span>
                  <span class="pdf-meta">
                    {{ pdf.file_name || 'PDF 资料' }}
                    <template v-if="pdf.file_size"> · {{ formatSize(pdf.file_size) }}</template>
                  </span>
                </span>
                <span class="pdf-actions">
                  <el-button link type="primary" @click="openPdf(pdf)">
                    <el-icon style="margin-right: 3px"><View /></el-icon>在线预览
                  </el-button>
                  <el-button link type="primary" tag="a" :href="api.pdfDownloadUrl(pdf.id)" download>
                    <el-icon style="margin-right: 3px"><Download /></el-icon>下载
                  </el-button>
                </span>
              </div>
            </div>
            <div class="pdf-tip">资料由本站提供，可在线预览或下载保存</div>
          </div>
        </div>

        <aside class="side-card">
          <h3>相关视频</h3>
          <div v-if="related.length" class="video-list video-list-compact">
            <router-link v-for="v in related" :key="v.id" class="video-item" :to="`/videos/${v.id}`">
              <div class="video-info">
                <div class="video-title" style="font-size: 14px">{{ v.title }}</div>
                <div class="video-meta">
                  <span style="color: var(--sv-muted); font-size: 12px">{{ formatDate(v.created_at) }}</span>
                </div>
              </div>
            </router-link>
          </div>
          <div v-else style="color: var(--sv-muted); font-size: 13px">暂无相关视频</div>
        </aside>
      </div>
    </template>
  </div>
</template>
