<script setup lang="ts">
import { computed, ref, onMounted } from 'vue';
import { isIPv6 } from 'is-ip';
import { config } from '../../config/index';
import { CircleCheckFilled, CircleCloseFilled, CopyDocument, Location, Loading, WarningFilled, InfoFilled, Terminal, Connection, Lock, Odometer, Search, List, Grid } from '@element-plus/icons-vue';

// 工具快捷入口
const tools = [
  { to: '/dns', name: 'DNS解析', desc: 'A / AAAA / MX', icon: Connection, color: 'green' },
  { to: '/ssl', name: 'SSL检查', desc: '证书有效期', icon: Lock, color: 'blue' },
  { to: '/tcping', name: 'TCPing', desc: '端口连通性', icon: Odometer, color: 'orange' },
  { to: '/whois', name: 'Whois查询', desc: '域名注册信息', icon: Search, color: 'purple' },
  { to: '/batch', name: '批量查询', desc: '多 IP 归属地', icon: List, color: 'red' },
  { to: '/cidr', name: '子网计算', desc: 'IPv4 / IPv6 网段', icon: Grid, color: 'cyan' },
];

// 复制 IP
const copied4 = ref(false);
const copied6 = ref(false);
async function copyText(ip: string, target: 'v4' | 'v6') {
  if (!ip) return;
  try {
    await navigator.clipboard.writeText(ip);
  } catch {
    const ta = document.createElement('textarea');
    ta.value = ip;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
  }
  if (target === 'v4') {
    copied4.value = true;
    setTimeout(() => (copied4.value = false), 1500);
  } else {
    copied6.value = true;
    setTimeout(() => (copied6.value = false), 1500);
  }
}

// 网络状态
const netStatusClass = computed(() => {
  if (yourIPv6.value && isIPv6(yourIPv6.value)) return 'v6';
  if (yourIPv4.value && isIPv4(yourIPv4.value)) return 'v4';
  return 'loading';
});
const netStatusText = computed(() => {
  if (yourIPv6.value && isIPv6(yourIPv6.value)) return '您的网络 IPv6 优先';
  if (yourIPv4.value && isIPv4(yourIPv4.value)) return '您的网络 IPv4 优先';
  return '正在检测您的网络...';
});


import { highlightCode } from '../../utils/shiki';
import { isIPv4 } from '../../utils/tools';
const route = useRoute();
const canonicalUrl = computed(() => new URL(route.path, config.siteUrl).toString());

