import {config} from "./config/index";
import { docConfig } from "./config/doc";
// https://nuxt.com/docs/api/configuration/nuxt-config
const extractDomains = (obj: any): string[] => {
  // 将对象转为 JSON 字符串，用正则匹配所有 https:// 开头的域名部分
  const urls = JSON.stringify(obj).match(/https?:\/\/[^"\/\\\s]+/g) || [];
  // 提取域名 (Origin) 并去重
  const domains = [...new Set(urls.map(url => new URL(url).origin))];
  return domains;
};

const allowedDomains = extractDomains(config);
// API 基址：默认 https://ipchk.cn/，运行时可用 NUXT_PUBLIC_API_BASE 环境变量覆盖
// （docker compose 启动时设置，无需重新 build）
const apiBase = process.env.NUXT_PUBLIC_API_BASE || 'https://ipchk.cn/'
// connect-src 白名单：静态纳入常见后端地址，build 一次本地/生产通用
const connectDomains = [...new Set([
  ...allowedDomains,
  new URL(apiBase).origin,
  'http://127.0.0.1:8080',    // 本地 docker 部署（host 网络）
  'http://localhost:8080',
])];
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: [
    '@element-plus/nuxt',
    '@nuxtjs/sitemap',
    '@nuxtjs/robots',
    '@vueuse/nuxt',
    "nuxt-security",
  ],
  vite: {
    optimizeDeps: {
      include: [
        'dayjs',
        'dayjs/plugin/*.js',
        'is-ip',
        'lodash-unified',
        'shiki',
      ]
    },
  },
  site: { 
  url: config.siteUrl, 
  name: 'ipchk.cn' 
  },
  css: [
    // 1. 引入 Element Plus 基础样式 (如果你还没有引入的话)
    'element-plus/dist/index.css',
    
    // 2. 🌟 关键：引入 Element Plus 官方的暗黑模式 CSS 变量文件
    'element-plus/theme-chalk/dark/css-vars.css',
  ],
  app:{
    head: {
      script: [
        {
          // 必须 innerHTML，不能 src（否则异步加载）
          innerHTML: `
            (function() {
              var stored = localStorage.getItem('vueuse-color-scheme');
              var prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
              var dark = stored === 'dark' || (!stored && prefersDark);
              if (dark) document.documentElement.classList.add('dark');
            })();
          `,
          // 关键：不加 async/defer，确保同步阻塞执行
        }
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.svg' }
      ]
    }
  },
  runtimeConfig: {
    indexnowKey: '',
    public: {
      siteUrl: config.siteUrl,
      docConfig: docConfig,
      // API 基址：本地部署时设 NUXT_PUBLIC_API_BASE=http://127.0.0.1:8080/，
      // 生产默认 https://ipchk.cn/
      apiBase: apiBase,

    },
  },
nitro: {
  preset: 'node-server',
    publicAssets: [
      {
        dir: 'public',
        maxAge: 0
      }
    ],
    esbuild: {
      options: {
        target: 'es2022'
      }
    }
  },
  security: {
    headers: {
      contentSecurityPolicy: {
        'upgrade-insecure-requests': false,

        'script-src': [
          "'self'",
          "'strict-dynamic'",
          "'nonce-{{nonce}}'",
          "'wasm-unsafe-eval'",
          ...allowedDomains // 允许 Umami 发送数据
        ],
        
        'connect-src': [
          "'self'",
          ...connectDomains,// 含 apiBase（本地部署时 http://127.0.0.1:8080）
        ],
        
        'style-src': ["'self'", 'https:', "'unsafe-inline'"],
        'font-src': ["'self'", 'https:', 'data:'],
      }
    }
  },
  sitemap: {
    sources: [
      '/api/__sitemap__/urls',
    ]
  }


})
