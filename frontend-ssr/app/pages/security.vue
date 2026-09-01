<script setup lang="ts">
import { ref, computed } from 'vue'

const config = useIpchkConfig()

useHead({
  title: '网站检测 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测网站的 HTTPS、安全响应头（HSTS/CSP/X-Frame-Options 等）与 HTTP 版本（HTTP/2/HTTP/3），给出安全评分。' },
    { name: 'keywords', content: '网站检测,网站安全检测,安全响应头,HTTPS检测,HSTS,CSP,HTTP/2,HTTP/3,security headers' },
  ],
})

const tmpurl = ref('')
const loading = ref(false)
const result = ref<any>(null)
const httpResult = ref<any>(null)
const error = ref('')

function gradeColor(grade: string): string {
  if (grade === 'A+' || grade === 'A') return '#67C23A'
  if (grade === 'B') return '#409EFF'
  if (grade === 'C') return '#E6A23C'
  return '#F56C6C'
}

const gradeStyle = computed(() => ({ color: gradeColor(result.value?.grade || '') }))

async function querySecurity() {
  loading.value = true
  error.value = ''
  result.value = null
  httpResult.value = null
  const url = tmpurl.value.trim()
  if (!url) {
    error.value = '请输入网址'
    loading.value = false
    return
  }
  try {
    const [sec, hv] = await Promise.all([
      $fetch(config.apiBaseUrls[0].url + 'v1/security/' + encodeURIComponent(url)),
      $fetch(config.apiBaseUrls[0].url + 'v1/http-version/' + encodeURIComponent(url)).catch(() => null),
    ])
    result.value = sec
    httpResult.value = hv
  } catch (e: any) {
    error.value = e?.message || '请求失败，请重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="title">
    <header>
      <h1>网站检测</h1>
      <p>检测网站的 HTTPS、安全响应头（HSTS / CSP / X-Frame-Options 等）与 HTTP 版本，给出安全评分</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input v-model="tmpurl" placeholder="输入网址（如：example.com）" @keyup.enter="querySecurity()" />
      <el-button @click="querySecurity()" type="primary" :loading="loading">检测</el-button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="result" class="result-section">
      <div class="score-card">
        <div class="grade" :style="gradeStyle">{{ result.grade }}</div>
        <div class="score-detail">
          <div>评分：<b>{{ result.score }}</b> / {{ result.max_score }}</div>
          <div>
            HTTPS：
            <span :class="result.https ? 'badge-green' : 'badge-red'">{{ result.https ? '已启用' : '未启用' }}</span>
            <span class="final-url">（{{ result.final_url }}，状态码 {{ result.status_code }}）</span>
          </div>
        </div>
      </div>

      <div v-if="httpResult" class="http-card">
        <span class="http-label">HTTP 版本</span>
        <span class="proto-tag" :class="httpResult.http2 ? 'proto-ok' : 'proto-no'">HTTP/2{{ httpResult.http2 ? ' ✓' : ' ✗' }}</span>
        <span class="proto-tag" :class="httpResult.http3 ? 'proto-ok' : 'proto-no'">HTTP/3{{ httpResult.http3 ? ' ✓' : ' ✗' }}</span>
        <span v-if="httpResult.negotiated" class="negotiated">实际协商 <code>{{ httpResult.negotiated }}</code></span>
      </div>

      <table class="result-table">
        <thead>
          <tr>
            <th class="table-header">检查项</th>
            <th class="table-header">状态</th>
            <th class="table-header">值</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ch in result.checks" :key="ch.key">
            <td class="table-label">
              <div>{{ ch.name }}</div>
              <div class="check-desc">{{ ch.description }}</div>
            </td>
            <td class="table-value">
              <span class="level-tag" :class="'level-' + ch.level">
                {{ ch.level === 'good' ? '✓ 已设置' : (ch.level === 'warn' ? '⚠ 已弃用' : '✗ 缺失') }}
              </span>
              <span class="check-score" v-if="ch.max_score > 0">+{{ ch.score }}/{{ ch.max_score }}</span>
            </td>
            <td class="table-value" style="word-break: break-all;">
              <code v-if="ch.value">{{ ch.value }}</code>
              <span v-else style="color: #999;">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <blockquote>
      评分参考 securityheaders.com 标准，仅反映响应头配置情况，不代表网站整体安全水平。<br/>
      检测默认先尝试 HTTPS，失败时回退到 HTTP。
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.error-message {
  margin-top: 20px;
  padding: 15px;
  background: #fef0f0;
  color: #F56C6C;
  border-radius: 6px;
  text-align: center;
  font-size: 1.1em;
}

.score-card {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 16px 20px;
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  margin-bottom: 16px;
}
html.dark .score-card { border-color: #333; }

.grade {
  font-size: 3em;
  font-weight: 800;
  line-height: 1;
}

.score-detail {
  font-size: 1.05em;
  line-height: 1.8;
}

.final-url {
  color: #999;
  font-size: 0.85em;
  margin-left: 6px;
}

.http-card {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 16px;
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  margin-bottom: 16px;
}
html.dark .http-card { border-color: #333; }

.http-label {
  font-weight: 700;
  font-size: 0.95em;
  color: #606266;
}
html.dark .http-label { color: #a0a0a0; }

.proto-tag {
  font-weight: 600;
  padding: 3px 12px;
  border-radius: 4px;
  font-size: 0.9em;
}
.proto-ok { background: #f0f9eb; color: #67C23A; }
.proto-no { background: #fef0f0; color: #F56C6C; }
html.dark .proto-ok { background: rgba(103, 194, 58, 0.15); }
html.dark .proto-no { background: rgba(245, 108, 108, 0.15); }

.negotiated {
  color: #999;
  font-size: 0.85em;
}
.negotiated code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: #3EAF7C;
}

.check-desc {
  color: #999;
  font-size: 0.85em;
  margin-top: 2px;
}

.level-tag {
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 4px;
  display: inline-block;
  white-space: nowrap;
}
.level-good { background: #f0f9eb; color: #67C23A; }
.level-warn { background: #fdf6ec; color: #E6A23C; }
.level-missing { background: #fef0f0; color: #F56C6C; }
html.dark .level-good { background: rgba(103, 194, 58, 0.15); }
html.dark .level-warn { background: rgba(230, 162, 60, 0.15); }
html.dark .level-missing { background: rgba(245, 108, 108, 0.15); }

.check-score {
  color: #999;
  font-size: 0.85em;
  margin-left: 6px;
}

.badge-green { color: #67C23A; font-weight: 600; }
.badge-red { color: #F56C6C; font-weight: 600; }
</style>

<style>
:root {
  --el-color-primary: #3EAF7C;
}
html.dark {
  --el-color-primary: #3EAF7C;
}
</style>
