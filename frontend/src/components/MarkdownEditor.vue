<script setup>
import { MdEditor } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '支持 Markdown 语法，例如 **加粗**、- 列表、## 小标题' },
  height: { type: String, default: '280px' }
})

const emit = defineEmits(['update:modelValue'])
</script>

<template>
  <MdEditor
    :model-value="props.modelValue"
    language="zh-CN"
    :toolbars-exclude="['github', 'save', 'htmlPreview', 'catalog', 'previewOnly', 'task', 'image', 'video', 'audio', 'mermaid', 'katex']"
    :placeholder="placeholder"
    :style="{ height }"
    preview-theme="github"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>

<style scoped>
:deep(.md-editor) {
  border-radius: 10px;
}
</style>
