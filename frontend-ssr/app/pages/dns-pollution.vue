<script setup lang="ts">
import { ref } from 'vue'

const config = useIpchkConfig()
const { t } = useI18n()

useHead({
  title: 'DNS 污染检测 | ipchk.cn',
  meta: [
    { name: 'description', content: t('用多个公共 DNS 服务器（阿里云/腾讯/Google/Cloudflare）解析同一域名对比结果，检测域名是否被 DNS 污染或劫持。') },
    { name: 'keywords', content: 'DNS污染,DNS劫持,DNS检测,域名污染,dns pollution,dns hijack' },
  ],
})

const tmpdomain = ref('')
const loading = ref(false)
const result = ref<any>(null)
const error = ref('')

async function queryPollution() {
  loading.value = true
  error.value = ''
  result.value = null
  const domain = tmpdomain.value.trim()
  if (!domain) {
    error.value = t('请输入域名')
    loading.value = false
    return
  }
  try {
    result.value = await $fetch(config.apiBaseUrls[0].url + 'v1/dns-pollution/' + encodeURIComponent(domain))
  } catch (e: any) {
    error.value = e?.message || t('请求失败，请重试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="title">
    <header>
      <h1>{{ $t('DNS 污染检测') }}</h1>
      <p>{{ $t('用多个公共 DNS 服务器解析同一域名，对比结果检测 DNS 污染 / 劫持') }}</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input v-model="tmpdomain" :placeholder="$t('输入域名（如：google.com）')" @keyup.enter="queryPollution()" />
      <el-button @click="queryPollution()" type="primary" :loading="loading">{{ $t('检测') }}</el-button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="result" class="result-section">
      <div class="conclusion" :class="result.consistent ? 'conclusion-good' : 'conclusion-warn'">
        {{ result.consistent ? '✓' : '⚠' }} {{ result.conclusion }}
      </div>

      <table class="result-table">
        <thead>
          <tr>
            <th class="table-header">{{ $t('DNS 服务器') }}</th>
            <th class="table-header">{{ $t('地区') }}</th>
            <th class="table-header">{{ $t('解析结果') }}</th>
            <th class="table-header">{{ $t('耗时') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in result.servers" :key="s.server">
            <td class="table-label">
              <div>{{ s.name }}</div>
              <div class="server-addr">{{ s.server }}</div>
            </td>
            <td class="table-value">
              <span class="region-tag" :class="s.region === '国内' ? 'region-cn' : 'region-overseas'">{{ s.region }}</span>
            </td>
            <td class="table-value">
              <template v-if="s.error">
                <span class="error-text">{{ $t('查询失败') }}</span>
              </template>
              <template v-else-if="s.records.length">
                <span v-for="ip in s.records" :key="ip" class="ip-chip"><code>{{ ip }}</code></span>
              </template>
              <template v-else>
                <span style="color: #999;">{{ $t('无 A 记录') }}</span>
              </template>
            </td>
            <td class="table-value">{{ s.duration ? s.duration.toFixed(0) + ' ms' : '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <blockquote>
      {{ $t('同时向国内（阿里云、腾讯 DNSPod）和国外（Google、Cloudflare）公共 DNS 查询 A 记录并对比。') }}<br/>
      {{ $t('结果一致说明解析正常；结果不一致可能是 DNS 污染/劫持，也可能是 CDN 分区域解析（国内网站常见），需结合 IP 归属综合判断。') }}
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

.conclusion {
  font-size: 1.1em;
  font-weight: 600;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 14px;
}
.conclusion-good { background: #f0f9eb; color: #67C23A; }
.conclusion-warn { background: #fdf6ec; color: #E6A23C; }
html.dark .conclusion-good { background: rgba(103, 194, 58, 0.15); }
html.dark .conclusion-warn { background: rgba(230, 162, 60, 0.15); }

.server-addr {
  color: #999;
  font-size: 0.82em;
  margin-top: 2px;
}

.region-tag {
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.9em;
}
.region-cn { background: #ecf5ff; color: #409EFF; }
.region-overseas { background: #fdf6ec; color: #E6A23C; }
html.dark .region-cn { background: rgba(64, 158, 255, 0.15); }
html.dark .region-overseas { background: rgba(230, 162, 60, 0.15); }

.ip-chip {
  display: inline-block;
  margin: 2px 4px 2px 0;
  padding: 2px 8px;
  background: rgba(62, 175, 124, 0.1);
  border-radius: 4px;
  font-size: 0.9em;
}
.ip-chip code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: #3EAF7C;
}

.error-text { color: #F56C6C; font-size: 0.9em; }
</style>

<style>
:root {
  --el-color-primary: #3EAF7C;
}
html.dark {
  --el-color-primary: #3EAF7C;
}
</style>
