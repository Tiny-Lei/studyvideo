<script setup>
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'

defineProps({
  content: { type: String, default: '' }
})
</script>

<template>
  <div class="md-preview">
    <MdPreview :model-value="content" language="zh-CN" preview-theme="github" />
  </div>
</template>

<style scoped>
.md-preview :deep(.md-editor-preview-wrapper) {
  padding: 0;
}

.md-preview :deep(.md-editor-preview) {
  font-size: 14.5px;
  line-height: 1.8;
  word-break: break-word;
}

.md-preview :deep(.md-editor-preview img) {
  max-width: 100%;
}

.md-preview :deep(.md-editor-preview pre) {
  background: #f6f8fa;
  border-radius: 8px;
}

.md-preview :deep(.md-editor-preview code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
</style>
