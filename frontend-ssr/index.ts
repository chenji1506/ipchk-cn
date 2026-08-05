/*
    前端的一系列配置
*/
const config = {
    siteUrl: "http://192.168.1.72:3000/",
    // Umami 统计（本地部署不启用）
    umamiHost: "",
    umamiScriptUrl: "",
    umamiWebsiteId: "",
    // 备案信息（本地部署不需要）
    ICP: "",
    GongAn: "",
    // Worker IP查询接口（指向本地后端）
    v4OnlyAPI: "http://64.181.240.4:8080/ip",
    v6OnlyAPI: "http://64.181.240.4:8080/ip",
    DualStackAPI: "http://64.181.240.4:8080/ip",
    apiBaseUrls: [
        {
            label: "本地后端",
            url: "http://192.168.1.72:8080/"
        }
    ],
    IPLocationAPIs: [
        {
            label: "本地后端",
            url: "http://192.168.1.72:8080/"
        }
    ],
    // 全站是否禁止搜索引擎索引
    noindex: true,
    TCPing: {
        DualStack: [
            {
                label: "本地后端",
                url: "http://192.168.1.72:8080/"
            }
        ],
        IPv4: [
            {
                label: "本地后端",
                url: "http://192.168.1.72:8080/"
            }
        ],
        IPv6: [
            {
                label: "本地后端",
                url: "http://192.168.1.72:8080/"
            }
        ]
    },
    SpeedTest: {
        DualStack: [
            {
                label: "本地后端",
                url: "http://192.168.1.72:8080/"
            }
        ],
        IPv4: [
            {
                label: "本地后端",
                url: "http://192.168.1.72:8080/"
            }
        ],
        IPv6: [
            {
                label: "本地后端",
                url: "http://192.168.1.72:8080/"
            }
        ]
    },
    NSLookup: [
        {
            label: "本地后端",
            url: "http://192.168.1.72:8080/"
        }
    ]
}
export { config }