useHead({
  title: 'ipchk.cn | IP查询工具 | IPv4/IPv6地址查询与网络测试平台',
  titleTemplate: '%s',
  link: [
    { rel: 'canonical', href: canonicalUrl.value }
  ],
  meta: [
    { name: 'description', content: 'ipchk.cn提供专业的IP查询服务,支持IPv4和IPv6地址在线查询、归属地定位、网络测速、DNS解析、SSL证书检测、TCPing测试等多种网络工具,致力于推进IPv6规模部署和应用,打造去中心化的IP查询平台' },
    { name: 'keywords', content: 'ipv6,ipv4,ip查询,ipv6查询,ipv4查询,ipv6地址查询,ipv4地址查询,网络测速,DNS查询,SSL检测,TCPing,IP归属地,IPv6优先' },
    { property: 'og:title', content: 'ipchk.cn - 专业IP查询与网络测试工具平台' },
    { property: 'og:description', content: '提供IPv4/IPv6地址查询、网络测速、DNS解析、SSL检测等全方位网络诊断工具,助力IPv6普及与部署' },
    { property: 'og:image', content: `${config.siteUrl}favicon.svg` },
    { property: 'og:type', content: 'website' },
    { property: 'og:url', content: canonicalUrl.value },
    { name: 'twitter:card', content: 'summary_large_image' },
  ],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'WebApplication',
        name: 'ipchk.cn',
        description: 'ipchk.cn提供专业的IP查询服务,支持IPv4和IPv6地址在线查询、归属地定位、网络测速、DNS解析、SSL证书检测、TCPing测试等多种网络工具,致力于推进IPv6规模部署和应用,打造去中心化的IP查询平台',
        url: canonicalUrl.value,
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'Any',
        offers: {
          '@type': 'Offer',
          price: '0',
          priceCurrency: 'CNY'
        },
        featureList: 'IPv4地址查询,IPv6地址查询,IP归属地定位,网络测速,DNS解析,SSL证书检测,TCPing测试,IPv6优先检测',
        about: [
          {
            '@type': 'Thing',
            name: 'IP地址查询',
            description: '通过特定的IP地址获取相关的地理位置、运营商、网络类型等信息的技术服务。IPv4地址是32位地址格式,IPv6地址是128位地址格式,IPv6能够提供更大的地址空间,解决IPv4地址枯竭问题。'
          },
          {
            '@type': 'Thing',
            name: 'IPv6优先检测',
            description: '当访问一个同时支持IPv4和IPv6的双栈网站时,如果网络IPv6优先,系统会优先使用IPv6地址进行连接。通过访问双栈测试域名来判断网络优先级。'
          },
          {
            '@type': 'Thing',
            name: 'IPv6部署',
            description: 'IPv6是全球下一代互联网协议标准,相比IPv4具有更大的地址空间、更好的安全性、更高的网络效率。国家正在大力推进IPv6规模部署和应用,以适应未来互联网发展需求。'
          }
        ]
      })
    }
  ]
});

const code = `
# 请勿用于商业用途，仅供个人测试学习之用，请遵守中国法律法规
# 查询本机公网 IP
curl https://ipchk.cn

# 查询本机公网 IPv4
curl https://4.ipchk.cn

# 查询本机公网 IPv6
curl https://6.ipchk.cn

# 查询指定 IP 的归属地
curl https://ipchk.cn/v1/location/8.8.8.8
`.trim(); // 关键：去掉首尾多余的空行
const highlightedCode = ref('');
const copied = ref(false);

// 复制命令行示例
async function copyCode() {
  try {
    await navigator.clipboard.writeText(code);
    copied.value = true;
    setTimeout(() => (copied.value = false), 2000);
  } catch {
    // 兼容非 HTTPS 环境
    const ta = document.createElement('textarea');
    ta.value = code;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    copied.value = true;
    setTimeout(() => (copied.value = false), 2000);
  }
}

const ipAddress = ref('');
const yourIPv4 = ref('');
const yourIPv6 = ref('');

// 带超时的请求（防止 IPv6 跨境链路挂起阻塞页面）
// 注意：用原生 fetch 而非 $fetch——$fetch 依赖 Nuxt auto-import，本地构建时 import 可能丢失导致 ReferenceError
function fetchWithTimeout(url: string, ms = 5000): Promise<string> {
  return Promise.race([
    fetch(url).then((r) => r.text()),
    new Promise<string>((_, reject) => setTimeout(() => reject(new Error('请求超时')), ms))
  ]);
}

// WebRTC 获取本机公网 IPv6（兜底方案：解决手机有 IPv6 但到服务器 IPv6 连接不通的场景）
function detectIPv6ViaWebRTC(): Promise<string> {
  return new Promise((resolve) => {
    try {
      if (typeof RTCPeerConnection === 'undefined') {
        resolve('');
        return;
      }
      const pc = new RTCPeerConnection({ iceServers: [] });
      pc.createDataChannel('');
      pc.onicecandidate = (e: any) => {
        if (e.candidate) {
          const match = e.candidate.candidate.match(/([0-9a-f:]+)/i);
          if (match) {
            const ip = match[1];
            // 只接受公网 IPv6（排除链路本地/回环）
            if (ip.includes(':') && !ip.startsWith('fe80') && ip !== '::1' && !ip.startsWith('fc') && !ip.startsWith('fd')) {
              resolve(ip);
            }
          }
        } else {
          resolve('');
        }
      };
      pc.createOffer().then((o: any) => pc.setLocalDescription(o)).catch(() => resolve(''));
      // 3 秒超时兜底
      setTimeout(() => resolve(''), 3000);
    } catch {
      resolve('');
    }
  });
}

