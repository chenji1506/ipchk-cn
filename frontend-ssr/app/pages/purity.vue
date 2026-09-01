<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';

useHead({
  title: 'IP纯净度检测工具 | IP风险评分 | ipchk.cn',
  meta: [
    { name: 'description', content: '批量检测 IP 纯净度、归属类型、代理风险、邮件黑名单与网络质量，多源证据交叉验证。' },
    { name: 'keywords', content: 'ip纯净度,ip风险检测,ip评分,代理检测,vpn检测,数据中心检测,黑名单检测' },
  ],
});

// ============ 类型定义（对齐后端 schema） ============
interface ScoreFormula {
  purity_weight: number;
  stability_weight: number;
  data_quality_weight: number;
  classification_cap: number;
}
interface PurityDetail {
  score: number;
  risk_value: number;
  signal_risk: number;
  identity_risk: number;
  coverage_ceiling: number;
  type_cap: number;
  effective_type_cap: number;
  label: string;
  tone: string;
  confidence: string;
  confidence_label: string;
  evidence_source_count: number;
  reasons: string[];
}
interface Tag { label: string; tone: string; }
interface DimItem {
  key?: string; label: string; tone: string;
  confidence?: string; detail?: string; level?: string; conflict?: boolean;
}
interface Profile {
  primary: string; primary_tone: string;
  native: string; native_tone: string;
  risk: string; risk_tone: string;
  summary: string;
  tags: Tag[];
  source_count: number; typed_sources: number;
  access_network: DimItem; privacy: DimItem; reputation: DimItem; network: DimItem;
  classification: Record<string, DimItem>;
}
interface DimensionItem { score: number; max_score: number; }
interface Stability {
  success_rate: number; avg_latency_ms: number; p50_latency_ms: number;
  p95_latency_ms: number; timeout_count: number; probed: boolean;
}
interface Rbl {
  listed_count: number; network_listed_count: number;
  risk_level: string; query_limited: boolean; probed: boolean;
}
interface PurityReport {
  rank: number; name: string; ip: string; score: number;
  score_formula: ScoreFormula;
  purity: PurityDetail;
  recommendation: string;
  ip_type: string;
  profile: Profile;
  asn: number; asOrganization: string;
  country: string; countryCode: string; region: string; city: string; isp: string;
  fraudScore: number; riskLevel: string; ipSource: string; ipProperties: string[];
  isDataCenter: boolean; isResidential: boolean; isBroadcast: boolean; source: string;
  dimensions: Record<string, DimensionItem>;
  stability: Stability;
  rbl: Rbl;
  dns_leak: { dns_leak_suspected: boolean };
  main_reasons: string[];
}
interface CheckResponse {
  ok: boolean;
  run_id?: string;
  input_errors?: { input: string; error: string }[];
  run_errors?: string[];
  reports?: PurityReport[];
  error?: string;
}

const runtimeConfig = useRuntimeConfig();
const apiBase = (runtimeConfig.public.apiBase as string) || 'https://ipchk.cn/';
const inputText = ref('');
const reports = ref<PurityReport[]>([]);
const selectedIp = ref<string | null>(null);
const loading = ref(false);
const error = ref('');
const currentExitIp = ref('');

const currentReport = computed(() =>
  reports.value.find((r) => r.ip === selectedIp.value) || reports.value[0] || null,
);

// tone → 颜色
function toneColor(tone?: string): string {
  switch (tone) {
    case 'good': return '#3EAF7C';
    case 'warn': return '#E6A23C';
    case 'bad': return '#F56C6C';
    default: return '#909399';
  }
}

// 解析输入：换行 / 逗号 / 分号 / 空格 分隔，去重、过滤空
function parseIPs(text: string): string[] {
  const parts = text.split(/[\s,;，；]+/).map((s) => s.trim()).filter(Boolean);
  return Array.from(new Set(parts));
}

