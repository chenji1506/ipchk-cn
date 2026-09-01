package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"ipchk-cn/ipdb"
	"ipchk-cn/ssrf"
	"ipchk-cn/webtest"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"
	"resty.dev/v3"
)

func initHTTPClients() {
	setTransport := func(network string) *http.Transport {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		return &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				if ssrf.Enabled() {
					host, port, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}
					var dnsResult webtest.DNSResult
					if network == "tcp4" {
						dnsResult, err = webtest.ResolveARecord(host)
					} else {
						dnsResult, err = webtest.ResolveAAAARecord(host)
					}
					if err != nil {
						return nil, err
					}
					for _, ipStr := range dnsResult.Record {
						ip := net.ParseIP(ipStr)
						if ip != nil && ssrf.IsPrivateIP(ip) {
							slog.Warn("Blocked connection to private IP", "host", host, "ip", ip)
							return nil, fmt.Errorf("request to private/internal address is not allowed")
						}
					}
					if len(dnsResult.Record) > 0 {
						addr = net.JoinHostPort(dnsResult.Record[0], port)
					}
				}
				return dialer.DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	V6Client = resty.New()
	V4Client = resty.New()
	V6Client.SetTransport(setTransport("tcp6"))
	V4Client.SetTransport(setTransport("tcp4"))
	V6Client.SetTimeout(10 * time.Second)
	V4Client.SetTimeout(10 * time.Second)
	V6Client.SetRedirectPolicy(resty.RedirectPolicyFunc(ssrf.SecureCheckRedirect))
	V4Client.SetRedirectPolicy(resty.RedirectPolicyFunc(ssrf.SecureCheckRedirect))
	V6Client.AddContentDecompresser("zstd", decompressZstd)
	V4Client.AddContentDecompresser("zstd", decompressZstd)

}

func fakePerfectWebsiteResult(host string) *WebsiteCheckDetail {
	cleanHost := strings.TrimPrefix(host, "https://")
	cleanHost = strings.TrimPrefix(cleanHost, "http://")
	return &WebsiteCheckDetail{
		HostRecord:       cleanHost,
		HTTPStatusCode:   200,
		HTTPSSStatusCode: 200,
		DNSLookupTime:    0.5,
		TCPConnectTime:   1.0,
		HTTPConnectTime:  1.5,
		FirstByteTime:    2.0,
		TotalTime:        100,
		PageSize:         52428,
		DownloadSpeed:    512.0,
		IsReachable:      true,
	}
}

func fakeInvalidSSLResult(host string) *SSLCheckDetail {
	return &SSLCheckDetail{
		CertValidityDays:   0,
		IsExpired:          true,
		CertStartTime:      time.Time{},
		CertEndTime:        time.Time{},
		HTTPVersion:        "",
		HostRecord:         host,
		HTTPSSStatusCode:   0,
		TotalTime:          0,
		DownloadSpeed:      0,
		Domain:             host,
		IssuerOrganization: []string{},
		IssuerCommonName:   "Invalid Certificate",
		SubjectCommonName:  host,
		IsReachable:        false,
	}
}

// Create Zstandard decompress logic
// 创建 Zstandard 解压缩逻辑
var zstdReaderPool = sync.Pool{
	New: func() interface{} {
		// 当池子空了，创建一个新的解码器
		decoder, _ := zstd.NewReader(nil)
		return decoder
	},
}

func decompressZstd(r io.ReadCloser) (io.ReadCloser, error) {
	zr := zstdReaderPool.Get().(*zstd.Decoder)

	err := zr.Reset(r)
	if err != nil {
		zstdReaderPool.Put(zr)
		zr, _ = zstd.NewReader(r)
	}
	defer zstdReaderPool.Put(zr)
	z := &zstdReader{s: r, r: zr}
	return z, nil
}

type zstdReader struct {
	s io.ReadCloser
	r *zstd.Decoder
}

func (b *zstdReader) Read(p []byte) (n int, err error) {
	return b.r.Read(p)
}

func (b *zstdReader) Close() error {
	b.r.Close()
	return b.s.Close()
}
func cleanHostRecord(addr string) string {
	if strings.HasPrefix(addr, "[") {
		rightBracket := strings.Index(addr, "]")
		if rightBracket != -1 {
			return addr[1:rightBracket]
		}
	}

	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		colonCount := strings.Count(addr, ":")
		if colonCount > 1 {
			return addr[:idx]
		}
		if colonCount == 1 {
			return addr[:idx]
		}
	}

	return addr
}

// normalizeURL normalizes the input URL by ensuring it has a scheme (http or https).
// normalizeURL 通过确保输入 URL 具有方案（http 或 https）来规范化输入 URL。
func normalizeURL(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "/")
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return input
	}
	if strings.HasPrefix(input, "//") {
		return "https:" + input
	}
	return "https://" + input
}

// parseURL parses the input string into a URL object after normalizing it.
// parseURL 在规范化输入字符串后，将其解析为 URL 对象。

func parseURL(input string) (*url.URL, error) {
	input = normalizeURL(input)
	return url.Parse(input)
}

// Setting struct represents the configuration settings for the application, including port, GitHub proxy, and single-stack mode.
// Setting 结构体表示应用程序的配置设置，包括端口、GitHub 代理和单栈模式。
type Setting struct {
	Port         any    `json:"port"`
	GHProxy      string `json:"gh-proxy"`
	SINGLE_STACK string `json:"single-stack"`
}

func (s *Setting) PortString() string {
	switch v := s.Port.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return ""
	}
}

// Global variables and structs
// 全局变量与结构体
var (
	PORTS        string
	GH_PROXY     string
	LOG_LEVEL    string
	websiteCache sync.Map
	SINGLE_STACK string
	DNS_SERVER   string
	sslCache     sync.Map
	pingCache    sync.Map
	speedCache   sync.Map
	sfGroup      singleflight.Group
	V6Client     *resty.Client
	V4Client     *resty.Client
	IPDB         string
)

type websiteCacheEntry struct {
	result    *WebsiteCheckResult
	timestamp time.Time
}

type sslCacheEntry struct {
	result    *SSLCheckResult
	timestamp time.Time
}

type pingCacheEntry struct {
	result    *TCPingResult
	timestamp time.Time
}

type speedCacheEntry struct {
	result    *WebsiteSpeedTestResult
	timestamp time.Time
}

type WebsiteCheckResult struct {
	IPv4 *WebsiteCheckDetail `json:"ipv4"`
	IPv6 *WebsiteCheckDetail `json:"ipv6"`
}

type WebsiteCheckDetail struct {
	HostRecord       string  `json:"host_record"`
	HTTPStatusCode   int     `json:"http_status_code"`
	HTTPSSStatusCode int     `json:"https_status_code"`
	DNSLookupTime    float64 `json:"dns_lookup_time"`
	TCPConnectTime   float64 `json:"tcp_connect_time"`
	HTTPConnectTime  float64 `json:"http_connect_time"`
	FirstByteTime    float64 `json:"first_byte_time"`
	TotalTime        float64 `json:"total_time"`
	PageSize         int64   `json:"page_size"`
	DownloadSpeed    float64 `json:"download_speed"`
	IsReachable      bool    `json:"is_reachable"`
}

type SSLCheckDetail struct {
	CertValidityDays   int       `json:"cert_validity_days"`
	CertStartTime      time.Time `json:"cert_start_time"`
	CertEndTime        time.Time `json:"cert_end_time"`
	HTTPVersion        string    `json:"http_version"`
	HostRecord         string    `json:"host_record"`
	HTTPSSStatusCode   int       `json:"https_status_code"`
	TotalTime          float64   `json:"total_time"`
	DownloadSpeed      float64   `json:"download_speed"`
	Domain             string    `json:"domain"`
	IssuerOrganization []string  `json:"issuer_organization"`
	IssuerCommonName   string    `json:"issuer_common_name"`
	SubjectCommonName  string    `json:"subject_common_name"`
	IsExpired          bool      `json:"is_expired"`
	IsReachable        bool      `json:"is_reachable"`
}

type SSLCheckResult struct {
	IPv4 *SSLCheckDetail `json:"ipv4"`
	IPv6 *SSLCheckDetail `json:"ipv6"`
}
type TCPingResult struct {
	IPv4 *webtest.TCPingStats `json:"ipv4"`
	IPv6 *webtest.TCPingStats `json:"ipv6"`
}
type WebsiteSpeedTestResult struct {
	Version          string  `json:"version"`
	HostRecord       string  `json:"host_record"`
	HTTPStatusCode   int     `json:"http_status_code"`
	HTTPSSStatusCode int     `json:"https_status_code"`
	DNSLookupTime    float64 `json:"dns_lookup_time"`
	TCPConnectTime   float64 `json:"tcp_connect_time"`
	HTTPConnectTime  float64 `json:"http_connect_time"`
	FirstByteTime    float64 `json:"first_byte_time"`
	TotalTime        float64 `json:"total_time"`
	PageSize         int64   `json:"page_size"`
	DownloadSpeed    float64 `json:"download_speed"`
	Message          string  `json:"message"`
	Headers          string  `json:"headers"`
	IsReachable      bool    `json:"is_reachable"`
}

