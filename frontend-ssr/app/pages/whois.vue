<script setup lang="ts">
import { ref } from 'vue';
import { config } from '../../config/index';

useHead({
  title: 'Whois 域名/IP 查询 | 注册信息 | ipchk.cn',
  meta: [
    { name: 'description', content: '在线查询域名或 IP 的注册信息（RDAP），包括注册商、名称服务器、状态等。' },
  ],
});

const apiBase = config.apiBaseUrls[0].url;
const target = ref('');
const result = ref<any>(null);
const loading = ref(false);
const error = ref('');
const showRaw = ref(false);

async function query() {
  const t = target.value.trim();
  if (!t) { error.value = '请输入域名或 IP'; return; }
  loading.value = true;
  error.value = '';
  try {
    result.value = await $fetch(`${apiBase}v1/whois/${encodeURIComponent(t)}`);
  } catch (e: any) {
    error.value = e?.message || '查询失败';
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="title">
    <header>
      <h1>Whois 查询</h1>
      <p>查询域名/IP 的注册信息（RDAP 协议）</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input v-model="target" placeholder="请输入域名或 IP（如：baidu.com / 8.8.8.8）" clearable @keyup.enter="query" />
      <el-button type="primary" :loading="loading" @click="query">查询</el-button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="result && !result.error" class="result-section">
      <table class="result-table">
                <tbody>
          <tr>
            <td class="table-label">目标</td>
            <td class="table-value"><span class="ip-highlight">{{ result.domain || result.target }}</span></td>
          </tr>
          <tr>
            <td class="table-label">注册商</td>
            <td class="table-value">
              <span v-if="result.registrar?.name">{{ result.registrar.name }} <span v-if="result.registrar.ianaId" class="sub-info">(IANA ID: {{ result.registrar.ianaId }})</span></span>
              <span v-else>{{ result.registrar || -- }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">注册时间</td>
            <td class="table-value">
              <span v-if="result.dates?.registration">{{ result.dates.registration }}</span>
              <span v-else>{{ result.creation || -- }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">过期时间</td>
            <td class="table-value">
              <span v-if="result.dates?.expiration">{{ result.dates.expiration }}</span>
              <span v-else>{{ result.expiry || -- }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">最后变更</td>
            <td class="table-value"><span>{{ result.dates?.lastChanged || -- }}</span></td>
          </tr>
          <tr>
            <td class="table-label">注册人</td>
            <td class="table-value">
              <span v-if="result.registrant?.org">{{ result.registrant.org }}</span>
              <span v-else-if="result.registrant?.name">{{ result.registrant.name }}</span>
              <span v-else>{{ result.registrant || -- }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">技术联系人</td>
            <td class="table-value">
              <span v-if="result.technical?.org">{{ result.technical.org }}</span>
              <span v-else-if="result.technical?.name">{{ result.technical.name }}</span>
              <span v-else>--</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">Abuse 联系</td>
            <td class="table-value">
              <span v-if="result.abuseContact?.email">{{ result.abuseContact.email }}</span>
              <span v-else-if="result.abuseContact?.name">{{ result.abuseContact.name }}</span>
              <span v-else>--</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">名称服务器</td>
            <td class="table-value">
              <span v-if="Array.isArray(result.nameservers)" v-for="(ns, i) in result.nameservers" :key="i" class="prop-tag">{{ ns }}</span>
              <span v-else>{{ result.ns || result.nameservers || -- }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">状态</td>
            <td class="table-value">
              <span v-if="Array.isArray(result.status)" v-for="(s, i) in result.status" :key="i" class="prop-tag">{{ s }}</span>
              <span v-else>{{ result.status || -- }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">Whois 服务器</td>
            <td class="table-value"><span>{{ result.whoisServer || result.server || -- }}</span></td>
          </tr>
        </tbody>
      </table>

      <div class="raw-toggle">
        <button class="action-btn" @click="showRaw = !showRaw">{{ showRaw ? '收起原始数据' : '查看原始数据' }}</button>
      </div>
      <pre v-if="showRaw" class="raw-json">{{ result.raw }}</pre>
    </div>

    <div v-if="result?.error" class="error-message">查询失败：{{ result.error }}</div>

    <blockquote>基于 RDAP 开放协议查询，数据来自各注册局，仅供参考。</blockquote>
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

.ip-highlight {
  color: #3EAF7C;
  font-weight: bold;
  font-size: 1.15em;
}

.sub-info { font-size: 0.85em; color: #909399; }
.prop-tag {
  display: inline-block;
  background: rgba(62, 175, 124, 0.1);
  color: #3EAF7C;
  border-radius: 4px;
  padding: 2px 10px;
  margin: 2px 6px 2px 0;
  font-size: 0.9em;
}

.raw-toggle {
  margin: 16px 0 8px;
}
.action-btn {
  padding: 6px 18px;
  border-radius: 8px;
  border: 1px solid #3EAF7C;
  background: transparent;
  color: #3EAF7C;
  cursor: pointer;
  font-size: 0.95em;
}
.action-btn:hover { background: #3EAF7C; color: #fff; }

.raw-json {
  max-height: 400px;
  overflow: auto;
  font-size: 0.8em;
}

@media (max-width: 768px) {
  .one-line { flex-direction: column; align-items: stretch; }
  .el-input { width: 100%; margin-right: 0; }
  .el-button { margin-top: 10px; }
}
</style>
<style>
:root { --el-color-primary: #3EAF7C; }
</style>
