<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import { formatDate } from '../../utils'

const props = defineProps({
  active: { type: String, default: '' }
})

const loading = ref(false)
const categories = ref([])
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const form = ref({ name: '', description: '', sort: 0 })

async function load() {
  loading.value = true
  try {
    const res = await api.adminMaterialCategories()
    categories.value = res.categories || []
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
  if (!form.value.name.trim()) return ElMessage.warning('请填写分类名称')
  saving.value = true
  try {
    if (editing.value) {
      await api.updateMaterialCategory(editing.value.id, form.value)
      ElMessage.success('已保存')
    } else {
      await api.createMaterialCategory(form.value)
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
  if (row.material_count > 0) {
    try {
      await ElMessageBox.confirm(
        `分类「${row.name}」下还有 ${row.material_count} 份资料，删除后资料及其文件将被一并删除且不可恢复。确定继续？`,
        '危险操作',
        { type: 'error', confirmButtonText: '连同资料一起删除', confirmButtonClass: 'el-button--danger' }
      )
    } catch {
      return
    }
  } else {
    try {
      await ElMessageBox.confirm(`确定删除分类「${row.name}」？`, '提示', { type: 'warning' })
    } catch {
      return
    }
  }
  try {
    await api.deleteMaterialCategory(row.id, row.material_count > 0 ? 'delete' : '')
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
watch(
  () => props.active,
  (v) => {
    if (v === 'material-categories') load()
  }
)
</script>

<template>
  <div class="admin-card">
    <div class="admin-toolbar">
      <el-button type="primary" @click="openCreate">
        <el-icon style="margin-right: 4px"><Plus /></el-icon>新建资料分类
      </el-button>
      <el-button @click="load">
        <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
      </el-button>
      <span style="color: var(--sv-muted); font-size: 13px; margin-left: auto">
        例如：课件、试卷、GESP真题、CSP真题；排序值越大越靠前
      </span>
    </div>

    <el-table v-loading="loading" :data="categories" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="分类名称" min-width="160" />
      <el-table-column prop="description" label="简介" min-width="220" show-overflow-tooltip />
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column label="资料数" width="100">
        <template #default="{ row }">
          <el-tag effect="plain" round>{{ row.material_count }}</el-tag>
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

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑资料分类' : '新建资料分类'" width="min(520px, 92vw)">
      <el-form label-width="auto" label-position="right">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" maxlength="120" placeholder="例如：GESP真题" />
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
