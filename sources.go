// sources.go — 多源 IP 证据聚合器（P1）
//
// 并行查询多个在线数据源，每个源解析为统一的 Evidence，
// 再做一致性投票（国家/ASN/类型），输出聚合结果供评分器使用。
// 查询结果 24h 内存缓存 + singleflight 防并发重复请求。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// ============ 统一证据结构 ============

// Evidence 单个数据源的解析结果
type Evidence struct {
	Source      string  `json:"source"`       // 数据源名
	CountryCode string  `json:"country_code"` // 国家代码（大写）
	Country     string  `json:"country"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	ASN         int     `json:"asn"`
	ASOrg       string  `json:"as_org"` // "AS15169 Google LLC" 或 org 名
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	Type        string  `json:"type"`    // hosting / business / isp / residential / mobile / unknown
	Proxy       bool    `json:"proxy"`   // 数据源显式标注的代理信号
	Hosting     bool    `json:"hosting"` // 数据源显式标注的机房信号
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone"`
}

// Aggregated 多源聚合结果
type Aggregated struct {
	IP string `json:"ip"`

	// 多数派结论
	CountryCode string `json:"country_code"` // 多数派国家
	Country     string `json:"country"`
	Region      string `json:"region"`
	City        string `json:"city"`
	ASN         int    `json:"asn"`
	ASOrg       string `json:"as_org"`
	ISP         string `json:"isp"`
	Org         string `json:"org"`
	Type        string `json:"type"` // 多数派类型

	// 一致性统计
	CountrySources  int    `json:"country_sources"`  // 参与国家投票的源数
	CountryMajority int    `json:"country_majority"` // 多数派国家源数
	CountryOutliers int    `json:"country_outliers"` // 离群国家源数
	ASNSources      int    `json:"asn_sources"`      // 参与 ASN 投票的源数
	ASNMajority     int    `json:"asn_majority"`     // 多数派 ASN 源数
	ASNOutliers     int    `json:"asn_outliers"`     // 离群 ASN 源数
	TypeSources     int    `json:"type_sources"`     // 参与类型投票的源数
	TypeMajority    int    `json:"type_majority"`    // 多数派类型源数
	CityConflict    bool   `json:"city_conflict"`    // 城市库差异
	RDAPRegistered  bool   `json:"rdap_registered"`  // RDAP 是否返回注册国
	RDAPCountry     string `json:"rdap_country"`     // RDAP 注册国
	RIR             string `json:"rir"`              // 所属 RIR

	// 信号
	ProxySignal       bool `json:"proxy_signal"`       // org/isp 关键词命中代理/VPN/Tor
	ProxySignalCount  int  `json:"proxy_signal_count"` // 命中代理信号的源数
	DataCenterSignal  bool `json:"data_center_signal"` // 机房关键词信号
	ResidentialSignal bool `json:"residential_signal"` // 住宅关键词信号

	// 覆盖
	Sources        []string  `json:"sources"`         // 成功源列表
	FailedSources  []string  `json:"failed_sources"`  // 失败源列表
	ExpectedSource int       `json:"expected_source"` // 期望源数
	ProbedAt       time.Time `json:"probed_at"`
}

// ============ 缓存 ============

var (
	multiSrcCache   = make(map[string]multiSrcCacheEntry)
	multiSrcCacheMu sync.Mutex
	multiSrcSingle  singleflightGroup
)

type multiSrcCacheEntry struct {
	data      *Aggregated
	expiresAt time.Time
}

// singleflightGroup 复用 golang.org/x/sync/singleflight
type singleflightGroup = singleflight.Group

// ============ 数据源定义 ============

type evidenceSource struct {
	name  string
	fetch func(ctx context.Context, ip string) *Evidence // 返回 nil 表示失败/无数据
}

// 期望源数（用于覆盖度计算）：6 个在线源
const expectedSourceCount = 6

