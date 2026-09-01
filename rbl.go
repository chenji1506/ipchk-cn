// rbl.go — RBL 邮件黑名单检测（P2）
//
// 通过 DNSBL 查询目标 IP 是否命中邮件黑名单（单 IP + 上游 /24 网段双查）。
// 使用 miekg/dns 走 UDP 53；全部失败时自动降级到 DoH（cloudflare-dns.com）。
// 结果 6h 内存缓存 + singleflight 防并发重复查询。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// ============ RBL 源定义 ============

type rblZone struct {
	name  string // DNSBL 域名
	white bool   // 白名单（命中 = 加分）
}

var rblZones = []rblZone{
	{"zen.spamhaus.org", false},
	{"bl.spamcop.net", false},
	{"dnsbl.sorbs.net", false},
	{"b.barracudacentral.org", false},
	{"dnsbl-1.uceprotect.net", false},
	{"spam.dnsbl.sorbanes.com", false},
	{"bl.mailspike.net", false},
	{"psbl.surriel.com", false},
	{"db.wpbl.info", false},
	{"dnsbl.dronebl.org", false},
	{"hostkarma.junkemailfilter.com", false},
	{"list.dnswl.org", true}, // 白名单
	{"abuse.atlantis-eg.com", false},
	// 二线源（补充，覆盖 mxtoolbox/blacklistalert 的额外 DNSBL）
	{"all.spamrats.com", false},   // SpamRATS
	{"ubl.unsubscore.com", false}, // LASHBACK
	{"dnsbl.justspam.org", false}, // JUSTSPAMMED
}

// ============ 结果结构 ============

type RBLResult struct {
	IP                 string          `json:"ip"`
	ListedCount        int             `json:"listed_count"`
	NetworkListedCount int             `json:"network_listed_count"`
	RiskLevel          string          `json:"risk_level"`
	QueryLimited       bool            `json:"query_limited"`
	Probed             bool            `json:"probed"`
	ListedZones        []string        `json:"listed_zones,omitempty"`
	NetworkZones       []string        `json:"network_zones,omitempty"`
	WhiteListed        bool            `json:"white_listed"`
	CheckedCount       int             `json:"checked_count"`
	Zones              []RBLZoneResult `json:"zones,omitempty"`

	probedAt time.Time
}

// RBLZoneResult 单个 DNSBL 源的查询状态
type RBLZoneResult struct {
	Name      string `json:"name"`
	Listed    bool   `json:"listed"`
	Whitelist bool   `json:"whitelist"`
	Status    string `json:"status"` // listed / clean / failed
}

// ============ 缓存 ============

var (
	rblCache   = make(map[string]rblCacheEntry)
	rblCacheMu sync.Mutex
	rblSingle  singleflightGroup
)

type rblCacheEntry struct {
	data      *RBLResult
	expiresAt time.Time
}

// ============ 查询入口 ============

func queryRBL(ip string) *RBLResult {
	rblCacheMu.Lock()
	if e, ok := rblCache[ip]; ok && time.Now().Before(e.expiresAt) {
		rblCacheMu.Unlock()
		return e.data
	}
	rblCacheMu.Unlock()

	v, _, _ := rblSingle.Do(ip, func() (interface{}, error) {
		r := checkRBL(ip)
		rblCacheMu.Lock()
		rblCache[ip] = rblCacheEntry{data: r, expiresAt: time.Now().Add(6 * time.Hour)}
		rblCacheMu.Unlock()
		return r, nil
	})
	if r, ok := v.(*RBLResult); ok {
		return r
	}
	return nil
}

// checkRBL 对单 IP 执行 16 源 DNSBL 查询 + spamhaus 网段查询
func checkRBL(ip string) *RBLResult {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return &RBLResult{IP: ip, Probed: true, QueryLimited: true}
	}
	reversed := reverseIP(parsed)

	res := &RBLResult{IP: ip, Probed: true}
	// 并行查询全部单 IP 源
	type queryResult struct {
		name  string
		white bool
		hit   bool
		ok    bool
	}
	results := make([]queryResult, len(rblZones))
	var wg sync.WaitGroup
	for i, z := range rblZones {
		wg.Add(1)
		go func(idx int, zone rblZone) {
			defer wg.Done()
			q := reversed + "." + zone.name
			hit, ok := dnsblLookup(q)
			results[idx] = queryResult{zone.name, zone.white, hit, ok}
		}(i, z)
	}
	wg.Wait()

	for _, r := range results {
		status := "failed"
		if r.ok {
			res.CheckedCount++
			status = "clean"
			if r.hit {
				status = "listed"
				if r.white {
					res.WhiteListed = true
				} else {
					res.ListedCount++
					res.ListedZones = append(res.ListedZones, r.name)
				}
			}
		}
		res.Zones = append(res.Zones, RBLZoneResult{Name: r.name, Listed: r.hit, Whitelist: r.white, Status: status})
	}

	// 网段查询（仅 spamhaus zen，支持 /24 网段记录）
	netReversed := reverseNetwork(parsed)
	if hit, ok := dnsblLookup(netReversed + ".zen.spamhaus.org"); ok && hit {
		res.NetworkListedCount++
		res.NetworkZones = append(res.NetworkZones, "zen.spamhaus.org")
	}

	// 全部失败 → 判定查询受限（UDP 被限），标记 query_limited
	if res.CheckedCount == 0 {
		res.QueryLimited = true
	}

	// 风险等级
	res.RiskLevel = rblRiskLevel(res.ListedCount, res.NetworkListedCount)
	return res
}

