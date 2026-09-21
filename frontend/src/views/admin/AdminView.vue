<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import AdminOverview from './AdminOverview.vue'
import AdminTopics from './AdminTopics.vue'
import AdminCategories from './AdminCategories.vue'
import AdminVideos from './AdminVideos.vue'
import AdminMaterialCategories from './AdminMaterialCategories.vue'
import AdminMaterials from './AdminMaterials.vue'
import AdminBatch from './AdminBatch.vue'
import AdminHealth from './AdminHealth.vue'
import AdminRisk from './AdminRisk.vue'

const route = useRoute()
const router = useRouter()
const tab = ref(String(route.query.tab || 'overview'))

function onTabChange(name) {
  router.replace({ query: { ...route.query, tab: name } })
}

async function logout() {
  try {
    await ElMessageBox.confirm('确定退出登录吗？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await api.logout()
  ElMessage.success('已退出登录')
  router.replace('/admin/login')
}

onMounted(() => {
  if (!['overview', 'topics', 'categories', 'videos', 'batch', 'material-categories', 'materials', 'health', 'risk'].includes(tab.value)) {
    tab.value = 'overview'
  }
})
</script>

<template>
  <div class="container admin-page">
    <div class="admin-head">
      <div>
        <h2>管理后台</h2>
        <div style="color: var(--sv-muted); font-size: 13px; margin-top: 4px">
          录入链接、维护主题、检查失效链接与访问风控
        </div>
      </div>
      <div style="display: flex; gap: 10px">
        <el-button round @click="$router.push('/')">
          <el-icon style="margin-right: 4px"><View /></el-icon>查看前台
        </el-button>
        <el-button round type="danger" plain @click="logout">
          <el-icon style="margin-right: 4px"><SwitchButton /></el-icon>退出登录
        </el-button>
      </div>
    </div>

    <el-tabs v-model="tab" class="admin-tabs" @tab-change="onTabChange">
      <el-tab-pane label="概览" name="overview" lazy>
        <AdminOverview :active="tab" @switch-tab="tab = $event" />
      </el-tab-pane>
      <el-tab-pane label="主题管理" name="topics" lazy>
        <AdminTopics :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="分类管理" name="categories" lazy>
        <AdminCategories :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="视频管理" name="videos" lazy>
        <AdminVideos :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="资料分类" name="material-categories" lazy>
        <AdminMaterialCategories :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="资料管理" name="materials" lazy>
        <AdminMaterials :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="批量录入" name="batch" lazy>
        <AdminBatch :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="链接健康" name="health" lazy>
        <AdminHealth :active="tab" />
      </el-tab-pane>
      <el-tab-pane label="访问风控" name="risk" lazy>
        <AdminRisk :active="tab" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.admin-tabs :deep(.el-tabs__header) {
  margin-bottom: 20px;
}
.admin-tabs :deep(.el-tabs__item) {
  font-size: 15px;
}
</style>
