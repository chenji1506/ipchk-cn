package main

import (
	"context"
	"fmt"
	"ipchk-cn/ssrf"
	"ipchk-cn/webtest"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ==================== HTTP 请求头检测 ====================

// headersHandler 返回客户端发送的完整 HTTP 请求头（自检：看网站能读到你的哪些信息）
func headersHandler(c *gin.Context) {
	ip := c.ClientIP()
	// 内网 IPv4 访问时改用服务器出网公网 IP（与 /ip 端点一致，避免显示 192.168.x.x）
	if isPrivateAddr(ip) && !strings.Contains(ip, ":") {
		if pub := fetchPublicIP(); pub != "" {
			ip = pub
		}
	}
	if isCLIUA(c.GetHeader("User-Agent")) {
		var b strings.Builder
		fmt.Fprintf(&b, "IP: %s\n请求行: %s %s %s\n\n", ip, c.Request.Method, c.Request.RequestURI, c.Request.Proto)
		for k, v := range c.Request.Header {
			for _, vv := range v {
				fmt.Fprintf(&b, "%s: %s\n", k, vv)
			}
		}
		c.String(http.StatusOK, b.String())
		return
	}
	headers := make(map[string][]string, len(c.Request.Header))
	for k, v := range c.Request.Header {
		headers[k] = v
	}
	writeJSON(c, map[string]interface{}{
		"ip":       ip,
		"method":   c.Request.Method,
		"protocol": c.Request.Proto,
		"host":     c.Request.Host,
		"headers":  headers,
	})
}

// ==================== Tor 出口节点检测 ====================

// torHandler 检测 IP 是否为 Tor 出口节点（Tor 官方 DNSEL）
func torHandler(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		ip = c.ClientIP()
		// 内网 IPv4 访问时改用公网 IP（与 /ip 端点一致）
		if isPrivateAddr(ip) && !strings.Contains(ip, ":") {
			if pub := fetchPublicIP(); pub != "" {
				ip = pub
			}
		}
	}
	if net.ParseIP(ip) == nil {
		writeJSON(c, gin.H{"error": "invalid IP address", "ip": ip})
		return
	}
	isTor, detail := queryTorDNSEL(ip)
	writeJSON(c, map[string]interface{}{
		"ip":         ip,
		"is_tor":     isTor,
		"detail":     detail,
		"checked_at": time.Now().Format(time.RFC3339),
	})
}

// queryTorDNSEL 通过 Tor 官方 DNSEL（dnsel.torproject.org）判断是否为 Tor 出口节点。
// 反转 IPv4 后加后缀，返回 127.0.0.x 表示该 IP 是 Tor 出口（x 为允许端口位掩码：1=80, 2=443, 4=6667）。
func queryTorDNSEL(ip string) (bool, string) {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return false, "IPv6 暂不支持 DNSEL 检测"
	}
	parts := strings.Split(parsed.String(), ".")
	if len(parts) != 4 {
		return false, "无效 IPv4 地址"
	}
	reversed := parts[3] + "." + parts[2] + "." + parts[1] + "." + parts[0] + ".dnsel.torproject.org"
	addrs, err := net.LookupHost(reversed)
	if err != nil || len(addrs) == 0 {
		return false, "非 Tor 出口节点"
	}
	for _, a := range addrs {
		if strings.HasPrefix(a, "127.0.0.") {
			return true, "Tor 出口节点（DNSEL 命中 " + a + "）"
		}
	}
	return false, "非 Tor 出口节点"
}

// ==================== 网站安全响应头检测 ====================

type securityHeaderItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Present     bool   `json:"present"`
	Value       string `json:"value"`
	Score       int    `json:"score"`
	MaxScore    int    `json:"max_score"`
	Level       string `json:"level"` // good / warn / missing
	Description string `json:"description"`
}

