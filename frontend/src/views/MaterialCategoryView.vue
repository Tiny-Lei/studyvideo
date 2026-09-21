<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'
import { gradientFor } from '../utils'
import MaterialRow from '../components/MaterialRow.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const category = ref(null)
const materials = ref([])
const groups = ref([])
const activeGroup = ref('')
const kw = ref('')

const filtered = computed(() => {
  if (!kw.value.trim()) return materials.value
  const q = kw.value.trim().toLowerCase()
  return materials.value.filter(
    (m) =>
      m.title.toLowerCase().includes(q) ||
      (m.tags || '').toLowerCase().includes(q) ||
      (m.description || '').toLowerCase().includes(q)
  )
})

// 有分组时按套题聚合展示：一个分组一张卡片，内含试卷/解析等多份资料
const grouped = computed(() => {
  const list = filtered.value
  if (!groups.value.length) return null
  const map = new Map()
  for (const g of groups.value) map.set(g, [])
  const ungrouped = []
  for (const m of list) {
    if (m.group_name) {
      if (!map.has(m.group_name)) map.set(m.group_name, [])
      map.get(m.group_name).push(m)
    } else {
      ungrouped.push(m)
    }
  }
  const sections = [...map.entries()]
    .filter(([, items]) => items.length)
    .map(([name, items]) => ({ name, items }))
  return { sections, ungrouped }
})

async function load() {
  loading.value = true
  try {
    const res = await api.materialCategory(route.params.id)
    category.value = res.category
    materials.value = res.materials || []
    groups.value = res.groups || []
    activeGroup.value = ''
  } catch (e) {
    ElMessage.error(e.message)
    if (e.status === 404) router.replace('/materials')
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div class="container" style="padding-top: 24px">
    <el-skeleton v-if="loading" :rows="6" animated />

    <template v-else-if="category">
      <el-breadcrumb class="watch-breadcrumb" separator="/">
        <el-breadcrumb-item :to="{ path: '/materials' }">资料</el-breadcrumb-item>
        <el-breadcrumb-item>{{ category.name }}</el-breadcrumb-item>
      </el-breadcrumb>

      <div class="topic-head">
        <div class="topic-icon" :style="{ background: gradientFor(category.id + 5) }">
          <el-icon><FolderOpened /></el-icon>
        </div>
        <div style="min-width: 0">
          <h2 style="margin: 0 0 6px">{{ category.name }}</h2>
          <div style="color: var(--sv-muted); font-size: 14px; line-height: 1.6">
            {{ category.description || '该分类下的学习资料' }}
          </div>
        </div>
        <el-tag class="topic-count" effect="light" round>{{ materials.length }} 份资料</el-tag>
      </div>

      <div class="topic-toolbar">
        <el-input
          v-model="kw"
          placeholder="在本分类内筛选…"
          clearable
          style="max-width: 260px"
          size="small"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <span v-if="groups.length" class="toolbar-label">共 {{ groups.length }} 套</span>
      </div>

      <!-- 有分组：按套题展示，套题内并列「试卷 / 解析卷」 -->
      <template v-if="grouped && grouped.sections.length">
        <div v-for="sec in grouped.sections" :key="sec.name" class="group-card">
          <div class="group-head">
            <el-icon class="group-icon"><Collection /></el-icon>
            <span class="group-name">{{ sec.name }}</span>
            <span class="group-count">{{ sec.items.length }} 份</span>
          </div>
          <div class="video-list">
            <MaterialRow v-for="m in sec.items" :key="m.id" :material="m" />
          </div>
        </div>
        <template v-if="grouped.ungrouped.length">
          <div class="section-head" style="margin-top: 24px">
            <div class="section-title"><span class="bar"></span>其它资料</div>
          </div>
          <div class="video-list">
            <MaterialRow v-for="m in grouped.ungrouped" :key="m.id" :material="m" />
          </div>
        </template>
      </template>

      <!-- 无分组：普通列表 -->
      <div v-else-if="filtered.length" class="video-list">
        <MaterialRow v-for="m in filtered" :key="m.id" :material="m" />
      </div>

      <div v-else class="empty-box">
        <div class="icon"><el-icon><FolderOpened /></el-icon></div>
        <p>{{ kw ? '没有匹配的资料' : '该分类下暂时没有资料' }}</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.topic-count {
  margin-left: auto;
  flex-shrink: 0;
}

.topic-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.toolbar-label {
  color: var(--sv-muted);
  font-size: 13px;
}

.group-card {
  background: #fbfdfd;
  border: 1px solid var(--sv-line);
  border-radius: 16px;
  padding: 16px;
  margin-bottom: 16px;
}

.group-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.group-icon {
  color: #0d9488;
  font-size: 18px;
}

.group-name {
  font-weight: 700;
  font-size: 16px;
}

.group-count {
  color: var(--sv-muted);
  font-size: 12.5px;
}

.video-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

@media (max-width: 720px) {
  .topic-head {
    flex-wrap: wrap;
  }
  .topic-count {
    margin-left: 0;
  }
}
</style>