// collectEvidence 并行查询全部数据源，返回成功/失败列表
func collectEvidence(ctx context.Context, ip string) []*Evidence {
	sources := []evidenceSource{
		{"ip-api.com", fetchIPAPI},
		{"ipwho.is", fetchIpwhoIs},
		{"api.ip.sb", fetchIpSb},
		{"ipinfo.io", fetchIpinfo},
		{"ip2location.io", fetchIP2Location},
		{"rdap", fetchRDAP},
	}

	results := make([]*Evidence, len(sources))
	var wg sync.WaitGroup
	for i, s := range sources {
		wg.Add(1)
		go func(idx int, src evidenceSource) {
			defer wg.Done()
			results[idx] = src.fetch(ctx, ip)
		}(i, s)
	}
	wg.Wait()
	return results
}

// aggregateEvidence 多源一致性投票
func aggregateEvidence(ip string, evs []*Evidence) *Aggregated {
	agg := &Aggregated{IP: ip, ProbedAt: time.Now()}

	var countries []string
	countryCnt := map[string]int{}
	asnCnt := map[int]int{}
	asOrgByASN := map[int]string{}
	typeCnt := map[string]int{}
	cityByCountry := map[string][]string{}

	for _, e := range evs {
		if e == nil {
			continue
		}
		agg.Sources = append(agg.Sources, e.Source)

		cc := strings.ToUpper(e.CountryCode)
		if cc != "" && cc != "XX" {
			countryCnt[cc]++
			countries = append(countries, cc)
			cityByCountry[cc] = append(cityByCountry[cc], strings.TrimSpace(e.City))
		}
		if e.ASN > 0 {
			asnCnt[e.ASN]++
			if _, ok := asOrgByASN[e.ASN]; !ok {
				asOrgByASN[e.ASN] = e.ASOrg
			}
		}
		t := e.Type
		if (t == "" || t == "unknown") && e.ASN > 0 && isKnownDCASN(e.ASN) {
			t = "hosting" // ASN 强信号：已知云厂商 ASN 判机房
		}
		if e.Hosting {
			t = "hosting" // 数据源显式标注机房
		}
		if t != "" && t != "unknown" {
			typeCnt[t]++
		}
		if e.Proxy {
			agg.ProxySignal = true
			agg.ProxySignalCount++
		}
		// 信号关键词
		hay := strings.ToLower(e.Org + " " + e.ISP + " " + e.ASOrg)
		if hasAnyKeyword(hay, proxyKeywords) {
			agg.ProxySignal = true
			agg.ProxySignalCount++
		}
		if hasAnyKeyword(hay, dcKeywords) {
			agg.DataCenterSignal = true
		}
		if hasAnyKeyword(hay, residentialKeywords) {
			agg.ResidentialSignal = true
		}
		if e.Source == "rdap" {
			agg.RDAPRegistered = e.CountryCode != ""
			agg.RDAPCountry = e.CountryCode
			agg.RIR = e.Org // RIR 名称放在 Org 字段传输
		}
	}

	// 国家多数派
	agg.CountryCode, agg.CountryMajority, agg.CountryOutliers = majorityCountry(countries, countryCnt)
	agg.CountrySources = len(countries)

	// ASN 多数派
	agg.ASN, agg.ASNMajority, agg.ASNOutliers = majorityASN(asnCnt)
	agg.ASNSources = len(asnCnt)
	agg.ASOrg = asOrgByASN[agg.ASN]

	// 类型多数派
	agg.Type, agg.TypeMajority, agg.TypeSources = majorityType(typeCnt)

	// 城市冲突检测（多数派国家内出现多个不同城市）
	if cities, ok := cityByCountry[agg.CountryCode]; ok && len(cities) >= 2 {
		uniq := map[string]bool{}
		for _, c := range cities {
			if c != "" {
				uniq[c] = true
			}
		}
		agg.CityConflict = len(uniq) >= 2
	}

	// 取多数派国家里出现最多的城市
	agg.City = topCity(cityByCountry[agg.CountryCode])

	// org/isp 取第一个非空源（ip-api 优先，含中文）
	for _, e := range evs {
		if e == nil {
			continue
		}
		if agg.Country == "" {
			agg.Country = e.Country
		}
		if agg.Region == "" {
			agg.Region = e.Region
		}
		if agg.Org == "" && e.Org != "" {
			agg.Org = e.Org
		}
		if agg.ISP == "" && e.ISP != "" {
			agg.ISP = e.ISP
		}
		if agg.Org == "" {
			agg.Org = e.ASOrg
		}
	}
	return agg
}

