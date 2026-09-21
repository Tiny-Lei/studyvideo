<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import { formatDateTime, formatSize, tagList } from '../../utils'

const props = defineProps({
  active: { type: String, default: '' }
})

const loading = ref(false)
const materials = ref([])
const total = ref(0)
const categories = ref([])
const filters = ref({ category_id: '', q: '', ext: '', page: 1, page_size: 20 })

const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const file = ref(null)
const uploadRef = ref(null)
const form = ref({ category_id: null, group_name: '', title: '', tags: '', description: '', sort: 0 })

const currentCategoryTags = computed(() => {
  const c = categories.value.find((x) => x.id === form.value.category_id)
  return c && c.tags ? c.tags.split(',').map((t) => t.trim()).filter(Boolean) : []
})

const selectedTags = computed(() =>
  String(form.value.tags || '')
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
)

function toggleTag(tag) {
  const list = selectedTags.value
  const idx = list.indexOf(tag)
  if (idx >= 0) list.splice(idx, 1)
  else list.push(tag)
  form.value.tags = list.join(',')
}

const extOptions = [
  { value: 'pdf', label: 'PDF' },
  { value: 'md', label: 'Markdown' },
  { value: 'docx', label: 'Word' },
  { value: 'pptx', label: 'PPT' },
  { value: 'xlsx', label: 'Excel' },
  { value: 'zip', label: '压缩包' }
]