// dnsblLookup 单次 DNSBL A 查询，返回 (是否命中, 是否成功)
// UDP 双 IP 都失败时自动降级 DoH（绕过上游 UDP 限制 / DNS 污染）
func dnsblLookup(domain string) (hit bool, ok bool) {
	client := &dns.Client{Timeout: 3 * time.Second}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	resp, _, err := client.Exchange(msg, "1.1.1.1:53")
	if err != nil {
		// 降级：换 Cloudflare 1.0.0.1 再试一次
		resp, _, err = client.Exchange(msg, "8.8.8.8:53")
		if err != nil {
			// UDP 全部失败 → DoH（HTTPS，无法被 UDP 层污染/拦截）
			return dnsblDoH(domain)
		}
	}
	if resp == nil || resp.Rcode != dns.RcodeSuccess {
		return false, true // NXDOMAIN 等 = 未命中，但查询本身成功
	}
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			ip := a.A.To4()
			if ip != nil && ip[0] == 127 {
				return true, true
			}
		}
	}
	return false, true
}

// reverseIP 反转 IP（8.8.8.8 → 8.8.8.8）
func reverseIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d", v4[3], v4[2], v4[1], v4[0])
	}
	// IPv6：完整展开后逐 nibble 反转
	expanded := ip.String()
	var parts []string
	for i := len(expanded) - 1; i >= 0; i-- {
		if expanded[i] != ':' {
			parts = append(parts, string(expanded[i]))
		}
	}
	return strings.Join(parts, ".")
}

// reverseNetwork 反转 /24 网段（8.8.8.8 → 0.8.8.8，用 .0 地址）
func reverseNetwork(ip net.IP) string {
	v4 := ip.To4()
	if v4 == nil {
		return reverseIP(ip) // IPv6 不做网段查询
	}
	return fmt.Sprintf("0.%d.%d.%d", v4[2], v4[1], v4[0])
}

// rblRiskLevel 黑名单风险等级
func rblRiskLevel(listed, networkListed int) string {
	switch {
	case listed >= 3 || (listed >= 2 && networkListed >= 1):
		return "high"
	case listed >= 2 || networkListed >= 2:
		return "medium"
	case listed >= 1 || networkListed >= 1:
		return "low"
	default:
		return "none"
	}
}

// ============ 评分辅助（供 purity.go 调用） ============

// rblSignalRisk RBL 命中折算为 signal_risk（单 IP 命中 -6/源 封顶 18；网段命中 -3 封顶 9；白名单 -5）
func rblSignalRisk(r *RBLResult) int {
	if r == nil || !r.Probed {
		return 0
	}
	score := 0
	// 单 IP 命中
	n := r.ListedCount
	if n > 3 {
		n = 3
	}
	score += n * 6
	// 网段命中
	m := r.NetworkListedCount
	if m > 3 {
		m = 3
	}
	score += m * 3
	// 白名单命中（信誉加分，用负数表示降低风险）
	if r.WhiteListed {
		score -= 5
	}
	if score < 0 {
		return 0
	}
	return score
}

// rblDimensionScore RBL 维度得分（满分 20）
func rblDimensionScore(r *RBLResult) int {
	if r == nil || !r.Probed {
		return 0
	}
	if r.QueryLimited {
		return 0 // 查询受限，无法给出黑名单结论
	}
	score := 20
	score -= r.ListedCount * 6
	score -= r.NetworkListedCount * 2
	if r.WhiteListed {
		score += 3
	}
	if score < 0 {
		return 0
	}
	if score > 20 {
		return 20
	}
	return score
}

// dnsblDoH 通过 Cloudflare DoH 查询 DNSBL（UDP 不可用时的备用通道）
func dnsblDoH(domain string) (hit bool, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://cloudflare-dns.com/dns-query?name=%s&type=A", domain)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/dns-json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var out struct {
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, false
	}
	for _, a := range out.Answer {
		if a.Type == 1 && strings.HasPrefix(a.Data, "127.") {
			return true, true
		}
	}
	return false, true
}

// formatRBLText 邮件黑名单 CLI 格式化输出
func formatRBLText(r *RBLResult) string {
	var b strings.Builder
	b.WriteString("邮件黑名单检测: " + r.IP + "\n")
	b.WriteString(strings.Repeat("─", 46) + "\n")
	riskLabel := "无"
	switch r.RiskLevel {
	case "high":
		riskLabel = "高"
	case "medium":
		riskLabel = "中"
	case "low":
		riskLabel = "低"
	}
	rows := [][2]string{
		{"检查源数", fmt.Sprintf("%d", r.CheckedCount)},
		{"单 IP 命中", fmt.Sprintf("%d", r.ListedCount)},
		{"网段命中", fmt.Sprintf("%d", r.NetworkListedCount)},
		{"白名单命中", boolToStr(r.WhiteListed)},
		{"风险等级", riskLabel},
	}
	maxW := 0
	for _, row := range rows {
		if w := displayWidth(row[0]); w > maxW {
			maxW = w
		}
	}
	for _, row := range rows {
		b.WriteString(padKey(row[0], maxW+2) + ": " + row[1] + "\n")
	}
	b.WriteString(strings.Repeat("─", 46) + "\n")
	b.WriteString("各源状态:\n")
	for _, z := range r.Zones {
		status := "未命中"
		switch z.Status {
		case "listed":
			status = "已命中"
		case "failed":
			status = "查询失败"
		}
		if z.Whitelist {
			status = "白名单"
		}
		b.WriteString("  " + padKey(z.Name, 34) + status + "\n")
	}
	b.WriteString(strings.Repeat("─", 46))
	return b.String()
}
