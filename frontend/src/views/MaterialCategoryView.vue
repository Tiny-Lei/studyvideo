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
const tagStats = ref([])
const activeTags = ref([])
const kw = ref('')

function toggleTag(name) {
  const list = activeTags.value
  const idx = list.indexOf(name)
  if (idx >= 0) list.splice(idx, 1)
  else list.push(name)
  activeTags.value = [...list]
}

function materialTags(m) {
  return String(m.tags || '')
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
}

// 关键词 + 标签双重筛选（多选标签为「满足任一」）
const filtered = computed(() => {
  let list = materials.value
  if (activeTags.value.length) {
    list = list.filter((m) => materialTags(m).some((t) => activeTags.value.includes(t)))
  }
  const q = kw.value.trim().toLowerCase()
  if (!q) return list
  return list.filter(
    (m) =>
      m.title.toLowerCase().includes(q) ||
      (m.tags || '').toLowerCase().includes(q) ||
      (m.description || '').toLowerCase().includes(q) ||
      (m.group_name || '').toLowerCase().includes(q)
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
    tagStats.value = res.tags || []
    activeTags.value = []
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

      <div v-if="tagStats.length" class="tag-filter">
        <span class="tag-filter-label">
          <el-icon style="margin-right: 4px"><PriceTag /></el-icon>按标签筛选
        </span>
        <el-tag
          class="tag-filter-chip"
          :effect="activeTags.length === 0 ? 'dark' : 'plain'"
          round
          @click="activeTags = []"
        >
          全部 {{ materials.length }}
        </el-tag>
        <el-tag
          v-for="t in tagStats"
          :key="t.name"
          class="tag-filter-chip"
          :class="{ 'is-empty': t.count === 0 }"
          :effect="activeTags.includes(t.name) ? 'dark' : 'plain'"
          :type="activeTags.includes(t.name) ? 'primary' : 'info'"
          round
          @click="toggleTag(t.name)"
        >
          {{ t.name }}<span class="tag-count">{{ t.count }}</span>
        </el-tag>
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
        <span v-if="activeTags.length || kw" class="toolbar-label">
          筛选出 {{ filtered.length }} 份资料
        </span>
        <span v-else-if="groups.length" class="toolbar-label">共 {{ groups.length }} 套</span>
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
        <p>{{ kw || activeTags.length ? '没有符合条件的资料，试试更换标签或关键词' : '该分类下暂时没有资料' }}</p>
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

.tag-filter {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  background: #f8fafc;
  border: 1px solid var(--sv-line);
  border-radius: 14px;
  padding: 12px 14px;
  margin-bottom: 14px;
}

.tag-filter-label {
  color: var(--sv-muted);
  font-size: 13px;
  display: inline-flex;
  align-items: center;
}

.tag-filter-chip {
  cursor: pointer;
  user-select: none;
}

.tag-filter-chip.is-empty:not(.el-tag--dark) {
  opacity: 0.55;
}

.tag-count {
  margin-left: 5px;
  opacity: 0.75;
  font-size: 11px;
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