// Business Endpoints
// 业务端点

func checkWebsite(url string, version string) (*WebsiteCheckDetail, error) {
	ctx := context.Background()
	var err error
	ctx, err = ssrf.ValidateOutboundTarget(ctx, url)
	if err != nil {
		return nil, err
	}

	client := V4Client
	if version == "v6" {
		client = V6Client
	}

	startTime := time.Now()
	resp, err := client.R().EnableTrace().SetContext(ctx).Get(url)

	// HTTPS 请求失败时 fallback 到 HTTP
	fallbackToHTTP := false
	if err != nil && strings.HasPrefix(url, "https://") {
		httpURL := strings.Replace(url, "https://", "http://", 1)
		startTime = time.Now()
		resp, err = client.R().EnableTrace().SetContext(ctx).Get(httpURL)
		fallbackToHTTP = true
	}

	if err != nil {
		return nil, err
	}
	endTime := time.Now()

	body := resp.Bytes()
	trace := resp.Request.TraceInfo()

	hostRecord := cleanHostRecord(trace.RemoteAddr)

	totalTime := float64(endTime.Sub(startTime).Milliseconds())
	var downloadSpeed float64
	if totalTime > 0 {
		downloadSpeed = float64(len(body)) / 1024.0 / (totalTime / 1000.0)
	}

	httpStatus := resp.StatusCode()
	httpsStatus := resp.StatusCode()
	if fallbackToHTTP {
		httpsStatus = 0
	}

	result := &WebsiteCheckDetail{
		HostRecord:       hostRecord,
		HTTPStatusCode:   httpStatus,
		HTTPSSStatusCode: httpsStatus,
		DNSLookupTime:    float64(trace.DNSLookup.Milliseconds()),
		TCPConnectTime:   float64(trace.TCPConnTime.Milliseconds()),
		HTTPConnectTime:  float64(trace.ConnTime.Milliseconds()),
		FirstByteTime:    float64(trace.ServerTime.Milliseconds()),
		TotalTime:        totalTime,
		PageSize:         int64(len(body)),
		DownloadSpeed:    downloadSpeed,
		IsReachable:      true,
	}

	return result, nil
}

func websiteSpeed(url string, version string) (*WebsiteSpeedTestResult, error) {
	ctx := context.Background()
	var err error
	ctx, err = ssrf.ValidateOutboundTarget(ctx, url)
	if err != nil {
		return nil, err
	}

	client := V4Client
	if version == "v6" {
		client = V6Client
	}

	startTime := time.Now()
	resp, err := client.R().EnableTrace().SetContext(ctx).Get(url)

	fallbackToHTTP := false
	if err != nil && strings.HasPrefix(url, "https://") {
		httpURL := strings.Replace(url, "https://", "http://", 1)
		startTime = time.Now()
		resp, err = client.R().EnableTrace().SetContext(ctx).Get(httpURL)
		fallbackToHTTP = true
	}

	if err != nil {
		return nil, err
	}
	endTime := time.Now()

	body := resp.Bytes()
	trace := resp.Request.TraceInfo()

	hostRecord := cleanHostRecord(trace.RemoteAddr)

	totalTime := float64(endTime.Sub(startTime).Milliseconds())
	var downloadSpeed float64
	if totalTime > 0 {
		downloadSpeed = float64(len(body)) / 1024.0 / (totalTime / 1000.0)
	}
	dumpBytes, _ := httputil.DumpResponse(resp.RawResponse, false)
	httpStatus := resp.StatusCode()
	httpsStatus := resp.StatusCode()
	if fallbackToHTTP {
		httpsStatus = 0
	}
	result := &WebsiteSpeedTestResult{
		Version:          version,
		Headers:          string(dumpBytes),
		HostRecord:       hostRecord,
		HTTPStatusCode:   httpStatus,
		HTTPSSStatusCode: httpsStatus,
		DNSLookupTime:    float64(trace.DNSLookup.Milliseconds()),
		TCPConnectTime:   float64(trace.TCPConnTime.Milliseconds()),
		HTTPConnectTime:  float64(trace.ConnTime.Milliseconds()),
		FirstByteTime:    float64(trace.ServerTime.Milliseconds()),
		TotalTime:        totalTime,
		PageSize:         int64(len(body)),
		DownloadSpeed:    downloadSpeed,
		IsReachable:      true,
	}

	return result, nil
}

func checkSSL(url string, version string) (*SSLCheckDetail, error) {
	ctx := context.Background()
	var err error
	ctx, err = ssrf.ValidateOutboundTarget(ctx, url)
	if err != nil {
		return nil, err
	}

	client := V4Client
	if version == "v6" {
		client = V6Client
	}

	startTime := time.Now()
	resp, err := client.R().EnableTrace().SetContext(ctx).Get(url)
	if err != nil {
		return nil, err
	}
	endTime := time.Now()

	trace := resp.Request.TraceInfo()
	hostRecord := cleanHostRecord(trace.RemoteAddr)

	totalTime := float64(endTime.Sub(startTime).Milliseconds())
	body := resp.Bytes()
	var downloadSpeed float64
	if totalTime > 0 {
		downloadSpeed = float64(len(body)) / 1024.0 / (totalTime / 1000.0)
	}

	rawResp := resp.RawResponse
	var cert *x509.Certificate
	var remainingDays int
	var isExpired bool
	var issuerOrganization []string
	var issuerCommonName, subjectCommonName, domain string

	if rawResp.TLS != nil && len(rawResp.TLS.PeerCertificates) > 0 {
		cert = rawResp.TLS.PeerCertificates[0]
		now := time.Now()
		remainingDays = int(cert.NotAfter.Sub(now).Hours() / 24)
		isExpired = now.After(cert.NotAfter) || now.Before(cert.NotBefore)
		issuerOrganization = cert.Issuer.Organization
		issuerCommonName = cert.Issuer.CommonName
		subjectCommonName = cert.Subject.CommonName
		domain = cleanHostRecord(cert.Subject.CommonName)
	} else {
		return nil, fmt.Errorf("no SSL certificate found")
	}

	result := &SSLCheckDetail{
		CertValidityDays:   remainingDays,
		IsExpired:          isExpired,
		CertStartTime:      cert.NotBefore,
		CertEndTime:        cert.NotAfter,
		HTTPVersion:        resp.Proto(),
		HostRecord:         hostRecord,
		HTTPSSStatusCode:   resp.StatusCode(),
		TotalTime:          totalTime,
		DownloadSpeed:      downloadSpeed,
		Domain:             domain,
		IssuerOrganization: issuerOrganization,
		IssuerCommonName:   issuerCommonName,
		SubjectCommonName:  subjectCommonName,
		IsReachable:        true,
	}

	return result, nil
}