async function fillCurrentExit() {
  try {
    const ip = (await $fetch<string>(apiBase + 'ip')).trim();
    if (ip) {
      currentExitIp.value = ip;
      inputText.value = ip;
    }
  } catch {
    currentExitIp.value = '';
  }
}

async function runCheck() {
  const ips = parseIPs(inputText.value);
  if (ips.length === 0) {
    error.value = '请输入至少一个 IP 地址';
    return;
  }
  if (ips.length > 10) {
    error.value = '单次最多检测 10 个 IP，已截断前 10 个';
  }
  loading.value = true;
  error.value = '';
  try {
    const data = await $fetch<CheckResponse>(apiBase + 'v1/purity/check', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ips: ips.slice(0, 10) }),
    });
    if (!data.ok) {
      error.value = data.error || '检测失败，请稍后重试';
      reports.value = [];
      return;
    }
    reports.value = data.reports || [];
    const invalid = data.input_errors?.length || 0;
    if (invalid > 0) {
      error.value = `已忽略 ${invalid} 条无效输入`;
    } else if (reports.value.length === 0) {
      error.value = '未返回有效结果';
    }
    selectedIp.value = reports.value[0]?.ip || null;
  } catch (e: any) {
    error.value = e?.message || '检测失败，请稍后重试';
    reports.value = [];
  } finally {
    loading.value = false;
  }
}

const activeTab = ref<'overview' | 'scenario' | 'quality'>('overview');
const dimensionLegend: Record<string, string> = {
  reputation: '信誉', consistency: '一致', rbl: '黑名单',
  stability: '稳定', data_quality: '数据',
};
const tabLabels: Record<string, string> = {
  overview: '概览', scenario: '场景风险', quality: '网络质量',
};

function rankScoreTone(score: number): string {
  if (score >= 65) return 'good';
  if (score >= 40) return 'warn';
  return 'bad';
}

function rblText(r: PurityReport): string {
  if (!r.rbl?.probed) return '未启用';
  if (r.rbl.query_limited) return '查询受限';
  if (r.rbl.listed_count > 0 || r.rbl.network_listed_count > 0) {
    return `命中 ${r.rbl.listed_count} 单 IP / ${r.rbl.network_listed_count} 网段`;
  }
  return '未检出';
}

async function copyIP(ip: string) {
  try {
    await navigator.clipboard.writeText(ip);
  } catch { /* ignore */ }
}

onMounted(() => {
  fillCurrentExit();
});
</script>

