<script setup lang="ts">
import { ref, onMounted, computed, nextTick } from 'vue'
import { useRoute } from 'vue-router'
const config = useIpchkConfig()
import { isIPv6 } from 'is-ip';
import { renderMarkdown } from "../../utils/markdown";
import { formatTime } from '../../utils/tools';
const route = useRoute()

useHead({
  title: 'DNS查询工具 | 多节点域名解析检测 | ipchk.cn',
  meta: [
    { name: 'description', content: '专业的多节点DNS查询工具,支持A记录、AAAA记录、CNAME记录、MX记录、NS记录、TXT记录、SRV记录、CAA记录等多种DNS解析记录查询,提供全国多节点并发检测,快速返回DNS解析结果,助力域名解析问题排查与优化' },
    { name: 'keywords', content: 'dns查询,dns解析,域名解析,a记录查询,aaaa记录,cname记录,mx记录,ns记录,txt记录,srv记录,dns服务器,域名dns检测' },
    { property: 'og:title', content: 'DNS查询工具 - 多节点域名解析记录检测' },
    { property: 'og:description', content: '多节点DNS查询工具,支持多种DNS记录类型查询,快速检测域名解析状态' },
    { property: 'og:image', content: config.siteUrl + 'favicon.svg' },
    { property: 'og:type', content: 'website' },
  ],
  script: [
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'WebApplication',
        name: 'DNS查询与解析检测工具',
        description: '专业的多节点DNS查询工具，支持A、AAAA、CNAME、MX、NS、TXT、SRV、CAA等多种记录类型，全国多节点并发检测。',
        url: config.siteUrl + 'dns',
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'Web',
        offers: {
          '@type': 'Offer',
          price: '0',
          priceCurrency: 'CNY'
        },
        provider: {
          '@type': 'Organization',
          name: 'ipchk.cn'
        }
      })
    }
  ]
});

const tmpDomain = ref('https://ipchk.cn')
const domain = ref('')
const recordType = ref('a')
const loading = ref(false)
const results = ref<any>([])
const isloading = ref(false)
const nowRecordType = ref('')
const userIP = ref('')

async function getUserIP(){
  
  await $fetch<string>(config.DualStackAPI).then(
  function (data){
    userIP.value = data
  })
  return userIP.value
}

const recordTypes = [
  { value: 'all', label: '全量记录' },
  { value: 'a', label: 'A 记录' },
  { value: 'aaaa', label: 'AAAA 记录' },
  { value: 'cname', label: 'CNAME 记录' },
  { value: 'mx', label: 'MX 记录' },
  { value: 'ns', label: 'NS 记录' },
  { value: 'txt', label: 'TXT 记录' },
  { value: 'srv', label: 'SRV 记录' },
  { value: 'caa', label: 'CAA 记录' },
  { value: 'ptr', label: 'PTR 记录' }
]

const dnsServerFetches = config.NSLookup.map((server) => {
  const url = computed(() => server.url + 'v1/dns/' + recordType.value + "/" + domain.value);
  const { data, error, execute } = useFetch(url, {
    immediate: false,
    watch: false,
  });
  return { label: server.label, data, error, execute };
});

async function queryDNS() {
  isloading.value = true
  domain.value = tmpDomain.value
  await nextTick()
  
  // 初始化结果数组，保持响应式结构
  results.value = dnsServerFetches.map(fetch => ({
    server: fetch.label,
    loading: true,
    data: null,
    error: null
  }));

  const promises = dnsServerFetches.map(async (fetch, index) => {
    try {
      await fetch.execute();
      
      //  直接更新对应索引的结果，保持响应式
      results.value[index] = {
        server: fetch.label,
        loading: false,
        data: fetch.data.value,
        error: null
      };
      
      return {
        server: fetch.label,
        data: fetch.data.value
      };
    } catch (err) {
      //  错误时也更新对应索引
      results.value[index] = {
        server: fetch.label,
        loading: false,
        data: null,
        error: err
      };
      
      return {
        server: fetch.label,
        error: err
      };
    }
  });
  nowRecordType.value = recordType.value
  const promiseResults = await Promise.all(promises)
  console.log(promiseResults)
  isloading.value = false
  
  return promiseResults
}


onMounted(() => {
  
  const domainParam = route.query.domain as string
  const typeParam = route.query.type as string
  if (domainParam) {
    tmpDomain.value = domainParam
  }
  if (typeParam && recordTypes.some(t => t.value === typeParam)) {
    recordType.value = typeParam
  }
  if (domainParam) {
    queryDNS()
  }
  getUserIP()
})
const { data: page, error } = await useAsyncData('dns-page', () =>
  $fetch('/api/markdown/dns')
)
const doc = page.value;
</script>

