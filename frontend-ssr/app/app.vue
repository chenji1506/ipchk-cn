<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue';
import { useDark, useToggle } from '@vueuse/core';
import { Moon, Sunny, Expand } from '@element-plus/icons-vue';
const config = useIpchkConfig()

const route = useRoute();

// 根据当前路由计算高亮的菜单项（对应 el-menu 的 index）
const activeMenu = computed(() => {
  const p = route.path;
  if (p === '/purity') return '1';
  if (p === '/rbl' || p.startsWith('/rbl')) return '5';
  if (p.startsWith('/ipv6webcheck')) return '2-0';
  if (p === '/location' || p === '/ipv6' || p.startsWith('/location?') || p.startsWith('/ipv6?')) return '2-1';
  if (p.startsWith('/ipv6tcping')) return '2-2';
  if (p.startsWith('/dns')) return '2-3';
  if (p.startsWith('/ssl')) return '2-4';
  if (p.startsWith('/ipv6speedtest')) return '2-5';
  if (p.startsWith('/speedtest')) return '3-0';
  if (p.startsWith('/tcping')) return '3-1';
  if (p.startsWith('/batch')) return '4-0';
  if (p.startsWith('/scan')) return '4-1';
  if (p.startsWith('/whois')) return '4-2';
  if (p.startsWith('/leak')) return '4-3';
  if (p.startsWith('/cidr')) return '4-4';
  if (p.startsWith('/api')) return '4-5';
  if (p.startsWith('/doc')) return '2-1';
  return '';
});

const isNarrow = ref(false);
let mediaQueryList: MediaQueryList | null = null;
const drawer = ref(false);

const isDark = useDark();
const toggleDark = useToggle(isDark);

function cleanChineseCharacters(input: string): string {
  // 使用正则表达式匹配中文字符
  const chineseRegex = /[\u4e00-\u9fa5]/g;
  // 将中文字符替换为空字符串
  return input.replace(chineseRegex, '');
}
onMounted(() => {
  mediaQueryList = window.matchMedia('(max-width: 768px)');
  isNarrow.value = mediaQueryList.matches;

  const handler = (e: MediaQueryListEvent) => {
    isNarrow.value = e.matches;
  };

  mediaQueryList.addEventListener('change', handler);

  onBeforeUnmount(() => {
    mediaQueryList?.removeEventListener('change', handler);
  });
});

useHead({
  // 在 HTML 解析前同步执行，防止明暗切换和页面闪烁
  script: [
    {
      defer: true,
      src: config.umamiScriptUrl,
      'data-website-id': config.umamiWebsiteId,
    },
  ],
  meta: config.noindex
    ? [
        { name: 'robots', content: 'noindex, nofollow' },
        { name: 'googlebot', content: 'noindex, nofollow' },
        { name: 'bingbot', content: 'noindex, nofollow' },
      ]
    : [],
});
</script>

