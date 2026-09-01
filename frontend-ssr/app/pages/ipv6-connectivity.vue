<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

const config = useIpchkConfig()

useHead({
  title: 'IPv6 连通性检测 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测你的网络 IPv4 / IPv6 双栈连通性，展示 IPv4/IPv6 出口地址，判断网络是否支持 IPv6。' },
    { name: 'keywords', content: 'IPv6连通性,双栈检测,ipv4检测,ipv6检测,ipv6支持,ipv6 connectivity' },
  ],
})

const detecting = ref(true)
const ipv4 = ref('')
const ipv6 = ref('')
const v4Done = ref(false)
const v6Done = ref(false)

function fetchWithTimeout(url: string, ms = 4000): Promise<string> {
  return Promise.race([
    fetch(url).then((r) => r.text()),
    new Promise<string>((_, reject) => setTimeout(() => reject(new Error('timeout')), ms))
  ])
}

// WebRTC 检测本机公网 IPv6（按 ICE candidate 标准解析）
function detectIPv6ViaWebRTC(): Promise<string> {
  return new Promise((resolve) => {
    try {
      if (typeof RTCPeerConnection === 'undefined') { resolve(''); return }
      const pc = new RTCPeerConnection({ iceServers: [] })
      pc.createDataChannel('')
      let settled = false
      const done = (v: string) => { if (!settled) { settled = true; resolve(v) } }
      pc.onicecandidate = (e: any) => {
        if (e.candidate) {
          const c = e.candidate.candidate || ''
          if (c.includes('typ host')) {
            const addr = c.split(' ')[4]
            if (addr && addr.includes(':') && !addr.startsWith('fe80') && addr !== '::1' && !addr.startsWith('fc') && !addr.startsWith('fd')) {
              done(addr)
            }
          }
        } else {
          done('')
        }
      }
      pc.createOffer().then((o: any) => pc.setLocalDescription(o)).catch(() => done(''))
      setTimeout(() => done(''), 3000)
    } catch {
      resolve('')
    }
  })
}

async function detect() {
  detecting.value = true
  ipv4.value = ''
  ipv6.value = ''
  v4Done.value = false
  v6Done.value = false

  // IPv4 检测（强制走 A 记录）
  fetchWithTimeout(config.v4OnlyAPI, 4000)
    .then((ip) => { if (/^\d+\.\d+\.\d+\.\d+$/.test(ip.trim())) ipv4.value = ip.trim() })
    .catch(() => {})
    .finally(() => { v4Done.value = true })

  // IPv6 检测：API + WebRTC 并行，谁先出用谁
  const fromApi = fetchWithTimeout(config.v6OnlyAPI, 3000)
    .then((ip) => (ip.trim().includes(':') ? ip.trim() : ''))
    .catch(() => '')
  const fromRtc = detectIPv6ViaWebRTC()
  const [a, b] = await Promise.all([fromApi, fromRtc])
  ipv6.value = a || b
  v6Done.value = true
  detecting.value = false
}

const dualStack = computed(() => {
  if (ipv4.value && ipv6.value) return 'dual'
  if (ipv4.value) return 'v4only'
  if (ipv6.value) return 'v6only'
  return 'none'
})

const dualStackText = computed(() => {
  switch (dualStack.value) {
    case 'dual': return '双栈网络（IPv4 + IPv6）'
    case 'v4only': return '仅 IPv4'
    case 'v6only': return '仅 IPv6'
    default: return '检测中...'
  }
})

onMounted(detect)
</script>

