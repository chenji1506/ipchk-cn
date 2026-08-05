<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { config } from '../../config/index';
import { Search } from '@element-plus/icons-vue';

useHead({
  title: 'IP纯净度检测工具 | IP风险评分 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测 IP 是否为代理/VPN/机房地址，评估 IP 纯净度风险等级，支持任意 IP 查询。' },
    { name: 'keywords', content: 'ip纯净度,ip风险检测,ip评分,代理检测,vpn检测,数据中心检测' },
  ],
});

interface PurityType {
  ip?: string;
  asn?: number;
  asOrganization?: string;
  country?: string;
  countryCode?: string;
  region?: string;
  city?: string;
  isp?: string;
  fraudScore?: number;
  ippureCoefficient?: number;
  cloudflareCoefficient?: number;
  riskLevel?: string;
  ipSource?: string;
  ipProperties?: string[];
  isDataCenter?: boolean;
  isResidential?: boolean;
  isBroadcast?: boolean;
  source?: string;
}

const ipAddress = ref('');
const result = ref<PurityType | null>(null);
const loading = ref(false);
const error = ref('');
const apiBase = config.apiBaseUrls[0].url;

// 风险等级颜色
function riskColor(level?: string): string {
  switch (level) {
    case '安全': return '#3EAF7C';
    case '轻度风险': return '#E6A23C';
    case '中度风险': return '#E67E22';
    case '高度风险': return '#F56C6C';
    default: return '#909399';
  }
}

// 风险评分条颜色
function scoreBarColor(score: number): string {
  if (score <= 25) return '#3EAF7C';
  if (score <= 50) return '#E6A23C';
  if (score <= 70) return '#E67E22';
  return '#F56C6C';
}

async function queryPurity(ip?: string) {
  loading.value = true;
  error.value = '';
  try {
    const url = ip ? `${apiBase}v1/purity/${encodeURIComponent(ip)}` : `${apiBase}v1/purity`;
    const data = await $fetch<PurityType>(url);
    result.value = data;
  } catch (e: any) {
    error.value = e?.message || '查询失败，请稍后重试';
    result.value = null;
  } finally {
    loading.value = false;
  }
}

function search() {
  const ip = ipAddress.value.trim();
  queryPurity(ip || undefined);
}

onMounted(() => {
  // 默认查询自己的出口 IP
  queryPurity();
});
</script>

