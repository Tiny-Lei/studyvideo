<template>
  <div class="empty-box" style="padding-top: 100px">
    <div class="icon"><el-icon><Compass /></el-icon></div>
    <p style="font-size: 16px">页面不存在或已被移除</p>
    <el-button type="primary" round @click="$router.push('/')">返回首页</el-button>
  </div>
</template>