func checkWebsiteHandler(c *gin.Context) {
	testUrl := c.Param("url")
	if testUrl == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "URL parameter is required",
		})
		return
	}

	testUrl = normalizeURL(testUrl)

	parsedURL, _ := url.Parse(testUrl)
	if ssrf.HasLocalOrPrivateIP(parsedURL.Hostname()) {
		c.JSON(200, &WebsiteCheckResult{
			IPv4: fakePerfectWebsiteResult(testUrl),
			IPv6: fakePerfectWebsiteResult(testUrl),
		})
		return
	}

	if cached, ok := websiteCache.Load(testUrl); ok {
		entry := cached.(websiteCacheEntry)
		if time.Since(entry.timestamp) < 5*time.Minute {
			c.JSON(200, entry.result)
			return
		}
		websiteCache.Delete(testUrl)
	}

	rawResult, _, _ := sfGroup.Do(testUrl, func() (interface{}, error) {
		result := &WebsiteCheckResult{}
		switch SINGLE_STACK {
		case "ipv4":
			ipv4, errV4 := checkWebsite(testUrl, "v4")
			if errV4 != nil {
				ipv4 = &WebsiteCheckDetail{
					HostRecord:  "Error: " + errV4.Error(),
					IsReachable: false,
				}
			}
			result.IPv4 = ipv4
			result.IPv6 = &WebsiteCheckDetail{
				HostRecord:  "Skipped due to SINGLE_STACK=ipv4",
				IsReachable: false,
			}
		case "ipv6":
			ipv6, errV6 := checkWebsite(testUrl, "v6")
			if errV6 != nil {
				ipv6 = &WebsiteCheckDetail{
					HostRecord:  "Error: " + errV6.Error(),
					IsReachable: false,
				}
			}
			result.IPv6 = ipv6
			result.IPv4 = &WebsiteCheckDetail{
				HostRecord:  "Skipped due to SINGLE_STACK=ipv6",
				IsReachable: false,
			}
		default:
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				ipv6, errV6 := checkWebsite(testUrl, "v6")
				if errV6 != nil {
					ipv6 = &WebsiteCheckDetail{
						HostRecord:  "Error: " + errV6.Error(),
						IsReachable: false,
					}
				}
				result.IPv6 = ipv6
			}()

			go func() {
				defer wg.Done()
				ipv4, errV4 := checkWebsite(testUrl, "v4")
				if errV4 != nil {
					ipv4 = &WebsiteCheckDetail{
						HostRecord:  "Error: " + errV4.Error(),
						IsReachable: false,
					}
				}
				result.IPv4 = ipv4
			}()

			wg.Wait()
		}

		websiteCache.Store(testUrl, websiteCacheEntry{result: result, timestamp: time.Now()})

		if (result.IPv4 != nil && !result.IPv4.IsReachable) || (result.IPv6 != nil && !result.IPv6.IsReachable) {
			go func() {
				time.Sleep(30 * time.Second)
				websiteCache.Delete(testUrl)
			}()
		}

		return result, nil
	})

	c.JSON(200, rawResult.(*WebsiteCheckResult))
}
func websiteSpeedTestHandler(c *gin.Context) {
	testUrl := c.Param("url")
	version := c.Param("version")
	if testUrl == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "URL parameter is required",
		})
		return
	}
	url := normalizeURL(testUrl)

	// 检查请求版本是否与 SINGLE_STACK 配置匹配
	switch SINGLE_STACK {
	case "ipv4":
		if version != "v4" {
			c.JSON(http.StatusBadRequest, &WebsiteSpeedTestResult{
				Version:    "v4",
				HostRecord: "Skipped due to SINGLE_STACK=ipv4",
			})
			return
		}
	case "ipv6":
		if version != "v6" {
			c.JSON(http.StatusBadRequest, &WebsiteSpeedTestResult{
				Version:    "v6",
				HostRecord: "Skipped due to SINGLE_STACK=ipv6",
			})
			return
		}
	}

	// 缓存键：URL + 版本
	cacheKey := fmt.Sprintf("%s:%s", url, version)

	// 检查缓存
	if cached, ok := speedCache.Load(cacheKey); ok {
		entry := cached.(speedCacheEntry)
		if time.Since(entry.timestamp) < 1*time.Minute {
			c.JSON(200, entry.result)
			return
		}
		speedCache.Delete(cacheKey)
	}

	var result *WebsiteSpeedTestResult

	switch version {
	case "v6", "v4":
		rawResult, _, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
			r, e := websiteSpeed(url, version)
			if e != nil {
				errorResult := &WebsiteSpeedTestResult{
					HostRecord: "Error: " + e.Error(),
				}
				speedCache.Store(cacheKey, speedCacheEntry{result: errorResult, timestamp: time.Now()})
				go func() {
					time.Sleep(30 * time.Second)
					speedCache.Delete(cacheKey)
				}()
				return errorResult, nil
			}
			speedCache.Store(cacheKey, speedCacheEntry{result: r, timestamp: time.Now()})
			return r, nil
		})
		result = rawResult.(*WebsiteSpeedTestResult)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid version",
		})
		return
	}

	c.JSON(200, result)
}

func sslCheckHandler(c *gin.Context) {
	testUrl := c.Param("url")
	if testUrl == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "URL parameter is required",
		})
		return
	}

	testUrl = normalizeURL(testUrl)

	parsedURL, _ := url.Parse(testUrl)
	if ssrf.HasLocalOrPrivateIP(parsedURL.Hostname()) {
		c.JSON(200, &SSLCheckResult{
			IPv4: fakeInvalidSSLResult(parsedURL.Hostname()),
			IPv6: fakeInvalidSSLResult(parsedURL.Hostname()),
		})
		return
	}

	if cached, ok := sslCache.Load(testUrl); ok {
		entry := cached.(sslCacheEntry)
		if time.Since(entry.timestamp) < 5*time.Minute {
			c.JSON(200, entry.result)
			return
		}
		sslCache.Delete(testUrl)
	}

	rawResult, _, _ := sfGroup.Do(testUrl, func() (interface{}, error) {
		result := &SSLCheckResult{}
		switch SINGLE_STACK {
		case "ipv4":
			ipv4, errV4 := checkSSL(testUrl, "v4")
			if errV4 != nil {
				ipv4 = &SSLCheckDetail{
					HostRecord:  "Error: " + errV4.Error(),
					IsExpired:   true,
					IsReachable: false,
				}
			}
			result.IPv4 = ipv4
			result.IPv6 = &SSLCheckDetail{
				HostRecord:  "Skipped due to SINGLE_STACK=ipv4",
				IsExpired:   true,
				IsReachable: false,
			}
		case "ipv6":
			ipv6, errV6 := checkSSL(testUrl, "v6")
			if errV6 != nil {
				ipv6 = &SSLCheckDetail{
					HostRecord:  "Error: " + errV6.Error(),
					IsExpired:   true,
					IsReachable: false,
				}
			}
			result.IPv6 = ipv6
			result.IPv4 = &SSLCheckDetail{
				HostRecord:  "Skipped due to SINGLE_STACK=ipv6",
				IsExpired:   true,
				IsReachable: false,
			}
		default:
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				ipv6, errV6 := checkSSL(testUrl, "v6")
				if errV6 != nil {
					ipv6 = &SSLCheckDetail{
						HostRecord:  "Error: " + errV6.Error(),
						IsExpired:   true,
						IsReachable: false,
					}
				}
				result.IPv6 = ipv6
			}()

			go func() {
				defer wg.Done()
				ipv4, errV4 := checkSSL(testUrl, "v4")
				if errV4 != nil {
					ipv4 = &SSLCheckDetail{
						HostRecord:  "Error: " + errV4.Error(),
						IsExpired:   true,
						IsReachable: false,
					}
				}
				result.IPv4 = ipv4
			}()

			wg.Wait()
		}

		sslCache.Store(testUrl, sslCacheEntry{result: result, timestamp: time.Now()})

		if (result.IPv4 != nil && !result.IPv4.IsReachable) || (result.IPv6 != nil && !result.IPv6.IsReachable) {
			go func() {
				time.Sleep(30 * time.Second)
				sslCache.Delete(testUrl)
			}()
		}

		return result, nil
	})

	c.JSON(200, rawResult.(*SSLCheckResult))
}

// writeJSON 统一 JSON 输出：默认紧凑；?pretty=1 时格式化（缩进换行）
func writeJSON(c *gin.Context, data interface{}) {
	if c.Query("pretty") == "1" {
		c.IndentedJSON(http.StatusOK, data)
		return
	}
	c.JSON(http.StatusOK, data)
}

func locateIP(c *gin.Context) {
	ip := c.Param("ip")
	slog.Debug("Locating IP", "ip", ip)
	writeJSON(c, mergeLocationSources(ip))
}
func locateUserIP(c *gin.Context) {
	ip := c.ClientIP()
	// 可能会有误报，因为某些环境下 ClientIP() 可能返回代理服务器的 IP 地址，而不是用户的真实 IP 地址
	slog.Debug("Locating user IP", "ip", ip)
	if isCLIUA(c.GetHeader("User-Agent")) {
		c.String(http.StatusOK, formatLocationText(ip, mergeLocationSources(ip)))
		return
	}
	writeJSON(c, mergeLocationSources(ip))
}

// mergeLocationSources 合并多数据源：ip-api.com（主）+ 本地库（bilibili/ip2region/qqwry/maxmind/dbip/geocn）
func mergeLocationSources(ip string) map[string]interface{} {
	result := map[string]interface{}{"ip": ip}
	// 主源：ip-api.com（在线，中文）
	if data := queryIPLocation(ip); data != nil {
		for k, v := range data {
			result[k] = v
		}
	}
	// 合并本地多源（不覆盖主源已有字段）
	multi := ipdb.SearchIP(ip)
	for k, v := range multi {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}
	return result
}

// isCLIUA 判断是否为 curl/wget 等命令行客户端
func isCLIUA(ua string) bool {
	return strings.HasPrefix(strings.ToLower(ua), "curl") ||
		strings.HasPrefix(strings.ToLower(ua), "wget")
}