// queryAggregated 带缓存的聚合查询入口（singleflight 防并发重复）
func queryAggregated(ip string) *Aggregated {
	multiSrcCacheMu.Lock()
	if e, ok := multiSrcCache[ip]; ok && time.Now().Before(e.expiresAt) {
		multiSrcCacheMu.Unlock()
		return e.data
	}
	multiSrcCacheMu.Unlock()

	v, _, _ := multiSrcSingle.Do(ip, func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		evs := collectEvidence(ctx, ip)
		agg := aggregateEvidence(ip, evs)
		// 记录失败源
		okMap := map[string]bool{}
		for _, e := range evs {
			if e != nil {
				okMap[e.Source] = true
			}
		}
		for _, s := range []string{"ip-api.com", "ipwho.is", "api.ip.sb", "ipinfo.io", "ip2location.io", "rdap"} {
			if !okMap[s] {
				agg.FailedSources = append(agg.FailedSources, s)
			}
		}
		agg.ExpectedSource = expectedSourceCount
		multiSrcCacheMu.Lock()
		multiSrcCache[ip] = multiSrcCacheEntry{data: agg, expiresAt: time.Now().Add(24 * time.Hour)}
		multiSrcCacheMu.Unlock()
		return agg, nil
	})
	if a, ok := v.(*Aggregated); ok {
		return a
	}
	return nil
}

// ============ HTTP 抓取公共函数 ============

func fetchJSON(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr) // 强制 IPv4（服务器无公网 IPv6）
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ipchk.cn-purity/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return body, nil
}

func warnSource(name, ip string, err error) {
	slog.Warn("purity source failed", "source", name, "ip", ip, "error", err)
}

// ============ 各数据源实现 ============

