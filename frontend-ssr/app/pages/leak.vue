<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';

useHead({
  title: '隐私泄露检测 | WebRTC/DNS/指纹 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测 WebRTC 与 DNS 是否泄露真实 IP，展示浏览器暴露的指纹信息，比对时区与 IP 地理位置是否一致。' },
  ],
});

const runtimeConfig = useRuntimeConfig();
const apiBase = (runtimeConfig.public.apiBase as string) || 'https://ipchk.cn/';

// ==================== WebRTC 泄露检测 ====================
interface WebRTCGroups {
  privateV4: string[];   // 内网 IPv4（10./192.168./172.16-31/127./169.254.）
  publicV4: string[];    // 公网 IPv4
  publicV6: string[];    // 公网 IPv6（2001:、2408: 等）
  linkLocalV6: string[]; // 链路本地 IPv6（fe80:）
  mdns: string[];        // mDNS 混淆地址（*.local）
}
const webrtcIPs = ref<string[]>([]);
const webrtcGroups = ref<WebRTCGroups>({ privateV4: [], publicV4: [], publicV6: [], linkLocalV6: [], mdns: [] });
const webrtcDetecting = ref(false);
const webrtcDone = ref(false);
const webrtcUnsupported = ref(false);

function isPrivateV4(ip: string): boolean {
  const p = ip.split('.').map(Number);
  if (p.length !== 4 || p.some(isNaN)) return false;
  const [a, b] = p;
  return a === 10 || a === 127 || a === 0 ||
    (a === 192 && b === 168) ||
    (a === 172 && b >= 16 && b <= 31) ||
    (a === 169 && b === 254);
}

function classifyIP(ip: string): void {
  if (ip.includes('.local')) {
    if (!webrtcGroups.value.mdns.includes(ip)) webrtcGroups.value.mdns.push(ip);
  } else if (ip.includes(':')) {
    if (ip.toLowerCase().startsWith('fe80:')) {
      if (!webrtcGroups.value.linkLocalV6.includes(ip)) webrtcGroups.value.linkLocalV6.push(ip);
    } else {
      if (!webrtcGroups.value.publicV6.includes(ip)) webrtcGroups.value.publicV6.push(ip);
    }
  } else if (isPrivateV4(ip)) {
    if (!webrtcGroups.value.privateV4.includes(ip)) webrtcGroups.value.privateV4.push(ip);
  } else {
    if (!webrtcGroups.value.publicV4.includes(ip)) webrtcGroups.value.publicV4.push(ip);
  }
}

const webrtcLeakLevel = computed<'bad' | 'warn' | 'good'>(() => {
  const g = webrtcGroups.value;
  if (g.privateV4.length || g.publicV4.length) return 'bad';       // 真实 IPv4 泄露
  if (g.publicV6.length) return 'warn';                             // 公网 IPv6 泄露
  return 'good';
});

const webrtcLeakText = computed(() => {
  if (webrtcUnsupported.value) return '当前浏览器不支持 WebRTC';
  if (!webrtcDone.value) return '';
  const g = webrtcGroups.value;
  if (g.privateV4.length && g.publicV4.length) return '⚠ 检测到 WebRTC 泄露：内网与公网真实 IP 均已暴露！';
  if (g.privateV4.length) return '⚠ 检测到 WebRTC 泄露：真实内网 IP 已暴露！';
  if (g.publicV4.length) return '⚠ 检测到 WebRTC 泄露：真实公网 IP 已暴露（绕过代理/VPN）！';
  if (g.publicV6.length) return '⚠ WebRTC 暴露了公网 IPv6 地址';
  if (g.mdns.length) return '✓ 未检测到泄露（浏览器已用 mDNS 混淆本地地址）';
  return '✓ 未检测到 WebRTC 泄露';
});