// formatLocationText 将归属地数据格式化为美观的文本输出
func formatLocationText(ip string, data map[string]interface{}) string {
	type field struct {
		key   string
		value string
	}
	var fields []field
	get := func(k string) string {
		if v, ok := data[k]; ok && v != nil {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	// ip-api.com 格式
	if get("country") != "" || get("region") != "" || get("city") != "" {
		fields = []field{
			{"IP", ip},
			{"国家", get("country")},
			{"省份", get("region")},
			{"城市", get("city")},
			{"运营商", get("isp")},
			{"组织", get("org")},
			{"AS 号", get("as")},
			{"经纬度", strings.TrimSpace(get("latitude") + ", " + get("longitude"))},
			{"时区", get("timezone")},
			{"数据来源", get("source")},
		}
	} else {
		// 本地数据库格式（回退），尝试提取主要数据源
		if b, ok := data["bilibili"].(map[string]interface{}); ok {
			fields = append(fields, field{"IP", ip},
				field{"国家", strval(b["country"])},
				field{"省份", strval(b["administrative_area"])},
				field{"城市", strval(b["city"])},
				field{"运营商", strval(b["isp"])},
				field{"数据来源", "bilibili"})
		}
		if q, ok := data["qqwry"].(map[string]interface{}); ok && len(fields) == 0 {
			fields = append(fields, field{"IP", ip},
				field{"归属地", strings.TrimSpace(strval(q["country"]) + " " + strval(q["administrative_area"]) + " " + strval(q["city"]))},
				field{"运营商", strval(q["isp"])},
				field{"数据来源", "qqwry(纯真)"})
		}
		if len(fields) == 0 {
			// 兜底：输出可读 JSON
			if b, err := json.MarshalIndent(data, "", "  "); err == nil {
				return string(b)
			}
		}
	}

	// 计算 key 对齐宽度（中文按 2 个字符宽）
	maxW := 0
	for _, f := range fields {
		if w := displayWidth(f.key); w > maxW {
			maxW = w
		}
	}
	width := maxW + 2

	var b strings.Builder
	b.WriteString("IP 归属地查询结果\n")
	b.WriteString(strings.Repeat("─", 46) + "\n")
	for _, f := range fields {
		if f.value == "" || f.value == ", " {
			continue
		}
		b.WriteString(padKey(f.key, width) + ": " + f.value + "\n")
	}
	b.WriteString(strings.Repeat("─", 46))
	return b.String()
}

// strval 安全取值
func strval(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// displayWidth 计算字符串显示宽度（中文等宽字符按 2）
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 127 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// padKey 按显示宽度补空格对齐
func padKey(s string, width int) string {
	return s + strings.Repeat(" ", width-displayWidth(s))
}

// ============ IP 归属地 API 查询（ip-api.com 免费接口，带缓存） ============
var (
	ipAPICache   = make(map[string]ipAPICacheEntry)
	ipAPICacheMu sync.Mutex
	ipAPISingle  singleflight.Group
)

type ipAPICacheEntry struct {
	data      map[string]interface{}
	expiresAt time.Time
}

// queryIPLocation 优先查询免费 API（ip-api.com），失败回退本地数据库
func queryIPLocation(ip string) map[string]interface{} {
	// 缓存命中直接返回
	ipAPICacheMu.Lock()
	if e, ok := ipAPICache[ip]; ok && time.Now().Before(e.expiresAt) {
		ipAPICacheMu.Unlock()
		return e.data
	}
	ipAPICacheMu.Unlock()

	// singleflight 防止同一 IP 并发重复请求
	v, _, _ := ipAPISingle.Do(ip, func() (interface{}, error) {
		data := queryIPAPIRemote(ip)
		if data != nil {
			ipAPICacheMu.Lock()
			ipAPICache[ip] = ipAPICacheEntry{data: data, expiresAt: time.Now().Add(24 * time.Hour)}
			ipAPICacheMu.Unlock()
		}
		return data, nil
	})
	if data, ok := v.(map[string]interface{}); ok {
		return data
	}
	return nil
}

// IP 归属地在线数据源列表（按顺序尝试，第一个成功即返回，全部失败回退本地库）
var ipAPISources = []ipAPISource{
	{"ip-api.com", "http://ip-api.com/json/%s?lang=zh-CN", parseIPAPICom},
	{"ipwho.is", "https://ipwho.is/%s", parseIpwhoIs},
	{"ip.sb", "https://api.ip.sb/geoip/%s", parseIpSb},
}

type ipAPISource struct {
	name  string
	url   string
	parse func(ip string, raw map[string]interface{}) map[string]interface{}
}

// queryIPAPIRemote 依次尝试多个在线数据源
func queryIPAPIRemote(ip string) map[string]interface{} {
	for _, src := range ipAPISources {
		if data := fetchIPSource(src, ip); data != nil {
			return data
		}
	}
	return nil
}

func fetchIPSource(src ipAPISource, ip string) map[string]interface{} {
	// 强制 IPv4 出口（服务器无公网 IPv6，避免 IPv6 优先连接卡死超时）
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		},
	}
	apiURL := fmt.Sprintf(src.url, ip)
	resp, err := client.Get(apiURL)
	if err != nil {
		slog.Warn("IP source request failed", "source", src.name, "ip", ip, "error", err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		slog.Warn("IP source bad status", "source", src.name, "ip", ip, "status", resp.StatusCode)
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		slog.Warn("IP source parse failed", "source", src.name, "ip", ip, "error", err)
		return nil
	}
	data := src.parse(ip, raw)
	if data == nil {
		slog.Warn("IP source returned no data", "source", src.name, "ip", ip)
		return nil
	}
	return data
}

// 数据源 1: ip-api.com（中文返回，45次/分钟限制，仅支持 HTTP）
func parseIPAPICom(ip string, raw map[string]interface{}) map[string]interface{} {
	if status, _ := raw["status"].(string); status != "success" {
		return nil
	}
	return map[string]interface{}{
		"ip":           ip,
		"country":      raw["country"],
		"country_code": raw["countryCode"],
		"region":       raw["regionName"],
		"city":         raw["city"],
		"isp":          raw["isp"],
		"org":          raw["org"],
		"as":           raw["as"],
		"latitude":     raw["lat"],
		"longitude":    raw["lon"],
		"timezone":     raw["timezone"],
		"source":       "ip-api.com",
	}
}

// 数据源 2: ipwho.is（免费无限制，英文返回，ISP 在 connection 嵌套对象）
func parseIpwhoIs(ip string, raw map[string]interface{}) map[string]interface{} {
	if ok, _ := raw["success"].(bool); !ok {
		return nil
	}
	conn, _ := raw["connection"].(map[string]interface{})
	return map[string]interface{}{
		"ip":           ip,
		"country":      raw["country"],
		"country_code": raw["country_code"],
		"region":       raw["region"],
		"city":         raw["city"],
		"isp":          mapGet(conn, "isp"),
		"org":          mapGet(conn, "org"),
		"as":           mapGet(conn, "asn"),
		"latitude":     raw["latitude"],
		"longitude":    raw["longitude"],
		"timezone":     raw["timezone"],
		"source":       "ipwho.is",
	}
}

// 数据源 3: api.ip.sb（免费，英文返回）
func parseIpSb(ip string, raw map[string]interface{}) map[string]interface{} {
	if raw["country"] == nil {
		return nil
	}
	asn := ""
	if v, ok := raw["asn"]; ok {
		asn = fmt.Sprintf("AS%v", v)
	}
	return map[string]interface{}{
		"ip":           ip,
		"country":      raw["country"],
		"country_code": raw["country_code"],
		"region":       raw["region"],
		"city":         raw["city"],
		"isp":          raw["isp"],
		"org":          raw["organization"],
		"as":           asn,
		"latitude":     raw["latitude"],
		"longitude":    raw["longitude"],
		"timezone":     raw["timezone"],
		"source":       "ip.sb",
	}
}

// mapGet 从嵌套 map 安全取值
func mapGet(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return strval(m[key])
}

// ============ IP 信息卡片 / 端口扫描 / Whois (RDAP) ============

// ============ 访问统计模块（基于 nginx access.log 实时解析） ============

var (
	logStatsMu          sync.Mutex
	logStatsTotalHits   int64
	logStatsUniqueIPs   = make(map[string]bool)
	logStatsPathCounts  = make(map[string]int64)
	logStatsStatusCount = make(map[string]int64)
	logStatsRecent      []statsRecentItem
	logStatsStartTime   = time.Now()
	logFileOffset       int64
)

type statsRecentItem struct {
	Time    string `json:"time"`
	IP      string `json:"ip"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
	UA      string `json:"ua"`
	Latency int64  `json:"latency_ms"`
}

// logLineRe 解析 nginx combined 日志行
var logLineRe = regexp.MustCompile(`^(\S+) - \S+ \[([^\]]+)\] "(\S+) ([^"]*?) HTTP[^"]*" (\d{3}) ([\d-]+) "([^"]*)" "([^"]*)"`)

// parseAccessLog 解析 access.log（支持增量：从 logFileOffset 继续读）
func parseAccessLog(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return
	}
	if stat.Size() < logFileOffset {
		// 日志轮转过，重置
		logFileOffset = 0
	}
	if _, err := f.Seek(logFileOffset, io.SeekStart); err != nil {
		return
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		m := logLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ip, timeStr, method, path, statusStr := m[1], m[2], m[3], m[4], m[5]
		// 过滤 API/静态资源/探测
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/_nuxt/") {
			continue
		}
		switch path {
		case "/favicon.ico", "/favicon.svg", "/ip", "/robots.txt", "/sitemap.xml", "/analytics":
			continue
		}
		status, _ := strconv.Atoi(statusStr)

		logStatsMu.Lock()
		logStatsTotalHits++
		logStatsUniqueIPs[ip] = true
		logStatsPathCounts[path]++
		logStatsStatusCount[strconv.Itoa(status)]++
		logStatsRecent = append(logStatsRecent, statsRecentItem{
			Time:   timeStr,
			IP:     ip,
			Method: method,
			Path:   path,
			Status: status,
			UA:     m[8],
		})
		if len(logStatsRecent) > 300 {
			logStatsRecent = logStatsRecent[len(logStatsRecent)-300:]
		}
		logStatsMu.Unlock()
	}
	logFileOffset, _ = f.Seek(0, io.SeekCurrent)
}

// initLogStats 启动时全量解析 + 每 10 秒增量
func initLogStats() {
	parseAccessLog("/var/log/ipchk-access.log")
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			parseAccessLog("/var/log/ipchk-access.log")
		}
	}()
	slog.Info("Log stats initialized")
}

// analyticsHandler 返回实时统计 JSON
func analyticsHandler(c *gin.Context) {
	logStatsMu.Lock()
	type pc struct {
		Path  string `json:"path"`
		Count int64  `json:"count"`
	}
	paths := []pc{}
	for p, cnt := range logStatsPathCounts {
		paths = append(paths, pc{p, cnt})
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Count > paths[j].Count })
	if len(paths) > 20 {
		paths = paths[:20]
	}

	resp := map[string]interface{}{
		"totalHits": logStatsTotalHits,
		"uniqueIPs": len(logStatsUniqueIPs),
		"status":    logStatsStatusCount,
		"topPaths":  paths,
		"recent":    logStatsRecent,
		"startTime": logStatsStartTime.Format("2006-01-02 15:04:05"),
		"now":       time.Now().Format("2006-01-02 15:04:05"),
	}
	logStatsMu.Unlock()
	writeJSON(c, resp)
}

// ============ 实时日志查看 ============

// logsHandler 返回 nginx access.log 尾部 N 行（用于实时日志页面）
func logsHandler(c *gin.Context) {
	lines := 100
	if l := c.Query("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 2000 {
			lines = n
		}
	}
	// 支持 ?after= 增量拉取（时间戳毫秒），用于前端轮询去重
	after := c.Query("after")
	afterMs, _ := strconv.ParseInt(after, 10, 64)

	logFile := "/var/log/ipchk-access.log"
	if _, err := os.Stat(logFile); err != nil {
		logFile = "/usr/local/openresty/nginx/logs/access.log"
	}
	rows, err := readTailLines(logFile, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 过滤增量（可选）
	filtered := rows
	if afterMs > 0 {
		filtered = []string{}
		for _, r := range rows {
			ts := parseLogTimestamp(r)
			if ts >= afterMs {
				filtered = append(filtered, r)
			}
		}
	}

	var lastTs int64
	if len(rows) > 0 {
		lastTs = parseLogTimestamp(rows[len(rows)-1])
	}

	writeJSON(c, map[string]interface{}{
		"lines":  len(filtered),
		"lastTs": lastTs,
		"logs":   filtered,
	})
}

// readTailLines 从文件尾部读取 N 行
func readTailLines(path string, lines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size == 0 {
		return []string{}, nil
	}

	const chunkSize = 32 * 1024
	offset := size
	var remainder []byte
	var result []string

	for offset > 0 && len(result) < lines {
		readSize := int64(chunkSize)
		if readSize > offset {
			readSize = offset
		}
		offset -= readSize
		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, offset); err != nil && err.Error() != "EOF" {
			return nil, err
		}
		data := append(buf, remainder...)
		parts := strings.Split(string(data), "\n")
		remainder = []byte(parts[0])
		for i := len(parts) - 1; i >= 1 && len(result) < lines; i-- {
			if s := strings.TrimSpace(parts[i]); s != "" {
				result = append(result, s)
			}
		}
	}
	if len(result) < lines && len(remainder) > 0 {
		if s := strings.TrimSpace(string(remainder)); s != "" {
			result = append(result, s)
		}
	}
	// 反转（因为是从尾部收集的倒序）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

// parseLogTimestamp 解析 nginx combined 日志的时间戳（毫秒）
// 格式: [04/Aug/2026:06:52:30 +0800]
func parseLogTimestamp(line string) int64 {
	idx := strings.Index(line, "[")
	if idx < 0 {
		return 0
	}
	end := strings.Index(line[idx:], "]")
	if end < 0 {
		return 0
	}
	timeStr := line[idx+1 : idx+end]
	// "04/Aug/2026:06:52:30 +0800" → 取 "04/Aug/2026:06:52:30"
	spaceIdx := strings.Index(timeStr, " ")
	if spaceIdx > 0 {
		timeStr = timeStr[:spaceIdx]
	}
	t, err := time.Parse("02/Jan/2006:15:04:05", timeStr)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// ipCardHandler 生成 IP 信息卡片 SVG（分享图）
func ipCardHandler(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		ip = c.ClientIP()
	}
	data := queryIPLocation(ip)
	if data == nil {
		data = ipdb.SearchIP(ip)
	}
	get := func(k string) string { return strings.TrimSpace(strval(data[k])) }

	asn := parseASN(get("as"))
	countryCode := strings.ToUpper(get("country_code"))
	org := get("org")
	fraudScore := calculateFraudScore(ip, asn, countryCode, org)
	riskLevel := riskLevelOf(fraudScore)
	riskColor := "#3EAF7C"
	switch riskLevel {
	case "轻度风险":
		riskColor = "#E6A23C"
	case "中度风险":
		riskColor = "#E67E22"
	case "高度风险":
		riskColor = "#F56C6C"
	}

	location := strings.TrimSpace(get("country") + " " + get("region") + " " + get("city"))
	if location == "" {
		location = "未知位置"
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="600" height="320" viewBox="0 0 600 320">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%%" stop-color="#1a2e2a"/>
      <stop offset="100%%" stop-color="#0d1a17"/>
    </linearGradient>
    <linearGradient id="accent" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0%%" stop-color="#3EAF7C"/>
      <stop offset="100%%" stop-color="#2E9A68"/>
    </linearGradient>
  </defs>
  <rect width="600" height="320" rx="20" fill="url(#bg)"/>
  <rect x="0" y="0" width="600" height="6" fill="url(#accent)"/>
  <text x="30" y="52" font-family="Arial" font-size="22" fill="#3EAF7C" font-weight="bold">ipchk.cn · IP 信息卡片</text>
  <text x="30" y="110" font-family="Consolas, monospace" font-size="44" fill="#ffffff" font-weight="bold">%s</text>
  <text x="30" y="150" font-family="Arial" font-size="20" fill="#9ca3af">%s</text>
  <text x="30" y="190" font-family="Arial" font-size="18" fill="#cfcfcf">ASN: %s</text>
  <text x="30" y="225" font-family="Arial" font-size="18" fill="#cfcfcf">IP 来源: %s</text>
  <rect x="30" y="250" width="200" height="8" rx="4" fill="#2a3a35"/>
  <rect x="30" y="250" width="%d" height="8" rx="4" fill="%s"/>
  <text x="240" y="272" font-family="Arial" font-size="18" fill="%s" font-weight="bold">%s %d/100</text>
  <text x="570" y="308" font-family="Arial" font-size="13" fill="#6b7280" text-anchor="end">ipchk.cn</text>
</svg>`, ip, location, get("as"), ipSourceOf(asn, ip, org), fraudScore*2, riskColor, riskColor, riskLevel, fraudScore)

	c.Header("Content-Type", "image/svg+xml")
	c.Header("Cache-Control", "public, max-age=300")
	c.String(http.StatusOK, svg)
}

