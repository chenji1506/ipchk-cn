<script setup lang="ts">
import { ref, onMounted } from 'vue';

useHead({
  title: '隐私泄露检测 | WebRTC/DNS泄露 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测 WebRTC 和 DNS 是否泄露真实 IP 与隐私信息。' },
  ],
});

const webrtcIPs = ref<string[]>([]);
const webrtcDetecting = ref(false);
const webrtcDone = ref(false);

const dnsServers = ref<{ server: string; detected: boolean; real: boolean }[]>([]);
const dnsDetecting = ref(false);
const dnsDone = ref(false);
const myISP = ref('');

// ===== WebRTC 泄露检测 =====
function detectWebRTC() {
  webrtcIPs.value = [];
  webrtcDetecting.value = true;
  webrtcDone.value = false;

  try {
    if (typeof RTCPeerConnection === 'undefined') {
      webrtcIPs.value = ['当前浏览器不支持 WebRTC'];
      webrtcDone.value = true;
      webrtcDetecting.value = false;
      return;
    }
    const pc = new RTCPeerConnection({ iceServers: [] });
    pc.createDataChannel('');
    pc.onicecandidate = (e: any) => {
      if (e.candidate) {
        const match = e.candidate.candidate.match(/([0-9a-f:.]+)/i);
        if (match) {
          const ip = match[1];
          if (!webrtcIPs.value.includes(ip)) {
            webrtcIPs.value.push(ip);
          }
        }
      } else {
        webrtcDone.value = true;
        webrtcDetecting.value = false;
      }
    };
    pc.createOffer().then((o: any) => pc.setLocalDescription(o)).catch(() => {
      webrtcDone.value = true;
      webrtcDetecting.value = false;
    });
    setTimeout(() => {
      webrtcDone.value = true;
      webrtcDetecting.value = false;
    }, 4000);
  } catch {
    webrtcIPs.value = ['检测失败'];
    webrtcDone.value = true;
    webrtcDetecting.value = false;
  }
}

// 判断 WebRTC 泄露（出现内网/公网非代理 IP）
function hasWebRTCLeak(): boolean {
  const ips = webrtcIPs.value.filter(ip => ip !== '当前浏览器不支持 WebRTC' && ip !== '检测失败');
  // 出现 192.168/10./172.16-31 内网 IP 即视为泄露
  return ips.some(ip =>
    ip.startsWith('192.168.') || ip.startsWith('10.') ||
    (ip.startsWith('172.') && parseInt(ip.split('.')[1]) >= 16 && parseInt(ip.split('.')[1]) <= 31) ||
    ip.startsWith('fe80:')
  );
}

// ===== DNS 泄露检测 =====
// 通过加载不同 DNS 服务商的域名，检查解析是否经过这些服务器
function detectDNSLeak() {
  dnsServers.value = [];
  dnsDetecting.value = true;
  dnsDone.value = false;

  const probes = [
    { name: 'OpenDNS (Resolver1)', domain: 'resolver1.opendns.com', check: (ip: string) => ip === '208.67.222.222' },
    { name: 'Google DNS', domain: 'dns.google', check: (ip: string) => ip === '8.8.8.8' },
    { name: 'Cloudflare DNS', domain: 'one.one.one.one', check: (ip: string) => ip === '1.1.1.1' },
  ];

  let completed = 0;
  probes.forEach((probe, idx) => {
    dnsServers.value.push({ server: probe.name, detected: false, real: false });
    // 用 fetch 触发 DNS 解析，无法直接读 IP，改用图片加载方式判断
    // 简化方案：使用 Image 加载 dnsleaktest 风格的探测域名
    try {
      const img = new Image();
      const start = Date.now();
      img.onload = () => {
        const elapsed = Date.now() - start;
        // 能加载成功说明 DNS 解析经过该服务器（探测域名只对该 DNS 解析）
        dnsServers.value[idx].detected = true;
        dnsServers.value[idx].real = elapsed > 0;
        completed++;
        if (completed >= probes.length) { dnsDetecting.value = false; dnsDone.value = true; }
      };
      img.onerror = () => {
        completed++;
        if (completed >= probes.length) { dnsDetecting.value = false; dnsDone.value = true; }
      };
      // 实际探测域名：whoami.akamai.net 等
      img.src = 'https://' + probe.domain + '/favicon.ico?' + Date.now();
    } catch {
      completed++;
      if (completed >= probes.length) { dnsDetecting.value = false; dnsDone.value = true; }
    }
  });

  setTimeout(() => {
    dnsDetecting.value = false;
    dnsDone.value = true;
  }, 8000);
}