// 1. ip-api.com（中文返回）
func fetchIPAPI(ctx context.Context, ip string) *Evidence {
	body, err := fetchJSON(ctx, fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=status,country,countryCode,regionName,city,isp,org,as,lat,lon,timezone,query,proxy,hosting,mobile", ip), 8*time.Second)
	if err != nil {
		warnSource("ip-api.com", ip, err)
		return nil
	}
	var raw struct {
		Status      string  `json:"status"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		RegionName  string  `json:"regionName"`
		City        string  `json:"city"`
		ISP         string  `json:"isp"`
		Org         string  `json:"org"`
		AS          string  `json:"as"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		Timezone    string  `json:"timezone"`
		Proxy       bool    `json:"proxy"`
		Hosting     bool    `json:"hosting"`
		Mobile      bool    `json:"mobile"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.Status != "success" {
		return nil
	}
	e := &Evidence{
		Source: "ip-api.com", CountryCode: raw.CountryCode, Country: raw.Country,
		Region: raw.RegionName, City: raw.City, ISP: raw.ISP, Org: raw.Org,
		Latitude: raw.Lat, Longitude: raw.Lon, Timezone: raw.Timezone,
		ASN: parseASN(raw.AS), ASOrg: raw.AS, Proxy: raw.Proxy, Hosting: raw.Hosting,
		Type: inferType(raw.Org + " " + raw.ISP + " " + raw.AS),
	}
	if raw.Hosting {
		e.Type = "hosting"
	} else if raw.Mobile {
		e.Type = "mobile"
	}
	return e
}

// 2. ipwho.is
func fetchIpwhoIs(ctx context.Context, ip string) *Evidence {
	body, err := fetchJSON(ctx, fmt.Sprintf("https://ipwho.is/%s", ip), 8*time.Second)
	if err != nil {
		warnSource("ipwho.is", ip, err)
		return nil
	}
	var raw struct {
		Success     bool   `json:"success"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		Timezone    struct {
			ID string `json:"id"`
		} `json:"timezone"`
		Connection struct {
			ASN    int    `json:"asn"`
			Org    string `json:"org"`
			ISP    string `json:"isp"`
			Domain string `json:"domain"`
		} `json:"connection"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || !raw.Success {
		return nil
	}
	e := &Evidence{
		Source: "ipwho.is", CountryCode: raw.CountryCode, Country: raw.Country,
		Region: raw.Region, City: raw.City, ISP: raw.Connection.ISP,
		Org: raw.Connection.Org, ASN: raw.Connection.ASN, Timezone: raw.Timezone.ID,
		Latitude: raw.Latitude, Longitude: raw.Longitude,
		Type: inferType(raw.Connection.Org + " " + raw.Connection.ISP),
	}
	if e.ASN > 0 {
		e.ASOrg = fmt.Sprintf("AS%d %s", e.ASN, e.Org)
	}
	return e
}

// 3. api.ip.sb
func fetchIpSb(ctx context.Context, ip string) *Evidence {
	body, err := fetchJSON(ctx, fmt.Sprintf("https://api.ip.sb/geoip/%s", ip), 8*time.Second)
	if err != nil {
		warnSource("api.ip.sb", ip, err)
		return nil
	}
	var raw struct {
		IP           string  `json:"ip"`
		CountryCode  string  `json:"country_code"`
		Country      string  `json:"country"`
		Region       string  `json:"region"`
		City         string  `json:"city"`
		Latitude     float64 `json:"latitude"`
		Longitude    float64 `json:"longitude"`
		ASN          int     `json:"asn"`
		Organization string  `json:"organization"`
		ISP          string  `json:"isp"`
		Timezone     string  `json:"timezone"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	e := &Evidence{
		Source: "api.ip.sb", CountryCode: raw.CountryCode, Country: raw.Country,
		Region: raw.Region, City: raw.City, ISP: raw.ISP, Org: raw.Organization,
		ASN: raw.ASN, Latitude: raw.Latitude, Longitude: raw.Longitude, Timezone: raw.Timezone,
		Type: inferType(raw.Organization + " " + raw.ISP),
	}
	if e.ASN > 0 {
		e.ASOrg = fmt.Sprintf("AS%d %s", e.ASN, e.Org)
	}
	return e
}

// 4. ipinfo.io（免费 token 可选，无 token 也可用但限流）
func fetchIpinfo(ctx context.Context, ip string) *Evidence {
	body, err := fetchJSON(ctx, fmt.Sprintf("https://ipinfo.io/%s/json", ip), 6*time.Second)
	if err != nil {
		warnSource("ipinfo.io", ip, err)
		return nil
	}
	var raw struct {
		IP       string `json:"ip"`
		City     string `json:"city"`
		Region   string `json:"region"`
		Country  string `json:"country"`
		Org      string `json:"org"`
		ISP      string `json:"isp"`
		Timezone string `json:"timezone"`
		Loc      string `json:"loc"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.IP == "" {
		return nil
	}
	e := &Evidence{
		Source: "ipinfo.io", CountryCode: raw.Country, City: raw.City, Region: raw.Region,
		Org: raw.Org, ISP: raw.ISP, Timezone: raw.Timezone,
		ASN: parseASN(raw.Org), ASOrg: raw.Org,
		Type: inferType(raw.Org + " " + raw.ISP),
	}
	if _, err := fmt.Sscanf(raw.Loc, "%f,%f", &e.Latitude, &e.Longitude); err != nil {
		e.Latitude, e.Longitude = 0, 0
	}
	return e
}

// 5. ip2location.io（免费 key 可选）
func fetchIP2Location(ctx context.Context, ip string) *Evidence {
	body, err := fetchJSON(ctx, fmt.Sprintf("https://api.ip2location.io/?ip=%s&format=json", ip), 6*time.Second)
	if err != nil {
		warnSource("ip2location.io", ip, err)
		return nil
	}
	var raw struct {
		IP          string  `json:"ip"`
		CountryCode string  `json:"country_code"`
		Country     string  `json:"country_name"`
		Region      string  `json:"region_name"`
		City        string  `json:"city_name"`
		ISP         string  `json:"isp"`
		Org         string  `json:"organization"`
		ASN         string  `json:"asn"`
		UsageType   string  `json:"usage_type"`
		IsProxy     bool    `json:"is_proxy"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		Timezone    string  `json:"timezone"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.IP == "" {
		return nil
	}
	e := &Evidence{
		Source: "ip2location.io", CountryCode: raw.CountryCode, Country: raw.Country,
		Region: raw.Region, City: raw.City, ISP: raw.ISP, Org: raw.Org,
		ASN: parseASN(raw.ASN), Latitude: raw.Latitude, Longitude: raw.Longitude,
		Timezone: raw.Timezone, Type: usageTypeToType(raw.UsageType), Proxy: raw.IsProxy,
	}
	if e.ASN > 0 {
		e.ASOrg = fmt.Sprintf("AS%d %s", e.ASN, e.Org)
	}
	return e
}

// 6. RDAP（通过 rdap.org 自动跳转到对应 RIR）
func fetchRDAP(ctx context.Context, ip string) *Evidence {
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				d := &net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "tcp4", addr)
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://rdap.org/ip/"+ip, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/rdap+json")
	resp, err := client.Do(req)
	if err != nil {
		warnSource("rdap", ip, err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	rir := strings.ToUpper(strings.Split(resp.Request.URL.Host, ".")[0])
	e := &Evidence{
		Source: "rdap", Org: rir, Type: "unknown",
	}
	// 注册国：vcard 的 adr.label 最后一行（ARIN 等不在顶层 country 字段）
	e.CountryCode = rdapCountryFromVcard(raw)
	return e
}

// rdapCountryFromVcard 递归查找 vcardArray 中 adr 条目的 label（多行地址，最后一行是国家名）
func rdapCountryFromVcard(v interface{}) string {
	var found string
	var walk func(interface{})
	walk = func(node interface{}) {
		if found != "" {
			return
		}
		switch t := node.(type) {
		case map[string]interface{}:
			if va, ok := t["vcardArray"].([]interface{}); ok && len(va) > 1 {
				if rows, ok := va[1].([]interface{}); ok {
					for _, row := range rows {
						arr, ok := row.([]interface{})
						if !ok || len(arr) < 4 {
							continue
						}
						name, _ := arr[0].(string)
						if !strings.EqualFold(name, "adr") {
							continue
						}
						params, ok := arr[1].(map[string]interface{})
						if !ok {
							continue
						}
						label, _ := params["label"].(string)
						if label == "" {
							continue
						}
						lines := strings.Split(label, "\n")
						last := strings.TrimSpace(lines[len(lines)-1])
						if last != "" {
							found = countryNameToCode(last)
							return
						}
					}
				}
			}
			for _, vv := range t {
				walk(vv)
			}
		case []interface{}:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return found
}

// countryNameToCode 常用国家英文名 → 两位代码（RDAP vcard label 用）
func countryNameToCode(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	table := map[string]string{
		"united states": "US", "united states of america": "US", "usa": "US", "u.s.a.": "US",
		"china": "CN", "people's republic of china": "CN",
		"hong kong": "HK", "taiwan": "TW", "macau": "MO", "macao": "MO",
		"japan": "JP", "south korea": "KR", "korea": "KR", "republic of korea": "KR",
		"singapore": "SG", "germany": "DE", "france": "FR", "netherlands": "NL",
		"united kingdom": "GB", "great britain": "GB", "uk": "GB",
		"canada": "CA", "australia": "AU", "russia": "RU", "russian federation": "RU",
		"india": "IN", "vietnam": "VN", "thailand": "TH", "indonesia": "ID",
		"malaysia": "MY", "philippines": "PH", "turkey": "TR", "united arab emirates": "AE",
		"uae": "AE", "saudi arabia": "SA", "brazil": "BR", "argentina": "AR",
		"mexico": "MX", "italy": "IT", "spain": "ES", "switzerland": "CH",
		"sweden": "SE", "norway": "NO", "finland": "FI", "denmark": "DK",
		"poland": "PL", "ukraine": "UA", "ireland": "IE", "belgium": "BE",
		"austria": "AT", "portugal": "PT", "czech republic": "CZ", "czechia": "CZ",
		"greece": "GR", "hungary": "HU", "romania": "RO", "israel": "IL",
		"south africa": "ZA", "egypt": "EG", "nigeria": "NG", "pakistan": "PK",
		"bangladesh": "BD", "kazakhstan": "KZ", "new zealand": "NZ", "chile": "CL",
		"colombia": "CO", "peru": "PE", "venezuela": "VE",
	}
	if code, ok := table[n]; ok {
		return code
	}
	return ""
}

// ============ 类型判定 ============

var proxyKeywords = []string{"vpn", "proxy", "tor", "anonymous", "anonymizer", "socks"}
var dcKeywords = []string{"cloud", "hosting", "datacenter", "data center", "server", "idc", "colo", "cloudflare", "amazon", "aws", "azure", "google cloud", "alibaba", "tencent cloud", "huawei cloud", "digitalocean", "linode", "ovh", "vultr", "oracle cloud", "akamai"}
var residentialKeywords = []string{"residential", "home", "broadband", "fiber", "ftth", "adsl"}
var ispKeywords = []string{"telecom", "unicom", "mobile", "chinatelecom", "chinaunicom", "chinamobile", "telkom", "vodafone", "verizon", "att", "t-mobile", "century", "comcast", "spectrum", "kddi", "softbank"}

// inferType 基于 org/isp 文本启发式推断线路类型（作为 1 个类型源投票）
func inferType(haystack string) string {
	h := strings.ToLower(haystack)
	if hasAnyKeyword(h, dcKeywords) {
		return "hosting"
	}
	if hasAnyKeyword(h, ispKeywords) {
		return "isp"
	}
	if hasAnyKeyword(h, residentialKeywords) {
		return "residential"
	}
	return "unknown"
}

// usageTypeToType ip2location usage_type → 类型
func usageTypeToType(ut string) string {
	switch strings.ToUpper(ut) {
	case "DCH":
		return "hosting"
	case "COM":
		return "business"
	case "EDU":
		return "business"
	case "ISP":
		return "isp"
	case "MOB":
		return "mobile"
	case "GOV":
		return "business"
	case "LIB":
		return "business"
	case "MIL":
		return "business"
	case "NGO":
		return "business"
	case "RES":
		return "residential"
	default:
		return "unknown"
	}
}

func hasAnyKeyword(hay string, kws []string) bool {
	for _, k := range kws {
		if strings.Contains(hay, k) {
			return true
		}
	}
	return false
}

// ============ 已知云厂商 ASN（类型投票强信号） ============

var knownDCASNs = map[int]bool{
	13335: true, 16509: true, 14061: true, 395747: true, 20473: true,
	44440: true, 54113: true, 15169: true, 8075: true, 31898: true,
	36492: true, 396982: true, 14618: true, 20940: true, 37963: true,
	132203: true, 45090: true, 136907: true, 45102: true, 32097: true,
	63949: true, 2635: true,
}

func isKnownDCASN(asn int) bool { return knownDCASNs[asn] }

// ============ 投票函数 ============

func majorityCountry(items []string, cnt map[string]int) (cc string, majority, outliers int) {
	if len(items) == 0 {
		return "", 0, 0
	}
	type kv struct {
		k string
		v int
	}
	var list []kv
	for k, v := range cnt {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	cc = list[0].k
	majority = list[0].v
	for _, it := range list[1:] {
		outliers += it.v
	}
	return cc, majority, outliers
}

func majorityASN(cnt map[int]int) (asn, majority, outliers int) {
	if len(cnt) == 0 {
		return 0, 0, 0
	}
	type kv struct {
		k int
		v int
	}
	var list []kv
	for k, v := range cnt {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	asn = list[0].k
	majority = list[0].v
	for _, it := range list[1:] {
		outliers += it.v
	}
	return asn, majority, outliers
}

func majorityType(cnt map[string]int) (t string, majority, sources int) {
	if len(cnt) == 0 {
		return "unknown", 0, 0
	}
	type kv struct {
		k string
		v int
	}
	var list []kv
	for k, v := range cnt {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	t = list[0].k
	majority = list[0].v
	for _, it := range list {
		sources += it.v
	}
	return t, majority, sources
}

func topCity(cities []string) string {
	cnt := map[string]int{}
	for _, c := range cities {
		if c != "" {
			cnt[c]++
		}
	}
	best, max := "", 0
	for c, n := range cnt {
		if n > max {
			best, max = c, n
		}
	}
	return best
}