// portScanHandler 端口扫描
func portScanHandler(c *gin.Context) {
	host := c.Param("ip")
	if host == "" {
		host = c.ClientIP()
	}
	portsStr := c.Query("ports")
	var ports []int
	if portsStr != "" {
		for _, p := range strings.Split(portsStr, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(p))
			if err == nil && n > 0 && n < 65536 {
				ports = append(ports, n)
			}
		}
	}
	if len(ports) == 0 {
		ports = []int{21, 22, 23, 25, 53, 80, 110, 135, 139, 143, 443, 445, 993, 995, 1433, 1521, 3306, 3389, 5432, 6379, 8080, 8443, 8888, 9090, 9200, 27017}
	}

	type result struct {
		Port  int    `json:"port"`
		State string `json:"state"`
	}
	results := make([]result, len(ports))
	var wg sync.WaitGroup
	for i, port := range ports {
		wg.Add(1)
		go func(i, port int) {
			defer wg.Done()
			addr := net.JoinHostPort(host, strconv.Itoa(port))
			conn, err := net.DialTimeout("tcp4", addr, 2*time.Second)
			if err == nil {
				conn.Close()
				results[i] = result{Port: port, State: "open"}
			} else {
				results[i] = result{Port: port, State: "closed"}
			}
		}(i, port)
	}
	wg.Wait()

	open := 0
	for _, r := range results {
		if r.State == "open" {
			open++
		}
	}
	writeJSON(c, map[string]interface{}{
		"host":  host,
		"total": len(ports),
		"open":  open,
		"ports": results,
	})
}

// whoisHandler 域名/IP 注册信息查询（域名用 whois 协议，IP 用 RDAP）
var whoisCache sync.Map

type whoisCacheEntry struct {
	result    interface{}
	timestamp time.Time
}

func whoisHandler(c *gin.Context) {
	target := c.Param("target")
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target required"})
		return
	}

	// IP 查询：使用 RDAP（whois 协议对 IP 是各 RIR 分散服务，RDAP 更统一）
	if net.ParseIP(target) != nil {
		whoisRDAP(c, target)
		return
	}

	// 域名查询：结构化 whois（likexian 库 + happy-eyeballs + 字段解析）
	domain := strings.ToLower(strings.TrimSpace(target))
	if cached, ok := whoisCache.Load(domain); ok {
		entry := cached.(whoisCacheEntry)
		if time.Since(entry.timestamp) < 5*time.Minute {
			writeJSON(c, entry.result)
			return
		}
		whoisCache.Delete(domain)
	}
	result, err := webtest.QueryWhois(domain)
	if err != nil {
		writeJSON(c, gin.H{"target": domain, "error": "Whois 查询失败：" + err.Error()})
		return
	}
	whoisCache.Store(domain, whoisCacheEntry{result: result, timestamp: time.Now()})
	writeJSON(c, result)
}

