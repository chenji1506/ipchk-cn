/*
    前端的一系列配置
*/
const config = {
    siteUrl: "https://ipchk.cn/",
    // Umami 统计（本地部署不启用）
    umamiHost: "",
    umamiScriptUrl: "",
    umamiWebsiteId: "",
    // 备案信息（本地部署不需要）
    ICP: "",
    GongAn: "",
    // IP查询接口：4.强制IPv4，6.强制IPv6，主域双栈判断优先
    v4OnlyAPI: "https://4.ipchk.cn/ip",
    v6OnlyAPI: "https://6.ipchk.cn/ip",
    DualStackAPI: "https://ipchk.cn/ip",
    apiBaseUrls: [
        {
            label: "本地服务器",
            url: "https://ipchk.cn/"
        }
    ],
    IPLocationAPIs: [
        {
            label: "本地服务器",
            url: "https://ipchk.cn/"
        }
    ],
    // 全站是否禁止搜索引擎索引
    noindex: false,
    TCPing: {
        DualStack: [
            {
                label: "本地服务器",
                url: "https://ipchk.cn/"
            }
        ],
        IPv4: [
            {
                label: "本地服务器",
                url: "https://ipchk.cn/"
            }
        ],
        IPv6: [
            {
                label: "本地服务器",
                url: "https://ipchk.cn/"
            }
        ]
    },
    SpeedTest: {
        DualStack: [
            {
                label: "本地服务器",
                url: "https://ipchk.cn/"
            }
        ],
        IPv4: [
            {
                label: "本地服务器",
                url: "https://ipchk.cn/"
            }
        ],
        IPv6: [
            {
                label: "本地服务器",
                url: "https://ipchk.cn/"
            }
        ]
    },
    NSLookup: [
        {
            label: "本地服务器",
            url: "https://ipchk.cn/"
        }
    ]
}
export { config }