<template>
  <div class="title">
    <header>
      <h1>IP 纯净度检测</h1>
      <p>批量检测 IP 纯净度 · 归属类型 · 代理风险 · 邮件黑名单 · 网络质量</p>
    </header>
  </div>

  <div class="content purity-content">
    <!-- 输入区 -->
    <div class="input-panel">
      <div class="input-head">
        <span class="input-hint">支持批量检测，每行一个 IP，或用逗号 / 空格分隔（最多 10 个）</span>
        <el-button size="small" text @click="fillCurrentExit">
          填入当前出口 IP
        </el-button>
      </div>
      <el-input
        v-model="inputText"
        type="textarea"
        :rows="3"
        placeholder="输入 IPv4 / IPv6，例如：&#10;8.8.8.8&#10;114.114.114.114&#10;2606:4700:4700::1111"
      />
      <div class="input-actions">
        <el-button type="primary" :loading="loading" @click="runCheck">
          开始检测
        </el-button>
        <el-button text @click="inputText = ''">清空</el-button>
        <span v-if="currentExitIp" class="exit-hint">当前出口：{{ currentExitIp }}</span>
      </div>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <!-- 结果区 -->
    <div v-if="reports.length" class="result-layout">
      <!-- 左侧：排序卡片 -->
      <div class="rank-list">
        <div class="rank-header">
          <span>检测结果</span>
          <span class="rank-count">{{ reports.length }} 个</span>
        </div>
        <div
          v-for="r in reports"
          :key="r.ip"
          class="rank-card"
          :class="{ selected: r.ip === currentReport?.ip }"
          @click="selectedIp = r.ip"
        >
          <div class="rank-top">
            <strong class="rank-ip" :title="r.ip">{{ r.ip }}</strong>
            <b class="rank-score" :style="{ color: toneColor(rankScoreTone(r.score)) }">{{ r.score }}</b>
          </div>
          <div class="rank-meta">
            <span :style="{ color: toneColor(r.purity.tone) }">{{ r.purity.label }} {{ r.purity.score }}</span>
            <span class="rank-type">{{ r.ip_type }}</span>
          </div>
          <div class="rank-sub">
            <span :style="{ color: toneColor(r.purity.signal_risk >= 20 ? 'bad' : r.purity.signal_risk >= 10 ? 'warn' : 'good') }">
              公开风险 {{ r.purity.signal_risk }}
            </span>
            <span v-if="r.stability?.probed && r.stability.p95_latency_ms > 0">
              P95 {{ r.stability.p95_latency_ms.toFixed(0) }}ms
            </span>
          </div>
        </div>
      </div>

      <!-- 右侧：详情档案 -->
      <div v-if="currentReport" class="detail-panel">
        <div class="detail-head">
          <span class="dossier-kicker">IP DOSSIER</span>
          <div class="dossier-ip">
            <h2>{{ currentReport.ip }}</h2>
            <button class="copy-btn" type="button" title="复制 IP" @click="copyIP(currentReport.ip)">
              <span>复制</span>
            </button>
          </div>
        </div>

        <!-- 得分总览 -->
        <div class="score-overview">
          <ScoreRing :score="currentReport.score" :size="110" label="/ 100" />
          <div class="score-summary">
            <div class="score-line">
              <label>综合质量分</label>
              <strong :style="{ color: toneColor(rankScoreTone(currentReport.score)) }">{{ currentReport.score }}</strong>
              <small>纯净度 {{ currentReport.score_formula?.purity_weight }}% · 网络 {{ currentReport.score_formula?.stability_weight }}% · 数据 {{ currentReport.score_formula?.data_quality_weight }}%</small>
            </div>
            <div class="score-line">
              <label>IP 纯净度</label>
              <strong :style="{ color: toneColor(currentReport.purity.tone) }">{{ currentReport.purity.score }}</strong>
              <small>{{ currentReport.purity.label }} · {{ currentReport.purity.confidence_label }}</small>
            </div>
            <div class="score-line">
              <label>主要类型</label>
              <strong :style="{ color: toneColor(currentReport.profile?.primary_tone) }">{{ currentReport.ip_type }}</strong>
              <small>{{ currentReport.profile?.source_count }} 个来源参与判断</small>
            </div>
          </div>
        </div>

        <!-- Tab 切换 -->
        <nav class="dossier-tabs">
          <button
            v-for="t in ['overview', 'scenario', 'quality']"
            :key="t"
            class="dossier-tab"
            :class="{ active: activeTab === t }"
            @click="activeTab = t"
          >{{ tabLabels[t] }}</button>
        </nav>

        <!-- 概览 -->
        <div v-if="activeTab === 'overview'" class="tab-panel">
          <!-- 综合结论 -->
          <section class="detail-section">
            <h4>结论与原因</h4>
            <div class="conclusion">
              <p class="conclusion-summary">{{ currentReport.profile?.summary }}</p>
              <ul class="reason-list">
                <li v-for="(reason, i) in currentReport.purity.reasons" :key="i">{{ reason }}</li>
              </ul>
              <p class="recommendation">
                <i class="rec-label">建议</i> {{ currentReport.recommendation }}
              </p>
            </div>
          </section>

          <!-- 五维画像 -->
          <section class="detail-section">
            <h4>IP 画像</h4>
            <div class="profile-grid">
              <div class="profile-row">
                <span class="p-label">IP 来源</span>
                <span class="p-value" :style="{ color: toneColor(currentReport.profile?.native_tone) }">{{ currentReport.profile?.native }}</span>
              </div>
              <div class="profile-row">
                <span class="p-label">IP 属性</span>
                <span class="p-value" :style="{ color: toneColor(currentReport.profile?.primary_tone) }">{{ currentReport.profile?.primary }}</span>
              </div>
              <div class="profile-row">
                <span class="p-label">接入网络</span>
                <span class="p-value" :style="{ color: toneColor(currentReport.profile?.access_network?.tone) }">{{ currentReport.profile?.access_network?.label }}</span>
              </div>
              <div class="profile-row">
                <span class="p-label">代理状态</span>
                <span class="p-value" :style="{ color: toneColor(currentReport.profile?.risk_tone) }">{{ currentReport.profile?.risk }}</span>
              </div>
              <div class="profile-row">
                <span class="p-label">邮件黑名单</span>
                <span class="p-value" :style="{ color: toneColor(currentReport.rbl?.probed ? (currentReport.rbl.listed_count > 0 ? 'bad' : 'good') : 'neutral') }">
                  {{ rblText(currentReport) }}
                </span>
              </div>
            </div>
            <div class="tag-list">
              <span v-for="(t, i) in currentReport.profile?.tags || []" :key="i" class="tag" :style="{ color: toneColor(t.tone), borderColor: toneColor(t.tone) }">{{ t.label }}</span>
            </div>
          </section>

          <!-- 基础信息 -->
          <section class="detail-section">
            <h4>基础信息</h4>
            <div class="base-grid">
              <div class="base-row"><span>归属地</span><b>{{ currentReport.country }} {{ currentReport.region }} {{ currentReport.city }}</b></div>
              <div class="base-row"><span>ASN</span><b>{{ currentReport.asOrganization || '未知' }}</b></div>
              <div class="base-row"><span>运营商</span><b>{{ currentReport.isp || '--' }}</b></div>
              <div class="base-row"><span>数据来源</span><b class="source-text">{{ currentReport.source }}</b></div>
            </div>
          </section>
        </div>

        <!-- 场景风险 -->
        <div v-if="activeTab === 'scenario'" class="tab-panel">
          <ScenarioRisk :report="currentReport" />
          <p class="disclaimer">基于当前 IP 画像与公开信誉估算，不代表目标平台实际可用或账号安全。</p>
        </div>

        <!-- 网络质量 -->
        <div v-if="activeTab === 'quality'" class="tab-panel">
          <section class="detail-section">
            <h4>五维评分</h4>
            <div class="radar-center">
              <RadarChart :dimensions="currentReport.dimensions" :size="240" />
            </div>
            <div class="dim-legend">
              <span v-for="(v, k) in dimensionLegend" :key="k">
                {{ v }} <b>{{ currentReport.dimensions?.[k]?.score ?? '—' }}/{{ currentReport.dimensions?.[k]?.max_score ?? '—' }}</b>
              </span>
            </div>
          </section>
          <section class="detail-section">
            <h4>网络稳定性</h4>
            <div v-if="currentReport.stability?.probed" class="quality-grid">
              <div class="quality-row"><span>成功率</span><b>{{ (currentReport.stability.success_rate * 100).toFixed(0) }}%</b></div>
              <div class="quality-row"><span>平均延迟</span><b>{{ currentReport.stability.avg_latency_ms.toFixed(1) }} ms</b></div>
              <div class="quality-row"><span>P50 延迟</span><b>{{ currentReport.stability.p50_latency_ms.toFixed(1) }} ms</b></div>
              <div class="quality-row"><span>P95 延迟</span><b>{{ currentReport.stability.p95_latency_ms.toFixed(1) }} ms</b></div>
            </div>
            <p v-else class="quality-empty">稳定性探测未启用</p>
          </section>
          <section class="detail-section">
            <h4>邮件黑名单（RBL）</h4>
            <div class="quality-grid">
              <div class="quality-row"><span>单 IP 命中</span><b :style="{ color: currentReport.rbl.listed_count > 0 ? '#F56C6C' : '#3EAF7C' }">{{ currentReport.rbl.listed_count }}</b></div>
              <div class="quality-row"><span>上游网段命中</span><b :style="{ color: currentReport.rbl.network_listed_count > 0 ? '#F56C6C' : '#3EAF7C' }">{{ currentReport.rbl.network_listed_count }}</b></div>
              <div class="quality-row"><span>DNS 泄漏</span><b :style="{ color: currentReport.dns_leak?.dns_leak_suspected ? '#F56C6C' : '#3EAF7C' }">{{ currentReport.dns_leak?.dns_leak_suspected ? '疑似' : '未发现' }}</b></div>
            </div>
          </section>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else-if="!loading" class="empty-state">
      <p>输入 IP 后点击「开始检测」，结果会按综合质量分排序展示。</p>
    </div>

    <blockquote>
      综合质量分 = 纯净度 85% + 网络稳定 10% + 数据质量 5%，由多源公开数据交叉验证得出，仅供参考，不代表目标平台的实际放行结果。
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";