// whoisRawQuery 向指定 whois 服务器发起 TCP 43 查询
func whoisRawQuery(server, query string) (string, error) {
	conn, err := net.DialTimeout("tcp4", server+":43", 10*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
		return "", err
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// extractWhoisServer 从 IANA whois 响应中提取注册局 whois 服务器
func extractWhoisServer(text string) string {
	re := regexp.MustCompile(`(?im)^refer:\s*(\S+)`)
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// defaultWhoisServer 各 TLD 默认 whois 服务器兜底
func defaultWhoisServer(tld string) string {
	switch strings.ToLower(tld) {
	case "com", "net":
		return "whois.verisign-grs.com"
	case "org":
		return "whois.pir.org"
	case "cn":
		return "whois.cnnic.cn"
	case "info":
		return "whois.afilias.net"
	case "xyz":
		return "whois.nic.xyz"
	case "top":
		return "whois.nic.top"
	case "cc":
		return "whois.nic.cc"
	case "tv":
		return "whois.nic.tv"
	case "io":
		return "whois.nic.io"
	case "me":
		return "whois.nic.me"
	case "co":
		return "whois.nic.co"
	case "us":
		return "whois.nic.us"
	case "uk":
		return "whois.nic.uk"
	case "de":
		return "whois.denic.de"
	case "jp":
		return "whois.jprs.jp"
	default:
		return ""
	}
}

// whoisLookup 完整 whois 查询：IANA 定位注册局 → 注册局查询
func whoisLookup(domain string) (string, string, error) {
	tld := ""
	if idx := strings.LastIndex(domain, "."); idx >= 0 {
		tld = strings.ToLower(domain[idx+1:])
	}

	// 1. 查 IANA 获取注册局 whois 服务器
	iana, err := whoisRawQuery("whois.iana.org", domain)
	if err == nil {
		if server := extractWhoisServer(iana); server != "" {
			result, err := whoisRawQuery(server, domain)
			if err == nil {
				return result, server, nil
			}
		}
	}

	// 2. IANA 失败 → 按 TLD 默认服务器
	if server := defaultWhoisServer(tld); server != "" {
		result, err := whoisRawQuery(server, domain)
		if err == nil {
			return result, server, nil
		}
	}

	// 3. 通用兜底
	for _, s := range []string{"whois.verisign-grs.com", "whois.pir.org"} {
		result, err := whoisRawQuery(s, domain)
		if err == nil {
			return result, s, nil
		}
	}

	if err != nil {
		return "", "", err
	}
	return "", "", fmt.Errorf("无法定位该域名的 whois 服务器")
}

// extractWhoisFields 从 whois 文本提取关键字段
func extractWhoisFields(text string) map[string]string {
	fields := map[string]string{}
	patterns := map[string]string{
		"registrar":  `(?im)^\s*(Sponsoring Registrar|Registrar):\s*(.+)$`,
		"creation":   `(?im)^\s*(Creation Date|Registered on|Created On|Registration Time|Created):\s*(.+)$`,
		"expiry":     `(?im)^\s*(Registry Expiry Date|Expiry Date|Expiration Time|Expires On|Expiration Date):\s*(.+)$`,
		"ns":         `(?im)^\s*(Name Server|Nameserver|NS):\s*(.+)$`,
		"status":     `(?im)^\s*(Domain Status|Status):\s*(.+)$`,
		"registrant": `(?im)^\s*(Registrant Organization|Registrant Name|Registrant):\s*(.+)$`,
	}
	for key, pat := range patterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(text); len(m) > 1 {
			if key == "ns" || key == "status" {
				// 多值合并
				var values []string
				for _, mm := range re.FindAllStringSubmatch(text, -1) {
					v := strings.TrimSpace(mm[len(mm)-1])
					if v != "" && !containsStr(values, v) {
						values = append(values, v)
					}
				}
				fields[key] = strings.Join(values, ", ")
			} else {
				fields[key] = strings.TrimSpace(m[len(m)-1])
			}
		}
	}
	return fields
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// whoisRDAP IP 查询使用 RDAP
func whoisRDAP(c *gin.Context, target string) {
	// IP 查询（whois 协议对 IP 是各 RIR 分散服务，RDAP 更统一）
	var candidates []string
	if net.ParseIP(target) != nil {
		candidates = []string{
			"https://rdap.org/ip/" + target,
			"https://rdap.db-ip.com/rdap/ip/" + target,
		}
	} else {
		// 按顶级域选择对应注册局的 RDAP 服务
		tld := ""
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			tld = strings.ToLower(target[idx+1:])
		}
		switch tld {
		case "cn":
			candidates = []string{
				"https://rdap.cnnic.cn/domain/" + target,
				"https://rdap.org/domain/" + target,
			}
		case "com":
			candidates = []string{
				"https://rdap.verisign.com/com/v1/domain/" + target,
				"https://rdap.org/domain/" + target,
			}
		case "net":
			candidates = []string{
				"https://rdap.verisign.com/net/v1/domain/" + target,
				"https://rdap.org/domain/" + target,
			}
		case "org":
			candidates = []string{
				"https://rdap.publicinterestregistry.org/rdap/domain/" + target,
				"https://rdap.org/domain/" + target,
			}
		default:
			candidates = []string{"https://rdap.org/domain/" + target}
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		},
	}

	var raw map[string]interface{}
	var body []byte
	var lastErr string
	for _, apiURL := range candidates {
		resp, err := client.Get(apiURL)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			lastErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
			continue
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			lastErr = "响应不是有效 JSON"
			continue
		}
		lastErr = ""
		break
	}
	if len(raw) == 0 {
		msg := "RDAP 查询失败"
		if lastErr != "" {
			msg += "：" + lastErr
		}
		msg += "（该域名可能不存在或无 RDAP 记录）"
		writeJSON(c, gin.H{"target": target, "error": msg})
		return
	}

	// 提取关键字段
	extract := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := raw[k]; ok && v != nil {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
				if f, ok := v.(float64); ok {
					return strconv.Itoa(int(f))
				}
			}
		}
		return ""
	}
	// 从 entities 提取注册商/组织
	registrar := ""
	orgName := ""
	if entities, ok := raw["entities"].([]interface{}); ok {
		for _, e := range entities {
			if em, ok := e.(map[string]interface{}); ok {
				if name, ok := em["name"].(string); ok && name != "" && orgName == "" {
					orgName = name
				}
				if roles, ok := em["roles"].([]interface{}); ok {
					for _, r := range roles {
						if r == "registrar" {
							if vcard, ok := em["vcardArray"].([]interface{}); ok && len(vcard) > 1 {
								if arr, ok := vcard[1].([]interface{}); ok {
									for _, item := range arr {
										if ia, ok := item.([]interface{}); ok && len(ia) > 3 {
											if ia[0] == "fn" {
												registrar = strval(ia[3])
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if registrar == "" {
		registrar = orgName
	}
	// IP 查询兜底：顶层 handle/name 字段（如 "NET-8-8-8-0-1"）
	if registrar == "" {
		if h, ok := raw["handle"].(string); ok && h != "" {
			registrar = h
		} else if n, ok := raw["name"].(string); ok && n != "" {
			registrar = n
		}
	}
	// 名称服务器
	var nameservers []string
	if ns, ok := raw["nameservers"].([]interface{}); ok {
		for _, n := range ns {
			if nm, ok := n.(map[string]interface{}); ok {
				if ldh, ok := nm["ldhName"].(string); ok {
					nameservers = append(nameservers, ldh)
				}
			}
		}
	}
	// 状态
	var status []string
	if st, ok := raw["status"].([]interface{}); ok {
		for _, s := range st {
			status = append(status, strval(s))
		}
	}

	writeJSON(c, map[string]interface{}{
		"target":       target,
		"type":         "domain",
		"registrar":    registrar,
		"creationDate": extract("events", "eventAction"),
		"updatedDate":  extract("updatedDate"),
		"nameservers":  nameservers,
		"status":       status,
		"raw":          string(body),
	})
}

func purityHandler(c *gin.Context) {
	ip := c.Param("ip")
	if ip == "" {
		ip = c.ClientIP()
	}
	agg := queryAggregated(ip)
	if agg == nil || len(agg.Sources) == 0 {
		// 全部数据源不可用：走旧逻辑兜底（本地 ipdb + 静态评分）
		data := queryIPLocation(ip)
		if data == nil {
			data = ipdb.SearchIP(ip)
		}
		if data != nil {
			legacyPurityResponse(c, ip, data)
			return
		}
		writeJSON(c, gin.H{"error": "数据源全部不可用", "ip": ip})
		return
	}
	rbl := queryRBL(ip)
	stab := queryStability(ip)
	rep := buildPurityReport(ip, agg, rbl, stab)
	if isCLIUA(c.GetHeader("User-Agent")) {
		c.String(http.StatusOK, formatPurityReport(rep))
		return
	}
	writeJSON(c, rep)
}

// purityCheckHandler POST /v1/purity/check 批量检测
func purityCheckHandler(c *gin.Context) {
	var req purityCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeJSON(c, gin.H{"ok": false, "error": "invalid request body"})
		return
	}
	if len(req.IPs) == 0 {
		writeJSON(c, gin.H{"ok": false, "error": "no IP addresses provided"})
		return
	}
	resp := purityCheck(req.IPs)
	writeJSON(c, resp)
}

// legacyPurityResponse 数据源全挂时的旧格式兜底响应
func legacyPurityResponse(c *gin.Context, ip string, data map[string]interface{}) {
	get := func(k string) string {
		if v, ok := data[k]; ok && v != nil {
			return strings.TrimSpace(strval(v))
		}
		return ""
	}
	asn := parseASN(get("as"))
	countryCode := strings.ToUpper(get("country_code"))
	org := get("org")
	asOrg := get("as")
	if asOrg == "" {
		asOrg = org
	}
	fraudScore := calculateFraudScore(ip, asn, countryCode, org)
	result := map[string]interface{}{
		"ip": ip, "asn": asn, "asOrganization": asOrg,
		"country": get("country"), "countryCode": countryCode,
		"region": get("region"), "city": get("city"), "isp": get("isp"),
		"fraudScore": fraudScore, "ippureCoefficient": fraudScore,
		"cloudflareCoefficient": calculateCloudflareCoefficient(fraudScore, asn, countryCode),
		"riskLevel":             riskLevelOf(fraudScore),
		"ipSource":              ipSourceOf(asn, ip, org),
		"ipProperties":          ipPropertiesOf(ip, asn, org),
		"isDataCenter":          isDataCenterIP(asn, ip, org),
		"isResidential":         isResidentialIP(asn),
		"isBroadcast":           isBroadcastIP(ip),
		"source":                get("source"),
	}
	if isCLIUA(c.GetHeader("User-Agent")) {
		c.String(http.StatusOK, formatPurityText(result))
		return
	}
	writeJSON(c, result)
}

// formatPurityText IP 纯净度 CLI 格式化输出
func formatPurityText(data map[string]interface{}) string {
	get := func(k string) string { return strval(data[k]) }
	var b strings.Builder
	b.WriteString("IP 纯净度检测结果\n")
	b.WriteString(strings.Repeat("─", 46) + "\n")
	rows := [][2]string{
		{"IP", get("ip")},
		{"归属地", strings.TrimSpace(get("country") + " " + get("region") + " " + get("city"))},
		{"ASN", get("asOrganization")},
		{"IP 来源", get("ipSource")},
		{"风险评分", get("fraudScore") + " / 100"},
		{"风险等级", get("riskLevel")},
		{"CF 系数", get("cloudflareCoefficient")},
		{"属性", strings.Join(mustStrSlice(data["ipProperties"]), "、")},
		{"数据中心", boolToStr(data["isDataCenter"])},
		{"住宅 IP", boolToStr(data["isResidential"])},
		{"广播地址", boolToStr(data["isBroadcast"])},
		{"数据来源", get("source")},
	}
	maxW := 0
	for _, r := range rows {
		if w := displayWidth(r[0]); w > maxW {
			maxW = w
		}
	}
	for _, r := range rows {
		if r[1] == "" {
			continue
		}
		b.WriteString(padKey(r[0], maxW+2) + ": " + r[1] + "\n")
	}
	b.WriteString(strings.Repeat("─", 46))
	return b.String()
}

func mustStrSlice(v interface{}) []string {
	if s, ok := v.([]string); ok {
		return s
	}
	return nil
}

func boolToStr(v interface{}) string {
	if b, ok := v.(bool); ok {
		if b {
			return "是"
		}
		return "否"
	}
	return ""
}

// parseASN 从 "AS15169 Google LLC" 提取 15169
func parseASN(as string) int {
	s := strings.TrimPrefix(as, "AS")
	num := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		num = num*10 + int(r-'0')
	}
	return num
}

// calculateFraudScore IP 纯净度评分（0-100，越高越危险）
func calculateFraudScore(ip string, asn int, countryCode string, org string) int {
	score := 0

	// 1. IP 类型和前缀分析（权重35%）
	if strings.Contains(ip, ":") {
		score += 8 // IPv6 略高风险
		if strings.HasPrefix(ip, "2a09:") {
			score += 15 // Cloudflare 特定 IPv6 前缀
		}
		if strings.HasPrefix(ip, "2a06:") {
			score += 12
		}
	} else {
		highRiskPrefixes := []string{"104.16.", "104.17.", "104.18.", "172.64.", "172.65.", "172.66.", "172.67.", "108.162."}
		mediumRiskPrefixes := []string{"192.0.", "185.", "193.", "45.", "147."}
		for _, p := range highRiskPrefixes {
			if strings.HasPrefix(ip, p) {
				score += 40
				break
			}
		}
		if score < 40 {
			for _, p := range mediumRiskPrefixes {
				if strings.HasPrefix(ip, p) {
					score += 20
					break
				}
			}
		}
	}

	// 2. ASN 分析（权重35%）
	highRiskASNs := map[int]bool{
		13335: true,  // Cloudflare
		16509: true,  // Amazon AWS
		14061: true,  // DigitalOcean
		395747: true, // Vultr
		20473: true,  // Linode
		44440: true,  // OVH
		54113: true,  // Fastly
		15169: true,  // Google
		8075:  true,  // Microsoft
		31898: true,  // Oracle Cloud
		36492: true,  // Oracle Cloud
		396982: true, // Google Cloud
		14618: true,  // Amazon AWS
		20940: true,  // Akamai
	}
	mediumRiskASNs := map[int]bool{
		32097:  true, // Alibaba
		45102:  true, // Tencent
		37963:  true, // Aliyun
		132203: true, // Tencent Cloud
		45090:  true, // Tencent Cloud
		136907: true, // Huawei Cloud
		63949:  true, // Linode/Akamai
		2635:   true, // Automattic
	}
	if asn != 0 {
		if highRiskASNs[asn] {
			score += 35
		} else if mediumRiskASNs[asn] {
			score += 18
		}
	}

	// 2.5 数据中心 IP 额外加分（ASN 未覆盖时按组织/IP 特征兜底）
	if score < 35 && isDataCenterIP(asn, ip, org) {
		score += 25
	}

	// 3. 地理位置分析（权重20%）
	switch countryCode {
	case "CN":
		score += 10
	case "US":
		score += 25
	case "NL", "DE", "SG":
		score += 15
	case "RU", "VN", "IR":
		score += 20
	default:
		score += 12
	}

	// 4. 本地网络检查（私有地址直接安全）
	if isPrivateAddr(ip) {
		return 0
	}

	// 5. 得分控制
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// calculateCloudflareCoefficient Cloudflare 系数
func calculateCloudflareCoefficient(baseScore int, asn int, countryCode string) int {
	score := float64(baseScore) * 0.75
	if asn == 13335 {
		score = float64(baseScore) * 0.95
		if score > 88 {
			score = 88
		}
	}
	if countryCode == "CN" {
		score *= 0.85
	}
	if countryCode == "IR" || countryCode == "KP" || countryCode == "CU" {
		score *= 1.3
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return int(score + 0.5)
}

// riskLevelOf 风险等级
func riskLevelOf(score int) string {
	switch {
	case score <= 25:
		return "安全"
	case score <= 50:
		return "轻度风险"
	case score <= 70:
		return "中度风险"
	default:
		return "高度风险"
	}
}

// isResidentialIP 住宅 IP 判断
func isResidentialIP(asn int) bool {
	residentialASNs := map[int]bool{13335: true, 15169: true, 8075: true}
	return residentialASNs[asn]
}

// isBroadcastIP 广播 IP 判断
func isBroadcastIP(ip string) bool {
	return strings.HasSuffix(ip, ".0") || strings.HasSuffix(ip, ".255")
}

// isDataCenterIP 数据中心 IP 判断（ASN + 前缀 + 组织名）
func isDataCenterIP(asn int, ip, org string) bool {
	dataCenterASNs := map[int]bool{
		13335: true, 16509: true, 8075: true, 54113: true, 44440: true,
		14061: true, 395747: true, 20473: true, 31898: true, 36492: true,
		396982: true, 14618: true, 20940: true, 37963: true, 132203: true,
		45090: true, 136907: true, 45102: true, 32097: true, 63949: true,
	}
	if dataCenterASNs[asn] {
		return true
	}
	if strings.HasPrefix(ip, "104.") || strings.HasPrefix(ip, "172.64.") {
		return true
	}
	// 组织名兜底判断（ip-api 的 org/isp 含典型云厂商关键词）
	orgLower := strings.ToLower(org)
	keywords := []string{"cloudflare", "amazon", "aws", "digitalocean", "linode", "ovh", "google cloud", "microsoft", "azure", "alibaba", "tencent", "oracle cloud", "vultr"}
	for _, k := range keywords {
		if strings.Contains(orgLower, k) {
			return true
		}
	}
	return false
}

// ipSourceOf IP 来源类型
func ipSourceOf(asn int, ip, org string) string {
	if ip != "" && (strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.16.")) {
		return "局域网"
	}
	if asn == 13335 {
		return "Cloudflare 网络"
	}
	if asn == 15169 {
		return "Google 网络"
	}
	if asn == 8075 {
		return "Microsoft Azure"
	}
	if asn == 16509 {
		return "Amazon AWS"
	}
	if asn == 54113 {
		return "Fastly"
	}
	if asn == 44440 {
		return "OVH"
	}
	if isDataCenterIP(asn, ip, org) {
		return "数据中心"
	}
	return "住宅/商业网络"
}

// ipPropertiesOf IP 属性列表
func ipPropertiesOf(ip string, asn int, org string) []string {
	var props []string
	if strings.Contains(ip, ":") {
		props = append(props, "IPv6")
	} else {
		props = append(props, "IPv4")
	}
	if isDataCenterIP(asn, ip, org) {
		props = append(props, "数据中心")
	} else {
		props = append(props, "住宅/商业")
	}
	if isBroadcastIP(ip) {
		props = append(props, "广播地址")
	}
	return props
}

// isPrivateAddr 私有地址判断
func isPrivateAddr(ip string) bool {
	if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		parts := strings.Split(ip, ".")
		if len(parts) >= 2 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n >= 16 && n <= 31 {
				return true
			}
		}
	}
	return ip == "::1" || strings.HasPrefix(ip, "fe80:")
}
func clientIPHandler(c *gin.Context) {
	ip := c.ClientIP()
	c.String(http.StatusOK, ip+"\n")
}
func dnsQueryHandler(c *gin.Context) {

	domain := c.Param("domain")
	parsedURL, err := parseURL(domain)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid domain",
		})
		return
	}
	domain = parsedURL.Host
	recodeType := c.Param("type")
	switch recodeType {
	case "a":
		result, err := webtest.ResolveARecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "aaaa":
		result, err := webtest.ResolveAAAARecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "cname":
		result, err := webtest.ResolveCNAMERecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "mx":
		result, err := webtest.ResolveMXRecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "ns":
		result, err := webtest.ResolveNSRecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "ptr":
		result, err := webtest.ResolvePTRRecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "srv":
		result, err := webtest.ResolveSRVRecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "txt":
		result, err := webtest.ResolveTXTRecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	case "caa":
		result, err := webtest.ResolveCAARecord(domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		writeJSON(c, result)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid record type",
		})
		return
	}
}
func pingHandler(c *gin.Context) {
	host := c.Param("ip")
	port := c.Query("port")
	if host == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "IP or hostname parameter is required",
		})
		return
	}
	if port == "" {
		port = "80"
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid port number",
		})
		return
	}

	count := 4
	if countStr := c.Query("count"); countStr != "" {
		n, err := strconv.Atoi(countStr)
		if err != nil || n < 1 || n > 20 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "count must be an integer between 1 and 20",
			})
			return
		}
		count = n
	}

	cacheKey := fmt.Sprintf("%s:%s:%d", host, port, count)
	if cached, ok := pingCache.Load(cacheKey); ok {
		entry := cached.(pingCacheEntry)
		if time.Since(entry.timestamp) < 1*time.Minute {
			c.JSON(200, entry.result)
			return
		}
		pingCache.Delete(cacheKey)
	}

	rawResult, _, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
		result := &TCPingResult{}

		switch SINGLE_STACK {
		case "ipv4":
			ipv4, errV4 := webtest.TCPingRun(host, port, count, "v4", 10*time.Second, 100*time.Millisecond)
			if errV4 != nil {
				ipv4 = &webtest.TCPingStats{
					IP: "Error: " + errV4.Error(),
				}
			}
			result.IPv4 = ipv4
			result.IPv6 = &webtest.TCPingStats{
				IP: "Skipped due to SINGLE_STACK=ipv4",
			}
		case "ipv6":
			ipv6, errV6 := webtest.TCPingRun(host, port, count, "v6", 10*time.Second, 100*time.Millisecond)
			if errV6 != nil {
				ipv6 = &webtest.TCPingStats{
					IP: "Error: " + errV6.Error(),
				}
			}
			result.IPv6 = ipv6
			result.IPv4 = &webtest.TCPingStats{
				IP: "Skipped due to SINGLE_STACK=ipv6",
			}
		default:
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				ipv6, errV6 := webtest.TCPingRun(host, port, count, "v6", 10*time.Second, 100*time.Millisecond)
				if errV6 != nil {
					ipv6 = &webtest.TCPingStats{
						IP: "Error: " + errV6.Error(),
					}
				}
				result.IPv6 = ipv6
			}()

			go func() {
				defer wg.Done()
				ipv4, errV4 := webtest.TCPingRun(host, port, count, "v4", 10*time.Second, 100*time.Millisecond)
				if errV4 != nil {
					ipv4 = &webtest.TCPingStats{
						IP: "Error: " + errV4.Error(),
					}
				}
				result.IPv4 = ipv4
			}()

			wg.Wait()
		}

		pingCache.Store(cacheKey, pingCacheEntry{result: result, timestamp: time.Now()})

		ipv4Failed := result.IPv4 != nil && strings.HasPrefix(result.IPv4.IP, "Error:")
		ipv6Failed := result.IPv6 != nil && strings.HasPrefix(result.IPv6.IP, "Error:")
		if ipv4Failed && ipv6Failed {
			go func() {
				time.Sleep(30 * time.Second)
				pingCache.Delete(cacheKey)
			}()
		}

		return result, nil
	})

	c.JSON(200, rawResult.(*TCPingResult))
}

