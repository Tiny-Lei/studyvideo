<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../api'
import { formatDateTime } from '../../utils'

const props = defineProps({
  active: { type: String, default: '' }
})

const stats = ref(null)
const progress = ref(null)
const failVideos = ref([])
const loading = ref(false)
let timer = null

async function loadStats() {
  try {
    const res = await api.overview()
    stats.value = res.stats
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadProgress() {
  try {
    const res = await api.checkStatus()
    progress.value = res.progress
    if (res.progress?.running) startPolling()
    else stopPolling()
  } catch {
    /* ignore */
  }
}

async function loadFail() {
  loading.value = true
  try {
    const res = await api.adminVideos({ status: 'fail', page: 1, page_size: 100 })
    failVideos.value = res.videos || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function checkAll() {
  try {
    const res = await api.checkAll()
    if (!res.started) {
      ElMessage.info('已有检测任务在执行中')
    } else {
      ElMessage.success('已开始检测全部链接')
    }
    progress.value = res.progress
    startPolling()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function checkOne(row) {
  row._checking = true
  try {
    const res = await api.checkVideo(row.id)
    const r = res.result
    r.status === 'ok' ? ElMessage.success(`已恢复正常（${r.latency_ms}ms）`) : ElMessage.error(`仍然失效：${r.err_msg}`)
    await Promise.all([loadFail(), loadStats()])
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    row._checking = false
  }
}

function startPolling() {
  if (timer) return
  timer = setInterval(async () => {
    await loadProgress()
    if (!progress.value?.running) {
      stopPolling()
      await Promise.all([loadFail(), loadStats()])
    }
  }, 2000)
}

function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(async () => {
  await Promise.all([loadStats(), loadFail(), loadProgress()])
})

// Tab 切回「链接健康」时刷新数据
watch(
  () => props.active,
  async (v) => {
    if (v === 'health') await Promise.all([loadStats(), loadFail(), loadProgress()])
  }
)

onBeforeUnmount(stopPolling)
</script>

<template>
  <div>
    <div v-if="stats" class="stat-grid">
      <div class="stat-card ok">
        <div class="label">链接正常</div>
        <div class="value">{{ stats.videos - stats.fail_videos - stats.unchecked }}</div>
      </div>
      <div class="stat-card danger">
        <div class="label">链接失效</div>
        <div class="value">{{ stats.fail_videos }}</div>
      </div>
      <div class="stat-card">
        <div class="label">未检测</div>
        <div class="value">{{ stats.unchecked }}</div>
      </div>
    </div>

    <div class="admin-card" style="margin-top: 16px">
      <div class="admin-toolbar" style="margin-bottom: 0">
        <el-button type="primary" :loading="progress?.running" @click="checkAll">
          <el-icon style="margin-right: 4px"><CircleCheck /></el-icon>立即全面检测
        </el-button>
        <el-button @click="loadFail">
          <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
        </el-button>
        <span style="color: var(--sv-muted); font-size: 13px; margin-left: auto">
          系统默认每 6 小时自动检测一次，失效链接会打日志并推送告警（如配置了 Webhook）
        </span>
      </div>

      <div v-if="progress && (progress.running || progress.total > 0)" style="margin-top: 16px">
        <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px">
          <span>
            {{ progress.trigger || '检查' }}
            <template v-if="progress.running">进行中…</template>
            <template v-else-if="progress.finished_at">· 完成于 {{ formatDateTime(progress.finished_at) }}</template>
          </span>
          <span>正常 {{ progress.ok }} / 失效 {{ progress.fail }} / 共 {{ progress.total }}</span>
        </div>
        <el-progress
          :percentage="progress.total ? Math.round((progress.done / progress.total) * 100) : 0"
          :status="progress.running ? undefined : progress.fail > 0 ? 'warning' : 'success'"
        />
      </div>
    </div>

    <div class="admin-card" style="margin-top: 16px">
      <h3 style="margin-top: 0">失效链接（{{ failVideos.length }}）</h3>
      <el-table v-loading="loading" :data="failVideos" stripe>
        <el-table-column prop="id" label="ID" width="64" />
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column prop="topic_name" label="主题" width="150" show-overflow-tooltip />
        <el-table-column prop="error_msg" label="失败原因" min-width="200" show-overflow-tooltip />
        <el-table-column label="检测时间" width="160">
          <template #default="{ row }">{{ formatDateTime(row.checked_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :loading="row._checking" @click="checkOne(row)">重新检测</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!loading && !failVideos.length" style="text-align: center; color: var(--sv-muted); padding: 24px">
        当前没有失效链接
      </div>
    </div>
  </div>
</template>
