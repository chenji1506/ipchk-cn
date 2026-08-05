<script setup lang="ts">
import { computed, ref, onMounted } from 'vue';
import { isIPv6 } from 'is-ip';
import { config } from '../../config/index';
import { CircleCheckFilled, CircleCloseFilled } from '@element-plus/icons-vue';
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

  // 三路独立探测：v4 强制 IPv4，v6 强制 IPv6，dual 判断优先级
  const [dualStack, ipV4, ipV6] = await Promise.allSettled([
    $fetch<string>(config.DualStackAPI),
    $fetch<string>(config.v4OnlyAPI),
    $fetch<string>(config.v6OnlyAPI)
  ]);

  if (dualStack.status === 'fulfilled') {
    ipAddress.value = dualStack.value;
  }
  if (ipV4.status === 'fulfilled' && isIPv4(ipV4.value.trim())) {
    yourIPv4.value = ipV4.value;
  }
  if (ipV6.status === 'fulfilled' && isIPv6(ipV6.value.trim())) {
    yourIPv6.value = ipV6.value;
  }
  // 兜底：API 探测不到 IPv6 时，用 WebRTC 读本机网卡 IPv6
  if (!yourIPv6.value) {
    const v6 = await detectIPv6ViaWebRTC();
    if (v6) {
      yourIPv6.value = v6;
    }
  }
});
</script>


<template>
  <div class="title">
    <header>
      <h1>IP查询</h1>
      <p>致力于IP查询去中心化,推进 IPv6 规模部署和应用</p>
    </header>
  </div>
  <div class="content">
    <!-- IP 展示卡片 -->
    <div class="ip-card">
      <div class="ip-card-header">
        <span class="ip-tag ipv4-tag">IPv4</span>
        <span class="ip-card-status">本机公网地址</span>
      </div>
      <div class="ip-card-body">
        <span class="ip-addr">{{ yourIPv4 }}</span>
        <RouterLink class="ip-loc-btn" :to="`/location?ip=${yourIPv4}`" target="_blank">查询归属地</RouterLink>
      </div>
    </div>

    <div class="ip-card">
      <div class="ip-card-header">
        <span class="ip-tag ipv6-tag">IPv6</span>
        <span class="ip-card-status">本机公网地址</span>
      </div>
      <div class="ip-card-body">
        <template v-if="yourIPv6">
          <span class="ip-addr">{{ yourIPv6 }}</span>
          <RouterLink class="ip-loc-btn" :to="`/location?ip=${yourIPv6}`" target="_blank">查询归属地</RouterLink>
        </template>
        <template v-else>
          <span class="ip-addr ip-addr-empty">未检测到 IPv6 地址</span>
          <RouterLink class="ip-loc-btn ip-loc-btn-ghost" to="/doc/user/enable_ipv6" target="_blank">查看如何开启 IPv6</RouterLink>
        </template>
      </div>
    </div>
    <div style="font-size: 1.5em;">
      <h3 v-if="yourIPv6 && isIPv6(yourIPv6)"><el-icon><CircleCheckFilled style="color: lightgreen;"/></el-icon>您的网络IPv6优先</h3>
      <h3 v-else-if="yourIPv4 && isIPv4(yourIPv4)"><el-icon><CircleCloseFilled style="color: red;"/></el-icon>您的网络IPv4优先</h3>
      <h3 v-else><el-icon><CircleCloseFilled /></el-icon>查询中，请稍后</h3>
    </div>
     <blockquote>
      手机默认开启 IPv6，宽带开启 IPv6 请咨询运营商或自行搜索相关教程。
    </blockquote>

    <div class="code-card">
      <div class="code-card-header">
        <span class="code-card-title">命令行示例</span>
        <button class="code-copy-btn" :class="{ 'copied': copied }" @click="copyCode">{{ copied ? '已复制 ✓' : '复制' }}</button>
      </div>
      <div v-if="highlightedCode" v-html="highlightedCode" class="code-block"></div>
      <div v-else class="code-block code-block--fallback">{{ code }}</div>
    </div>
  </div>

</template>
<style scoped>
@import "../style.css";
.el-menu--horizontal > .el-menu-item:nth-child(1) {
  margin-right: auto;
}

/* IP 展示卡片 */
.ip-card {
  background: linear-gradient(135deg, rgba(62, 175, 124, 0.07), rgba(62, 175, 124, 0.02));
  border: 1px solid rgba(62, 175, 124, 0.22);
  border-radius: 12px;
  padding: 14px 18px;
  margin-bottom: 12px;
  transition: box-shadow 0.3s ease, transform 0.2s ease;
}
html.dark .ip-card {
  background: linear-gradient(135deg, rgba(62, 175, 124, 0.12), rgba(62, 175, 124, 0.03));
  border-color: rgba(62, 175, 124, 0.3);
}
.ip-card:hover {
  box-shadow: 0 4px 18px rgba(62, 175, 124, 0.18);
  transform: translateY(-1px);
}

.ip-card-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.ip-tag {
  font-size: 0.78em;
  font-weight: 700;
  letter-spacing: 0.5px;
  padding: 2px 10px;
  border-radius: 5px;
  color: #fff;
}
.ipv4-tag { background: #3EAF7C; }
.ipv6-tag { background: #7C4DFF; }
.ip-card-status {
  font-size: 0.85em;
  color: #909399;
}

.ip-card-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
}
.ip-addr {
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', Consolas, Monaco, monospace;
  font-size: 1.55em;
  font-weight: 700;
  background: linear-gradient(135deg, #3EAF7C, #2E9A68);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  word-break: break-all;
}
.ip-addr-empty {
  -webkit-text-fill-color: #909399;
  font-size: 1.2em;
  font-weight: 500;
}
html.dark .ip-addr-empty {
  -webkit-text-fill-color: #8a8f98;
}
.ip-loc-btn {
  font-size: 0.95em;
  color: #3EAF7C;
  border: 1px solid rgba(62, 175, 124, 0.45);
  border-radius: 6px;
  padding: 5px 16px;
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

@media (max-width: 768px) {
  .ip-addr {
    font-size: 1.2em;
  }
  .ip-card-body {
    flex-direction: column;
    align-items: flex-start;
  }
}

.code-card {
  margin-top: 1rem;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(62, 175, 124, 0.25);
  background: rgba(62, 175, 124, 0.04);
}

.code-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  border-bottom: 1px solid rgba(62, 175, 124, 0.18);
}

.code-card-title {
  font-size: 0.9em;
  font-weight: 600;
  color: #3EAF7C;
  letter-spacing: 0.5px;
}

.code-copy-btn {
  font-size: 0.85em;
  padding: 3px 14px;
  border-radius: 6px;
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
  padding: 1rem;
  border-radius: 0;
  overflow-x: auto;
  max-width: 100%;
  white-space: pre-wrap;
  overflow-wrap: break-word;
}

.code-block--fallback {
  background: rgb(48, 46, 46);
  border: 1px solid rgba(62, 175, 124, 0.18);
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', 'Monaco', 'Courier New', monospace !important;
  color: rgb(255, 255, 255);
}

@media (max-width: 768px) {
  .code-block {
    padding: 0.75rem;
    font-size: 0.8em;
  }
}
</style>
<style>
:root {
  --el-color-primary: #3EAF7C;
}
</style>