<template>
  <div class="title">
    <header>
      <h1>DNS查询</h1>
      <p>多节点 DNS 查询，检测域名解析记录</p>
    </header>
  </div>
  <div class="content">
    <div class="one-line">
      <el-input 
        v-model="tmpDomain" 
        placeholder="请输入域名（如：ipchk.cn）" 
      />
      <el-select v-model="recordType" style="width: 150px;" class="custom-height-select">
        <el-option 
          v-for="item in recordTypes" 
          :key="item.value" 
          :label="item.label" 
          :value="item.value" 
          
        />
      </el-select>
      <el-button 
        @click="queryDNS()" 
        type="primary" 
        :loading="isloading"
      >
        查询
      </el-button>
    </div>
        <div class="result-section">
        <table class="result-table" v-if="results.length > 0 && nowRecordType !== 'all'">
        <thead>
          <tr>
            
            <th class="table-header">服务器</th>
            <th class="table-header">类型</th>
            <th class="table-header">记录</th>
            <th class="table-header">记录数</th>
            <th class="table-header">耗时</th>
            <th class="table-header">TTL (S)</th>
            
          </tr>
        </thead>
        <tbody>
          <tr v-for="(result) in results" :key="result.server">
            <td class="table-label">{{result.server}}</td>
            <td class="table-value">{{nowRecordType.toUpperCase() || '--'}}</td>
            <td class="table-value" v-if="result.loading" colspan="4" style="text-align: left;">加载中...</td>
            <td class="table-value" v-if="!result.loading">
              <template v-if="result && result.data?.record">
                <div v-for="(ip, index) in result.data.record.slice(0, 5)" :key="index" class="ip-address">
                  <span>{{ ip }}</span>
                </div>
              </template>
              
              <span v-else-if="!result.loading && result.data?.record?.length === 0" class="status-code" style="color: #F56C6C; background: #fef0f0;">
                解析失败
              </span>
            </td>
            
            <td class="table-value" v-if="!result.loading">{{result.data?.record?.length || 0}}</td>
            <td class="table-value" v-if="!result.loading">{{formatTime(result.data?.duration)}}</td>
            <td class="table-value" v-if="!result.loading">{{result.data?.ttl}}</td>
          </tr>
        </tbody>
        
        </table>

        <div v-if="nowRecordType === 'all' && results.length > 0" class="all-results">
          <div v-for="(result, idx) in results" :key="'all-' + idx" class="all-server">
            <div class="all-server-name">{{ result.server }}</div>
            <table class="result-table" v-if="!result.loading && result.data">
              <tbody>
                <tr v-for="t in ['a','aaaa','cname','mx','ns','txt','srv','caa']" :key="t">
                  <td class="table-label">{{ t.toUpperCase() }}</td>
                  <td class="table-value">
                    <template v-if="result.data[t] && result.data[t].record && result.data[t].record.length">
                      <span v-for="(r, i) in result.data[t].record" :key="i" class="ip-address"><span>{{ r }}</span></span>
                    </template>
                    <span v-else style="color:#999;">—</span>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-else-if="result.loading" class="all-loading">加载中...</div>
          </div>
        </div>
    </div>
    <blockquote>
      <div class="visitor-ip">
        <span class="vip-label">访客IP</span>
        <span class="vip-addr">{{ userIP }}</span>
        <span class="vip-net" :class="isIPv6(userIP) ? 'net-v6' : 'net-v4'">
          {{ isIPv6(userIP) ? 'IPv6 访问优先' : 'IPv4 访问优先' }}
        </span>
      </div>
    </blockquote>
    <div class="markdown">
      <div v-html="doc"></div>
    </div>
    </div>

</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.el-menu--horizontal > .el-menu-item:nth-child(1) {
  margin-right: auto;
}

.markdown :deep(a) {
  color: #3EAF7C !important;
  font-size: 1.3em;
  text-decoration: none
}

.all-results { display: flex; flex-direction: column; gap: 16px; }
.all-server {
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  padding: 12px 16px;
}
html.dark .all-server { border-color: #333; }
.all-server-name {
  font-weight: 700;
  font-size: 1.05em;
  margin-bottom: 8px;
  color: #3EAF7C;
}
.all-loading {
  color: #909399;
  padding: 8px 0;
}
</style>

<style>
:root {
  --el-color-primary: #3EAF7C;
}

html.dark {
  --el-color-primary: #3EAF7C;
}

.el-icon {
  font-size: 1.3em;
}

/* ===== 访客IP 高亮特效 ===== */
.visitor-ip {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding: 10px 14px;
  border-radius: 10px;
  background: linear-gradient(135deg, rgba(62, 175, 124, 0.08), rgba(62, 175, 124, 0.02));
  border: 1px solid rgba(62, 175, 124, 0.22);
}

.vip-label {
  font-size: 0.85em;
  font-weight: 700;
  letter-spacing: 0.5px;
  padding: 3px 12px;
  border-radius: 6px;
  color: #fff;
  background: #3EAF7C;
  white-space: nowrap;
}

/* IP 地址：等宽渐变 + 呼吸发光动画 */
.vip-addr {
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', Consolas, Monaco, monospace;
  font-size: 1.35em;
  font-weight: 700;
  background: linear-gradient(135deg, #3EAF7C, #2E9A68);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  word-break: break-all;
  animation: vip-glow 2.2s ease-in-out infinite;
}

@keyframes vip-glow {
  0%, 100% {
    filter: drop-shadow(0 0 3px rgba(62, 175, 124, 0.35));
  }
  50% {
    filter: drop-shadow(0 0 10px rgba(62, 175, 124, 0.75));
  }
}

/* 网络类型徽章 */
.vip-net {
  font-size: 0.85em;
  font-weight: 600;
  padding: 3px 12px;
  border-radius: 20px;
  color: #fff;
  white-space: nowrap;
}
.net-v4 {
  background: #3EAF7C;
}
.net-v6 {
  background: #7C4DFF;
}

/* 暗色模式适配 */
html.dark .visitor-ip {
  background: linear-gradient(135deg, rgba(62, 175, 124, 0.14), rgba(62, 175, 124, 0.04));
  border-color: rgba(62, 175, 124, 0.32);
}

@media (max-width: 768px) {
  .vip-addr {
    font-size: 1.05em;
  }
}
</style>
