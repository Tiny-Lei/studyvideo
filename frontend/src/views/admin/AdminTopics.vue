<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import { formatDate } from '../../utils'

const props = defineProps({
  active: { type: String, default: '' }
})

const loading = ref(false)
const topics = ref([])
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const form = ref({ name: '', description: '', sort: 0 })

async function load() {
  loading.value = true
  try {
    const res = await api.adminTopics()
    topics.value = res.topics || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = { name: '', description: '', sort: 0 }
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = row
  form.value = { name: row.name, description: row.description || '', sort: row.sort }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请填写主题名称')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await api.updateTopic(editing.value.id, form.value)
      ElMessage.success('已保存')
    } else {
      await api.createTopic(form.value)
      ElMessage.success('主题已创建')
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
    await ElMessageBox.confirm(
      `删除主题「${row.name}」会同时删除其下 ${row.video_count} 个视频记录（视频文件不受影响），确定继续？`,
      '危险操作',
      { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' }
    )
  } catch {
    return
  }
  try {
    await api.deleteTopic(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)

// Tab 切回「主题管理」时刷新，保证数据最新
watch(
  () => props.active,
  (v) => {
    if (v === 'topics') load()
  }
)
</script>

<template>
  <div class="admin-card">
    <div class="admin-toolbar">
      <el-button type="primary" @click="openCreate">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建主题
      </el-button>
      <el-button @click="load">
        <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
      </el-button>
      <span style="color: var(--sv-muted); font-size: 13px; margin-left: auto">
        排序值越大越靠前；例如：GESP 真题讲解、CSP 真题讲解、GESP 考点讲解
      </span>
    </div>

    <el-table v-loading="loading" :data="topics" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="主题名称" min-width="180" />
      <el-table-column prop="description" label="简介" min-width="220" show-overflow-tooltip />
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column label="视频数" width="100">
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

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑主题' : '新建主题'" width="min(520px, 92vw)">
      <el-form label-width="auto" label-position="right">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" maxlength="120" placeholder="例如：GESP 真题讲解" />
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.description" type="textarea" :rows="3" maxlength="500" placeholder="一句话介绍该主题" />
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