func healchCheck(c *gin.Context) {
	writeJSON(c, gin.H{
		"status": "ok",
	})
}
func readConfig() {
	PORTS = os.Getenv("PORTS")
	GH_PROXY = os.Getenv("GH_PROXY")
	// SINGLE_STACK can be "ipv4", "ipv6", or empty for both.
	// Empty string is a valid value meaning dual-stack, not "unconfigured".
	// 如果当前测速节点机器是单栈网络，建议设置 SINGLE_STACK 环境变量来跳过另一个协议的测试，以避免不必要的错误日志和延迟
	SINGLE_STACK = strings.ToLower(strings.TrimSpace(os.Getenv("SINGLE_STACK")))
	DNS_SERVER = os.Getenv("DNS_SERVER")
	IPDB = os.Getenv("IPDB")
	ssrf.SetEnabled(os.Getenv("BLOCK_PRIVATE_IPS") != "false" && os.Getenv("BLOCK_PRIVATE_IPS") != "0")

	// SINGLE_STACK is intentionally excluded: empty string is a valid value (dual-stack).

	viper.SetConfigName("setting")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		slog.Warn("Failed to read config file, using defaults", "error", err)
	}
	if PORTS == "" {
		PORTS = viper.GetString("port")
	}
	if GH_PROXY == "" {
		GH_PROXY = viper.GetString("gh-proxy")
	}
	if SINGLE_STACK == "" {
		SINGLE_STACK = strings.ToLower(strings.TrimSpace(viper.GetString("single-stack")))
	}
	if DNS_SERVER == "" {
		DNS_SERVER = viper.GetString("dns-server")
	}
	if IPDB == "" {
		IPDB = viper.GetString("ipdb")
	}
	if PORTS == "" {
		PORTS = "8080"
	}
	slog.Info("SSRF protection initialized", "blockPrivateIPs", ssrf.Enabled())
}

