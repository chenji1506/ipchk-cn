// stability.go — 网络稳定性 TCP 探测（P2）
//
// 对目标 IP 的 80/443 端口各发 4 个 TCP SYN，统计成功率与延迟分位数。
// 不做 ICMP（海外 VPS 常被限流，TCP 更可靠）。结果 24h 缓存 + singleflight。
package main

import (
	"net"
	"sort"
	"sync"
	"time"
)

// ============ 结果结构 ============

type StabilityResult struct {
	SuccessRate   float64 `json:"success_rate"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	TimeoutCount  int     `json:"timeout_count"`
	Probed        bool    `json:"probed"`

	probedAt time.Time
}

// ============ 缓存 ============

var (
	stabilityCache   = make(map[string]stabilityCacheEntry)
	stabilityCacheMu sync.Mutex
	stabilitySingle  singleflightGroup
)

type stabilityCacheEntry struct {
	data      *StabilityResult
	expiresAt time.Time
}

// ============ 查询入口 ============

func queryStability(ip string) *StabilityResult {
	stabilityCacheMu.Lock()
	if e, ok := stabilityCache[ip]; ok && time.Now().Before(e.expiresAt) {
		stabilityCacheMu.Unlock()
		return e.data
	}
	stabilityCacheMu.Unlock()

	v, _, _ := stabilitySingle.Do(ip, func() (interface{}, error) {
		r := probeStability(ip)
		stabilityCacheMu.Lock()
		stabilityCache[ip] = stabilityCacheEntry{data: r, expiresAt: time.Now().Add(24 * time.Hour)}
		stabilityCacheMu.Unlock()
		return r, nil
	})
	if r, ok := v.(*StabilityResult); ok {
		return r
	}
	return nil
}

// probeStability 对 80/443 各 4 次 TCP 探测
func probeStability(ip string) *StabilityResult {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return &StabilityResult{Probed: true}
	}

	ports := []string{"80", "443"}
	var latencies []float64 // 毫秒
	timeouts := 0
	attempts := 0

	for _, port := range ports {
		addr := net.JoinHostPort(ip, port)
		for i := 0; i < 4; i++ {
			attempts++
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				timeouts++
				continue
			}
			latencies = append(latencies, float64(time.Since(start).Microseconds())/1000.0)
			conn.Close()
		}
	}

	res := &StabilityResult{
		TimeoutCount: timeouts,
		Probed:       true,
	}
	if attempts == 0 {
		return res
	}
	res.SuccessRate = float64(len(latencies)) / float64(attempts)

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		res.AvgLatencyMs = round2(mean(latencies))
		res.P50LatencyMs = round2(percentile(latencies, 0.50))
		res.P95LatencyMs = round2(percentile(latencies, 0.95))
	}
	return res
}

// ============ 评分辅助 ============

// stabilityDimensionScore 稳定性维度得分（满分 15）
func stabilityDimensionScore(s *StabilityResult) int {
	if s == nil || !s.Probed {
		return 10 // 未探测中性值
	}
	if s.SuccessRate == 0 {
		return 3 // 完全不可达
	}
	score := 15.0
	// 成功率权重（60%）
	score -= (1.0 - s.SuccessRate) * 60.0
	// 延迟权重（40%）：p95 超过 500ms 开始扣分
	if s.P95LatencyMs > 500 {
		excess := (s.P95LatencyMs - 500) / 100.0
		if excess > 4 {
			excess = 4
		}
		score -= excess * 3.0
	}
	if score < 0 {
		score = 0
	}
	return int(score + 0.5)
}

// mean 均值
func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// percentile 分位数（vals 需已排序）
func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	idx := int(float64(len(vals)-1) * p)
	return vals[idx]
}

// round2 保留两位小数
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
