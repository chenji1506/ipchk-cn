<script setup lang="ts">
import { ref, onMounted } from 'vue'

const config = useIpchkConfig()

useHead({
  title: 'Tor 出口节点检测 | ipchk.cn',
  meta: [
    { name: 'description', content: '检测 IP 是否为 Tor 出口节点，基于 Tor 官方 DNSEL 数据源，免费无需密钥。' },
    { name: 'keywords', content: 'Tor检测,Tor出口节点,匿名网络检测,暗网IP检测,tor exit node' },
  ],
})

const tmpip = ref('')
const loading = ref(false)
const result = ref<any>(null)
const error = ref('')
const userIP = ref('')

async function getUserIP() {
  try {
    userIP.value = await $fetch<string>(config.DualStackAPI)
  } catch {
    userIP.value = ''
  }
  return userIP.value
}

async function queryTor() {
  loading.value = true
  error.value = ''
  result.value = null
  let ip = tmpip.value.trim()
  if (!ip) {
    ip = await getUserIP()
    tmpip.value = ip
  }
  if (!ip) {
    error.value = '请输入 IP 地址'
    loading.value = false
    return
  }
  try {
    result.value = await $fetch(config.apiBaseUrls[0].url + 'v1/tor/' + encodeURIComponent(ip))
  } catch (e: any) {
    error.value = e?.message || '请求失败，请重试'
  } finally {
    loading.value = false
  }
}

onMounted(getUserIP)
</script>

<template>
  <div class="title">
    <header>
      <h1>Tor 出口节点检测</h1>
      <p>检测 IP 是否为 Tor 出口节点，基于 Tor 官方 DNSEL 数据源</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input v-model="tmpip" placeholder="输入 IP（留空则检测你的 IP）" @keyup.enter="queryTor()" />
      <el-button @click="queryTor()" type="primary" :loading="loading">检测</el-button>
    </div>

    <div v-if="error" class="error-message">{{ error }}</div>

    <div v-if="result" class="result-section">
      <div class="tor-result" :class="result.is_tor ? 'tor-bad' : 'tor-good'">
        <template v-if="result.is_tor">
          ⚠ {{ result.detail }}
        </template>
        <template v-else>
          ✓ {{ result.detail }}
        </template>
      </div>
      <table class="result-table">
        <tbody>
          <tr>
            <td class="table-label">检测 IP</td>
            <td class="table-value"><code>{{ result.ip }}</code></td>
          </tr>
          <tr>
            <td class="table-label">是否 Tor 出口</td>
            <td class="table-value">
              <span :class="result.is_tor ? 'badge-red' : 'badge-green'">{{ result.is_tor ? '是' : '否' }}</span>
            </td>
          </tr>
          <tr>
            <td class="table-label">检测时间</td>
            <td class="table-value">{{ result.checked_at }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <blockquote>
      数据来源：Tor 官方 DNSEL（dnsel.torproject.org），通过反向 DNS 查询判断 IP 是否在 Tor 出口列表。<br/>
      仅支持 IPv4；IPv6 暂不支持。检测结果存在几分钟缓存延迟。<br/>
      访客IP: {{ userIP || '获取中...' }}
    </blockquote>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.error-message {
  margin-top: 20px;
  padding: 15px;
  background: #fef0f0;
  color: #F56C6C;
  border-radius: 6px;
  text-align: center;
  font-size: 1.1em;
}

.tor-result {
  font-size: 1.15em;
  font-weight: 700;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.tor-good { background: #f0f9eb; color: #67C23A; }
.tor-bad { background: #fef0f0; color: #F56C6C; }
html.dark .tor-good { background: rgba(103, 194, 58, 0.15); }
html.dark .tor-bad { background: rgba(245, 108, 108, 0.15); }

.badge-green { color: #67C23A; font-weight: 600; }
.badge-red { color: #F56C6C; font-weight: 600; }
</style>

<style>
:root {
  --el-color-primary: #3EAF7C;
}
html.dark {
  --el-color-primary: #3EAF7C;
}
</style>