.purity-content {
  width: 78%;
  max-width: 1200px;
}
@media (max-width: 768px) {
  .purity-content { width: 95%; }
}

/* 输入区 */
.input-panel {
  margin-bottom: 20px;
}
.input-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.input-hint {
  color: #909399;
  font-size: 0.9em;
}
.input-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}
.exit-hint {
  color: #909399;
  font-size: 0.9em;
  margin-left: auto;
}
.error-message {
  color: #F56C6C;
  margin: 12px 0;
  font-size: 0.95em;
}

/* 结果布局 */
.result-layout {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  margin-top: 10px;
}
.rank-list {
  width: 300px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.rank-header {
  display: flex;
  justify-content: space-between;
  font-size: 0.95em;
  color: #606266;
  padding: 0 4px;
}
html.dark .rank-header { color: #c0c4cc; }
.rank-count {
  color: #3EAF7C;
  font-weight: 600;
}
.rank-card {
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  padding: 12px 14px;
  cursor: pointer;
  background: #fff;
  transition: border-color 0.2s, box-shadow 0.2s;
}
html.dark .rank-card { background: #1a1a1a; border-color: #333; }
.rank-card:hover { border-color: #3EAF7C; }
.rank-card.selected {
  border-color: #3EAF7C;
  box-shadow: 0 0 0 2px rgba(62, 175, 124, 0.2);
}
.rank-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.rank-ip {
  font-family: monospace;
  font-size: 1.05em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rank-score {
  font-size: 1.5em;
  font-weight: 700;
  margin-left: 8px;
}
.rank-meta {
  display: flex;
  justify-content: space-between;
  font-size: 0.85em;
  margin-top: 4px;
}
.rank-type { color: #909399; }
.rank-sub {
  display: flex;
  justify-content: space-between;
  font-size: 0.8em;
  color: #909399;
  margin-top: 6px;
}

/* 详情 */
.detail-panel {
  flex: 1;
  min-width: 0;
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  padding: 20px;
  background: #fff;
}
html.dark .detail-panel { background: #1a1a1a; border-color: #333; }
.detail-head {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 16px;
}
.dossier-kicker {
  font-size: 0.75em;
  letter-spacing: 2px;
  color: #909399;
}
.dossier-ip {
  display: flex;
  align-items: center;
  gap: 12px;
}
.dossier-ip h2 {
  font-family: monospace;
  margin: 0;
  font-size: 1.6em;
}
.copy-btn {
  border: 1px solid #dcdfe6;
  background: transparent;
  border-radius: 6px;
  padding: 4px 10px;
  cursor: pointer;
  color: #606266;
  font-size: 0.85em;
}
html.dark .copy-btn { border-color: #444; color: #c0c4cc; }
.copy-btn:hover { border-color: #3EAF7C; color: #3EAF7C; }

.score-overview {
  display: flex;
  gap: 24px;
  align-items: center;
  padding: 16px;
  background: #fafbfc;
  border-radius: 10px;
  margin-bottom: 16px;
}
html.dark .score-overview { background: #222; }
.score-summary {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.score-line {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
}
.score-line label {
  width: 88px;
  color: #909399;
  font-size: 0.9em;
}
.score-line strong {
  font-size: 1.5em;
  font-weight: 700;
}
.score-line small {
  color: #909399;
  font-size: 0.82em;
  flex-basis: 100%;
}

.dossier-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid #e4e7ed;
  margin-bottom: 16px;
}
html.dark .dossier-tabs { border-color: #333; }
.dossier-tab {
  border: none;
  background: transparent;
  padding: 10px 16px;
  cursor: pointer;
  color: #606266;
  font-size: 0.95em;
  border-bottom: 2px solid transparent;
}
html.dark .dossier-tab { color: #c0c4cc; }
.dossier-tab.active {
  color: #3EAF7C;
  border-bottom-color: #3EAF7C;
  font-weight: 600;
}

.tab-panel { animation: fadeIn 0.2s ease; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: none; } }

.detail-section { margin-bottom: 22px; }
.detail-section h4 {
  font-size: 1em;
  color: #606266;
  margin: 0 0 12px;
  padding-left: 8px;
  border-left: 3px solid #3EAF7C;
}
html.dark .detail-section h4 { color: #c0c4cc; }
.conclusion-summary { margin: 0 0 8px; line-height: 1.6; }
.reason-list {
  margin: 0;
  padding-left: 20px;
  color: #606266;
  line-height: 1.9;
}
html.dark .reason-list { color: #c0c4cc; }
.recommendation {
  margin: 10px 0 0;
  padding: 10px 14px;
  background: rgba(62, 175, 124, 0.08);
  border-radius: 8px;
  line-height: 1.6;
}
.rec-label {
  font-style: normal;
  color: #3EAF7C;
  font-weight: 600;
  margin-right: 6px;
}

.profile-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}
.profile-row {
  display: flex;
  align-items: center;
}
.p-label {
  width: 100px;
  color: #909399;
  font-size: 0.9em;
  flex-shrink: 0;
}
.p-value { font-weight: 500; }
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.tag {
  border: 1px solid;
  border-radius: 20px;
  padding: 3px 12px;
  font-size: 0.82em;
}

.base-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.base-row {
  display: flex;
  align-items: baseline;
}
.base-row span {
  width: 100px;
  color: #909399;
  font-size: 0.9em;
  flex-shrink: 0;
}
.base-row b { font-weight: 500; font-size: 1em; }
.source-text { color: #909399; font-size: 0.85em !important; }

.disclaimer {
  color: #909399;
  font-size: 0.85em;
  margin-top: 16px;
  line-height: 1.6;
}

.radar-center {
  display: flex;
  justify-content: center;
  margin-bottom: 10px;
}
.dim-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 20px;
  justify-content: center;
  font-size: 0.85em;
  color: #606266;
}
html.dark .dim-legend { color: #c0c4cc; }
.dim-legend b { color: #3EAF7C; font-size: 1em; }

.quality-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 12px;
}
.quality-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  background: #fafbfc;
  border-radius: 8px;
}
html.dark .quality-row { background: #222; }
.quality-row span { color: #909399; font-size: 0.85em; }
.quality-row b { font-size: 1.15em; }
.quality-empty { color: #909399; }

.empty-state {
  text-align: center;
  color: #909399;
  padding: 40px 0;
}

@media (max-width: 900px) {
  .result-layout { flex-direction: column; }
  .rank-list { width: 100%; }
  .score-overview { flex-direction: column; }
}
</style>
