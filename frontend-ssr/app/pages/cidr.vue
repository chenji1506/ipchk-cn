<script setup lang="ts">
const { t } = useI18n()
import { ref, computed } from 'vue';

useHead({
  title: '子网计算器 | CIDR计算 | ipchk.cn',
  meta: [
    { name: 'description', content: 'IPv4/IPv6 CIDR 子网计算器：网络地址、广播地址、可用主机数、子网掩码、地址总数等。' },
  ],
});

const cidrInput = ref('192.168.1.0/24');
const result = ref<any>(null);
const error = ref('');
const isV6 = computed(() => cidrInput.value.includes(':'));

// ===== IPv4 =====
function ipToInt(ip: string): number {
  return ip.split('.').reduce((acc, oct) => (acc << 8) + parseInt(oct), 0) >>> 0;
}
function intToIp(n: number): string {
  return [24, 16, 8, 0].map(s => (n >>> s) & 255).join('.');
}
function toBinary4(ip: string): string {
  return ip.split('.').map(oct => parseInt(oct).toString(2).padStart(8, '0')).join('.');
}

function calcIPv4(ip: string, prefix: number) {
  const ipInt = ipToInt(ip);
  const mask = prefix === 0 ? 0 : (0xFFFFFFFF << (32 - prefix)) >>> 0;
  const network = ipInt & mask;
  const broadcast = network | (~mask >>> 0);
  const hosts = prefix >= 31 ? Math.pow(2, 32 - prefix) : Math.pow(2, 32 - prefix) - 2;
  result.value = {
    version: 4,
    cidr: `${ip}/${prefix}`,
    network: intToIp(network),
    broadcast: intToIp(broadcast),
    firstHost: prefix >= 31 ? intToIp(network) : intToIp(network + 1),
    lastHost: prefix >= 31 ? intToIp(broadcast) : intToIp(broadcast - 1),
    mask: intToIp(mask),
    maskBinary: toBinary4(intToIp(mask)),
    networkBinary: toBinary4(intToIp(network)),
    totalIPs: Math.pow(2, 32 - prefix),
    hosts,
    prefix,
    wildcard: intToIp(~mask >>> 0),
  };
}

// ===== IPv6 =====
// 解析 IPv6 地址为 128 位 BigInt（支持 :: 压缩、内嵌 IPv4）
function parseIPv6(ip: string): bigint {
  // 处理内嵌 IPv4（如 ::ffff:192.168.1.1）
  let ipStr = ip;
  if (ipStr.includes('.')) {
    const v4 = ipStr.split(':').pop()!;
    const v4int = ipToInt(v4);
    ipStr = ipStr.slice(0, ipStr.lastIndexOf(':') + 1) + (v4int >> 16).toString(16) + ':' + (v4int & 0xFFFF).toString(16);
  }
  let head = '', tail = '';
  if (ipStr.includes('::')) {
    [head, tail] = ipStr.split('::');
  } else {
    head = ipStr;
  }
  const headParts = head ? head.split(':').filter(Boolean) : [];
  const tailParts = tail ? tail.split(':').filter(Boolean) : [];
  const zeros = 8 - headParts.length - tailParts.length;
  const full = [...headParts, ...Array(Math.max(zeros, 0)).fill('0'), ...tailParts];
  let val = 0n;
  for (const p of full.slice(0, 8)) {
    val = (val << 16n) | BigInt(parseInt(p || '0', 16));
  }
  return val;
}

// BigInt → 完整 8 组十六进制
function ipv6Groups(val: bigint): string[] {
  const groups: string[] = [];
  for (let i = 7; i >= 0; i--) {
    groups.push(((val >> BigInt(i * 16)) & 0xFFFFn).toString(16));
  }
  return groups;
}

// BigInt → 压缩格式（RFC 5952）
function formatIPv6(val: bigint): string {
  const groups = ipv6Groups(val);
  // 找最长连续 0 组（≥2 个才压缩）
  let bestStart = -1, bestLen = 0;
  let curStart = -1, curLen = 0;
  for (let i = 0; i < groups.length; i++) {
    if (groups[i] === '0') {
      if (curStart === -1) curStart = i;
      curLen++;
      if (curLen > bestLen) { bestLen = curLen; bestStart = curStart; }
    } else {
      curStart = -1; curLen = 0;
    }
  }
  if (bestLen >= 2) {
    const before = groups.slice(0, bestStart);
    const after = groups.slice(bestStart + bestLen);
    // 标准 :: 压缩：before + '::' + after
    return before.join(':') + '::' + after.join(':');
  }
  return groups.join(':');
}

// BigInt → 完整展开格式
function formatIPv6Full(val: bigint): string {
  return ipv6Groups(val).join(':');
}

// IPv6 掩码十六进制（如 /64 → ffff:ffff:ffff:ffff:0:0:0:0）
function ipv6MaskHex(prefix: number): string {
  const mask = prefix === 0 ? 0n : ((1n << 128n) - (1n << BigInt(128 - prefix)));
  return ipv6Groups(mask).join(':');
}

function calcIPv6(ip: string, prefix: number) {
  const val = parseIPv6(ip);
  const mask = prefix === 0 ? 0n : ((1n << 128n) - (1n << BigInt(128 - prefix)));
  const network = val & mask;
  // 子网最后一个地址（广播等价物）
  const last = network | ((1n << BigInt(128 - prefix)) - 1n);
  const total = 1n << BigInt(128 - prefix);

  result.value = {
    version: 6,
    cidr: `${formatIPv6(val)}/${prefix}`,
    network: formatIPv6(network),
    networkFull: formatIPv6Full(network),
    last: formatIPv6(last),
    maskHex: ipv6MaskHex(prefix),
    prefix,
    totalIPs: total,
    totalDisplay: prefix >= 64 ? `2^${128 - prefix}` : total.toString(),
  };
}

