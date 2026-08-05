<script setup lang="ts">
import { ref } from 'vue';
import { config } from '../../config/index';

useHead({
  title: '端口扫描工具 | 在线端口检测 | ipchk.cn',
  meta: [
    { name: 'description', content: '在线扫描指定 IP 或域名的常见端口开放情况，检测服务暴露风险。' },
  ],
});

const apiBase = config.apiBaseUrls[0].url;
const target = ref('');
const portsInput = ref('');
const results = ref<any>(null);
const loading = ref(false);
const error = ref('');

const COMMON_PORTS = [
  { port: 21, name: 'FTP' }, { port: 22, name: 'SSH' }, { port: 23, name: 'Telnet' },
  { port: 25, name: 'SMTP' }, { port: 53, name: 'DNS' }, { port: 80, name: 'HTTP' },
  { port: 110, name: 'POP3' }, { port: 135, name: 'RPC' }, { port: 139, name: 'NetBIOS' },
  { port: 143, name: 'IMAP' }, { port: 443, name: 'HTTPS' }, { port: 445, name: 'SMB' },
  { port: 993, name: 'IMAPS' }, { port: 995, name: 'POP3S' }, { port: 1433, name: 'MSSQL' },
  { port: 1521, name: 'Oracle' }, { port: 3306, name: 'MySQL' }, { port: 3389, name: 'RDP' },
  { port: 5432, name: 'PostgreSQL' }, { port: 6379, name: 'Redis' }, { port: 8080, name: 'HTTP-Alt' },
  { port: 8443, name: 'HTTPS-Alt' }, { port: 8888, name: 'Web' }, { port: 9090, name: 'Dashboard' },
  { port: 9200, name: 'Elasticsearch' }, { port: 27017, name: 'MongoDB' },
];

function portName(port: number): string {
  const found = COMMON_PORTS.find(p => p.port === port);
  return found ? found.name : '未知服务';
}

async function scan() {
  const t = target.value.trim();
  if (!t) { error.value = '请输入 IP 或域名'; return; }
  loading.value = true;
  error.value = '';
  try {
    const url = `${apiBase}v1/scan/${encodeURIComponent(t)}` +
      (portsInput.value.trim() ? `?ports=${encodeURIComponent(portsInput.value.trim())}` : '');
    results.value = await $fetch(url);
  } catch (e: any) {
    error.value = e?.message || '扫描失败';
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="title">
    <header>
      <h1>端口扫描</h1>
      <p>检测目标 IP/域名的常见端口开放情况</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input v-model="target" placeholder="请输入 IP 或域名（如：8.8.8.8）" clearable @keyup.enter="scan" />
      <el-button type="primary" :loading="loading" @click="scan">开始扫描</el-button>
    </div>
    <div class="ports-hint">
      <p>自定义端口（逗号分隔，留空扫默认 26 个常见端口）：</p>
      <el-input v-model="portsInput" placeholder="如：22,80,443,3306" clearable />
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="results" class="result-section">
      <div class="scan-summary">
        <span>目标: <b>{{ results.host }}</b></span>
        <span>扫描端口: <b>{{ results.total }}</b></span>
        <span>开放端口: <b class="open-count">{{ results.open }}</b></span>
      </div>
      <table class="result-table">
        <thead>
          <tr>
            <th class="table-header">端口</th>
            <th class="table-header">服务</th>
            <th class="table-header">状态</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in results.ports" :key="p.port">
            <td class="table-label">{{ p.port }}</td>
            <td class="table-value">{{ portName(p.port) }}</td>
            <td class="table-value">
              <span :class="p.state === 'open' ? 'status-success' : 'status-error'" class="status-code">
                {{ p.state === 'open' ? '开放' : '关闭' }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <blockquote>扫描基于 TCP 连接测试（2 秒超时），仅用于网络诊断，请勿用于非法用途。</blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.one-line {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  margin-bottom: 10px;
}

.ports-hint {
  margin-bottom: 10px;
}
.ports-hint p {
  font-size: 0.9em;
  color: #909399;
  margin: 0 0 6px;
}
.ports-hint .el-input {
  width: 300px;
  height: 40px;
}

.scan-summary {
  display: flex;
  gap: 24px;
  padding: 14px 18px;
  border-radius: 10px;
  background: rgba(62, 175, 124, 0.06);
  border: 1px solid rgba(62, 175, 124, 0.2);
  margin-bottom: 16px;
  font-size: 1em;
  flex-wrap: wrap;
}
.open-count {
  color: #3EAF7C;
  font-size: 1.2em;
}

.el-input {
  width: 420px;
  height: 50px;
  font: 1.2em sans-serif;
  margin-right: 10px;
}
.el-button {
  height: 50px;
  padding: 0 24px;
}

@media (max-width: 768px) {
  .one-line { flex-direction: column; align-items: stretch; }
  .el-input, .ports-hint .el-input { width: 100%; margin-right: 0; }
  .el-button { margin-top: 10px; }
}
</style>
<style>
:root { --el-color-primary: #3EAF7C; }
</style>