function detectWebRTC() {
  webrtcIPs.value = [];
  webrtcGroups.value = { privateV4: [], publicV4: [], publicV6: [], linkLocalV6: [], mdns: [] };
  webrtcDetecting.value = true;
  webrtcDone.value = false;
  webrtcUnsupported.value = false;

  try {
    if (typeof RTCPeerConnection === 'undefined') {
      webrtcUnsupported.value = true;
      webrtcDetecting.value = false;
      webrtcDone.value = true;
      return;
    }
    const pc = new RTCPeerConnection({ iceServers: [] });
    pc.createDataChannel('');
    pc.onicecandidate = (e: any) => {
      if (e.candidate) {
        const c = e.candidate.candidate || '';
        // 仅 host candidate 暴露本机真实地址（srflx/prflx/relay 经 STUN/TURN，不算泄露）
        if (c.includes('typ host')) {
          // candidate:<foundation> <component> <transport> <priority> <address> <port> typ host ...
          const address = c.split(' ')[4];
          if (address && address !== '0.0.0.0' && !webrtcIPs.value.includes(address)) {
            webrtcIPs.value.push(address);
            classifyIP(address);
          }
        }
      } else {
        webrtcDetecting.value = false;
        webrtcDone.value = true;
        pc.close();
      }
    };
    pc.createOffer().then((o: any) => pc.setLocalDescription(o)).catch(() => {
      webrtcDetecting.value = false;
      webrtcDone.value = true;
      pc.close();
    });
    setTimeout(() => {
      webrtcDetecting.value = false;
      webrtcDone.value = true;
    }, 4000);
  } catch {
    webrtcUnsupported.value = true;
    webrtcDetecting.value = false;
    webrtcDone.value = true;
  }
}

// ==================== DNS 泄露检测 ====================
interface DnsProbe { server: string; detected: boolean; real: boolean; }
const dnsServers = ref<DnsProbe[]>([]);
const dnsDetecting = ref(false);
const dnsDone = ref(false);
const myISP = ref('');

