<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { formatSize, tagList } from '../utils'

const props = defineProps({
  material: { type: Object, required: true },
  showCategory: { type: Boolean, default: false }
})

const router = useRouter()

const ext = computed(() => (props.material.file_ext || '').toLowerCase())
const isMarkdown = computed(() => ext.value === 'md' || ext.value === 'markdown')
const isPDF = computed(() => ext.value === 'pdf')

const meta = computed(() => {
  const parts = []
  if (props.material.group_name) parts.push(props.material.group_name)
  if (props.material.file_size) parts.push(formatSize(props.material.file_size))
  return parts.join(' · ')
})

// MD 进详情页渲染；其它类型新窗口打开（浏览器原生预览/下载）
function open() {
  if (isMarkdown.value) {
    router.push(`/materials/${props.material.id}`)
    return
  }
  window.open(`/api/materials/${props.material.id}/file`, '_blank', 'noopener')
}

function download() {
  window.location.href = `/api/materials/${props.material.id}/download`
}
</script>

<template>
  <div class="material-item" @click="open">
    <div class="material-ext" :class="`ext-${ext}`">{{ ext.toUpperCase() || 'FILE' }}</div>
    <div class="material-info">
      <div class="material-title">{{ material.title }}</div>
      <div v-if="material.description" class="material-desc">{{ material.description }}</div>
      <div class="material-meta">
        <el-tag v-if="showCategory && material.category_name" size="small" effect="light" round>
          {{ material.category_name }}
        </el-tag>
        <el-tag v-for="tag in tagList(material.tags)" :key="tag" size="small" type="info" effect="plain" round>
          {{ tag }}
        </el-tag>
        <span v-if="meta" class="material-meta-text">{{ meta }}</span>
      </div>
    </div>
    <div class="material-actions" @click.stop>
      <el-button link type="primary" @click="open">
        <el-icon style="margin-right: 3px"><View /></el-icon>{{ isMarkdown ? '在线阅读' : '预览' }}
      </el-button>
      <el-button link type="primary" @click="download">
        <el-icon style="margin-right: 3px"><Download /></el-icon>下载
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.material-item {
  display: flex;
  align-items: center;
  gap: 14px;
  background: var(--sv-card);
  border: 1px solid var(--sv-line);
  border-radius: 14px;
  padding: 14px 16px;
  cursor: pointer;
  transition: transform 0.16s ease, box-shadow 0.16s ease, border-color 0.16s;
}

.material-item:hover {
  transform: translateY(-2px);
  box-shadow: var(--sv-shadow);
  border-color: #14b8a6;
}

.material-ext {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 800;
  color: #fff;
  flex-shrink: 0;
  letter-spacing: 0.5px;
  background: #64748b;
}

.ext-pdf { background: #dc2626; }
.ext-md, .ext-markdown { background: #2563eb; }
.ext-ppt, .ext-pptx { background: #ea580c; }
.ext-doc, .ext-docx { background: #1d4ed8; }
.ext-xls, .ext-xlsx { background: #16a34a; }
.ext-zip, .ext-rar, .ext-7z { background: #7c3aed; }
.ext-txt { background: #475569; }

.material-info {
  flex: 1;
  min-width: 0;
}

.material-title {
  font-size: 15.5px;
  font-weight: 650;
  line-height: 1.45;
  margin-bottom: 4px;
}

.material-desc {
  color: var(--sv-muted);
  font-size: 13px;
  line-height: 1.5;
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.material-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.material-meta-text {
  color: var(--sv-muted);
  font-size: 12.5px;
}

.material-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

@media (max-width: 720px) {
  .material-item {
    flex-wrap: wrap;
  }
  .material-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
