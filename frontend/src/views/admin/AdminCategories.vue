<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import { formatDate } from '../../utils'

const props = defineProps({
  active: { type: String, default: '' }
})

const loading = ref(false)
const topics = ref([])
const categories = ref([])
const filterTopicID = ref('')

const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const form = ref({ topic_id: null, name: '', description: '', sort: 0 })

const filtered = computed(() =>
  filterTopicID.value ? categories.value.filter((c) => c.topic_id === filterTopicID.value) : categories.value
)

async function load() {
  loading.value = true
  try {
    const [t, c] = await Promise.all([api.adminTopics(), api.adminCategories()])
    topics.value = t.topics || []
    categories.value = c.categories || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  if (!topics.value.length) {
    ElMessage.warning('请先到「主题管理」创建主题')
    return
  }
  editing.value = null
  form.value = {
    topic_id: filterTopicID.value || topics.value[0].id,
    name: '',
    description: '',
    sort: 0
  }
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = row
  form.value = { topic_id: row.topic_id, name: row.name, description: row.description || '', sort: row.sort }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.topic_id) return ElMessage.warning('请选择所属主题')
  if (!form.value.name.trim()) return ElMessage.warning('请填写分类名称')
  saving.value = true
  try {
    if (editing.value) {
      await api.updateCategory(editing.value.id, form.value)
      ElMessage.success('已保存')
    } else {
      await api.createCategory(form.value)
      ElMessage.success('分类已创建')
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
  let mode = 'keep'
  try {
    const action = await ElMessageBox.confirm(
      `删除分类「${row.name}」：其中 ${row.video_count} 个视频会移动到该主题的「未分类」，视频与资料文件都会保留。`,
      '删除分类',
      {
        type: 'warning',
        confirmButtonText: '保留视频并删除',
        cancelButtonText: '连同视频一起删除',
        distinguishCancelAndClose: true
      }
    )
    void action
  } catch (action) {
    if (action === 'close') return
    mode = 'delete'
    try {
      await ElMessageBox.confirm(
        `将连同分类下的 ${row.video_count} 个视频及其配套资料文件一起删除，且不可恢复，确定继续？`,
        '危险操作',
        { type: 'error', confirmButtonText: '全部删除', confirmButtonClass: 'el-button--danger' }
      )
    } catch {
      return
    }
  }
  try {
    const res = await api.deleteCategory(row.id, mode)
    ElMessage.success(mode === 'delete' ? `已删除分类及 ${res.removed_videos} 个视频` : '已删除分类，视频已移到「未分类」')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)

watch(
  () => props.active,
  (v) => {
    if (v === 'categories') load()
  }
)
</script>

<template>
  <div class="admin-card">
    <div class="admin-toolbar">
      <el-select v-model="filterTopicID" placeholder="全部主题" clearable style="width: 200px">
        <el-option v-for="t in topics" :key="t.id" :label="t.name" :value="t.id" />
      </el-select>
      <el-button type="primary" @click="openCreate">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建分类
      </el-button>
      <el-button @click="load">
        <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
      </el-button>
      <span style="color: var(--sv-muted); font-size: 13px; margin-left: auto">
        用户进入主题后先看到分类，进入分类才是视频列表；排序值越大越靠前
      </span>
    </div>

    <el-table v-loading="loading" :data="filtered" stripe>
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="topic_name" label="所属主题" width="160" show-overflow-tooltip />
      <el-table-column prop="name" label="分类名称" min-width="180" />
      <el-table-column prop="description" label="简介" min-width="200" show-overflow-tooltip />
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column label="视频数" width="110">
        <template #default="{ row }">
          <el-tag effect="plain" round>{{ row.video_count }}</el-tag>
          <el-tag v-if="row.fail_count" type="danger" effect="light" round style="margin-left: 4px">
            {{ row.fail_count }} 失效
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="120">
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑分类' : '新建分类'" width="min(520px, 92vw)">
      <el-form label-width="auto" label-position="right">
        <el-form-item label="所属主题" required>
          <el-select v-model="form.topic_id" placeholder="请选择主题" style="width: 100%">
            <el-option v-for="t in topics" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="分类名称" required>
          <el-input v-model="form.name" maxlength="120" placeholder="例如：2024 年 3 月真题、一级选择题" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="500" placeholder="一句话介绍该分类" />
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
