<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import { formatDate, formatDateTime, formatSize, plainText, tagList } from '../../utils'
import MarkdownEditor from '../../components/MarkdownEditor.vue'

const props = defineProps({
  active: { type: String, default: '' }
})

const maxPdfMB = 50

const loading = ref(false)
const videos = ref([])
const total = ref(0)
const topics = ref([])
const categories = ref([])
const filters = ref({ topic_id: '', category_id: '', status: '', q: '', page: 1, page_size: 20 })

const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const form = ref({ topic_id: null, category_id: null, title: '', url: '', tags: '', sort: 0, description: '' })

const pdfDialogVisible = ref(false)
const pdfVideo = ref(null)
const pdfs = ref([])
const pdfSaving = ref(false)
const pdfEditing = ref(null)
const pdfForm = ref({ title: '', sort: 0 })
const pdfFile = ref(null)
const pdfUploadRef = ref(null)

const statusMap = {
  ok: { label: '有效', type: 'success' },
  fail: { label: '失效', type: 'danger' },
  unchecked: { label: '未检测', type: 'info' }
}

async function loadTopics() {
  try {
    const res = await api.adminTopics()
    topics.value = res.topics || []
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

function onFormTopicChange() {
  form.value.category_id = null
  loadCategories(form.value.topic_id)
}

function onFilterTopicChange() {
  filters.value.category_id = ''
  loadCategories(filters.value.topic_id)
  filters.value.page = 1
  load()
}

async function load() {
  loading.value = true
  try {
    const res = await api.adminVideos(filters.value)
    videos.value = res.videos || []
    total.value = res.total || 0
    if (!res.videos?.length && total.value > 0 && filters.value.page > 1) {
      filters.value.page = 1
      await load()
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.value = { topic_id: '', category_id: '', status: '', q: '', page: 1, page_size: 20 }
  load()
}

async function openCreate() {
  await loadTopics()
  editing.value = null
  const topicId = topics.value.length ? topics.value[0].id : null
  form.value = {
    topic_id: topicId,
    category_id: null,
    title: '',
    url: '',
    tags: '',
    sort: 0,
    description: ''
  }
  await loadCategories(topicId)
  if (categories.value.length) form.value.category_id = categories.value[0].id
  dialogVisible.value = true
}

async function openEdit(row) {
  await loadTopics()
  await loadCategories(row.topic_id)
  editing.value = row
  form.value = {
    topic_id: row.topic_id,
    category_id: row.category_id,
    title: row.title,
    url: row.url,
    tags: row.tags,
    sort: row.sort,
    description: row.description || ''
  }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.topic_id) return ElMessage.warning('请选择所属主题')
  if (!form.value.category_id) return ElMessage.warning('请选择所属分类')
  if (!form.value.title.trim()) return ElMessage.warning('请填写标题')
  if (!/^https?:\/\//i.test(form.value.url.trim())) return ElMessage.warning('请粘贴以 http(s):// 开头的 MP4 直链')
  saving.value = true
  try {
    if (editing.value) {
      await api.updateVideo(editing.value.id, form.value)
      ElMessage.success('已保存，链接变化后会自动重新检测')
    } else {
      await api.createVideo(form.value)
      ElMessage.success('已添加，正在后台检测链接')
    }
    dialogVisible.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除视频「${row.title}」？仅删除记录，不影响源文件。`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.deleteVideo(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function check(row) {
  row._checking = true
  try {
    const res = await api.checkVideo(row.id)
    const r = res.result
    if (r.status === 'ok') {
      ElMessage.success(`链接正常（HTTP ${r.code}，${r.latency_ms}ms）`)
    } else {
      ElMessage.error(`链接异常：${r.err_msg}`)
    }
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    row._checking = false
  }
}

async function copyUrl(url) {
  try {
    await navigator.clipboard.writeText(url)
    ElMessage.success('链接已复制')
  } catch {
    const ta = document.createElement('textarea')
    ta.value = url
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    ElMessage.success('链接已复制')
  }
}

// ---------------- 配套资料（本站存储） ----------------

async function openPdf(row) {
  pdfVideo.value = row
  resetPdfForm()
  pdfDialogVisible.value = true
  await loadPdfs(row)
}

async function loadPdfs(row) {
  try {
    const res = await api.adminPdfs(row.id)
    pdfs.value = res.pdfs || []
    row._pdfCount = pdfs.value.length
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function resetPdfForm() {
  pdfEditing.value = null
  pdfForm.value = { title: '', sort: 0 }
  pdfFile.value = null
  if (pdfUploadRef.value) pdfUploadRef.value.clearFiles()
}

function editPdf(row) {
  pdfEditing.value = row
  pdfForm.value = { title: row.title, sort: row.sort }
  pdfFile.value = null
  if (pdfUploadRef.value) pdfUploadRef.value.clearFiles()
}

function onPdfFileChange(uploadFile) {
  pdfFile.value = uploadFile.raw || null
  if (!pdfForm.value.title.trim() && pdfFile.value) {
    pdfForm.value.title = pdfFile.value.name.replace(/\.[^.]+$/, '')
  }
}

function beforePdfUpload(file) {
  const isPdf = file.type === 'application/pdf' || /\.pdf$/i.test(file.name)
  if (!isPdf) {
    ElMessage.error('请选择 PDF 文件')
    return false
  }
  return true
}

async function savePdf() {
  if (!pdfEditing.value && !pdfFile.value) return ElMessage.warning('请选择要上传的 PDF 文件')
  if (!pdfForm.value.title.trim()) return ElMessage.warning('请填写资料名称')
  const fd = new FormData()
  fd.append('title', pdfForm.value.title.trim())
  fd.append('sort', String(pdfForm.value.sort ?? 0))
  if (pdfFile.value) fd.append('file', pdfFile.value)

  pdfSaving.value = true
  try {
    if (pdfEditing.value) {
      await api.updatePdf(pdfEditing.value.id, fd)
      ElMessage.success(pdfFile.value ? '已替换文件并保存' : '已保存')
    } else {
      await api.uploadPdf(pdfVideo.value.id, fd)
      ElMessage.success('上传成功')
    }
    resetPdfForm()
    await loadPdfs(pdfVideo.value)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    pdfSaving.value = false
  }
}

async function removePdf(row) {
  try {
    await ElMessageBox.confirm(`确定删除资料「${row.title}」？文件将从服务器一并删除。`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.deletePdf(row.id)
    ElMessage.success('已删除')
    await loadPdfs(pdfVideo.value)
  } catch (e) {
    ElMessage.error(e.message)
  }
}
onMounted(async () => {
  await loadTopics()
  await load()
})

// Tab 切回「视频管理」时刷新主题与列表，避免显示旧数据
watch(
  () => props.active,
  async (v) => {
    if (v === 'videos') {
      await loadTopics()
      await load()
    }
  }
)
</script>

<template>
  <div class="admin-card">
    <div class="admin-toolbar">
      <el-select v-model="filters.topic_id" placeholder="全部主题" clearable style="width: 180px" @change="onFilterTopicChange">
        <el-option v-for="t in topics" :key="t.id" :label="t.name" :value="t.id" />
      </el-select>
      <el-select
        v-model="filters.category_id"
        placeholder="全部分类"
        clearable
        :disabled="!filters.topic_id"
        style="width: 160px"
        @change="filters.page = 1; load()"
      >
        <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-select v-model="filters.status" placeholder="全部状态" clearable style="width: 130px" @change="filters.page = 1; load()">
        <el-option label="有效" value="ok" />
        <el-option label="失效" value="fail" />
        <el-option label="未检测" value="unchecked" />
      </el-select>
      <el-input v-model="filters.q" placeholder="搜索标题 / 标签 / 链接" clearable style="width: 220px" @keyup.enter="filters.page = 1; load()" />
      <el-button @click="filters.page = 1; load()">
        <el-icon style="margin-right: 4px"><Search /></el-icon>查询
      </el-button>
      <el-button @click="resetFilters">重置</el-button>
      <div style="margin-left: auto; display: flex; gap: 10px">
        <el-button type="primary" @click="openCreate">
          <el-icon style="margin-right: 4px"><Plus /></el-icon>新增视频
        </el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="videos" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column label="标题 / 备注" min-width="220">
        <template #default="{ row }">
          <div style="font-weight: 600">{{ row.title }}</div>
          <div v-if="plainText(row.description)" style="color: var(--sv-muted); font-size: 12px; margin-top: 2px">
            {{ plainText(row.description) }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="主题 / 分类" width="200">
        <template #default="{ row }">
          <div>{{ row.topic_name }}</div>
          <div style="color: var(--sv-muted); font-size: 12px">{{ row.category_name || '—' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="标签" width="150">
        <template #default="{ row }">
          <el-tag v-for="tag in tagList(row.tags)" :key="tag" size="small" effect="plain" round style="margin: 2px">
            {{ tag }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="链接" min-width="180">
        <template #default="{ row }">
          <el-tooltip :content="row.url" placement="top">
            <span class="url-cell" style="cursor: pointer" @click="copyUrl(row.url)">{{ row.url }}</span>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="70" />
      <el-table-column label="状态" width="140">
        <template #default="{ row }">
          <el-tooltip
            :content="row.status === 'ok' ? `HTTP ${row.status_code} · ${row.latency_ms}ms · ${formatDateTime(row.checked_at)}` : row.error_msg || '尚未检测'"
            placement="top"
          >
            <el-tag :type="statusMap[row.status]?.type" effect="light" round>
              {{ statusMap[row.status]?.label || row.status }}
            </el-tag>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" @click="openPdf(row)">
            资料<span v-if="row._pdfCount" style="color: var(--sv-muted)">({{ row._pdfCount }})</span>
          </el-button>
          <el-button link type="primary" :loading="row._checking" @click="check(row)">检测</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="display: flex; justify-content: flex-end; margin-top: 14px">
      <el-pagination
        v-model:current-page="filters.page"
        v-model:page-size="filters.page_size"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="filters.page = 1; load()"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑视频' : '新增视频'" width="min(640px, 94vw)">
      <el-form label-width="auto" label-position="right">
        <el-form-item label="所属主题" required>
          <el-select v-model="form.topic_id" placeholder="请选择主题" style="width: 100%" @change="onFormTopicChange">
            <el-option v-for="t in topics" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
          <el-alert
            v-if="!topics.length"
            style="width: 100%; margin-top: 8px"
            type="warning"
            show-icon
            :closable="false"
            title="还没有主题，请先到「主题管理」创建主题，再回来添加视频"
          />
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
        <el-form-item label="标题" required>
          <el-input v-model="form.title" maxlength="255" placeholder="例如：GESP 一级 2024 年 3 月 真题第 1 题讲解" />
        </el-form-item>
        <el-form-item label="MP4 直链" required>
          <el-input v-model="form.url" placeholder="粘贴视频直链，例如 https://static.hetaoimg.com/crmFiles/xxxx.mp4" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="form.tags" placeholder="多个标签用逗号分隔，例如：GESP,一级,选择题" />
        </el-form-item>
        <el-form-item label="备注/简介">
          <MarkdownEditor v-model="form.description" placeholder="支持 Markdown：**加粗**、- 列表、## 小标题、`代码`、[链接](https://...)" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="-9999" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="pdfDialogVisible" width="min(720px, 94vw)" :title="`配套资料 · ${pdfVideo?.title || ''}`">
      <el-form label-width="auto" label-position="right" @submit.prevent>
        <el-form-item :label="pdfEditing ? '替换文件' : '选择文件'" :required="!pdfEditing">
          <el-upload
            ref="pdfUploadRef"
            class="pdf-upload"
            drag
            :auto-upload="false"
            :limit="1"
            accept="application/pdf,.pdf"
            :before-upload="beforePdfUpload"
            :on-change="onPdfFileChange"
            :on-remove="() => (pdfFile = null)"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              {{ pdfEditing ? '拖拽或点击选择新的 PDF 以替换原文件（可留空只改名称）' : '将 PDF 拖到此处，或点击选择文件' }}
            </div>
            <template #tip>
              <div class="el-upload__tip">
                支持 PDF 格式，单个文件不超过 {{ maxPdfMB }} MB；文件保存在本站服务器，用户可直接在线预览与下载。
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="资料名称" required>
          <el-input v-model="pdfForm.title" maxlength="255" placeholder="例如：2024 年 3 月 GESP 一级真题" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="pdfForm.sort" :min="-9999" :max="9999" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="pdfSaving" @click="savePdf">
            <el-icon style="margin-right: 4px"><Plus /></el-icon>{{ pdfEditing ? '保存修改' : '上传资料' }}
          </el-button>
          <el-button v-if="pdfEditing" @click="resetPdfForm">取消编辑</el-button>
        </el-form-item>
      </el-form>

      <el-table :data="pdfs" size="small">
        <el-table-column prop="title" label="资料名称" min-width="180" show-overflow-tooltip />
        <el-table-column label="文件" min-width="160">
          <template #default="{ row }">
            <div style="font-size: 12px; color: var(--sv-muted)">{{ row.file_name || '（旧记录，文件缺失）' }}</div>
            <div v-if="row.file_size" style="font-size: 12px; color: var(--sv-muted)">{{ formatSize(row.file_size) }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="70" />
        <el-table-column label="操作" width="190" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="previewPdf(row)">预览</el-button>
            <el-button link type="primary" @click="editPdf(row)">编辑</el-button>
            <el-button link type="danger" @click="removePdf(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!pdfs.length" style="text-align: center; color: var(--sv-muted); padding: 18px">
        该视频还没有配套资料，可在上方上传
      </div>
      <div style="color: var(--sv-muted); font-size: 12.5px; margin-top: 12px">
        文件由本站服务器存储管理，删除视频或资料时会同步清理磁盘文件。
      </div>
    </el-dialog>
  </div>
</template>