<template>
  
  <el-drawer v-model="drawer" direction="ltr" style="height: 100%;" size="50%">
      <div class="drawer-section">
        <p class="drawer-title">常用</p>
        <router-link to="/purity" style="font-size: 1em;">
          <p style="display: inline-block; margin-left: 10px">IP纯净度检测</p>
        </router-link>
        <router-link to="/rbl" style="font-size: 1em;">
          <p style="display: inline-block; margin-left: 10px">邮件黑名单检测</p>
        </router-link>
      </div>
      <div class="drawer-section">
        <p class="drawer-title">IPv6 工具箱</p>
        <router-link to="/ipv6webcheck" style="font-size: 1em;">
          <p style="display: inline-block; margin-left: 10px">IPv6 网站检测</p>
        </router-link>
        <router-link to="/location" style="font-size: 1em;">
          <p style="display: inline-block; margin-left: 10px">IPv6/IPv4 地址查询</p>
        </router-link>
        <router-link to="/ipv6tcping" style="font-size: 1em;">
          <p style="display: inline-block; margin-left: 10px">IPv6 TCPing</p>
        </router-link>
        <router-link to="/dns"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 DNS解析</p></router-link>
        <router-link to="/ssl" style="font-size: 1em;">
          <p style="display: inline-block; margin-left: 10px">IPv6 SSL检查</p>
        </router-link>
        <a href="/ipv6speedtest"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 网站测速</p></a>
      </div>
      <div class="drawer-section">
        <p class="drawer-title">IPv4 工具箱</p>
        <a href="/speedtest"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv4 网站测速</p></a>
        <a href="/tcping"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv4 TCPing</p></a>
      </div>
      <div class="drawer-section">
        <p class="drawer-title">更多工具</p>
        <router-link to="/batch" style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">批量 IP 查询</p></router-link>
        <router-link to="/scan" style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">端口扫描</p></router-link>
        <router-link to="/whois" style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">Whois 查询</p></router-link>
        <router-link to="/leak" style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">隐私泄露检测</p></router-link>
        <router-link to="/cidr" style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">子网计算器</p></router-link>
        <router-link to="/api" style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">API 文档</p></router-link>
      </div>
  </el-drawer>
  <el-menu
      mode="horizontal"
      :ellipsis="false"
      :default-active="activeMenu"
    >
    <el-menu-item index="0">
      <el-icon v-if="isNarrow" @click="drawer = !drawer"><Expand /></el-icon>
      <router-link to="/">
        <img
          src="/favicon.svg"
          alt="IPW logo"
          width="48"
          height="48"
          loading="eager"
          decoding="async"
        />
        <h2 style="display: inline-block; margin-left: 10px">ipchk.cn</h2>
      </router-link>
    </el-menu-item>
    
    <el-menu-item index="1" v-if="!isNarrow">
      <router-link to="/purity" style="font-size: 1em;">
        <p style="display: inline-block; margin-left: 10px">IP纯净度检测</p>
      </router-link>
    </el-menu-item>

    <el-menu-item index="5" v-if="!isNarrow">
      <router-link to="/rbl" style="font-size: 1em;">
        <p style="display: inline-block; margin-left: 10px">邮件黑名单检测</p>
      </router-link>
    </el-menu-item>

    <el-sub-menu index="2" v-if="!isNarrow">
      <template #title>IPv6 工具箱</template>
      <el-menu-item index="2-0">
        <router-link to="/ipv6webcheck"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 网站检测</p></router-link>
      </el-menu-item>
      <el-menu-item index="2-1">
        <router-link to="/location"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6/IPv4 地址查询</p></router-link>
      </el-menu-item>
      <el-menu-item index="2-2">
        <router-link to="/ipv6tcping"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 TCPing测试</p></router-link>
      </el-menu-item>
      <el-menu-item index="2-3">
        <router-link to="/dns"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 DNS解析</p></router-link>
      </el-menu-item>
      <el-menu-item index="2-4">
        <router-link to="/ssl"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 SSL检查</p></router-link>
      </el-menu-item>
      <el-menu-item index="2-5">
        <router-link to="/ipv6speedtest"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv6 网站测速</p></router-link>
      </el-menu-item>
    </el-sub-menu>

    <el-sub-menu index="3" v-if="!isNarrow">
      <template #title>IPv4 工具箱</template>
      <el-menu-item index="3-0">
        <router-link to="/speedtest"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv4 网站测速</p></router-link>
      </el-menu-item>
      <el-menu-item index="3-1">
        <router-link to="/tcping"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">IPv4 TCPing测试</p></router-link>
      </el-menu-item>
    </el-sub-menu>

    <el-sub-menu index="4" v-if="!isNarrow">
      <template #title>更多工具</template>
      <el-menu-item index="4-0">
        <router-link to="/batch"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">批量 IP 查询</p></router-link>
      </el-menu-item>
      <el-menu-item index="4-1">
        <router-link to="/scan"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">端口扫描</p></router-link>
      </el-menu-item>
      <el-menu-item index="4-2">
        <router-link to="/whois"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">Whois 查询</p></router-link>
      </el-menu-item>
      <el-menu-item index="4-3">
        <router-link to="/leak"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">隐私泄露检测</p></router-link>
      </el-menu-item>
      <el-menu-item index="4-4">
        <router-link to="/cidr"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">子网计算器</p></router-link>
      </el-menu-item>
      <el-menu-item index="4-5">
        <router-link to="/api"  style="font-size: 1em;"><p style="display: inline-block; margin-left: 10px">API 文档</p></router-link>
      </el-menu-item>
    </el-sub-menu>

    <el-menu-item index="10">
      <ClientOnly>
      <el-icon @click="toggleDark()" v-if="isDark" style="cursor: pointer;"><Moon style="height: 20px; width: 20px;"/></el-icon>
      <el-icon @click="toggleDark()" v-else style="cursor: pointer;"><Sunny style="height: 20px; width: 20px;"/></el-icon>
      </ClientOnly>
    </el-menu-item>


  </el-menu>
  
  <NuxtLoadingIndicator />
  <main id="main-content" role="main">
    <NuxtPage />
  </main>

  <footer>
    <div class="one-line">
      Copyright © ip查询&ipchk.cn&nomdn 2026  | <img src="/ipv6-s1.svg" alt="IPv6 相关标识"/> | <img src="/ssl-s1.svg" alt="SSL 相关标识"/> | All right reserved
    </div>
    <div class="one-line">
      <a v-if="config.ICP" href="https://beian.miit.gov.cn/" target="_blank" rel="noreferrer" >{{ config.ICP }}</a>
      <span v-if="config.ICP">&nbsp;|&nbsp;</span>
      <el-image v-if="config.GongAn" style="height: 1em; width: 1em;" src="/备案图标.png" />
      <a :href="'https://beian.mps.gov.cn/#/query/webSearch?code=' + cleanChineseCharacters(config.GongAn)" target="_blank" rel="noreferrer" >{{ config.GongAn }}</a>
      <span v-if="config.GongAn">&nbsp;|&nbsp;</span>
      <a href="https://www.china-ipv6.cn/" target="_blank" rel="noreferrer" >国家IPv6发展监测平台</a>
      &nbsp;|&nbsp;请遵守中国法律法规
   </div>
   <div class="one-line">
      致力于普及IPv6，推进IPv6规模部署和应用，以全面推进IPv6技术创新与融合应用为主线，以提升应用广度深度为主攻方向
  </div>
  </footer>

