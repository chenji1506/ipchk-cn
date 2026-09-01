<script setup lang="ts">
useHead({
  title: 'API 文档 | 开放接口 | ipchk.cn',
  meta: [
    { name: 'description', content: 'ipchk.cn 开放 API 接口文档，支持 IP 归属地、纯净度、端口扫描等查询。' },
  ],
});

const endpoints = [
  {
    method: 'GET', path: '/ip', desc: '查询本机公网 IP（纯文本返回）',
    example: 'curl https://ipchk.cn/ip',
  },
  {
    method: 'GET', path: '/v1/location', desc: '查询请求方 IP 的归属地（JSON）',
    example: 'curl https://ipchk.cn/v1/location',
  },
  {
    method: 'GET', path: '/v1/location/:ip', desc: '查询指定 IP 的归属地',
    example: 'curl https://ipchk.cn/v1/location/8.8.8.8',
  },
  {
    method: 'GET', path: '/v1/purity', desc: '查询本机 IP 纯净度评分',
    example: 'curl https://ipchk.cn/v1/purity',
  },
  {
    method: 'GET', path: '/v1/purity/:ip', desc: '查询指定 IP 纯净度评分',
    example: 'curl https://ipchk.cn/v1/purity/8.8.8.8',
  },
  {
    method: 'POST', path: '/v1/purity/check', desc: '批量 IP 纯净度检测（最多 10 个）',
    example: 'curl -X POST https://ipchk.cn/v1/purity/check -H "Content-Type: application/json" -d \'{"ips":["8.8.8.8","1.1.1.1"]}\'',
  },
  {
    method: 'GET', path: '/v1/rbl', desc: '查询本机 IP 邮件黑名单（16 个 DNSBL 源）',
    example: 'curl https://ipchk.cn/v1/rbl',
  },
  {
    method: 'GET', path: '/v1/rbl/:ip', desc: '查询指定 IP 邮件黑名单（16 个 DNSBL 源）',
    example: 'curl https://ipchk.cn/v1/rbl/8.8.8.8',
  },
  {
    method: 'GET', path: '/v1/card/:ip', desc: '生成 IP 信息卡片图（SVG）',
    example: 'curl https://ipchk.cn/v1/card/8.8.8.8',
  },
  {
    method: 'GET', path: '/v1/scan/:ip', desc: '端口扫描（?ports=22,80,443 自定义）',
    example: 'curl "https://ipchk.cn/v1/scan/8.8.8.8?ports=53,80,443"',
  },
  {
    method: 'GET', path: '/v1/whois/:target', desc: '域名/IP 注册信息（RDAP）',
    example: 'curl https://ipchk.cn/v1/whois/baidu.com',
  },
  {
    method: 'GET', path: '/v1/dns/:type/:domain', desc: 'DNS 解析（type: a/aaaa/cname/mx/ns/txt）',
    example: 'curl https://ipchk.cn/v1/dns/a/baidu.com',
  },
  {
    method: 'GET', path: '/v1/ssl/:domain', desc: 'SSL 证书检测',
    example: 'curl https://ipchk.cn/v1/ssl/baidu.com',
  },
  {
    method: 'GET', path: '/v1/tcping/:host', desc: 'TCP 连接测试（?port=80&count=4）',
    example: 'curl "https://ipchk.cn/v1/tcping/baidu.com?port=80&count=4"',
  },
  {
    method: 'GET', path: '/v1/speed/:ver/:url', desc: '网站测速（ver: v4/v6/dual）',
    example: 'curl https://ipchk.cn/v1/speed/v4/baidu.com',
  },
];
</script>

<template>
  <div class="title">
    <header>
      <h1>API 文档</h1>
      <p>ipchk.cn 开放接口，全部免费公开</p>
    </header>
  </div>
  <div class="content">
    <div class="notice-card">
      所有接口返回结构化 JSON（<code>/ip</code> 返回纯文本，<code>/v1/card</code> 返回 SVG 卡片图）。命令行访问归属地/纯净度/黑名单接口自动返回格式化文本。批量检测接口 <code>/v1/purity/check</code> 使用 <b>POST</b>。
    </div>

    <table class="result-table">
      <thead>
        <tr>
          <th class="table-header" style="width: 70px">方法</th>
          <th class="table-header">接口路径</th>
          <th class="table-header">说明</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(ep, i) in endpoints" :key="i">
          <td class="table-value"><span class="method" :class="ep.method === 'POST' ? 'method-post' : 'method-get'">{{ ep.method }}</span></td>
          <td class="table-value"><code class="path">{{ ep.path }}</code></td>
          <td class="table-value">{{ ep.desc }}</td>
        </tr>
      </tbody>
    </table>

    <h3 class="example-title">使用示例</h3>
    <div v-for="(ep, i) in endpoints.slice(0, 8)" :key="'ex' + i" class="example-block">
      <div class="example-desc">{{ ep.desc }}</div>
      <pre><code>{{ ep.example }}</code></pre>
    </div>
  </div>
</template>

<style scoped>
@import "../style.css";
@import "../../assets/css/tool-common.css";

.notice-card {
  padding: 14px 18px;
  border-radius: 10px;
  background: rgba(62, 175, 124, 0.06);
  border: 1px solid rgba(62, 175, 124, 0.2);
  margin-bottom: 18px;
  line-height: 1.7;
}

.method-get {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 4px;
  background: rgba(62, 175, 124, 0.15);
  color: #3EAF7C;
  font-weight: 700;
  font-size: 0.85em;
}

.method-post {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 4px;
  background: rgba(230, 162, 60, 0.15);
  color: #E6A23C;
  font-weight: 700;
  font-size: 0.85em;
}

.path {
  font-family: 'JetBrains Mono', Consolas, monospace;
  color: #303133;
}
html.dark .path { color: #cfcfcf; }

.example-title {
  margin: 24px 0 12px;
  color: #3EAF7C;
}

.example-block {
  margin-bottom: 16px;
}
.example-desc {
  font-size: 0.95em;
  color: #909399;
  margin-bottom: 6px;
}
</style>
<style>
:root { --el-color-primary: #3EAF7C; }
</style>
