# ipchk.cn - IP 查询与网络诊断工具

基于 GPL-3.0 开源协议构建的 IP 查询与网络诊断工具站，源码遵循 GPL v3.0 开源。

在线体验：https://ipchk.cn

## 功能

### IP 查询
- **IP 归属地查询** — 支持 IPv4/IPv6，优先调用 ip-api.com 在线 API（中文数据），失败自动回退本地数据库（ip2region、qqwry 纯真、DB-IP 等）
- **IP 纯净度检测** — 风险评分（0-100）、风险等级、数据中心/住宅 IP 识别、代理/VPN 特征分析
- **IP 信息卡片** — 生成 IP 归属地 + 纯净度评分的 SVG 分享图
- **批量 IP 查询** — 一次查询最多 20 个 IP 的归属地，带查询历史记录
- **子域名查询** — `curl 4.ipchk.cn` 查 IPv4、`curl 6.ipchk.cn` 查 IPv6

### 网络诊断
- **网站检测** — HTTP 状态码、响应时间、Host 记录
- **SSL 证书检查** — 证书有效期、颁发机构、剩余天数
- **DNS 解析** — 支持 A/AAAA/CNAME/MX/TXT/NS/SRV/PTR/CAA 等记录类型
- **TCPing 测试** — TCP 连接延迟测试，支持 IPv4/IPv6 双栈
- **网站测速** — 下载速度、响应头、Host 记录
- **端口扫描** — 26 个常见端口并发扫描，支持自定义端口
- **Whois 查询** — 基于 whois 协议（TCP 43）支持全部域名后缀，IP 走 RDAP 协议

### 实用工具
- **子网计算器** — IPv4/IPv6 CIDR 计算（网络地址/广播地址/掩码/主机范围/地址总数）
- **隐私泄露检测** — WebRTC 泄露检测 + DNS 泄露检测
- **API 文档** — 全部开放接口说明与示例

### 其他特性
- **命令行友好** — curl 访问归属地/纯净度接口返回格式化文本
- **暗色模式** — 支持明暗主题切换
- **多子域名** — 4.ipchk.cn（IPv4）/ 6.ipchk.cn（IPv6）协议强制探测
- **WebRTC 兜底** — 首页 IPv6 探测失败时自动用 WebRTC 读取本机 IPv6

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/ip` | 查询本机公网 IP（纯文本） |
| GET | `/v1/location/:ip` | IP 归属地查询（JSON，curl 返回格式化文本） |
| GET | `/v1/purity/:ip` | IP 纯净度评分 |
| GET | `/v1/card/:ip` | IP 信息卡片（SVG 图片） |
| GET | `/v1/scan/:ip` | 端口扫描（?ports=22,80,443 自定义） |
| GET | `/v1/whois/:target` | Whois 查询（域名/IP） |
| GET | `/v1/dns/:type/:domain` | DNS 解析 |
| GET | `/v1/ssl/:domain` | SSL 证书检测 |
| GET | `/v1/tcping/:host` | TCP 连接测试（?port=80&count=4） |
| GET | `/v1/speed/:ver/:url` | 网站测速（ver: v4/v6/dual） |

## 项目结构

```
ipw-cn/
├── main.go                  # Go 后端入口（Gin 框架）
├── go.mod                   # Go 模块定义（ipchk-cn）
├── Dockerfile               # 后端镜像（多阶段编译）
├── Dockerfile.frontend      # 前端镜像（源码构建）
├── docker-compose.yml       # Docker Compose 编排
├── setting.json             # 后端运行配置
├── deploy/
│   ├── install.sh           # 一键部署脚本
│   ├── DEPLOY.md            # 部署/迁移指南
│   └── nginx/               # nginx 配置（域名/反向代理/gzip）
├── frontend-ssr/            # Nuxt 4 SSR 前端源码
│   └── app/pages/           # 全部页面（归属地/纯净度/DNS/SSL/扫描/whois/批量/子网/泄露）
├── ipdb/                    # IP 数据库查询模块（本地库兜底）
├── webtest/                 # 网络测试工具（DNS/TCPing）
└── ssrf/                    # SSRF 防护
```

## 部署

```bash
# Docker Compose 部署（源码即镜像）
docker compose build
docker compose up -d

# 或一键部署脚本（含 nginx 配置/认证/构建）
bash deploy/install.sh
```

详细部署与迁移指南见 [deploy/DEPLOY.md](deploy/DEPLOY.md)。

## 技术栈

- 后端：Go 1.26 + Gin
- 前端：Nuxt 4 SSR + Vue 3 + Element Plus
- IP 数据：ip-api.com（在线）+ ip2region/qqwry（本地兜底）
- 部署：Docker Compose + OpenResty

## 许可证

GPL-3.0（保留 GPL-3.0 许可证）