<template>
  <div class="title">
    <header>
      <h1>IP 纯净度检测</h1>
      <p>检测 IP 是否为代理/机房/VPN，评估风险等级</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input
        v-model="ipAddress"
        placeholder="请输入IP地址（留空查询自己的出口IP）"
        clearable
        @keyup.enter="search"
      />
      <el-button
        @click="search"
        type="primary"
        :loading="loading"
      >
        <el-icon style="margin-right: 4px;"><Search /></el-icon>
        查询
      </el-button>
    </div>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <!-- 查询结果 -->
    <div v-if="result" class="result-section">
      <!-- 风险总览卡片 -->
      <div class="risk-card">
        <div class="risk-left">
          <div class="risk-level" :style="{ color: riskColor(result.riskLevel) }">
            {{ result.riskLevel || '未知' }}
          </div>
          <div class="risk-ip">{{ result.ip }}</div>
        </div>
        <div class="risk-right">
          <div class="score-label">风险评分</div>
          <div class="score-bar">
            <div
              class="score-fill"
              :style="{
                width: (result.fraudScore || 0) + '%',
                background: scoreBarColor(result.fraudScore || 0)
              }"
            ></div>
          </div>
          <div class="score-num" :style="{ color: scoreBarColor(result.fraudScore || 0) }">
            {{ result.fraudScore }} / 100
          </div>
        </div>
      </div>

      <!-- 详细信息表格 -->
      <table class="result-table">
        <tbody>
          <tr>
            <td class="table-label">IP 地址</td>
            <td class="table-value"><span class="ip-highlight">{{ result.ip }}</span></td>
          </tr>
          <tr>
            <td class="table-label">归属地</td>
            <td class="table-value"><span>{{ result.country }} {{ result.region }} {{ result.city }}</span></td>
          </tr>
          <tr>
            <td class="table-label">ASN</td>
            <td class="table-value"><span>{{ result.asOrganization || '未知' }}</span></td>
          </tr>
          <tr>
            <td class="table-label">运营商</td>
            <td class="table-value"><span>{{ result.isp || '--' }}</span></td>
          </tr>
          <tr>
            <td class="table-label">IP 来源</td>
            <td class="table-value"><span>{{ result.ipSource || '--' }}</span></td>
          </tr>
          <tr>
            <td class="table-label">属性</td>
            <td class="table-value">
              <span v-for="(p, i) in (result.ipProperties || [])" :key="i" class="prop-tag">{{ p }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">数据中心</td>
            <td class="table-value">
              <span :class="result.isDataCenter ? 'badge-red' : 'badge-green'">
                {{ result.isDataCenter ? '是' : '否' }}
              </span>
            </td>
          </tr>
          <tr>
            <td class="table-label">住宅 IP</td>
            <td class="table-value">
              <span :class="result.isResidential ? 'badge-green' : 'badge-red'">
                {{ result.isResidential ? '是' : '否' }}
              </span>
            </td>
          </tr>
          <tr>
            <td class="table-label">广播地址</td>
            <td class="table-value"><span>{{ result.isBroadcast ? '是' : '否' }}</span></td>
          </tr>
          <tr>
            <td class="table-label">CF 系数</td>
            <td class="table-value"><span>{{ result.cloudflareCoefficient }} / 100</span></td>
          </tr>
          <tr>
            <td class="table-label">数据来源</td>
            <td class="table-value"><span>{{ result.source || '--' }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>

    <blockquote>
      评分说明：0-25 安全 | 26-50 轻度风险 | 51-70 中度风险 | 71-100 高度风险<br>
      检测基于 IP 前缀、ASN 归属和地理位置综合评估，仅供参考。
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.one-line {
  display: flex;
  flex-direction: row;
  align-items: center;
  flex-wrap: nowrap;
  margin-bottom: 20px;
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
  font-size: 1.1em;
}

.error-message {
  color: #F56C6C;
  margin: 15px 0;
  font-size: 1.1em;
}

/* 风险卡片 */
.risk-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-radius: 12px;
  background: #f6f8fa;
  border: 1px solid #e4e7ed;
  margin-bottom: 20px;
}

html.dark .risk-card {
  background: #1a1a1a;
  border-color: #333;
}

.risk-left {
  display: flex;
  flex-direction: column;
}

.risk-level {
  font-size: 2.2em;
  font-weight: bold;
  line-height: 1.2;
}

.risk-ip {
  font-size: 1em;
  color: #909399;
  margin-top: 4px;
}

.risk-right {
  width: 50%;
}

.score-label {
  font-size: 0.9em;
  color: #909399;
  margin-bottom: 6px;
}

.score-bar {
  height: 14px;
  background: #ebeef5;
  border-radius: 7px;
  overflow: hidden;
}

html.dark .score-bar {
  background: #333;
}

.score-fill {
  height: 100%;
  border-radius: 7px;
  transition: width 0.6s ease;
}

.score-num {
  text-align: right;
  font-size: 1.3em;
  font-weight: bold;
  margin-top: 6px;
}

/* 表格 */
.result-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 20px;
}

.result-table td {
  padding: 12px 15px;
  border: 1px solid #dcdfe6;
}

html.dark .result-table td {
  border-color: #1a1919;
}

.table-label {
  width: 140px;
  font-weight: 600;
  color: #606266;
  background: #f5f7fa;
}

html.dark .table-label {
  color: #c0c4cc;
  background: #222;
}

.table-value {
  color: #303133;
}

html.dark .table-value {
  color: #cfcfcf;
}

.ip-highlight {
  color: #3EAF7C;
  font-weight: bold;
  font-size: 1.2em;
}

.prop-tag {
  display: inline-block;
  background: rgba(62, 175, 124, 0.12);
  color: #3EAF7C;
  border-radius: 4px;
  padding: 2px 10px;
  margin-right: 8px;
  font-size: 0.95em;
}

.badge-green {
  color: #3EAF7C;
  font-weight: 600;
}

.badge-red {
  color: #F56C6C;
  font-weight: 600;
}

@media (max-width: 768px) {
  .el-input {
    width: 100%;
  }
  .one-line {
    flex-direction: column;
    align-items: stretch;
  }
  .el-button {
    margin-top: 10px;
    width: 100%;
  }
  .risk-card {
    flex-direction: column;
    align-items: flex-start;
  }
  .risk-right {
    width: 100%;
    margin-top: 15px;
  }
}
</style>
<style>
:root {
  --el-color-primary: #3EAF7C;
}
</style>