// securityHeadersHandler 检测网站的 HTTPS 与安全响应头
func securityHeadersHandler(c *gin.Context) {
	host := strings.TrimSpace(c.Param("url"))
	host = strings.TrimPrefix(host, "/")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSpace(host)
	if host == "" {
		writeJSON(c, gin.H{"error": "url required"})
		return
	}

	httpsURL := "https://" + host
	ctx := context.Background()
	var err error
	ctx, err = ssrf.ValidateOutboundTarget(ctx, httpsURL)
	if err != nil {
		writeJSON(c, gin.H{"error": err.Error()})
		return
	}

	resp, err := V4Client.R().SetContext(ctx).Get(httpsURL)
	httpsEnabled := true
	finalURL := httpsURL
	if err != nil {
		httpURL := "http://" + host
		resp2, err2 := V4Client.R().SetContext(ctx).Get(httpURL)
		if err2 != nil {
			writeJSON(c, gin.H{"url": host, "error": "请求失败（HTTPS/HTTP 均不可达）：" + err.Error()})
			return
		}
		resp = resp2
		httpsEnabled = false
		finalURL = httpURL
	}

	get := func(k string) string { return resp.RawResponse.Header.Get(k) }

	checks := []securityHeaderItem{
		{Key: "hsts", Name: "Strict-Transport-Security (HSTS)", Value: get("Strict-Transport-Security"), MaxScore: 15, Level: "missing", Description: "强制浏览器使用 HTTPS，防止降级攻击"},
		{Key: "csp", Name: "Content-Security-Policy (CSP)", Value: get("Content-Security-Policy"), MaxScore: 25, Level: "missing", Description: "限制可加载的资源来源，防 XSS/注入"},
		{Key: "xfo", Name: "X-Frame-Options", Value: get("X-Frame-Options"), MaxScore: 10, Level: "missing", Description: "防止页面被 iframe 嵌入（点击劫持）"},
		{Key: "xcto", Name: "X-Content-Type-Options", Value: get("X-Content-Type-Options"), MaxScore: 10, Level: "missing", Description: "禁止浏览器 MIME 嗅探，防类型混淆"},
		{Key: "referrer", Name: "Referrer-Policy", Value: get("Referrer-Policy"), MaxScore: 10, Level: "missing", Description: "控制跨域请求时的来源泄露"},
		{Key: "permissions", Name: "Permissions-Policy", Value: get("Permissions-Policy"), MaxScore: 10, Level: "missing", Description: "限制浏览器特性（摄像头/麦克风等）"},
		{Key: "xxss", Name: "X-XSS-Protection", Value: get("X-XSS-Protection"), MaxScore: 5, Level: "missing", Description: "旧版 XSS 过滤（已弃用，现代浏览器忽略）"},
	}

	total := 0
	maxTotal := 0
	for i := range checks {
		ch := &checks[i]
		maxTotal += ch.MaxScore
		if ch.Value != "" {
			ch.Present = true
			if ch.Key == "xxss" {
				ch.Level = "warn" // 已弃用，不计分
				ch.Score = 0
			} else {
				ch.Level = "good"
				ch.Score = ch.MaxScore
				total += ch.Score
			}
		}
	}

	// HTTPS 单独计分
	httpsScore := 0
	if httpsEnabled {
		httpsScore = 15
	}
	maxTotal += 15
	total += httpsScore

	writeJSON(c, map[string]interface{}{
		"url":         host,
		"final_url":   finalURL,
		"https":       httpsEnabled,
		"status_code": resp.StatusCode(),
		"https_score": httpsScore,
		"checks":      checks,
		"score":       total,
		"max_score":   maxTotal,
		"grade":       securityGrade(total, maxTotal),
	})
}

func securityGrade(score, max int) string {
	if max <= 0 {
		return "F"
	}
	pct := score * 100 / max
	switch {
	case pct >= 95:
		return "A+"
	case pct >= 85:
		return "A"
	case pct >= 70:
		return "B"
	case pct >= 55:
		return "C"
	case pct >= 35:
		return "D"
	default:
		return "F"
	}
}

// ==================== DNS 污染/劫持检测 ====================