onMounted(async () => {
  try {
    highlightedCode.value = await highlightCode(code, 'bash');
  } catch {
    highlightedCode.value = '';
  }

  // IPv4/双栈结果一到就立即显示，不等 IPv6（跨境 IPv6 链路常不通会挂超时）
  fetchWithTimeout(config.DualStackAPI, 3000).then((ip) => {
    ipAddress.value = ip.trim();
  }).catch(() => {});

  fetchWithTimeout(config.v4OnlyAPI, 3000).then((ip) => {
    if (isIPv4(ip.trim())) {
      yourIPv4.value = ip.trim();
    }
  }).catch(() => {});

  // IPv6 后台并行探测：API + WebRTC 谁先出用谁，不阻塞 IPv4 显示
  const rtcV6Promise = detectIPv6ViaWebRTC();
  const v6FromApi = fetchWithTimeout(config.v6OnlyAPI, 2500)
    .then((ip) => (isIPv6(ip.trim()) ? ip.trim() : ''))
    .catch(() => '');
  const [v6a, v6b] = await Promise.all([v6FromApi, rtcV6Promise]);
  const v6 = v6a || v6b;
  if (v6) {
    yourIPv6.value = v6;
  }
});
</script>


<template>
  <!-- Hero 区 -->
  <div class="hero">
    <div class="hero-glow hero-glow-1"></div>
    <div class="hero-glow hero-glow-2"></div>
    <div class="hero-content">
      <h1 class="hero-title">IP 查询</h1>
      <p class="hero-subtitle">致力于 IP 查询去中心化，推进 IPv6 规模部署和应用</p>
      <div class="hero-badges">
        <span class="hero-badge"><span class="hero-badge-dot"></span>IPv4 / IPv6 双栈</span>
        <span class="hero-badge hero-badge-free">免费 · 无需注册</span>
      </div>
    </div>
  </div>

  <div class="content">
    <!-- IP 展示卡片 -->
    <div class="ip-cards">
      <div class="ip-card">
        <div class="ip-card-header">
          <span class="ip-tag ipv4-tag">IPv4</span>
          <span class="ip-card-status"><span class="status-dot status-dot-green"></span>本机公网地址</span>
          <button v-if="yourIPv4" class="ip-copy-btn" @click="copyText(yourIPv4, 'v4')">
            <el-icon><CopyDocument /></el-icon>{{ copied4 ? '已复制' : '复制' }}
          </button>
        </div>
        <div class="ip-card-body">
          <template v-if="yourIPv4">
            <span class="ip-addr">{{ yourIPv4 }}</span>
            <RouterLink class="ip-loc-btn" :to="`/location?ip=${yourIPv4}`" target="_blank">
              <el-icon><Location /></el-icon>查询归属地
            </RouterLink>
          </template>
          <div v-else class="ip-loading"><el-icon class="is-loading"><Loading /></el-icon>获取中...</div>
        </div>
      </div>

      <div class="ip-card">
        <div class="ip-card-header">
          <span class="ip-tag ipv6-tag">IPv6</span>
          <span class="ip-card-status"><span class="status-dot status-dot-purple"></span>本机公网地址</span>
          <button v-if="yourIPv6" class="ip-copy-btn" @click="copyText(yourIPv6, 'v6')">
            <el-icon><CopyDocument /></el-icon>{{ copied6 ? '已复制' : '复制' }}
          </button>
        </div>
        <div class="ip-card-body">
          <template v-if="yourIPv6">
            <span class="ip-addr ip-addr-v6">{{ yourIPv6 }}</span>
            <RouterLink class="ip-loc-btn" :to="`/location?ip=${yourIPv6}`" target="_blank">
              <el-icon><Location /></el-icon>查询归属地
            </RouterLink>
          </template>
          <template v-else>
            <div class="ip-loading ip-loading-empty"><el-icon><WarningFilled /></el-icon>未检测到 IPv6 地址</div>
            <RouterLink class="ip-loc-btn ip-loc-btn-ghost" to="/doc/user/enable_ipv6" target="_blank">如何开启 IPv6</RouterLink>
          </template>
        </div>
      </div>
    </div>

    <!-- 网络状态徽章 -->
    <div class="net-status" :class="'net-' + netStatusClass">
      <el-icon v-if="netStatusClass === 'loading'" class="is-loading"><Loading /></el-icon>
      <el-icon v-else-if="netStatusClass === 'v6'"><CircleCheckFilled /></el-icon>
      <el-icon v-else><CircleCloseFilled /></el-icon>
      <span>{{ netStatusText }}</span>
    </div>

    <!-- 工具快捷入口 -->
    <div class="tool-grid">
      <RouterLink v-for="tool in tools" :key="tool.to" :to="tool.to" class="tool-item">
        <el-icon class="tool-icon" :class="'tool-icon-' + tool.color"><component :is="tool.icon" /></el-icon>
        <span class="tool-name">{{ tool.name }}</span>
        <span class="tool-desc">{{ tool.desc }}</span>
      </RouterLink>
    </div>

    <!-- 命令行示例 -->
    <div class="code-card">
      <div class="code-card-header">
        <div class="code-card-title-wrap">
          <span class="code-card-title"><el-icon><Terminal /></el-icon>命令行示例</span>
          <span class="code-card-sub">使用 curl 快速获取本机 IP</span>
        </div>
        <button class="code-copy-btn" :class="{ 'copied': copied }" @click="copyCode">{{ copied ? '已复制 ✓' : '复制' }}</button>
      </div>
      <div v-if="highlightedCode" v-html="highlightedCode" class="code-block"></div>
      <div v-else class="code-block code-block--fallback">{{ code }}</div>
    </div>

    <blockquote class="ipv6-tip">
      <el-icon><InfoFilled /></el-icon>
      <span>手机默认开启 IPv6，宽带开启 IPv6 请咨询运营商或自行搜索相关教程。</span>
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";
.el-menu--horizontal > .el-menu-item:nth-child(1) {
  margin-right: auto;
}

