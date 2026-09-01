<script setup lang="ts">
import { ref, onMounted } from 'vue';

useHead({
  title: '邮件黑名单检测 | DNSBL 查询 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测 IP 是否命中邮件黑名单（DNSBL），覆盖 Spamhaus、SpamCop、SORBS、Barracuda 等 16 个权威黑名单源。' },
    { name: 'keywords', content: '邮件黑名单,DNSBL,spamhaus,spamcop,sorbs,rbl检测,IP黑名单' },
  ],
});

interface RBLZone {
  name: string;
  listed: boolean;
  whitelist: boolean;
  status: string; // listed / clean / failed
}
interface RBLResult {
  ip?: string;
  checked_count?: number;
  listed_count?: number;
  network_listed_count?: number;
  white_listed?: boolean;
  query_limited?: boolean;
  risk_level?: string;
  listed_zones?: string[];
  zones?: RBLZone[];
}

const runtimeConfig = useRuntimeConfig();
const apiBase = (runtimeConfig.public.apiBase as string) || 'https://ipchk.cn/';
const ipAddress = ref('');
const result = ref<RBLResult | null>(null);
const loading = ref(false);
const error = ref('');
const currentExitIp = ref('');

async function fillCurrentExit() {
  try {
    const ip = (await $fetch<string>(apiBase + 'ip')).trim();
    if (ip) {
      currentExitIp.value = ip;
      ipAddress.value = ip;
    }
  } catch {
    currentExitIp.value = '';
  }
}

async function query() {
  const ip = ipAddress.value.trim();
  if (!ip) {
    error.value = '请输入 IP 地址';
    return;
  }
  loading.value = true;
  error.value = '';
  try {
    const data = await $fetch<RBLResult>(apiBase + 'v1/rbl/' + encodeURIComponent(ip));
    result.value = data;
  } catch (e: any) {
    error.value = e?.message || '查询失败，请稍后重试';
    result.value = null;
  } finally {
    loading.value = false;
  }
}

function statusText(z: RBLZone): string {
  if (z.whitelist) return '白名单';
  switch (z.status) {
    case 'listed': return '已命中';
    case 'failed': return '查询失败';
    default: return '未命中';
  }
}
function statusTone(z: RBLZone): string {
  if (z.whitelist) return 'good';
  switch (z.status) {
    case 'listed': return 'bad';
    case 'failed': return 'neutral';
    default: return 'good';
  }
}
function toneColor(tone: string): string {
  switch (tone) {
    case 'good': return '#3EAF7C';
    case 'bad': return '#F56C6C';
    case 'warn': return '#E6A23C';
    default: return '#909399';
  }
}
function riskLabel(level?: string): string {
  switch (level) {
    case 'high': return '高';
    case 'medium': return '中';
    case 'low': return '低';
    default: return '无';
  }
}

onMounted(() => {
  fillCurrentExit();
});
</script>