// dnsPollutionHandler 用多个公共 DNS 服务器解析同一域名，对比结果检测污染/劫持
func dnsPollutionHandler(c *gin.Context) {
	domain := strings.TrimSpace(c.Param("domain"))
	domain = strings.TrimPrefix(domain, "/")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	domain = strings.TrimSpace(domain)
	if domain == "" {
		writeJSON(c, gin.H{"error": "domain required"})
		return
	}

	type dnsServer struct {
		Name   string
		Server string
		Region string
	}
	servers := []dnsServer{
		{Name: "阿里云 DNS", Server: "223.5.5.5:53", Region: "国内"},
		{Name: "腾讯 DNSPod", Server: "119.29.29.29:53", Region: "国内"},
		{Name: "Google DNS", Server: "8.8.8.8:53", Region: "国外"},
		{Name: "Cloudflare DNS", Server: "1.1.1.1:53", Region: "国外"},
	}

	type serverResult struct {
		Name     string   `json:"name"`
		Server   string   `json:"server"`
		Region   string   `json:"region"`
		Records  []string `json:"records"`
		Duration float64  `json:"duration"`
		Error    string   `json:"error,omitempty"`
	}

	results := make([]serverResult, len(servers))
	var wg sync.WaitGroup
	for i, s := range servers {
		wg.Add(1)
		go func(i int, s dnsServer) {
			defer wg.Done()
			r, err := webtest.ResolveARecordWithServer(domain, s.Server)
			results[i] = serverResult{
				Name: s.Name, Server: s.Server, Region: s.Region,
				Records: r.Record, Duration: r.Duration,
			}
			if err != nil {
				results[i].Error = err.Error()
			}
		}(i, s)
	}
	wg.Wait()

	// 统计不同结果集（排除查询失败的服务器）
	recordSetCount := map[string]int{}
	for _, r := range results {
		if r.Error == "" {
			key := strings.Join(r.Records, ",")
			recordSetCount[key]++
		}
	}

	consistent := len(recordSetCount) <= 1
	conclusion := "各 DNS 服务器解析结果一致，未发现异常"
	if !consistent {
		conclusion = "解析结果存在差异：可能是 DNS 污染/劫持，也可能是 CDN 分区域解析（国内/国外解析到不同节点），请结合 IP 归属判断"
	}

	writeJSON(c, map[string]interface{}{
		"domain":     domain,
		"servers":    results,
		"consistent": consistent,
		"conclusion": conclusion,
	})
}

// ==================== HTTP 版本检测 ====================

// httpVersionHandler 检测网站支持的 HTTP 版本（HTTP/1.1 / HTTP/2 / HTTP/3）
func httpVersionHandler(c *gin.Context) {
	host := strings.TrimSpace(c.Param("url"))
	host = strings.TrimPrefix(host, "/")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSpace(host)
	if host == "" {
		writeJSON(c, gin.H{"error": "url required"})
		return
	}

	httpsURL := "https://" + host
	ctx := context.Background()
	var err error
	ctx, err = ssrf.ValidateOutboundTarget(ctx, httpsURL)
	if err != nil {
		writeJSON(c, gin.H{"error": err.Error()})
		return
	}

	// HTTP/2 检测：Go 的 http.Transport 默认对 HTTPS 启用 HTTP/2（ALPN 协商），
	// 协商结果反映在 resp.Proto（HTTP/2.0 或 HTTP/1.1）。
	httpProtocol := ""
	http2Supported := false
	resp, err := V4Client.R().SetContext(ctx).Get(httpsURL)
	if err == nil {
		httpProtocol = resp.RawResponse.Proto
		http2Supported = resp.RawResponse.Proto == "HTTP/2.0"
	}

	// HTTP/3 检测：查 DNS HTTPS/SVCB 记录（type 65），alpn 含 h3 即支持
	alpns := webtest.QueryHTTPSAlpn(host)
	http3Supported := false
	for _, a := range alpns {
		if strings.HasPrefix(a, "h3") {
			http3Supported = true
			break
		}
	}

	// 汇总支持版本
	var supported []string
	if httpProtocol != "" {
		supported = append(supported, strings.Replace(httpProtocol, "HTTP/2.0", "HTTP/2", 1))
	}
	if http3Supported {
		supported = append(supported, "HTTP/3")
	}

	writeJSON(c, map[string]interface{}{
		"url":        host,
		"reachable":  err == nil,
		"negotiated": httpProtocol,
		"http2":      http2Supported,
		"http3":      http3Supported,
		"alpn":       alpns,
		"supported":  supported,
	})
}
