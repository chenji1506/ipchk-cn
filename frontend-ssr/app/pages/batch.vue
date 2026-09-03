<script setup lang="ts">
import { ref, onMounted } from 'vue';
const config = useIpchkConfig()
const { t } = useI18n()

useHead({
  title: '批量 IP 查询 | 多IP归属地 | ipchk.cn',
  meta: [
    { name: 'description', content: t('批量查询多个 IP 的归属地信息，支持换行分隔。') },
  ],
});

const apiBase = config.apiBaseUrls[0].url;
const ipsInput = ref('');
const results = ref<any[]>([]);
const loading = ref(false);
const error = ref('');
const history = ref<string[]>([]);

// 历史记录（localStorage）
function loadHistory() {
  try {
    history.value = JSON.parse(localStorage.getItem('ipchk_history') || '[]');
  } catch { history.value = []; }
}
function saveHistory(ip: string) {
  const h = [ip, ...history.value.filter(x => x !== ip)].slice(0, 10);
  history.value = h;
  localStorage.setItem('ipchk_history', JSON.stringify(h));
}
function useHistory(ip: string) {
  ipsInput.value = (ipsInput.value ? ipsInput.value + '\n' : '') + ip;
}
function clearHistory() {
  history.value = [];
  localStorage.removeItem('ipchk_history');
}

async function batchQuery() {
  const lines = ipsInput.value.split('\n').map(s => s.trim()).filter(Boolean);
  if (!lines.length) { error.value = t('请输入至少一个 IP'); return; }
  loading.value = true;
  error.value = '';
  results.value = [];

  const uniqueIPs = [...new Set(lines)].slice(0, 20);
  for (const ip of uniqueIPs) {
    try {
      const data = await $fetch(`${apiBase}v1/location/${encodeURIComponent(ip)}`);
      results.value.push({
        ip,
        country: data?.country || '',
        region: data?.region || '',
        city: data?.city || '',
        isp: data?.isp || '',
        source: data?.source || '',
      });
      saveHistory(ip);
    } catch {
      results.value.push({ ip, country: t('查询失败'), region: '', city: '', isp: '', source: '' });
    }
  }
  loading.value = false;
}

onMounted(loadHistory);
</script>

<template>
  <div class="title">
    <header>
      <h1>{{ $t('批量 IP 查询') }}</h1>
      <p>{{ $t('一次查询多个 IP 的归属地（最多 20 个，换行分隔）') }}</p>
    </header>
  </div>
  <div class="content">
    <el-input
      v-model="ipsInput"
      type="textarea"
      :rows="5"
      placeholder="每行一个 IP，如：&#10;8.8.8.8&#10;114.114.114.114&#10;223.5.5.5"
      class="ips-input"
    />
    <div class="btn-row">
      <el-button type="primary" :loading="loading" @click="batchQuery">{{ $t('批量查询') }}</el-button>
      <el-button @click="ipsInput = ''">{{ $t('清空') }}</el-button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <!-- 历史记录 -->
    <div v-if="history.length" class="history-section">
      <div class="history-header">
        <span class="history-title">{{ $t('最近查询') }}</span>
        <button class="clear-btn" @click="clearHistory">{{ $t('清空历史') }}</button>
      </div>
      <div class="history-tags">
        <span v-for="(ip, i) in history" :key="i" class="history-tag" @click="useHistory(ip)">{{ ip }}</span>
      </div>
    </div>

    <!-- 结果表格 -->
    <div v-if="results.length" class="result-section">
      <table class="result-table">
        <thead>
          <tr>
            <th class="table-header">IP</th>
            <th class="table-header">{{ $t('归属地') }}</th>
            <th class="table-header">{{ $t('运营商') }}</th>
            <th class="table-header">{{ $t('数据源') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in results" :key="i">
            <td class="table-value"><span class="ip-highlight">{{ r.ip }}</span></td>
            <td class="table-value">{{ r.country }} {{ r.region }} {{ r.city }}</td>
            <td class="table-value">{{ r.isp || '--' }}</td>
            <td class="table-value">{{ r.source || '--' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.ips-input {
  font-family: 'JetBrains Mono', Consolas, monospace;
  margin-bottom: 12px;
}
.ips-input :deep(.el-textarea__inner) {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 1em;
}

.btn-row {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.ip-highlight {
  color: #3EAF7C;
  font-weight: 600;
  font-family: 'JetBrains Mono', Consolas, monospace;
}

.history-section {
  margin: 16px 0;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid rgba(62, 175, 124, 0.2);
  background: rgba(62, 175, 124, 0.04);
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.history-title {
  font-weight: 600;
  color: #3EAF7C;
  font-size: 0.95em;
}
.clear-btn {
  background: transparent;
  border: none;
  color: #909399;
  cursor: pointer;
  font-size: 0.85em;
}
.clear-btn:hover { color: #F56C6C; }

.history-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.history-tag {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 0.85em;
  padding: 4px 12px;
  border-radius: 6px;
  background: rgba(62, 175, 124, 0.1);
  color: #3EAF7C;
  cursor: pointer;
  transition: all 0.2s;
}
.history-tag:hover {
  background: #3EAF7C;
  color: #fff;
}
</style>
<style>
:root { --el-color-primary: #3EAF7C; }
</style>
