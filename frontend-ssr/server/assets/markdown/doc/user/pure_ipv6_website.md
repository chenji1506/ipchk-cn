# <span style="background-color: #b95442;color: white;font-size: 0.43em;border-radius: 5px;padding: 2px 5px;">转载</span> 国内纯 IPv6 网站导航

纯 IPv6 网站意味网站域名 [只解析 IPv6 地址](https://www.ipchk.cn/dns)，没有解析 IPv4 地址，目前这类网站特别少，目前网站支持 IPv6 的主流方式为 IPv4/IPv6双栈访问。

下面是整理的纯 IPv6 网站资源。

## 网络工具类

| 名称 | IPv6访问地址 | 最近核验时间 |
| --- | --- | --- |
| IPv6查询 | ipchk.cn | 2022-3-6 |
| 清华大学TUNA镜像站（IPv6） | mirrors6.tuna.tsinghua.edu.cn | 2024 |

## 大学类

| 名称 | IPv6访问地址 | 最近核验时间 |
| --- | --- | --- |
| 北京大学 | [2001:da8:201:1512::a269:83a0] | 2022-3-6 |
| 中国科学院大学 | [2400:dd01:103a:4041::101] | 2022-3-6 |

## 说明

> - 国内纯 IPv6 网站数量极少，绝大多数网站均为 IPv4/IPv6 双栈
> - 纯 IPv6 站点主要集中在 CERNET2（教育科研网），但大量站点已下线或不可访问
> - 商业网站（百度、腾讯、阿里、网易等）均已部署双栈，不属于纯 IPv6 范畴
> - 验证纯 IPv6 方法：`dig AAAA <域名>` 确认仅有 AAAA 记录而无 A 记录