<template>
  <div class="title">
    <header>
      <h1>IPv6 连通性检测</h1>
      <p>检测你的网络 IPv4 / IPv6 双栈连通性，展示两个协议的出口地址</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-button @click="detect()" type="primary" :loading="detecting">重新检测</el-button>
    </div>

    <div class="status-banner" :class="'status-' + dualStack">
      <template v-if="detecting">
        <el-icon class="is-loading"><i class="el-icon-loading" /></el-icon> 正在检测网络连通性...
      </template>
      <template v-else>
        {{ dualStackText }}
      </template>
    </div>

    <div class="stack-grid">
      <div class="stack-card">
        <div class="stack-header">
          <span class="stack-label v4-label">IPv4</span>
          <span class="stack-status" :class="ipv4 ? 'ok' : (v4Done ? 'fail' : 'waiting')">
            {{ detecting && !v4Done ? '检测中' : (ipv4 ? '✓ 已连通' : '✗ 不可用') }}
          </span>
        </div>
        <div class="stack-ip">
          <code v-if="ipv4">{{ ipv4 }}</code>
          <span v-else-if="v4Done" class="empty">未检测到 IPv4 出口</span>
          <span v-else class="empty">检测中...</span>
        </div>
        <div class="stack-desc">通过 IPv4 协议访问本站</div>
      </div>

      <div class="stack-card">
        <div class="stack-header">
          <span class="stack-label v6-label">IPv6</span>
          <span class="stack-status" :class="ipv6 ? 'ok' : (v6Done ? 'fail' : 'waiting')">
            {{ detecting && !v6Done ? '检测中' : (ipv6 ? '✓ 已连通' : '✗ 不可用') }}
          </span>
        </div>
        <div class="stack-ip">
          <code v-if="ipv6">{{ ipv6 }}</code>
          <span v-else-if="v6Done" class="empty">未检测到 IPv6 出口</span>
          <span v-else class="empty">检测中...</span>
        </div>
        <div class="stack-desc">通过 IPv6 协议（含本机 WebRTC 探测）</div>
      </div>
    </div>

    <blockquote>
      IPv4 通过强制走 A 记录的子域检测；IPv6 通过强制 AAAA 记录的子域 + 浏览器 WebRTC 本机地址探测（二者取先到结果）。<br/>
      「双栈」表示你的网络同时支持 IPv4 和 IPv6；「仅 IPv4」表示暂不支持 IPv6（国内多数宽带默认情况）。
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.status-banner {
  font-size: 1.2em;
  font-weight: 700;
  padding: 14px 18px;
  border-radius: 10px;
  margin: 14px 0;
  text-align: center;
}
.status-dual { background: #f0f9eb; color: #67C23A; }
.status-v4only { background: #fdf6ec; color: #E6A23C; }
.status-v6only { background: #ecf5ff; color: #409EFF; }
.status-none { background: #f4f4f5; color: #909399; }
html.dark .status-dual { background: rgba(103, 194, 58, 0.15); }
html.dark .status-v4only { background: rgba(230, 162, 60, 0.15); }
html.dark .status-v6only { background: rgba(64, 158, 255, 0.15); }
html.dark .status-none { background: rgba(144, 147, 153, 0.15); }

.stack-grid {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
.stack-card {
  flex: 1;
  min-width: 260px;
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  padding: 16px 18px;
}
html.dark .stack-card { border-color: #333; }

.stack-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.stack-label {
  font-weight: 700;
  font-size: 1.05em;
  padding: 3px 12px;
  border-radius: 6px;
}
.v4-label { background: #ecf5ff; color: #409EFF; }
.v6-label { background: #f0f9eb; color: #67C23A; }
html.dark .v4-label { background: rgba(64, 158, 255, 0.15); }
html.dark .v6-label { background: rgba(103, 194, 58, 0.15); }

.stack-status { font-weight: 600; }
.stack-status.ok { color: #67C23A; }
.stack-status.fail { color: #F56C6C; }
.stack-status.waiting { color: #909399; }

.stack-ip {
  font-size: 1.15em;
  margin: 6px 0;
}
.stack-ip code {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: #3EAF7C;
  word-break: break-all;
}
.empty { color: #999; }

.stack-desc {
  color: #999;
  font-size: 0.85em;
}
</style>

<style>
:root {
  --el-color-primary: #3EAF7C;
}
html.dark {
  --el-color-primary: #3EAF7C;
}
</style>
