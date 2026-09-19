<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../api'

const props = defineProps({
  active: { type: String, default: '' }
})

const emit = defineEmits(['switch-tab'])
const stats = ref(null)
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    const res = await api.overview()
    stats.value = res.stats
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

// Tab 切回「概览」时刷新统计
watch(
  () => props.active,
  (v) => {
    if (v === 'overview') load()
  }
)
</script>

<template>
  <div v-loading="loading">
    <div v-if="stats" class="stat-grid">
      <div class="stat-card">
        <div class="label">主题数量</div>
        <div class="value">{{ stats.topics }}</div>
      </div>
      <div class="stat-card">
        <div class="label">分类数量</div>
        <div class="value">{{ stats.categories }}</div>
      </div>
      <div class="stat-card">
        <div class="label">视频总数</div>
        <div class="value">{{ stats.videos }}</div>
      </div>
      <div class="stat-card ok">
        <div class="label">链接正常</div>
        <div class="value">{{ stats.videos - stats.fail_videos - stats.unchecked }}</div>
      </div>
      <div class="stat-card danger">
        <div class="label">失效链接</div>
        <div class="value">{{ stats.fail_videos }}</div>
      </div>
      <div class="stat-card">
        <div class="label">未检测</div>
        <div class="value">{{ stats.unchecked }}</div>
      </div>
      <div class="stat-card">
        <div class="label">今日播放请求</div>
        <div class="value">{{ stats.today_plays }}</div>
      </div>
      <div class="stat-card">
        <div class="label">今日访问请求</div>
        <div class="value">{{ stats.today_visits }}</div>
      </div>
      <div class="stat-card danger">
        <div class="label">当前封禁 IP</div>
        <div class="value">{{ stats.blocked_ips }}</div>
      </div>
    </div>

    <div class="admin-card" style="margin-top: 18px">
      <h3 style="margin-top: 0">快捷操作</h3>
      <div style="display: flex; gap: 10px; flex-wrap: wrap">
        <el-button type="primary" round @click="emit('switch-tab', 'videos')">
          <el-icon style="margin-right: 4px"><Plus /></el-icon>录入视频
        </el-button>
        <el-button round @click="emit('switch-tab', 'categories')">
          <el-icon style="margin-right: 4px"><FolderOpened /></el-icon>管理分类
        </el-button>
        <el-button round @click="emit('switch-tab', 'batch')">
          <el-icon style="margin-right: 4px"><DocumentCopy /></el-icon>批量粘贴链接
        </el-button>
        <el-button round @click="emit('switch-tab', 'health')">
          <el-icon style="margin-right: 4px"><CircleCheck /></el-icon>链接健康检查
        </el-button>
        <el-button round @click="load">
          <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新数据
        </el-button>
      </div>
      <el-alert
        v-if="stats && stats.fail_videos > 0"
        style="margin-top: 16px"
        type="error"
        show-icon
        :closable="false"
        :title="`有 ${stats.fail_videos} 个视频链接失效，请前往「链接健康」查看并处理`"
      />
    </div>
  </div>
</template>