/* ===== Hero 区 ===== */
.hero {
  position: relative;
  text-align: center;
  padding: 56px 20px 40px;
  overflow: hidden;
}
.hero-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.28;
  pointer-events: none;
  z-index: 0;
}
.hero-glow-1 {
  width: 460px;
  height: 460px;
  background: #3EAF7C;
  top: -200px;
  left: 50%;
  transform: translateX(-130%);
}
.hero-glow-2 {
  width: 400px;
  height: 400px;
  background: #7C4DFF;
  top: -180px;
  right: 50%;
  transform: translateX(130%);
}
html.dark .hero-glow { opacity: 0.16; }
.hero-content { position: relative; z-index: 1; }
.hero-title {
  font-size: 2.6em;
  font-weight: 800;
  margin: 0 0 10px;
  background: linear-gradient(135deg, #2E9A68, #3EAF7C 45%, #7C4DFF);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  letter-spacing: 1px;
}
.hero-subtitle {
  margin: 0 0 18px;
  font-size: 1.05em;
  color: #6b7280;
}
html.dark .hero-subtitle { color: #9ca3af; }
.hero-badges {
  display: flex;
  justify-content: center;
  gap: 10px;
  flex-wrap: wrap;
}
.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85em;
  padding: 5px 14px;
  border-radius: 999px;
  background: rgba(62, 175, 124, 0.1);
  border: 1px solid rgba(62, 175, 124, 0.3);
  color: #2E9A68;
  font-weight: 600;
}
.hero-badge-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #3EAF7C;
  animation: pulse 1.8s ease-in-out infinite;
}
.hero-badge-free {
  background: rgba(124, 77, 255, 0.08);
  border-color: rgba(124, 77, 255, 0.28);
  color: #6a3de0;
}
@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.45; transform: scale(0.8); }
}

