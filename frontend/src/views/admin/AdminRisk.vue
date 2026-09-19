<script setup>
import { onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../api'
import { formatDateTime } from '../../utils'

const props = defineProps({
  active: { type: String, default: '' }
})

const blocked = ref([])
const usage = ref([])
const limits = ref(null)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const [b, u, c] = await Promise.all([api.blocked(), api.usage(50), api.config()])
    blocked.value = b.blocked || []
    usage.value = u.usage || []
    limits.value = c
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function unblock(row) {
  try {
    await ElMessageBox.confirm(`确定解除对 IP ${row.ip} 的限制？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.unblock(row.ip)
    ElMessage.success('已解除限制')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)

// Tab 切回「访问风控」时刷新数据
watch(
  () => props.active,
  (v) => {
    if (v === 'risk') load()
  }
)
</script>

<template>
  <div v-loading="loading">
    <div v-if="limits" class="admin-card" style="margin-bottom: 16px">
      <h3 style="margin-top: 0">当前风控策略</h3>
      <el-descriptions :column="3" border>
        <el-descriptions-item label="单日播放上限">{{ limits.daily_video_limit }} 次 / IP</el-descriptions-item>
        <el-descriptions-item label="单日总请求上限">{{ limits.daily_total_limit }} 次 / IP</el-descriptions-item>
        <el-descriptions-item label="短时播放限制">
          {{ limits.burst_window_seconds }} 秒内 {{ limits.burst_video_limit }} 次
        </el-descriptions-item>
        <el-descriptions-item label="短时总请求限制">
          {{ limits.burst_window_seconds }} 秒内 {{ limits.burst_total_limit }} 次
        </el-descriptions-item>
        <el-descriptions-item label="触发后封禁时长">{{ limits.block_minutes }} 分钟</el-descriptions-item>
        <el-descriptions-item label="链接检测间隔">{{ limits.check_interval_minutes }} 分钟</el-descriptions-item>
      </el-descriptions>
      <div style="color: var(--sv-muted); font-size: 13px; margin-top: 12px">
        说明：服务端不转发视频字节，仅统计播放页取链接的请求；单日超限将封禁至次日 00:00。可通过环境变量调整以上阈值。
      </div>
    </div>

    <div class="admin-card" style="margin-bottom: 16px">
      <div class="admin-toolbar" style="margin-bottom: 10px">
        <h3 style="margin: 0">封禁中的 IP（{{ blocked.length }}）</h3>
        <el-button style="margin-left: auto" @click="load">
          <el-icon style="margin-right: 4px"><Refresh /></el-icon>刷新
        </el-button>
      </div>
      <el-table :data="blocked" stripe>
        <el-table-column prop="ip" label="IP" width="180" />
        <el-table-column prop="reason" label="原因" min-width="240" show-overflow-tooltip />
        <el-table-column label="解封时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.blocked_until) }}</template>
        </el-table-column>
        <el-table-column label="封禁时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="unblock(row)">解封</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!blocked.length" style="text-align: center; color: var(--sv-muted); padding: 24px">
        当前没有被封禁的 IP
      </div>
    </div>

    <div class="admin-card">
      <h3 style="margin-top: 0">今日访问排行（Top 50）</h3>
      <el-table :data="usage" stripe>
        <el-table-column prop="ip" label="IP" width="180" />
        <el-table-column prop="video_hits" label="播放请求" width="120" sortable />
        <el-table-column prop="total_hits" label="总请求" width="120" sortable />
        <el-table-column label="最后活跃" min-width="170">
          <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
        </el-table-column>
      </el-table>
      <div v-if="!usage.length" style="text-align: center; color: var(--sv-muted); padding: 24px">
        今天还没有访问记录
      </div>
    </div>
  </div>
</template>
