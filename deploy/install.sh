#!/bin/bash
# ipchk.cn 一键部署/迁移脚本
# 用法: bash deploy/install.sh
# 前置: 服务器已安装 Docker + Docker Compose + OpenResty
#       DNS 已解析 ipchk.cn / www / 4 / 6 到本机
#       证书在 /etc/openresty/ssl/ipchk.cn/（含 www.ipchk.cn.pem + key）

set -e
cd "$(dirname "$0")/.."
echo "=== ipchk.cn 部署脚本 ==="

# 1. 部署 nginx 配置（含 Basic Auth 认证文件）
echo "[1/4] 部署 nginx 配置..."
mkdir -p /etc/openresty/conf.d
cp deploy/nginx/ipchk.cn.conf /etc/openresty/conf.d/
cp deploy/nginx/ipchk-locations.inc /etc/openresty/conf.d/
cp deploy/nginx/zz-gzip.conf /etc/openresty/conf.d/

# 确保 nginx.conf 包含 conf.d（幂等）
grep -q "conf.d/\*.conf" /etc/openresty/nginx.conf || \
  sed -i '/include       mime.types;/a\    include /etc/openresty/conf.d/*.conf;' /etc/openresty/nginx.conf

# 2. 创建认证文件（如果不存在）
echo "[2/4] 配置 Basic Auth..."
if [ ! -f /etc/openresty/.htpasswd ]; then
  if [ -z "$STATS_PASSWORD" ]; then
    echo "提示: 设置 STATS_PASSWORD 环境变量可自定义统计密码（默认 Ipchk@2026）"
    STATS_PASSWORD="Ipchk@2026"
  fi
  echo "ipchk:$(openssl passwd -apr1 "$STATS_PASSWORD")" > /etc/openresty/.htpasswd
  chmod 644 /etc/openresty/.htpasswd
  echo "已创建 /etc/openresty/.htpasswd"
fi

# 3. 检查证书
echo "[3/4] 检查证书..."
CERT=/etc/openresty/ssl/ipchk.cn/www.ipchk.cn.pem
if [ ! -f "$CERT" ]; then
  echo "⚠️ 未找到证书 $CERT"
  echo "   请先放置证书（SAN 需覆盖 ipchk.cn www.ipchk.cn 4.ipchk.cn 6.ipchk.cn）"
  echo "   或用 acme.sh: acme.sh --issue -d ipchk.cn -d '*.ipchk.cn' --dns dns_xxx"
  exit 1
fi
echo "证书: $CERT ✓"

# 4. 构建并启动容器
echo "[4/4] 构建并启动 Docker 容器..."
docker compose build
docker compose up -d

echo ""
echo "=== 部署完成 ==="
echo "前端: https://ipchk.cn"
echo "统计: https://ipchk.cn/analytics  (ipchk / 密码)"
echo "日志: https://ipchk.cn/logs"
echo "API:  https://ipchk.cn/v1/location/8.8.8.8"
echo ""
echo "验证:"
echo "  curl https://ipchk.cn/ip"
echo "  curl https://4.ipchk.cn"
echo "  curl https://ipchk.cn/v1/analytics -u ipchk:密码"
