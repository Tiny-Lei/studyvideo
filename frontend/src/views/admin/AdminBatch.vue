<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../api'

const props = defineProps({
  active: { type: String, default: '' }
})

const topics = ref([])
const categories = ref([])
const form = ref({ topic_id: null, category_id: null, default_tags: '', text: '' })
const submitting = ref(false)
const result = ref(null)

const placeholder = `每行一条，支持以下格式（链接必须在行内）：
https://static.hetaoimg.com/crmFiles/abc.mp4
GESP 一级 2024-03 第 1 题讲解 | https://static.hetaoimg.com/crmFiles/abc.mp4
GESP 一级 2024-03 第 2 题讲解 | GESP,一级,选择题 | https://static.hetaoimg.com/crmFiles/def.mp4 | 备注写在这里
# 以 # 开头的行会被忽略`

async function loadTopics() {
  try {
    const res = await api.adminTopics()
    topics.value = res.topics || []
    if (!form.value.topic_id && topics.value.length) {
      form.value.topic_id = topics.value[0].id
      await loadCategories(form.value.topic_id)
      if (categories.value.length) form.value.category_id = categories.value[0].id
    }
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadCategories(topicId) {
  if (!topicId) {
    categories.value = []
    return
  }
  try {
    const res = await api.adminCategories(topicId)
    categories.value = res.categories || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function onTopicChange() {
  form.value.category_id = null
  loadCategories(form.value.topic_id)
}

async function submit() {
  if (!form.value.topic_id) return ElMessage.warning('请选择所属主题')
  if (!form.value.category_id) return ElMessage.warning('请选择所属分类')
  if (!form.value.text.trim()) return ElMessage.warning('请粘贴视频链接')
  submitting.value = true
  result.value = null
  try {
    const res = await api.batchCreate(form.value)
    result.value = res
    if (res.created > 0) {
      ElMessage.success(`成功导入 ${res.created} 条，正在后台检测链接`)
    } else {
      ElMessage.warning('没有新增记录，请检查粘贴内容')
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

onMounted(loadTopics)

// Tab 切回「批量录入」时刷新主题下拉
watch(
  () => props.active,
  (v) => {
    if (v === 'batch') loadTopics()
  }
)
</script>

<template>
  <div class="admin-card">
    <h3 style="margin-top: 0">批量粘贴链接快速创建</h3>
    <el-form label-width="auto" label-position="right" style="max-width: 760px">
      <el-form-item label="所属主题" required>
        <el-select v-model="form.topic_id" placeholder="请选择主题" style="width: 100%" @change="onTopicChange">
          <el-option v-for="t in topics" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="所属分类" required>
        <el-select v-model="form.category_id" placeholder="请选择分类" style="width: 100%" :disabled="!form.topic_id">
          <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
        <el-alert
          v-if="form.topic_id && !categories.length"
          style="width: 100%; margin-top: 8px"
          type="warning"
          show-icon
          :closable="false"
          title="该主题下还没有分类，请先到「分类管理」创建分类"
        />
      </el-form-item>
      <el-form-item label="默认标签">
        <el-input v-model="form.default_tags" placeholder="可选：未单独填写标签的行将使用该标签，例如 GESP,真题" />
      </el-form-item>
      <el-form-item label="粘贴内容" required>
        <el-input
          v-model="form.text"
          class="batch-textarea"
          type="textarea"
          :rows="10"
          :placeholder="placeholder"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="submitting" @click="submit">
          <el-icon style="margin-right: 4px"><Upload /></el-icon>导入
        </el-button>
        <el-button @click="form.text = ''">清空</el-button>
      </el-form-item>
    </el-form>

    <el-alert
      type="info"
      show-icon
      :closable="false"
      title="提示"
      description="重复链接会自动跳过；导入成功后系统会在后台立即检测链接可用性，可到「链接健康」查看进度。"
    />

    <template v-if="result">
      <div style="display: flex; gap: 20px; margin: 18px 0 10px; font-size: 14px">
        <span>成功：<b style="color: #059669">{{ result.created }}</b></span>
        <span>跳过（重复）：<b style="color: #d97706">{{ result.skipped }}</b></span>
        <span>失败：<b style="color: #dc2626">{{ result.failed }}</b></span>
      </div>
      <el-table :data="result.results" size="small" max-height="360">
        <el-table-column prop="line" label="行号" width="70" />
        <el-table-column prop="title" label="标题" min-width="180" show-overflow-tooltip />
        <el-table-column prop="url" label="链接" min-width="220" show-overflow-tooltip />
        <el-table-column label="结果" width="180">
          <template #default="{ row }">
            <el-tag v-if="!row.error" type="success" effect="light" round>已导入</el-tag>
            <el-tag v-else type="danger" effect="light" round>{{ row.error }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </template>
  </div>
</template>
