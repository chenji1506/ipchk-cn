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
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: [
    '@element-plus/nuxt',
    '@nuxtjs/sitemap',
    '@nuxtjs/robots',
    '@vueuse/nuxt',
    "@nuxtjs/i18n",
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
  i18n: {
    locales: [
      { code: 'zh', name: '中文', language: 'zh-CN', file: 'zh.json' },
      { code: 'en', name: 'English', language: 'en-US', file: 'en.json' },
    ],
    defaultLocale: 'zh',
    langDir: 'locales',
    strategy: 'prefix_except_default',
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
      docConfig: docConfig,
      // 域名配置：启动时可用 NUXT_PUBLIC_* 环境变量覆盖（无需重新 build）
      siteUrl: process.env.NUXT_PUBLIC_SITE_URL || config.siteUrl,
      v4OnlyAPI: process.env.NUXT_PUBLIC_V4_API || config.v4OnlyAPI,
      v6OnlyAPI: process.env.NUXT_PUBLIC_V6_API || config.v6OnlyAPI,
      dualStackAPI: process.env.NUXT_PUBLIC_DUALSTACK_API || config.DualStackAPI,
      // API 基址：本地部署时设 NUXT_PUBLIC_API_BASE=http://127.0.0.1:8080/，
      // 生产默认 https://ipchk.cn/
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'https://ipchk.cn/',

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
          'http:',  // 后端地址可配置（本地/内网 http），静态枚举无法覆盖，用 scheme 通配
          'https:',
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