async function loadCategories() {
  try {
    const res = await api.adminMaterialCategories()
    categories.value = res.categories || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function load() {
  loading.value = true
  try {
    const res = await api.adminMaterials(filters.value)
    materials.value = res.materials || []
    total.value = res.total || 0
    if (!res.materials?.length && total.value > 0 && filters.value.page > 1) {
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
  filters.value = { category_id: '', q: '', ext: '', page: 1, page_size: 20 }
  load()
}

function resetForm() {
  editing.value = null
  file.value = null
  form.value = { category_id: categories.value[0]?.id || null, group_name: '', title: '', tags: '', description: '', sort: 0 }
  if (uploadRef.value) uploadRef.value.clearFiles()
}

async function openCreate() {
  await loadCategories()
  if (!categories.value.length) {
    ElMessage.warning('请先到「资料分类」创建分类')
    return
  }
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  await loadCategories()
  editing.value = row
  file.value = null
  form.value = {
    category_id: row.category_id,
    group_name: row.group_name || '',
    title: row.title,
    tags: row.tags || '',
    description: row.description || '',
    sort: row.sort
  }
  if (uploadRef.value) uploadRef.value.clearFiles()
  dialogVisible.value = true
}

function onFileChange(uploadFile) {
  file.value = uploadFile.raw || null
  if (!form.value.title.trim() && file.value) {
    form.value.title = file.value.name.replace(/\.[^.]+$/, '')
  }
}

function beforeUpload(uploadFile) {
  const ok = /\.(pdf|md|markdown|doc|docx|ppt|pptx|xls|xlsx|txt|zip)$/i.test(uploadFile.name)
  if (!ok) {
    ElMessage.error('支持 PDF / Markdown / Office 文档 / TXT / ZIP')
    return false
  }
  return true
}

async function save() {
  if (!editing.value && !file.value) return ElMessage.warning('请选择要上传的文件')
  if (!form.value.category_id) return ElMessage.warning('请选择资料分类')
  if (!form.value.title.trim()) return ElMessage.warning('请填写资料标题')

  const fd = new FormData()
  fd.append('category_id', String(form.value.category_id))
  fd.append('group_name', form.value.group_name || '')
  fd.append('title', form.value.title.trim())
  fd.append('tags', form.value.tags || '')
  fd.append('description', form.value.description || '')
  fd.append('sort', String(form.value.sort ?? 0))
  if (file.value) fd.append('file', file.value)

  saving.value = true
  try {
    if (editing.value) {
      await api.updateMaterial(editing.value.id, fd)
      ElMessage.success(file.value ? '已替换文件并保存' : '已保存')
    } else {
      await api.uploadMaterial(fd)
      ElMessage.success('上传成功')
    }
    dialogVisible.value = false
    resetForm()
    await Promise.all([load(), loadCategories()])
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除资料「${row.title}」？文件将从服务器一并删除。`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.deleteMaterial(row.id)
    ElMessage.success('已删除')
    await Promise.all([load(), loadCategories()])
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function preview(row) {
  window.open(api.materialFileUrl(row.id), '_blank', 'noopener')
}

function download(row) {
  window.location.href = api.materialDownloadUrl(row.id)
}

onMounted(async () => {
  await loadCategories()
  await load()
})

watch(
  () => props.active,
  async (v) => {
    if (v === 'materials') {
      await loadCategories()
      await load()
    }
  }
)
</script>

<template>
  <div class="admin-card">
    <div class="admin-toolbar">
      <el-select v-model="filters.category_id" placeholder="全部资料分类" clearable style="width: 170px" @change="filters.page = 1; load()">
        <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-select v-model="filters.ext" placeholder="全部格式" clearable style="width: 130px" @change="filters.page = 1; load()">
        <el-option v-for="o in extOptions" :key="o.value" :label="o.label" :value="o.value" />
      </el-select>
      <el-input v-model="filters.q" placeholder="搜索标题 / 标签 / 套题" clearable style="width: 220px" @keyup.enter="filters.page = 1; load()" />
      <el-button @click="filters.page = 1; load()">
        <el-icon style="margin-right: 4px"><Search /></el-icon>查询
      </el-button>
      <el-button @click="resetFilters">重置</el-button>
      <div style="margin-left: auto">
        <el-button type="primary" @click="openCreate">
          <el-icon style="margin-right: 4px"><Upload /></el-icon>上传资料
        </el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="materials" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column label="标题 / 备注" min-width="220">
        <template #default="{ row }">
          <div style="font-weight: 600">{{ row.title }}</div>
          <div v-if="row.description" style="color: var(--sv-muted); font-size: 12px; margin-top: 2px">
            {{ row.description }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="分类 / 套题" width="190">
        <template #default="{ row }">
          <div>{{ row.category_name }}</div>
          <div style="color: var(--sv-muted); font-size: 12px">{{ row.group_name || '—' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="标签" width="140">
        <template #default="{ row }">
          <el-tag v-for="tag in tagList(row.tags)" :key="tag" size="small" effect="plain" round style="margin: 2px">
            {{ tag }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="文件" min-width="170">
        <template #default="{ row }">
          <div class="url-cell" :title="row.file_name">{{ row.file_name }}</div>
          <div style="color: var(--sv-muted); font-size: 12px">
            {{ (row.file_ext || '').toUpperCase() }} · {{ formatSize(row.file_size) }}
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="70" />
      <el-table-column label="更新时间" width="150">
        <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="preview(row)">预览</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" @click="download(row)">下载</el-button>
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

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑资料' : '上传资料'" width="min(680px, 94vw)">
      <el-form label-width="auto" label-position="right">
        <el-form-item :label="editing ? '替换文件' : '选择文件'" :required="!editing">
          <el-upload
            ref="uploadRef"
            class="pdf-upload"
            drag
            :auto-upload="false"
            :limit="1"
            accept=".pdf,.md,.markdown,.doc,.docx,.ppt,.pptx,.xls,.xlsx,.txt,.zip"
            :before-upload="beforeUpload"
            :on-change="onFileChange"
            :on-remove="() => (file = null)"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              {{ editing ? '拖拽或点击选择新文件以替换（可留空只改信息）' : '将文件拖到此处，或点击选择' }}
            </div>
            <template #tip>
              <div class="el-upload__tip">
                支持 PDF、Markdown、Word、PPT、Excel、TXT、ZIP，单文件不超过 {{ Math.floor(100) }} MB
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="资料分类" required>
          <el-select v-model="form.category_id" placeholder="请选择分类" style="width: 100%">
            <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="套题/分组">
          <el-input v-model="form.group_name" maxlength="160" placeholder="可选。同套资料填相同名称，例如：2024-03 一级" />
        </el-form-item>
        <el-form-item label="标题" required>
          <el-input v-model="form.title" maxlength="255" placeholder="例如：2024-03 一级 试卷" />
        </el-form-item>
        <el-form-item label="标签">
          <div v-if="currentCategoryTags.length" class="tag-picker">
            <el-tag
              v-for="t in currentCategoryTags"
              :key="t"
              class="tag-chip"
              :effect="selectedTags.includes(t) ? 'dark' : 'plain'"
              round
              @click="toggleTag(t)"
            >
              {{ t }}
            </el-tag>
          </div>
          <el-input v-model="form.tags" placeholder="多个标签用逗号分隔，例如：GESP1级,试卷,解析" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="2000" placeholder="资料说明、适用范围等" />
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
  </div>
</template>

<style scoped>
.tag-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
  width: 100%;
}

.tag-chip {
  cursor: pointer;
  user-select: none;
}
</style>