func main() {
	readConfig()
	webtest.SetDNSServer(DNS_SERVER)
	initHTTPClients()
	initLogStats()
	if IPDB != "false" {
		ipdb.Init(GH_PROXY)
	}
	slog.Info("Starting server", "port", PORTS, "gh_proxy", GH_PROXY, "single_stack", SINGLE_STACK, "dns_server", DNS_SERVER)

	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/v1/detail/*url", checkWebsiteHandler)
	r.GET("/v1/ssl/*url", sslCheckHandler)

	r.GET("/v1/tcping/:ip", pingHandler)
	r.GET("/v1/dns/:type/*domain", dnsQueryHandler)
	r.GET("/v1/speed/:version/*url", websiteSpeedTestHandler)

	r.GET("/", healchCheck)
	r.GET("/ip", clientIPHandler)

	// 以下路由无条件注册：IPDB 开关只控制本地数据库下载（见上面的 ipdb.Init），
	// 不影响路由可用性。数据源未初始化时 ipdb.SearchIP 返回 "not loaded" 而非 panic，
	// purity 等接口走在线 API，IPDB=false 时依然可用。
	r.GET("/v1/location/:ip", locateIP)
	r.GET("/v1/location", locateUserIP)
	r.GET("/v1/purity/:ip", purityHandler)
	r.GET("/v1/purity", purityHandler)
	r.POST("/v1/purity/check", purityCheckHandler)
	r.GET("/v1/card/:ip", ipCardHandler)
	r.GET("/v1/card", ipCardHandler)
	r.GET("/v1/scan/:ip", portScanHandler)
	r.GET("/v1/whois/:target", whoisHandler)
	r.GET("/v1/logs", logsHandler)
	r.GET("/v1/analytics", analyticsHandler)

	if err := r.Run(":" + PORTS); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}