</template>
<style scoped>
@import "~/style.css";
.el-menu--horizontal > .el-menu-item:nth-child(1) {
  margin-right: auto;
}
:deep(.shiki span) {
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', 'Monaco', 'Courier New', monospace !important;
  word-wrap:break-word;
}
:deep(.shiki){
  padding: 20px;
  border-radius: 10px;
}
:deep(.el-menu-item a) {
  font-size: 1em;
}
:deep(.el-menu-item a p) {
  font-size: 1em;
}
:deep(.el-menu-item a img) {
  width: 50px;
  margin-bottom: 20px;
  
}
</style>
<style>
:root {
  --el-color-primary: #3EAF7C;
}
html.dark {
  --el-color-primary: #3EAF7C;
}

/* 防止窄屏设备在 Vue 水合前出现宽屏布局闪烁 */
html.is-narrow .el-drawer__container {
  display: none !important;
}
html.is-narrow .el-menu--horizontal > .el-menu-item[index="1"],
html.is-narrow .el-menu--horizontal > .el-sub-menu[index="2"],
html.is-narrow .el-menu--horizontal > .el-sub-menu[index="3"],
html.is-narrow .el-menu--horizontal > .el-sub-menu[index="4"] {
  display: none !important;
}

/* ===== 顶部菜单高亮美化 ===== */
.el-menu--horizontal {
  border: 1px solid rgba(62, 175, 124, 0.25) !important;
  border-radius: 14px;
  margin: 12px 14px 0;
  box-shadow: 0 2px 12px rgba(62, 175, 124, 0.08);
  position: relative;
  overflow: hidden;
}