<template>
  <div class="title">
    <header>
      <h1>邮件黑名单检测</h1>
      <p>检测 IP 是否命中 DNSBL 邮件黑名单，覆盖 Spamhaus、SpamCop、SORBS、Barracuda 等 16 个权威源</p>
    </header>
  </div>

  <div class="content rbl-content">
    <div class="input-panel">
      <div class="one-line">
        <el-input
          v-model="ipAddress"
          placeholder="请输入 IP 地址（留空查询自己的出口 IP）"
          clearable
          @keyup.enter="query"
        />
        <el-button type="primary" :loading="loading" @click="query">
          查询
        </el-button>
        <el-button text @click="fillCurrentExit">填入当前出口 IP</el-button>
      </div>
      <div v-if="currentExitIp" class="exit-hint">当前出口：{{ currentExitIp }}</div>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <!-- 结果 -->
    <div v-if="result" class="result-section">
      <!-- 汇总卡片 -->
      <div class="summary-cards">
        <div class="sum-card">
          <span>检查源数</span>
          <b>{{ result.checked_count ?? 0 }}</b>
        </div>
        <div class="sum-card" :class="(result.listed_count || 0) > 0 ? 'risk' : 'ok'">
          <span>单 IP 命中</span>
          <b :style="{ color: (result.listed_count || 0) > 0 ? '#F56C6C' : '#3EAF7C' }">{{ result.listed_count ?? 0 }}</b>
        </div>
        <div class="sum-card" :class="(result.network_listed_count || 0) > 0 ? 'risk' : 'ok'">
          <span>网段命中</span>
          <b :style="{ color: (result.network_listed_count || 0) > 0 ? '#F56C6C' : '#3EAF7C' }">{{ result.network_listed_count ?? 0 }}</b>
        </div>
        <div class="sum-card">
          <span>白名单</span>
          <b :style="{ color: result.white_listed ? '#3EAF7C' : '#909399' }">{{ result.white_listed ? '是' : '否' }}</b>
        </div>
        <div class="sum-card">
          <span>风险等级</span>
          <b :style="{ color: (result.listed_count || 0) > 0 ? '#F56C6C' : '#3EAF7C' }">{{ riskLabel(result.risk_level) }}</b>
        </div>
      </div>

      <div v-if="result.query_limited" class="warn-tip">
        ⚠️ 部分 DNSBL 查询受限（上游 DNS 被拦截），结果可能不完整。
      </div>

      <!-- 命中的黑名单 -->
      <div v-if="(result.listed_zones?.length || 0) > 0" class="hit-list">
        <h4>命中的黑名单源</h4>
        <div class="hit-tags">
          <span v-for="z in result.listed_zones" :key="z" class="hit-tag">{{ z }}</span>
        </div>
      </div>

      <!-- 各源状态表 -->
      <div class="zone-table-wrap">
        <h4>各黑名单源状态</h4>
        <table class="zone-table">
          <thead>
            <tr>
              <th>DNSBL 源</th>
              <th style="width: 90px;">类型</th>
              <th style="width: 110px;">状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="z in result.zones" :key="z.name">
              <td class="zone-name">{{ z.name }}</td>
              <td>{{ z.whitelist ? '白名单' : '黑名单' }}</td>
              <td>
                <span class="zone-status" :style="{ color: toneColor(statusTone(z)) }">
                  {{ statusText(z) }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="!loading" class="empty-state">
      <p>输入 IP 后点击「查询」，查看该 IP 在 16 个 DNSBL 黑名单源的命中情况。</p>
    </div>

    <blockquote>
      邮件黑名单（DNSBL）反映 IP 的邮件发送与滥用历史。单 IP 命中与上游网段命中会分开统计；结果仅供参考。
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";

.rbl-content {
  width: 70%;
  max-width: 900px;
}
@media (max-width: 768px) {
  .rbl-content { width: 95%; }
}

.input-panel { margin-bottom: 16px; }
.exit-hint {
  color: #909399;
  font-size: 0.88em;
  margin-top: 6px;
}
.error-message {
  color: #F56C6C;
  margin: 10px 0;
  font-size: 0.95em;
}

.summary-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.sum-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px;
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  background: #fff;
}
html.dark .sum-card { background: #1a1a1a; border-color: #333; }
.sum-card span {
  color: #909399;
  font-size: 0.85em;
}
.sum-card b {
  font-size: 1.5em;
  font-weight: 700;
}

.warn-tip {
  color: #E6A23C;
  background: rgba(230, 162, 60, 0.08);
  border-radius: 8px;
  padding: 10px 14px;
  margin-bottom: 16px;
  font-size: 0.9em;
}

.hit-list { margin-bottom: 16px; }
.hit-list h4 {
  font-size: 1em;
  color: #606266;
  margin: 0 0 10px;
  padding-left: 8px;
  border-left: 3px solid #F56C6C;
}
html.dark .hit-list h4 { color: #c0c4cc; }
.hit-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.hit-tag {
  border: 1px solid #F56C6C;
  color: #F56C6C;
  border-radius: 6px;
  padding: 3px 10px;
  font-size: 0.85em;
  font-family: monospace;
}

.zone-table-wrap { margin-bottom: 16px; }
.zone-table-wrap h4 {
  font-size: 1em;
  color: #606266;
  margin: 0 0 10px;
  padding-left: 8px;
  border-left: 3px solid #3EAF7C;
}
html.dark .zone-table-wrap h4 { color: #c0c4cc; }
.zone-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.92em;
}
.zone-table th, .zone-table td {
  text-align: left;
  padding: 9px 12px;
  border-bottom: 1px solid #e4e7ed;
}
html.dark .zone-table th, html.dark .zone-table td { border-color: #333; }
.zone-table th {
  color: #909399;
  font-weight: 500;
  font-size: 0.85em;
}
.zone-name {
  font-family: monospace;
  color: #303133;
}
html.dark .zone-name { color: #cfcfcf; }
.zone-status {
  font-weight: 600;
}

.empty-state {
  text-align: center;
  color: #909399;
  padding: 40px 0;
}
</style>