function detectDNSLeak() {
  dnsServers.value = [];
  dnsDetecting.value = true;
  dnsDone.value = false;

  // 这些探测域名：仅当 DNS 解析经过对应公共解析器时才有特殊含义。
  // myip.opendns.com 仅 OpenDNS 用户能解析到出口 IP；whoami.akamai.net 返回请求者 IP。
  const probes = [
    { server: 'OpenDNS (myip.opendns.com)', domain: 'myip.opendns.com' },
    { server: 'Akamai (whoami.akamai.net)', domain: 'whoami.akamai.net' },
    { server: 'Google (o-o.myaddr.l.google.com)', domain: 'o-o.myaddr.l.google.com' },
  ];

  let completed = 0;
  probes.forEach((probe, idx) => {
    dnsServers.value.push({ server: probe.server, detected: false, real: false });
    try {
      const img = new Image();
      const start = Date.now();
      img.onload = () => {
        const elapsed = Date.now() - start;
        dnsServers.value[idx].detected = true;
        dnsServers.value[idx].real = elapsed > 0;
        completed++;
        if (completed >= probes.length) { dnsDetecting.value = false; dnsDone.value = true; }
      };
      img.onerror = () => {
        completed++;
        if (completed >= probes.length) { dnsDetecting.value = false; dnsDone.value = true; }
      };
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

// ==================== 浏览器指纹 ====================
interface Fingerprint {
  timezone: string;
  timezoneOffset: string;
  language: string;
  languages: string;
  platform: string;
  cores: string;
  memory: string;
  screen: string;
  colorDepth: string;
  doNotTrack: string;
  cookies: string;
  canvas: string;
  userAgent: string;
}
const fingerprint = ref<Fingerprint | null>(null);
const canvasComputing = ref(false);

function hashString(str: string): string {
  let h = 5381;
  for (let i = 0; i < str.length; i++) {
    h = ((h << 5) + h + str.charCodeAt(i)) >>> 0;
  }
  return h.toString(16).padStart(8, '0');
}

function getCanvasFingerprint(): Promise<string> {
  return new Promise((resolve) => {
    try {
      const canvas = document.createElement('canvas');
      canvas.width = 220;
      canvas.height = 60;
      const ctx = canvas.getContext('2d');
      if (!ctx) { resolve('不支持'); return; }
      ctx.textBaseline = 'top';
      ctx.font = '14px "Arial"';
      ctx.fillStyle = '#f60';
      ctx.fillRect(125, 1, 62, 20);
      ctx.fillStyle = '#069';
      ctx.fillText('ipchk.cn 指纹测试 <canvas>', 2, 15);
      ctx.fillStyle = 'rgba(102, 204, 0, 0.7)';
      ctx.fillText('ipchk.cn 指纹测试 <canvas>', 4, 17);
      resolve(hashString(canvas.toDataURL()));
    } catch {
      resolve('不支持');
    }
  });
}

// 获取真实操作系统平台。
// navigator.platform 已弃用（Chromium 冻结为固定值，无法反映真实 OS），
// 优先用 User-Agent Client Hints（Chromium 返回 "Windows"/"macOS"/"Linux" 等），再回退到 UA 解析。
function detectPlatform(): string {
  const nav = navigator as any;
  if (nav.userAgentData && typeof nav.userAgentData.platform === 'string' && nav.userAgentData.platform) {
    return nav.userAgentData.platform;
  }
  const ua = navigator.userAgent || '';
  if (/Windows/i.test(ua)) return 'Windows';
  if (/iPhone|iPad|iPod/i.test(ua)) return 'iOS';
  if (/Mac OS X|Macintosh/i.test(ua)) return 'macOS';
  if (/Android/i.test(ua)) return 'Android';
  if (/CrOS/i.test(ua)) return 'Chrome OS';
  if (/Linux/i.test(ua)) return 'Linux';
  return nav.platform || '未知';
}

async function collectFingerprint() {
  const nav = navigator as any;
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || '未知';
  const offsetMin = -new Date().getTimezoneOffset();
  const offsetStr = offsetMin === 0 ? 'UTC±0' : `UTC${offsetMin > 0 ? '+' : ''}${offsetMin / 60}`;
  canvasComputing.value = true;
  const canvas = await getCanvasFingerprint();
  canvasComputing.value = false;
  fingerprint.value = {
    timezone: tz,
    timezoneOffset: offsetStr,
    language: navigator.language || '未知',
    languages: navigator.languages ? navigator.languages.join(', ') : '未知',
    platform: detectPlatform(),
    cores: navigator.hardwareConcurrency ? String(navigator.hardwareConcurrency) : '未知',
    memory: nav.deviceMemory ? `约 ${nav.deviceMemory} GB（浏览器估算）` : '不可读（需 HTTPS 安全上下文）',
    screen: `${window.screen.width}×${window.screen.height}`,
    colorDepth: window.screen.colorDepth ? window.screen.colorDepth + ' bit' : '未知',
    doNotTrack: navigator.doNotTrack === '1' ? '已开启' : (navigator.doNotTrack === '0' ? '已关闭' : '未设置'),
    cookies: navigator.cookieEnabled ? '已启用' : '已禁用',
    canvas,
    userAgent: navigator.userAgent || '未知',
  };
}

// ==================== 时区 / IP 地理位置比对 ====================
const browserTZ = ref('');
const ipCountry = ref('');
const ipLocation = ref('');
const tzStatus = ref<'match' | 'mismatch' | 'unknown' | 'loading'>('loading');
const tzChecking = ref(false);

// 常见国家 → 所在时区洲（粗粒度判断时区是否与 IP 地理位置一致）
const countryContinent: Record<string, string> = {
  '中国': 'Asia', '中国大陆': 'Asia', '香港': 'Asia', '台湾': 'Asia', '澳门': 'Asia',
  '日本': 'Asia', '韩国': 'Asia', '朝鲜': 'Asia', '新加坡': 'Asia', '马来西亚': 'Asia',
  '泰国': 'Asia', '越南': 'Asia', '印度尼西亚': 'Asia', '菲律宾': 'Asia', '印度': 'Asia',
  '阿联酋': 'Asia', '沙特阿拉伯': 'Asia', '土耳其': 'Europe', '以色列': 'Asia',
  '美国': 'America', '加拿大': 'America', '墨西哥': 'America', '巴西': 'America', '阿根廷': 'America',
  '英国': 'Europe', '德国': 'Europe', '法国': 'Europe', '意大利': 'Europe', '西班牙': 'Europe',
  '荷兰': 'Europe', '瑞士': 'Europe', '瑞典': 'Europe', '波兰': 'Europe', '俄罗斯': 'Europe',
  '澳大利亚': 'Australia', '新西兰': 'Pacific',
};

function extractCountry(data: any): { country: string; detail: string } {
  if (!data || typeof data !== 'object') return { country: '', detail: '' };
  const values = Object.values(data);
  for (const src of values) {
    if (src && typeof src === 'object' && typeof (src as any).country === 'string') {
      const c = (src as any).country.trim();
      const city = (src as any).city || (src as any).administrative_area || '';
      if (c && c !== '局域网' && c !== '内网IP' && !c.toLowerCase().startsWith('error') && c !== 'not loaded') {
        return { country: c, detail: city ? `${c} · ${city}` : c };
      }
    }
  }
  return { country: '', detail: '' };
}

function tzMatchesContinent(tz: string, country: string): boolean {
  const continent = countryContinent[country];
  if (!continent) return false;
  // 俄罗斯跨欧洲/亚洲，单独放宽
  if (country === '俄罗斯') return tz.startsWith('Europe/') || tz.startsWith('Asia/');
  return tz.startsWith(continent + '/');
}

async function detectTimezoneMismatch() {
  tzChecking.value = true;
  tzStatus.value = 'loading';
  browserTZ.value = Intl.DateTimeFormat().resolvedOptions().timeZone || '未知';
  try {
    const data = await $fetch<any>(apiBase + 'v1/location', { method: 'GET', timeout: 8000 });
    const { country, detail } = extractCountry(data);
    ipCountry.value = country;
    ipLocation.value = detail;
    if (!country) {
      tzStatus.value = 'unknown';
    } else if (tzMatchesContinent(browserTZ.value, country)) {
      tzStatus.value = 'match';
    } else {
      tzStatus.value = 'mismatch';
    }
  } catch {
    tzStatus.value = 'unknown';
  } finally {
    tzChecking.value = false;
  }
}

// ===== HTTP 请求头检测 =====
const headersData = ref<any>(null);
const headersLoading = ref(false);

async function loadHeaders() {
  headersLoading.value = true;
  try {
    headersData.value = await $fetch(apiBase + 'v1/headers');
  } catch {
    headersData.value = null;
  } finally {
    headersLoading.value = false;
  }
}

onMounted(() => {
  detectWebRTC();
  collectFingerprint();
  detectTimezoneMismatch();
  loadHeaders();
});
</script>

<template>
  <div class="title">
    <header>
      <h1>隐私泄露检测</h1>
      <p>检测 WebRTC / DNS 是否泄露真实 IP，展示浏览器暴露的指纹，比对时区与 IP 地理位置</p>
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
      <div v-if="webrtcDone" class="result-area">
        <div class="leak-result" :class="webrtcLeakLevel === 'bad' ? 'leak-bad' : webrtcLeakLevel === 'warn' ? 'leak-warn' : 'leak-good'">
          {{ webrtcLeakText }}
        </div>
        <div v-if="webrtcGroups.privateV4.length" class="group-row">
          <span class="group-label leak-bad">内网 IPv4</span>
          <div class="ip-list">
            <span v-for="(ip, i) in webrtcGroups.privateV4" :key="'pv4' + i" class="ip-chip">{{ ip }}</span>
          </div>
        </div>
        <div v-if="webrtcGroups.publicV4.length" class="group-row">
          <span class="group-label leak-bad">公网 IPv4</span>
          <div class="ip-list">
            <span v-for="(ip, i) in webrtcGroups.publicV4" :key="'pb4' + i" class="ip-chip">{{ ip }}</span>
          </div>
        </div>
        <div v-if="webrtcGroups.publicV6.length" class="group-row">
          <span class="group-label leak-warn">公网 IPv6</span>
          <div class="ip-list">
            <span v-for="(ip, i) in webrtcGroups.publicV6" :key="'pb6' + i" class="ip-chip">{{ ip }}</span>
          </div>
        </div>
        <div v-if="webrtcGroups.linkLocalV6.length" class="group-row">
          <span class="group-label">链路本地 IPv6</span>
          <div class="ip-list">
            <span v-for="(ip, i) in webrtcGroups.linkLocalV6" :key="'ll6' + i" class="ip-chip">{{ ip }}</span>
          </div>
        </div>
        <div v-if="webrtcGroups.mdns.length" class="group-row">
          <span class="group-label leak-good">mDNS 混淆</span>
          <div class="ip-list">
            <span v-for="(ip, i) in webrtcGroups.mdns" :key="'mdns' + i" class="ip-chip">{{ ip }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- DNS 泄露检测 -->
    <div class="section-card">
      <div class="section-header">
        <span class="section-tag tag-dns">DNS</span>
        <span class="section-desc">检查 DNS 解析是否经公共解析器（仅能判定可达性）</span>
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
                <span v-if="s.detected" class="badge-green">可达</span>
                <span v-else class="badge-red">不可达</span>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="leak-result leak-warn" v-if="dnsDone">
          ⚠ 以上仅为探测域名可达性。精确的 DNS 泄露检测需服务端配合记录解析来源，本页仅供参考。
        </div>
      </div>
    </div>

    <!-- 时区 / IP 地理位置比对 -->
    <div class="section-card">
      <div class="section-header">
        <span class="section-tag tag-timezone">时区</span>
        <span class="section-desc">比对浏览器时区与出口 IP 地理位置是否一致</span>
        <button class="action-btn" @click="detectTimezoneMismatch" :disabled="tzChecking">
          {{ tzChecking ? '检测中...' : '重新检测' }}
        </button>
      </div>
      <div class="result-area">
        <div class="tz-row">
          <span class="tz-label">浏览器时区</span>
          <span class="tz-value">{{ browserTZ || '—' }}</span>
        </div>
        <div class="tz-row">
          <span class="tz-label">出口 IP 归属</span>
          <span class="tz-value">{{ ipLocation || (tzStatus === 'loading' ? '检测中...' : '未知') }}</span>
        </div>
        <div v-if="tzStatus !== 'loading'" class="leak-result"
             :class="tzStatus === 'match' ? 'leak-good' : tzStatus === 'mismatch' ? 'leak-bad' : 'leak-warn'">
          <template v-if="tzStatus === 'match'">✓ 时区与 IP 地理位置一致</template>
          <template v-else-if="tzStatus === 'mismatch'">⚠ 时区与 IP 地理位置不一致，可能泄露了真实所在地区</template>
          <template v-else>无法判定（IP 库未加载或网络异常）</template>
        </div>
      </div>
    </div>

    <!-- 浏览器指纹 -->
    <div class="section-card">
      <div class="section-header">
        <span class="section-tag tag-fingerprint">指纹</span>
        <span class="section-desc">浏览器默认暴露给网站的环境信息</span>
        <button class="action-btn" @click="collectFingerprint" :disabled="canvasComputing">
          {{ canvasComputing ? '计算中...' : '重新收集' }}
        </button>
      </div>
      <div v-if="fingerprint" class="result-area">
        <table class="result-table">
          <tbody>
            <tr><td class="table-label">时区</td><td class="table-value">{{ fingerprint.timezone }}（{{ fingerprint.timezoneOffset }}）</td></tr>
            <tr><td class="table-label">语言</td><td class="table-value">{{ fingerprint.language }}</td></tr>
            <tr><td class="table-label">接受语言</td><td class="table-value">{{ fingerprint.languages }}</td></tr>
            <tr><td class="table-label">平台</td><td class="table-value">{{ fingerprint.platform }}</td></tr>
            <tr><td class="table-label">CPU 逻辑核数</td><td class="table-value">{{ fingerprint.cores }}</td></tr>
            <tr><td class="table-label">设备内存</td><td class="table-value">{{ fingerprint.memory }}</td></tr>
            <tr><td class="table-label">屏幕分辨率</td><td class="table-value">{{ fingerprint.screen }}</td></tr>
            <tr><td class="table-label">色彩深度</td><td class="table-value">{{ fingerprint.colorDepth }}</td></tr>
            <tr><td class="table-label">Do Not Track</td><td class="table-value">{{ fingerprint.doNotTrack }}</td></tr>
            <tr><td class="table-label">Cookie</td><td class="table-value">{{ fingerprint.cookies }}</td></tr>
            <tr><td class="table-label">Canvas 指纹</td><td class="table-value"><code>{{ fingerprint.canvas }}</code></td></tr>
          </tbody>
        </table>
        <div class="leak-result leak-warn">
          ⚠ 以上信息任何网站都能读取，构成你的「浏览器指纹」。时区/语言/分辨率等可用于跨站追踪，Canvas 指纹可唯一定位浏览器。
        </div>
      </div>
    </div>

    <!-- HTTP 请求头 -->
    <div class="section-card">
      <div class="section-header">
        <span class="section-tag tag-headers">请求头</span>
        <span class="section-desc">浏览器发送给网站的 HTTP 请求头</span>
        <button class="action-btn" @click="loadHeaders" :disabled="headersLoading">
          {{ headersLoading ? '加载中...' : '重新加载' }}
        </button>
      </div>
      <div v-if="headersData" class="result-area">
        <div class="tz-row">
          <span class="tz-label">你的 IP</span>
          <span class="tz-value"><code>{{ headersData.ip }}</code></span>
        </div>
        <div class="tz-row">
          <span class="tz-label">协议 / Host</span>
          <span class="tz-value">{{ headersData.protocol }} · {{ headersData.host }}</span>
        </div>
        <table class="result-table">
          <tbody>
            <tr v-for="(values, name) in headersData.headers" :key="name">
              <td class="table-label">{{ name }}</td>
              <td class="table-value"><code>{{ (values as string[]).join(', ') }}</code></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <blockquote>
      说明：WebRTC 检测读取浏览器本地网络地址并分类（内网/公网/mDNS）；时区比对调用本站 IP 库判断出口 IP 归属；请求头为服务器实际收到的信息；DNS 与指纹为浏览器自检，结果仅供参考。
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
  background: #1d1e1f;
  border-color: #333;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.section-tag {
  font-size: 0.8em;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 6px;
  letter-spacing: 0.5px;
}

.tag-webrtc { background: #e6f4ff; color: #409eff; }
.tag-dns { background: #f0f9eb; color: #67c23a; }
.tag-timezone { background: #fdf6ec; color: #e6a23c; }
.tag-fingerprint { background: #f4f4f5; color: #909399; }
.tag-headers { background: #f3e8ff; color: #9c27b0; }
html.dark .tag-webrtc { background: rgba(64, 158, 255, 0.15); }
html.dark .tag-dns { background: rgba(103, 194, 58, 0.15); }
html.dark .tag-timezone { background: rgba(230, 162, 60, 0.15); }
html.dark .tag-fingerprint { background: rgba(144, 147, 153, 0.15); }
html.dark .tag-headers { background: rgba(156, 39, 176, 0.15); }

.section-desc {
  font-size: 0.9em;
  color: #909399;
  flex: 1;
}

.action-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 1px solid #3EAF7C;
  background: transparent;
  color: #3EAF7C;
  cursor: pointer;
  transition: all 0.25s;
  font-size: 0.95em;
}
.action-btn:hover { background: #3EAF7C; color: #fff; }
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.result-area { margin-top: 12px; }

.leak-result {
  font-size: 1.05em;
  font-weight: 700;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 12px;
  line-height: 1.6;
}
.leak-good { background: #f0f9eb; color: #67C23A; }
.leak-bad { background: #fef0f0; color: #F56C6C; }
.leak-warn { background: #fdf6ec; color: #E6A23C; }
html.dark .leak-good { background: rgba(103, 194, 58, 0.15); }
html.dark .leak-bad { background: rgba(245, 108, 108, 0.15); }
html.dark .leak-warn { background: rgba(230, 162, 60, 0.15); }

.group-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.group-label {
  font-size: 0.85em;
  font-weight: 600;
  min-width: 96px;
  color: #606266;
}

.ip-list { display: flex; flex-wrap: wrap; gap: 8px; }
.ip-chip {
  font-family: 'JetBrains Mono', Consolas, monospace;
  background: rgba(62, 175, 124, 0.1);
  color: #3EAF7C;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 0.9em;
  word-break: break-all;
}

.result-table { width: 100%; border-collapse: collapse; margin-bottom: 12px; }
.table-header { text-align: left; color: #909399; font-size: 0.85em; padding: 6px 8px; border-bottom: 1px solid #ebeef5; }
.table-label { color: #606266; font-size: 0.9em; padding: 8px; white-space: nowrap; vertical-align: top; }
.table-value { color: #303133; font-size: 0.9em; padding: 8px; word-break: break-all; }
html.dark .table-header { border-color: #333; }
html.dark .table-label { color: #a0a0a0; }
html.dark .table-value { color: #d0d0d0; }
.table-value code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  background: rgba(62, 175, 124, 0.1);
  color: #3EAF7C;
  padding: 2px 6px;
  border-radius: 4px;
}

.tz-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}
.tz-label { color: #606266; font-size: 0.9em; min-width: 96px; }
.tz-value { color: #303133; font-size: 0.95em; font-weight: 600; }
html.dark .tz-label { color: #a0a0a0; }
html.dark .tz-value { color: #d0d0d0; }

.badge-green { color: #67C23A; font-weight: 600; }
.badge-red { color: #F56C6C; font-weight: 600; }
</style>
<style>
:root { --el-color-primary: #3EAF7C; }
</style>