/* 底部主色渐变指示线 */
.el-menu--horizontal::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 2px;
  background: linear-gradient(90deg, #3EAF7C 0%, #2E9A68 20%, rgba(62, 175, 124, 0.35) 45%, rgba(124, 77, 255, 0.28) 70%, transparent 100%);
  pointer-events: none;
}

html.dark .el-menu--horizontal {
  border-color: rgba(62, 175, 124, 0.35) !important;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.35);
}

.el-menu--horizontal .el-menu-item,
.el-menu--horizontal .el-sub-menu__title {
  font-size: 1em;
  transition: color 0.2s ease;
}

/* 当前激活菜单项 */
.el-menu--horizontal .el-menu-item.is-active,
.el-menu--horizontal .el-sub-menu.is-active > .el-sub-menu__title {
  color: #3EAF7C !important;
  font-weight: 600;
}

/* 悬停高亮 */
.el-menu--horizontal .el-menu-item:hover,
.el-menu--horizontal .el-sub-menu__title:hover {
  color: #3EAF7C !important;
}

/* 激活项下划线指示条 */
.el-menu--horizontal .el-menu-item.is-active::after {
  content: '';
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: 6px;
  height: 2px;
  border-radius: 2px;
  background: #3EAF7C;
}
.el-menu--horizontal .el-sub-menu.is-active > .el-sub-menu__title::after {
  content: '';
  position: absolute;
  left: 16px;
  right: 16px;
  bottom: 6px;
  height: 2px;
  border-radius: 2px;
  background: #3EAF7C;
}

/* 子菜单弹出面板项高亮 */
.el-menu--horizontal .el-menu--popup .el-menu-item.is-active {
  color: #3EAF7C !important;
  font-weight: 600;
  background: rgba(62, 175, 124, 0.08) !important;
}

/* ===== 移动端抽屉分组样式 ===== */
.drawer-section {
  margin-bottom: 6px;
}
.drawer-title {
  font-size: 0.85em;
  font-weight: 600;
  color: #909399;
  margin: 14px 10px 4px;
  letter-spacing: 1px;
}
.drawer-section > a {
  display: block;
  padding: 9px 10px;
  border-radius: 8px;
  color: #303133;
  text-decoration: none;
  transition: all 0.2s ease;
}
.drawer-section > a:hover {
  background: rgba(62, 175, 124, 0.1);
  color: #3EAF7C;
}
html.dark .drawer-section > a {
  color: #cfcfcf;
}
html.dark .drawer-section > a:hover {
  background: rgba(62, 175, 124, 0.15);
  color: #4BC98E;
}

/* GitHub 图标 */
.github-link {
  display: flex;
  align-items: center;
  color: #606266;
  transition: color 0.2s ease;
}
.github-link:hover {
  color: #3EAF7C;
}
html.dark .github-link {
  color: #cfcfcf;
}
html.dark .github-link:hover {
  color: #4BC98E;
}

/* ===== 页脚美化 ===== */
footer {
  margin-top: 40px;
  padding: 24px 16px 16px;
  border-top: 1px solid rgba(62, 175, 124, 0.2);
  background: linear-gradient(180deg, transparent, rgba(62, 175, 124, 0.04));
  text-align: center;
}
footer .one-line {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  line-height: 1.8;
  font-size: 0.95em;
  color: #606266;
}
html.dark footer .one-line {
  color: #9ca3af;
}
footer .one-line a {
  color: #3EAF7C;
  text-decoration: none;
  transition: color 0.2s;
}
footer .one-line a:hover {
  color: #2E9A68;
  text-decoration: underline;
}
footer .one-line img {
  height: 1.1em;
  vertical-align: middle;
}
</style>
