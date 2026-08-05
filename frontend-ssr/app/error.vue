<script setup lang="ts">
const props = defineProps<{ error?: any }>();

const statusCode = computed(() => props.error?.statusCode || 404);
const is404 = statusCode.value === 404;

function goHome() {
  clearError({ redirect: '/' });
}
</script>

<template>
  <div class="error-page">
    <div class="error-card">
      <div class="error-code">{{ statusCode }}</div>
      <div class="error-title">{{ is404 ? '页面不存在' : '服务器开小差了' }}</div>
      <div class="error-desc">
        {{ is404
          ? '你访问的页面不存在或已被移除，请检查地址是否正确。'
          : '服务器处理请求时出现错误，请稍后重试。' }}
      </div>
      <div class="error-actions">
        <button class="error-btn error-btn-primary" @click="goHome">返回首页</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.error-page {
  min-height: 60vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
}

.error-card {
  text-align: center;
  padding: 48px 56px;
  border-radius: 16px;
  border: 1px solid rgba(62, 175, 124, 0.25);
  background: linear-gradient(135deg, rgba(62, 175, 124, 0.06), rgba(62, 175, 124, 0.02));
  max-width: 480px;
  width: 100%;
}

.error-code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 4.5em;
  font-weight: 800;
  background: linear-gradient(135deg, #3EAF7C, #2E9A68);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  line-height: 1.2;
}

.error-title {
  font-size: 1.5em;
  font-weight: 700;
  margin: 12px 0 8px;
  color: #303133;
}

html.dark .error-title {
  color: #e5eaf3;
}

.error-desc {
  color: #909399;
  margin-bottom: 28px;
  line-height: 1.6;
}

.error-btn {
  font-size: 1em;
  padding: 10px 36px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.25s ease;
  border: none;
}

.error-btn-primary {
  background: #3EAF7C;
  color: #fff;
}

.error-btn-primary:hover {
  background: #2E9A68;
  box-shadow: 0 4px 14px rgba(62, 175, 124, 0.35);
}
</style>
