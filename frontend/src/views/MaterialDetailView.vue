<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { formatSize, formatDate, tagList } from '../utils'
import MarkdownPreview from '../components/MarkdownPreview.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const material = ref(null)
const category = ref(null)
const siblings = ref([])
const mdContent = ref('')
const mdError = ref('')
const pdfError = ref(false)

const ext = computed(() => (material.value?.file_ext || '').toLowerCase())
const isMarkdown = computed(() => ext.value === 'md' || ext.value === 'markdown')
const isPDF = computed(() => ext.value === 'pdf')
const fileUrl = computed(() => (material.value ? api.materialFileUrl(material.value.id) : ''))

async function load() {
  loading.value = true
  mdContent.value = ''
  mdError.value = ''
  pdfError.value = false
  try {
    const res = await api.material(route.params.id)
    material.value = res.material
    category.value = res.category
    siblings.value = res.siblings || []
  } catch (e) {
    material.value = null
    if (e.status === 404) router.replace('/materials')
    return
  } finally {
    loading.value = false
  }
  // Markdown 资料：拉取原文做站内渲染
  if (isMarkdown.value) {
    try {
      const resp = await fetch(fileUrl.value, { credentials: 'same-origin' })
      if (!resp.ok) throw new Error('加载失败')
      mdContent.value = await resp.text()
    } catch {
      mdError.value = '资料内容加载失败，请刷新重试或直接下载'
    }
  }
}

function download() {
  window.location.href = api.materialDownloadUrl(material.value.id)
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div class="container" style="padding-top: 24px">
    <el-skeleton v-if="loading" :rows="8" animated />

    <template v-else-if="material">
      <el-breadcrumb class="watch-breadcrumb" separator="/">
        <el-breadcrumb-item :to="{ path: '/materials' }">资料</el-breadcrumb-item>
        <el-breadcrumb-item v-if="category" :to="`/materials/categories/${category.id}`">
          {{ category.name }}
        </el-breadcrumb-item>
        <el-breadcrumb-item>{{ material.title }}</el-breadcrumb-item>
      </el-breadcrumb>

      <div class="material-head">
        <div style="min-width: 0; flex: 1">
          <h1 class="material-title">{{ material.title }}</h1>
          <div class="material-meta">
            <el-tag v-if="material.group_name" effect="light" round>{{ material.group_name }}</el-tag>
            <el-tag v-for="tag in tagList(material.tags)" :key="tag" type="info" effect="plain" round>
              {{ tag }}
            </el-tag>
            <span class="meta-text">{{ material.file_name }}</span>
            <span class="meta-text">{{ formatSize(material.file_size) }}</span>
            <span class="meta-text">更新于 {{ formatDate(material.updated_at) }}</span>
          </div>
        </div>
        <div class="material-head-actions">
          <el-button v-if="isPDF" type="primary" round tag="a" :href="fileUrl" target="_blank" rel="noopener">
            <el-icon style="margin-right: 4px"><View /></el-icon>新窗口打开
          </el-button>
          <el-button type="primary" round plain @click="download">
            <el-icon style="margin-right: 4px"><Download /></el-icon>下载
          </el-button>
        </div>
      </div>

      <div v-if="material.description" class="material-note">{{ material.description }}</div>

      <!-- Markdown：站内渲染 -->
      <article v-if="isMarkdown" class="doc-shell">
        <el-alert v-if="mdError" type="error" show-icon :closable="false" :title="mdError" />
        <MarkdownPreview v-else :content="mdContent" />
      </article>

      <!-- PDF：站内嵌预览（移动端可点上方按钮新窗口打开） -->
      <div v-else-if="isPDF" class="doc-shell pdf-shell">
        <iframe
          v-if="!pdfError"
          :src="fileUrl"
          class="pdf-frame"
          title="PDF 预览"
          @error="pdfError = true"
        ></iframe>
        <el-alert
          v-if="pdfError"
          type="info"
          show-icon
          :closable="false"
          title="当前环境无法内嵌预览 PDF"
          description="请点击右上角「新窗口打开」或「下载」查看"
        />
      </div>

      <!-- 其它格式：仅下载 -->
      <div v-else class="doc-shell download-shell">
        <el-icon size="46" color="#94a3b8"><Document /></el-icon>
        <p>{{ ext.toUpperCase() }} 文件暂不支持在线预览</p>
        <el-button type="primary" round @click="download">下载文件</el-button>
      </div>

      <!-- 同套题下的其它资料（试卷 / 解析卷） -->
      <div v-if="siblings.length" class="siblings">
        <h3>同套资料</h3>
        <div class="sibling-list">
          <router-link v-for="s in siblings" :key="s.id" class="sibling-item" :to="`/materials/${s.id}`">
            <span class="sibling-ext ext-badge">{{ (s.file_ext || '').toUpperCase() }}</span>
            <span class="sibling-title">{{ s.title }}</span>
          </router-link>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.material-head {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}

.material-title {
  font-size: 24px;
  font-weight: 750;
  margin: 0 0 10px;
  line-height: 1.4;
}

.material-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.meta-text {
  color: var(--sv-muted);
  font-size: 13px;
}

.material-head-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.material-note {
  background: #f0fdfa;
  border: 1px solid #ccfbf1;
  border-radius: 12px;
  padding: 12px 16px;
  color: #0f766e;
  font-size: 14px;
  margin-bottom: 16px;
  white-space: pre-wrap;
}

.doc-shell {
  background: var(--sv-card);
  border: 1px solid var(--sv-line);
  border-radius: 16px;
  padding: 28px 32px;
  box-shadow: 0 4px 18px rgba(31, 35, 51, 0.04);
}

.pdf-shell {
  padding: 0;
  overflow: hidden;
}

.pdf-frame {
  display: block;
  width: 100%;
  height: 78vh;
  border: 0;
  background: #525659;
}

.download-shell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 60px 24px;
  color: var(--sv-muted);
}

.siblings {
  margin-top: 24px;
}

.siblings h3 {
  font-size: 16px;
  margin: 0 0 12px;
}

.sibling-list {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.sibling-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: var(--sv-card);
  border: 1px solid var(--sv-line);
  border-radius: 12px;
  padding: 10px 16px;
  font-size: 14px;
  transition: all 0.16s;
}

.sibling-item:hover {
  border-color: #0d9488;
  color: #0f766e;
  transform: translateY(-1px);
}

.ext-badge {
  background: #0d9488;
  color: #fff;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 6px;
}

@media (max-width: 720px) {
  .material-head {
    flex-direction: column;
  }
  .material-title {
    font-size: 19px;
  }
  .doc-shell {
    padding: 18px 16px;
  }
  .pdf-frame {
    height: 60vh;
  }
}
</style>
