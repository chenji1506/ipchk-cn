import { config as staticConfig } from '../../config/index';

// 运行时站点配置：域名字段可通过 NUXT_PUBLIC_* 环境变量在启动时覆盖
// （docker compose 环境变量注入，无需重新 build）。
// 覆盖字段：siteUrl / v4OnlyAPI / v6OnlyAPI / DualStackAPI / apiBaseUrls / IPLocationAPIs
// 其余字段（umami、ICP、noindex 等）保持 config/index.ts 静态值。
// 注意：不能命名 useSiteConfig（与 @nuxtjs/sitemap 内置函数重名会被覆盖）。
export function useIpchkConfig() {
  const rc = useRuntimeConfig();
  const p = rc.public as Record<string, any>;
  const apiBase = (p.apiBase as string) || staticConfig.apiBaseUrls[0].url;
  return {
    ...staticConfig,
    siteUrl: (p.siteUrl as string) || staticConfig.siteUrl,
    v4OnlyAPI: (p.v4OnlyAPI as string) || staticConfig.v4OnlyAPI,
    v6OnlyAPI: (p.v6OnlyAPI as string) || staticConfig.v6OnlyAPI,
    DualStackAPI: (p.dualStackAPI as string) || staticConfig.DualStackAPI,
    apiBaseUrls: [{ label: '本地服务器', url: apiBase }],
    IPLocationAPIs: [{ label: '本地服务器', url: apiBase }],
  };
}
