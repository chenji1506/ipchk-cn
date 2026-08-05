# ipchk.cn - IP 查询工具反向代理
# 80 与 443 行为完全一致：curl/wget 出 IP，浏览器出页面，API 相同

# 共享 locations 配置
# curl/wget 命令行访问根路径 → 直接返回 IP 地址
    location = / {
        if ($http_user_agent ~* "^(curl|wget)") {
            rewrite ^ /ip break;
            proxy_pass http://127.0.0.1:8080;
        }
        # 浏览器访问 → 前端页面
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }

    # 前端页面 → Nuxt SSR (3000)
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }

    # 静态资源（文件名带 hash，可长期缓存，二次访问秒开）
    location /_nuxt/ {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }

    # 后端 API → Go 服务 (8080)
    location /v1/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }

    # IP 查询接口
    location = /ip {
        proxy_pass http://127.0.0.1:8080/ip;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 源码下载（GPL-3.0 合规）
    location /source/ {
        alias /var/www/ipchk-source/;
        autoindex on;
    }