// ===== 计算入口 =====
function calculate() {
  error.value = '';
  const parts = cidrInput.value.trim().split('/');
  if (parts.length !== 2) { error.value = t('格式应为 CIDR，如 192.168.1.0/24 或 2001:db8::/32'); return; }
  const ip = parts[0].trim();
  const prefix = parseInt(parts[1]);
  if (isNaN(prefix)) { error.value = t('前缀长度不正确'); return; }

  if (ip.includes(':')) {
    if (prefix < 0 || prefix > 128) { error.value = 'IPv6 前缀长度应为 0-128'; return; }
    calcIPv6(ip, prefix);
  } else {
    if (!/^\d+\.\d+\.\d+\.\d+$/.test(ip)) { error.value = 'IPv4 格式不正确'; return; }
    if (prefix < 0 || prefix > 32) { error.value = 'IPv4 前缀长度应为 0-32'; return; }
    calcIPv4(ip, prefix);
  }
}
</script>

<template>
  <div class="title">
    <header>
      <h1>{{ $t('子网计算器') }}</h1>
      <p>{{ $t('IPv4 / IPv6 CIDR 网段计算（自动识别）') }}</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input v-model="cidrInput" :placeholder="$t('如：192.168.1.0/24 或 2001:db8::/32')" clearable @keyup.enter="calculate" />
      <el-button type="primary" @click="calculate">{{ $t('计算') }}</el-button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="result && result.version === 4" class="result-section">
      <table class="result-table">
        <tbody>
          <tr><td class="table-label">CIDR</td><td class="table-value"><span class="ip-highlight">{{ result.cidr }}</span></td></tr>
          <tr><td class="table-label">{{ $t('网络地址') }}</td><td class="table-value"><span class="mono">{{ result.network }}</span></td></tr>
          <tr><td class="table-label">{{ $t('广播地址') }}</td><td class="table-value"><span class="mono">{{ result.broadcast }}</span></td></tr>
          <tr><td class="table-label">{{ $t('子网掩码') }}</td><td class="table-value"><span class="mono">{{ result.mask }}</span></td></tr>
          <tr><td class="table-label">{{ $t('掩码（二进制）') }}</td><td class="table-value"><span class="mono bin">{{ result.maskBinary }}</span></td></tr>
          <tr><td class="table-label">{{ $t('网络地址（二进制）') }}</td><td class="table-value"><span class="mono bin">{{ result.networkBinary }}</span></td></tr>
          <tr><td class="table-label">{{ $t('IP 总数') }}</td><td class="table-value"><b>{{ result.totalIPs.toLocaleString() }}</b> 个</td></tr>
          <tr><td class="table-label">{{ $t('可用主机') }}</td><td class="table-value"><b>{{ result.hosts.toLocaleString() }}</b> 个</td></tr>
          <tr><td class="table-label">{{ $t('主机范围') }}</td><td class="table-value"><span class="mono">{{ result.firstHost }} ~ {{ result.lastHost }}</span></td></tr>
          <tr><td class="table-label">{{ $t('通配掩码') }}</td><td class="table-value"><span class="mono">{{ result.wildcard }}</span></td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="result && result.version === 6" class="result-section">
      <table class="result-table">
        <tbody>
          <tr><td class="table-label">CIDR</td><td class="table-value"><span class="ip-highlight">{{ result.cidr }}</span></td></tr>
          <tr><td class="table-label">{{ $t('网络地址（压缩）') }}</td><td class="table-value"><span class="mono">{{ result.network }}</span></td></tr>
          <tr><td class="table-label">{{ $t('网络地址（完整）') }}</td><td class="table-value"><span class="mono v6full">{{ result.networkFull }}</span></td></tr>
          <tr><td class="table-label">{{ $t('子网末地址') }}</td><td class="table-value"><span class="mono">{{ result.last }}</span></td></tr>
          <tr><td class="table-label">{{ $t('前缀长度') }}</td><td class="table-value"><b>/{{ result.prefix }}</b></td></tr>
          <tr><td class="table-label">{{ $t('掩码（十六进制）') }}</td><td class="table-value"><span class="mono v6full">{{ result.maskHex }}</span></td></tr>
          <tr><td class="table-label">{{ $t('地址总数') }}</td><td class="table-value"><b>{{ result.totalDisplay }}</b> 个</td></tr>
        </tbody>
      </table>
      <blockquote>{{ $t('IPv6 使用前缀长度表示网段（无点分掩码），掩码以十六进制分组显示。') }}</blockquote>
    </div>
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
  width: 320px;
  height: 50px;
  font: 1.2em monospace;
  margin-right: 10px;
}
.el-button { height: 50px; padding: 0 24px; }

.ip-highlight {
  color: #3EAF7C;
  font-weight: bold;
  font-family: 'JetBrains Mono', Consolas, monospace;
}
.bin {
  font-size: 0.9em;
  color: #67C23A;
  letter-spacing: 0.5px;
}
.v6full {
  font-size: 0.85em;
  word-break: break-all;
}
.mono {
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-weight: 600;
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