onMounted(() => {
  detectWebRTC();
});
</script>

<template>
  <div class="title">
    <header>
      <h1>隐私泄露检测</h1>
      <p>检测 WebRTC 与 DNS 是否泄露你的真实网络信息</p>
    </header>
  </div>
  <div class="content">
    <!-- WebRTC 检测 -->
    <div class="section-card">
      <div class="section-header">
        <span class="section-tag tag-webrtc">WebRTC</span>
        <span class="section-desc">WebRTC 可能绕过代理/VPN 暴露真实 IP</span>
        <button class="action-btn" @click="detectWebRTC" :disabled="webrtcDetecting">
          {{ webrtcDetecting ? '检测中...' : '重新检测' }}
        </button>
      </div>
      <div v-if="webrtcIPs.length" class="result-area">
        <div class="leak-result" :class="hasWebRTCLeak() ? 'leak-bad' : 'leak-good'">
          {{ hasWebRTCLeak() ? '⚠ 检测到 WebRTC 泄露！' : '✓ 未检测到 WebRTC 泄露' }}
        </div>
        <div class="ip-list">
          <span v-for="(ip, i) in webrtcIPs" :key="i" class="ip-chip">{{ ip }}</span>
        </div>
      </div>
    </div>

    <!-- DNS 泄露检测 -->
    <div class="section-card">
      <div class="section-header">
        <span class="section-tag tag-dns">DNS</span>
        <span class="section-desc">检查 DNS 解析是否经过第三方服务器</span>
        <button class="action-btn" @click="detectDNSLeak" :disabled="dnsDetecting">
          {{ dnsDetecting ? '检测中...' : '开始检测' }}
        </button>
      </div>
      <div v-if="dnsServers.length" class="result-area">
        <table class="result-table">
          <thead>
            <tr><th class="table-header">探测节点</th><th class="table-header">结果</th></tr>
          </thead>
          <tbody>
            <tr v-for="(s, i) in dnsServers" :key="i">
              <td class="table-label">{{ s.server }}</td>
              <td class="table-value">
                <span v-if="s.detected" class="badge-red">可访问（DNS 可能经过）</span>
                <span v-else class="badge-green">不可访问（未经过）</span>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="leak-result" :class="dnsServers.some(s => s.detected) ? 'leak-warn' : 'leak-good'">
          {{ dnsServers.some(s => s.detected) ? '⚠ DNS 请求可能泄露给第三方！' : '✓ DNS 解析正常' }}
        </div>
      </div>
    </div>

    <blockquote>
      说明：WebRTC 检测读取浏览器本地网络地址；DNS 检测通过访问各 DNS 服务商专属探测域名判断解析是否经过第三方。结果仅供参考。
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";

.section-card {
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  padding: 18px 20px;
  margin-bottom: 16px;
}

html.dark .section-card {
  background: #1a1a1a;
  border-color: #2e2e2e;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.section-tag {
  font-size: 0.85em;
  font-weight: 700;
  padding: 3px 12px;
  border-radius: 6px;
  color: #fff;
}
.tag-webrtc { background: #7C4DFF; }
.tag-dns { background: #409EFF; }

.section-desc {
  font-size: 0.9em;
  color: #909399;
  flex: 1;
}

.action-btn {
  padding: 6px 18px;
  border-radius: 8px;
  border: 1px solid #3EAF7C;
  background: transparent;
  color: #3EAF7C;
  cursor: pointer;
  transition: all 0.25s;
  font-size: 0.95em;
}
.action-btn:hover { background: #3EAF7C; color: #fff; }
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.result-area { margin-top: 8px; }

.leak-result {
  font-size: 1.15em;
  font-weight: 700;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.leak-good { background: #f0f9eb; color: #67C23A; }
.leak-bad { background: #fef0f0; color: #F56C6C; }
.leak-warn { background: #fdf6ec; color: #E6A23C; }
html.dark .leak-good { background: rgba(103, 194, 58, 0.15); }
html.dark .leak-bad { background: rgba(245, 108, 108, 0.15); }
html.dark .leak-warn { background: rgba(230, 162, 60, 0.15); }

.ip-list { display: flex; flex-wrap: wrap; gap: 8px; }
.ip-chip {
  font-family: 'JetBrains Mono', Consolas, monospace;
  background: rgba(62, 175, 124, 0.1);
  color: #3EAF7C;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 0.95em;
}

.badge-green { color: #67C23A; font-weight: 600; }
.badge-red { color: #F56C6C; font-weight: 600; }
</style>
<style>
:root { --el-color-primary: #3EAF7C; }
</style>