/* ===== IP 卡片区 ===== */
.ip-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
@media (max-width: 768px) {
  .ip-cards { grid-template-columns: 1fr; }
}

.ip-card {
  position: relative;
  background: #ffffff;
  border: 1px solid #e8ecf0;
  border-radius: 16px;
  padding: 18px 20px;
  box-shadow: 0 2px 12px rgba(31, 45, 61, 0.06);
  transition: box-shadow 0.3s ease, transform 0.2s ease, border-color 0.3s ease;
}
html.dark .ip-card {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.12);
  box-shadow: none;
}
.ip-card:hover {
  box-shadow: 0 8px 28px rgba(31, 45, 61, 0.1);
  transform: translateY(-2px);
}
html.dark .ip-card:hover {
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.35);
}

.ip-card-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.ip-tag {
  font-size: 0.78em;
  font-weight: 700;
  letter-spacing: 0.5px;
  padding: 3px 12px;
  border-radius: 999px;
  color: #fff;
}
.ipv4-tag { background: linear-gradient(135deg, #3EAF7C, #2E9A68); box-shadow: 0 2px 8px rgba(62, 175, 124, 0.35); }
.ipv6-tag { background: linear-gradient(135deg, #7C4DFF, #6a3de0); box-shadow: 0 2px 8px rgba(124, 77, 255, 0.35); }
.ip-card-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85em;
  color: #909399;
  flex: 1;
}
.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.status-dot-green { background: #3EAF7C; }
.status-dot-purple { background: #7C4DFF; }

.ip-copy-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.8em;
  padding: 4px 12px;
  border-radius: 999px;
  border: 1px solid #dcdfe6;
  background: transparent;
  color: #606266;
  cursor: pointer;
  transition: all 0.25s ease;
}
.ip-copy-btn:hover {
  border-color: #3EAF7C;
  color: #3EAF7C;
  background: rgba(62, 175, 124, 0.06);
}

.ip-card-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}
.ip-addr {
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', Consolas, Monaco, monospace;
  font-size: 1.6em;
  font-weight: 700;
  background: linear-gradient(135deg, #3EAF7C, #2E9A68);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  word-break: break-all;
}
.ip-addr-v6 {
  background: linear-gradient(135deg, #7C4DFF, #9a6bff);
  -webkit-background-clip: text;
  background-clip: text;
}
.ip-loading {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #909399;
  font-size: 1.1em;
  padding: 6px 0;
}
.ip-loading-empty { font-size: 1em; }
html.dark .ip-loading-empty { color: #8a8f98; }
.is-loading { animation: rotating 1.2s linear infinite; }
@keyframes rotating {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.ip-loc-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.95em;
  color: #3EAF7C;
  border: 1px solid rgba(62, 175, 124, 0.45);
  border-radius: 8px;
  padding: 6px 16px;
  text-decoration: none;
  white-space: nowrap;
  transition: all 0.25s ease;
}
.ip-loc-btn:hover {
  background: #3EAF7C;
  color: #fff;
}
.ip-loc-btn-ghost {
  color: #909399;
  border-color: #dcdfe6;
}
.ip-loc-btn-ghost:hover {
  background: #909399;
  border-color: #909399;
  color: #fff;
}

/* ===== 网络状态徽章 ===== */
.net-status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 22px;
  border-radius: 999px;
  font-size: 0.95em;
  font-weight: 600;
  margin: 22px 0 4px;
  justify-self: center;
}
.net-v4 {
  background: rgba(62, 175, 124, 0.1);
  border: 1px solid rgba(62, 175, 124, 0.3);
  color: #2E9A68;
}
.net-v6 {
  background: rgba(124, 77, 255, 0.1);
  border: 1px solid rgba(124, 77, 255, 0.3);
  color: #6a3de0;
}
.net-loading {
  background: rgba(144, 147, 153, 0.1);
  border: 1px solid rgba(144, 147, 153, 0.3);
  color: #909399;
}

/* ===== 工具快捷入口 ===== */
.tool-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 12px;
  margin: 26px 0 8px;
}
@media (max-width: 900px) {
  .tool-grid { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 480px) {
  .tool-grid { grid-template-columns: repeat(2, 1fr); }
}
.tool-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 18px 8px 14px;
  border-radius: 14px;
  border: 1px solid #e8ecf0;
  background: #fff;
  text-decoration: none;
  transition: all 0.25s ease;
}
html.dark .tool-item {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
}
.tool-item:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(31, 45, 61, 0.1);
  border-color: rgba(62, 175, 124, 0.4);
}
html.dark .tool-item:hover {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
}
.tool-icon {
  font-size: 1.7em;
}
.tool-icon-green { color: #3EAF7C; }
.tool-icon-blue { color: #409EFF; }
.tool-icon-orange { color: #E6A23C; }
.tool-icon-purple { color: #7C4DFF; }
.tool-icon-red { color: #F56C6C; }
.tool-icon-cyan { color: #00b3a4; }
.tool-name {
  font-size: 0.95em;
  font-weight: 600;
  color: #303133;
}
html.dark .tool-name { color: #e5e7eb; }
.tool-desc {
  font-size: 0.75em;
  color: #909399;
}

/* ===== 命令行示例 ===== */
.code-card {
  margin-top: 1.2rem;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid #e8ecf0;
  background: #fff;
}
html.dark .code-card {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.1);
}
.code-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid #eef1f4;
}
html.dark .code-card-header { border-color: rgba(255, 255, 255, 0.08); }
.code-card-title-wrap {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.code-card-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.9em;
  font-weight: 700;
  color: #3EAF7C;
  letter-spacing: 0.5px;
}
.code-card-sub {
  font-size: 0.78em;
  color: #a8abb2;
}
.code-copy-btn {
  font-size: 0.85em;
  padding: 4px 14px;
  border-radius: 999px;
  border: 1px solid rgba(62, 175, 124, 0.4);
  background: transparent;
  color: #3EAF7C;
  cursor: pointer;
  transition: all 0.25s ease;
}
.code-copy-btn:hover {
  background: #3EAF7C;
  color: #fff;
}
.code-copy-btn.copied {
  background: #3EAF7C;
  color: #fff;
}

.code-block {
  margin-top: 0;
  padding: 1rem 1.1rem;
  border-radius: 0;
  overflow-x: auto;
  max-width: 100%;
  white-space: pre-wrap;
  overflow-wrap: break-word;
}
.code-block--fallback {
  background: #1e1e2e;
  border: none;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', 'Monaco', 'Courier New', monospace !important;
  color: #e5e7eb;
  font-size: 0.9em;
  line-height: 1.6;
}

/* ===== IPv6 提示 ===== */
.ipv6-tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 18px 0 0;
  padding: 12px 16px;
  border-radius: 10px;
  background: rgba(62, 175, 124, 0.06);
  border: 1px solid rgba(62, 175, 124, 0.18);
  border-left: 4px solid #3EAF7C;
  color: #6b7280;
  font-size: 0.9em;
}
html.dark .ipv6-tip { color: #9ca3af; }
.ipv6-tip .el-icon {
  color: #3EAF7C;
  margin-top: 3px;
  flex-shrink: 0;
}

@media (max-width: 768px) {
  .hero { padding: 40px 16px 28px; }
  .hero-title { font-size: 2em; }
  .ip-addr { font-size: 1.25em; }
  .ip-card-body { flex-direction: column; align-items: flex-start; }
  .code-block { padding: 0.8rem; font-size: 0.8em; }
}
</style>
<style>
:root {
  --el-color-primary: #3EAF7C;
}
</style>