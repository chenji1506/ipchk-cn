// purity.go — IP 纯净度评分器（P1：多源证据交叉 + 可解释评分）
//
// 输入：Aggregated（多源聚合结果）
// 输出：PurityReport（新 schema，兼容旧字段）
// 评分模型：purity×85% + stability×10% + data_quality×5%，
// 纯净度 = 100 - (signal_risk + identity_risk)，受类型上限与覆盖上限封顶。
package main

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============ 评分输出结构（对齐 ipchk.cc schema） ============

type ScoreFormula struct {
	PurityWeight      int `json:"purity_weight"`
	StabilityWeight   int `json:"stability_weight"`
	DataQualityWeight int `json:"data_quality_weight"`
	ClassificationCap int `json:"classification_cap"`
}

type PurityDetail struct {
	Score                int      `json:"score"`
	RiskValue            int      `json:"risk_value"`
	SignalRisk           int      `json:"signal_risk"`
	IdentityRisk         int      `json:"identity_risk"`
	UncertaintyDeduction int      `json:"uncertainty_deduction"`
	CoverageCeiling      int      `json:"coverage_ceiling"`
	TypeCap              int      `json:"type_cap"`
	EffectiveTypeCap     int      `json:"effective_type_cap"`
	CapSignalAdjustment  int      `json:"cap_signal_adjustment"`
	TypeCapDeduction     int      `json:"type_cap_deduction"`
	Label                string   `json:"label"`
	Tone                 string   `json:"tone"`
	Confidence           string   `json:"confidence"`
	ConfidenceLabel      string   `json:"confidence_label"`
	EvidenceSourceCount  int      `json:"evidence_source_count"`
	RblCheckedCount      int      `json:"rbl_checked_count"`
	Definition           string   `json:"definition"`
	Reasons              []string `json:"reasons"`
}

type ProfileDetail struct {
	Primary        string             `json:"primary"`
	PrimaryTone    string             `json:"primary_tone"`
	Native         string             `json:"native"`
	NativeTone     string             `json:"native_tone"`
	Risk           string             `json:"risk"`
	RiskTone       string             `json:"risk_tone"`
	Summary        string             `json:"summary"`
	Tags           []Tag              `json:"tags"`
	SourceCount    int                `json:"source_count"`
	TypedSources   int                `json:"typed_sources"`
	DefinitionNote string             `json:"definition_note"`
	AccessNetwork  DimItem            `json:"access_network"`
	Privacy        DimItem            `json:"privacy"`
	Reputation     DimItem            `json:"reputation"`
	Network        DimItem            `json:"network"`
	Classification map[string]DimItem `json:"classification"`
}

type Tag struct {
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

type DimItem struct {
	Key        string `json:"key,omitempty"`
	Label      string `json:"label"`
	Tone       string `json:"tone"`
	Confidence string `json:"confidence,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Level      string `json:"level,omitempty"`
	BaseLabel  string `json:"base_label,omitempty"`
	Conflict   bool   `json:"conflict,omitempty"`
}

type Dimension struct {
	Score    int `json:"score"`
	MaxScore int `json:"max_score"`
}

type StabilityDetail struct {
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P50LatencyMs float64 `json:"p50_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	TimeoutCount int     `json:"timeout_count"`
	Probed       bool    `json:"probed"` // P1 未接入探测时为 false
}

type RBLDetail struct {
	ListedCount        int    `json:"listed_count"`
	NetworkListedCount int    `json:"network_listed_count"`
	RiskLevel          string `json:"risk_level"`
	QueryLimited       bool   `json:"query_limited"`
	Probed             bool   `json:"probed"` // P1 未接入 RBL 时为 false
}

type DNSLeakDetail struct {
	DNSLeakSuspected bool `json:"dns_leak_suspected"`
}

type PurityReport struct {
	Rank  int    `json:"rank"`
	Name  string `json:"name"`
	IP    string `json:"ip"`
	Score int    `json:"score"`

	ScoreFormula   ScoreFormula  `json:"score_formula"`
	Purity         PurityDetail  `json:"purity"`
	Recommendation string        `json:"recommendation"`
	IPType         string        `json:"ip_type"`
	Profile        ProfileDetail `json:"profile"`

	// 兼容旧字段（老调用方不受影响）
	ASN                   int      `json:"asn"`
	AsOrganization        string   `json:"asOrganization"`
	Country               string   `json:"country"`
	CountryCode           string   `json:"countryCode"`
	Region                string   `json:"region"`
	City                  string   `json:"city"`
	ISP                   string   `json:"isp"`
	FraudScore            int      `json:"fraudScore"`
	IPPureCoefficient     int      `json:"ippureCoefficient"`
	CloudflareCoefficient int      `json:"cloudflareCoefficient"`
	RiskLevel             string   `json:"riskLevel"`
	IPSource              string   `json:"ipSource"`
	IPProperties          []string `json:"ipProperties"`
	IsDataCenter          bool     `json:"isDataCenter"`
	IsResidential         bool     `json:"isResidential"`
	IsBroadcast           bool     `json:"isBroadcast"`
	Source                string   `json:"source"`

	// 新维度
	Dimensions  map[string]Dimension `json:"dimensions"`
	Stability   StabilityDetail      `json:"stability"`
	RBL         RBLDetail            `json:"rbl"`
	DNSLeak     DNSLeakDetail        `json:"dns_leak"`
	MainReasons []string             `json:"main_reasons"`
}

// ============ 类型上限 ============

func typeCapOf(t string) int {
	switch t {
	case "hosting":
		return 78
	case "business":
		return 82
	case "mobile":
		return 92
	case "isp":
		return 90
	case "residential":
		return 95
	default:
		return 100
	}
}

func typeLabelOf(t string) string {
	switch t {
	case "hosting":
		return "机房IP"
	case "business":
		return "商业IP"
	case "mobile":
		return "移动网络IP"
	case "isp":
		return "运营商IP"
	case "residential":
		return "住宅IP"
	default:
		return "IP类型未知"
	}
}

func toneOf(t string) string {
	switch t {
	case "hosting", "business":
		return "bad"
	case "isp", "mobile":
		return "warn"
	case "residential":
		return "good"
	default:
		return "neutral"
	}
}

// ============ 评分主函数 ============

func buildPurityReport(ip string, agg *Aggregated, rbl *RBLResult, stab *StabilityResult) *PurityReport {
	r := &PurityReport{IP: ip}
	r.Profile.DefinitionNote = "IP来源、IP属性、接入网络、代理状态和邮件黑名单是五个独立维度，不能互相替代。"

	// ---- 基础字段 ----
	r.ASN = agg.ASN
	r.AsOrganization = firstNonEmpty(agg.ASOrg, agg.Org)
	r.Country = agg.Country
	r.CountryCode = agg.CountryCode
	r.Region = agg.Region
	r.City = agg.City
	r.ISP = agg.ISP
	r.Source = strings.Join(agg.Sources, ",")
	r.IPProperties = ipPropertiesOf(ip, agg.ASN, agg.Org)
	r.IsDataCenter = agg.Type == "hosting"
	r.IsResidential = agg.Type == "residential"
	r.IsBroadcast = isBroadcastIP(ip)
	r.IPSource = ipSourceOf(agg.ASN, ip, agg.Org)

	// ---- 类型判定 ----
	ipType := agg.Type
	if ipType == "" || ipType == "unknown" {
		// 退化：用现有静态判定兜底
		if isDataCenterIP(agg.ASN, ip, agg.Org) {
			ipType = "hosting"
		} else if agg.ASN != 0 {
			ipType = "isp"
		}
	}
	r.IPType = typeLabelOf(ipType)
	typeCap := typeCapOf(ipType)
	typeTone := toneOf(ipType)
	typeConflict := agg.TypeSources > 0 && agg.TypeMajority < agg.TypeSources

	// ---- signal_risk（公开风险信号） ----
	var reasons []string
	signalRisk := 0
	if agg.ProxySignal {
		if agg.ProxySignalCount >= 2 {
			signalRisk += 26
			reasons = append(reasons, "检测到代理/VPN 出口风险信号（多源共识）")
		} else {
			signalRisk += 13
			reasons = append(reasons, "检测到代理/VPN 出口风险信号（单源，已减半扣分）")
		}
	}
	if typeConflict {
		signalRisk += 10
		reasons = append(reasons, "住宅、商业或机房属性判断存在明显冲突")
	}
	// RBL 邮件黑名单命中（P2）
	if rbl != nil && rbl.Probed && (rbl.ListedCount > 0 || rbl.NetworkListedCount > 0) {
		signalRisk += rblSignalRisk(rbl)
		if rbl.ListedCount > 0 {
			reasons = append(reasons, fmt.Sprintf("命中 %d 个邮件黑名单（%s）", rbl.ListedCount, strings.Join(rbl.ListedZones, "、")))
		}
		if rbl.NetworkListedCount > 0 {
			reasons = append(reasons, fmt.Sprintf("上游网段命中 %d 个黑名单（%s）", rbl.NetworkListedCount, strings.Join(rbl.NetworkZones, "、")))
		}
	}
	if rbl != nil && rbl.Probed && rbl.WhiteListed {
		reasons = append(reasons, "命中邮件白名单（list.dnswl.org）")
	}

	// ---- identity_risk（归属一致性风险） ----
	identityRisk := 0
	if agg.CountryOutliers >= 2 {
		identityRisk += 8
		reasons = append(reasons, "定位国家存在少数离群结果")
	} else if agg.CountryOutliers == 1 {
		identityRisk += 4
	}
	if agg.ASNOutliers >= 2 {
		identityRisk += 8
		reasons = append(reasons, "ASN 归属缺少稳定共识")
	} else if agg.ASNOutliers == 1 {
		identityRisk += 4
	}
	if agg.CityConflict {
		identityRisk += 6
		reasons = append(reasons, "城市库差异")
	}
	if agg.RDAPRegistered {
		if agg.RDAPCountry != "" && agg.CountryCode != "" && agg.RDAPCountry != agg.CountryCode {
			identityRisk += 6
			reasons = append(reasons, "注册国与定位国不一致，疑似广播IP")
		}
	} else {
		identityRisk += 8
		reasons = append(reasons, "RIR 未提供注册国，无法区分原生IP和广播IP")
	}

	// ---- 覆盖度 ----
	okCount := len(agg.Sources)
	missing := agg.ExpectedSource - okCount
	if missing < 0 {
		missing = 0
	}
	coverageCeiling := 97 - missing*3
	if coverageCeiling < 80 {
		coverageCeiling = 80
	}
	if missing > 0 {
		reasons = append(reasons, fmt.Sprintf("部分数据源不可用，覆盖扣分 %d（%s）", missing*3, strings.Join(agg.FailedSources, ",")))
	}
	uncertaintyDeduction := 0
	if coverageCeiling < 97 {
		uncertaintyDeduction = 97 - coverageCeiling
	}

	// ---- 风险值 → 纯净度 ----
	riskValue := signalRisk + identityRisk
	raw := 100 - riskValue

	// 公开风险对类型上限的压制（封顶 26）
	adjustment := signalRisk
	if adjustment > 26 {
		adjustment = 26
	}
	effectiveCap := typeCap - adjustment
	if effectiveCap > coverageCeiling {
		effectiveCap = coverageCeiling
	}
	purityScore := raw
	if purityScore > effectiveCap {
		purityScore = effectiveCap
	}
	if purityScore < 0 {
		purityScore = 0
	}
	if effectiveCap < typeCap {
		reasons = append(reasons, fmt.Sprintf("%s基础上限 %d；公开风险 %d 后有效上限 %d", typeLabelOf(ipType), typeCap, adjustment, effectiveCap))
	} else if typeCap < 100 {
		reasons = append(reasons, fmt.Sprintf("%s基础上限 %d", typeLabelOf(ipType), typeCap))
	}

	// ---- 等级/置信度 ----
	label, tone := purityLabelOf(purityScore)
	confidence, confidenceLabel := confidenceOf(okCount)

	// ---- 数据质量与稳定性 ----
	dataQuality := int(float64(okCount)/float64(agg.ExpectedSource)*10 + 0.5)
	if dataQuality > 10 {
		dataQuality = 10
	}
	stabilityScore := stabilityDimensionScore(stab)
	rblScore := rblDimensionScore(rbl)
	rblChecked := 0
	if rbl != nil {
		rblChecked = rbl.CheckedCount
	}

	// ---- 综合分 ----
	score := int(float64(purityScore)*0.85 + float64(stabilityScore)*0.10 + float64(dataQuality)*0.05 + 0.5)

	r.Score = score
	r.ScoreFormula = ScoreFormula{PurityWeight: 85, StabilityWeight: 10, DataQualityWeight: 5, ClassificationCap: typeCap}
	r.Purity = PurityDetail{
		Score: purityScore, RiskValue: riskValue, SignalRisk: signalRisk, IdentityRisk: identityRisk,
		UncertaintyDeduction: uncertaintyDeduction, CoverageCeiling: coverageCeiling,
		TypeCap: typeCap, EffectiveTypeCap: effectiveCap, CapSignalAdjustment: adjustment,
		TypeCapDeduction: typeCap - effectiveCap,
		Label:            label, Tone: tone, Confidence: confidence, ConfidenceLabel: confidenceLabel,
		EvidenceSourceCount: okCount, RblCheckedCount: rblChecked,
		Definition: "综合衡量代理与滥用信誉、黑名单、归属稳定性、IP 属性与类型置信度；机房、商业、运营商、类型冲突和证据不足会设置可解释的分数上限",
		Reasons:    reasons,
	}

	// ---- 建议 ----
	r.Recommendation = recommendationOf(score)

	// ---- 画像 ----
	r.Profile = buildProfile(agg, ipType, typeTone, typeConflict, signalRisk, identityRisk, okCount)

	// ---- 维度 ----
	consistencyScore := 20 - (consistencyDeduction(agg))
	if consistencyScore < 0 {
		consistencyScore = 0
	}
	repScore := 35 - signalRisk
	if repScore < 0 {
		repScore = 0
	}
	if repScore > 35 {
		repScore = 35
	}
	r.Dimensions = map[string]Dimension{
		"reputation":   {Score: repScore, MaxScore: 35},
		"consistency":  {Score: consistencyScore, MaxScore: 20},
		"rbl":          {Score: rblScore, MaxScore: 20},
		"stability":    {Score: stabilityScore, MaxScore: 15},
		"data_quality": {Score: dataQuality, MaxScore: 10},
	}
	r.Stability = stabilityToDetail(stab)
	r.RBL = rblToDetail(rbl)
	r.DNSLeak = DNSLeakDetail{DNSLeakSuspected: false}
	r.MainReasons = reasons

	// ---- 兼容旧字段 ----
	r.FraudScore = 100 - purityScore
	r.IPPureCoefficient = r.FraudScore
	r.CloudflareCoefficient = calculateCloudflareCoefficient(r.FraudScore, agg.ASN, agg.CountryCode)
	r.RiskLevel = riskLevelOf(r.FraudScore)
	return r
}

// ============ 辅助函数 ============

func purityLabelOf(score int) (string, string) {
	switch {
	case score >= 85:
		return "优秀", "good"
	case score >= 65:
		return "良好", "good"
	case score >= 40:
		return "一般风险", "warn"
	default:
		return "高风险", "bad"
	}
}

func confidenceOf(okCount int) (string, string) {
	switch {
	case okCount >= 6:
		return "high", "证据充分"
	case okCount >= 4:
		return "medium", "证据一般"
	default:
		return "low", "证据较少"
	}
}

func recommendationOf(score int) string {
	switch {
	case score >= 85:
		return "信誉良好，适合多数常规用途"
	case score >= 65:
		return "信誉良好，建议按目标场景复核"
	case score >= 40:
		return "存在风险信号，建议复核后再决定用途"
	default:
		return "风险较高，不建议用于注册、支付等敏感操作"
	}
}

func consistencyDeduction(agg *Aggregated) int {
	d := 0
	if agg.CountryOutliers >= 2 {
		d += 6
	} else if agg.CountryOutliers == 1 {
		d += 3
	}
	if agg.ASNOutliers >= 2 {
		d += 6
	} else if agg.ASNOutliers == 1 {
		d += 3
	}
	if agg.CityConflict {
		d += 4
	}
	if agg.RDAPRegistered && agg.RDAPCountry != "" && agg.CountryCode != "" && agg.RDAPCountry != agg.CountryCode {
		d += 3
	}
	return d
}

func buildProfile(agg *Aggregated, ipType, typeTone string, typeConflict bool, signalRisk, identityRisk, okCount int) ProfileDetail {
	p := ProfileDetail{
		Primary:      typeLabelOf(ipType),
		PrimaryTone:  typeTone,
		SourceCount:  okCount,
		TypedSources: agg.TypeSources,
	}

	// 来源判定（原生/广播/待确认）
	switch {
	case !agg.RDAPRegistered:
		p.Native, p.NativeTone = "IP来源待确认", "warn"
	case agg.RDAPCountry != "" && agg.CountryCode != "" && agg.RDAPCountry == agg.CountryCode:
		p.Native, p.NativeTone = "原生IP", "good"
	default:
		p.Native, p.NativeTone = "广播IP（注册国与定位国不一致）", "warn"
	}

	// 代理状态
	if agg.ProxySignal {
		if agg.ProxySignalCount >= 2 {
			p.Risk, p.RiskTone = "代理/VPN出口风险（多源共识）", "bad"
		} else {
			p.Risk, p.RiskTone = "代理/VPN出口风险（单源）", "warn"
		}
	} else {
		p.Risk, p.RiskTone = "未检出明显匿名风险", "good"
	}

	// summary 拼接
	blacklistText := "未检出滥用记录"
	p.Summary = fmt.Sprintf("IP来源：%s；IP属性：%s；接入网络：%s；代理状态：%s；黑名单：%s。",
		p.Native, p.Primary, accessNetworkLabel(ipType), p.Risk, blacklistText)

	// tags
	p.Tags = append(p.Tags, Tag{p.Native, p.NativeTone})
	p.Tags = append(p.Tags, Tag{p.Primary, p.PrimaryTone})
	if agg.ProxySignal {
		p.Tags = append(p.Tags, Tag{p.Risk, p.RiskTone})
	}
	if agg.CityConflict {
		p.Tags = append(p.Tags, Tag{"城市库差异", "warn"})
	}
	if typeConflict {
		p.Tags = append(p.Tags, Tag{"线路类型有冲突", "warn"})
	}

	// 各维度
	networkDetail := fmt.Sprintf("%d/%d 个类型源结论一致", agg.TypeMajority, agg.TypeSources)
	if agg.TypeSources == 0 {
		networkDetail = "类型源均不可用，按 ASN/组织特征兜底判断"
	}
	if typeConflict {
		networkDetail += "，存在结论冲突"
	}
	p.AccessNetwork = DimItem{Key: accessNetworkKey(ipType), Label: accessNetworkLabel(ipType), Tone: typeTone, Detail: networkDetail}
	p.Network = DimItem{Key: accessNetworkKey(ipType), Label: p.Primary, BaseLabel: p.Primary, Tone: typeTone, Confidence: "high", Detail: networkDetail, Conflict: typeConflict}
	p.Privacy = DimItem{Key: privacyKey(agg.ProxySignal, agg.ProxySignalCount), Label: p.Risk, Tone: p.RiskTone, Confidence: privacyConfidence(agg.ProxySignalCount), Detail: privacyDetail(agg.ProxySignalCount)}
	p.Reputation = DimItem{Key: reputationKey(signalRisk), Label: reputationLabel(signalRisk), Tone: reputationTone(signalRisk), Detail: reputationDetail(signalRisk)}
	p.Classification = map[string]DimItem{
		"ip_source":      {Label: p.Native, Tone: p.NativeTone, Detail: classificationSourceDetail(agg)},
		"ip_attribute":   {Label: p.Primary, Tone: p.PrimaryTone, Detail: networkDetail},
		"access_network": {Label: p.AccessNetwork.Label, Tone: typeTone, Detail: networkDetail},
		"proxy_status":   {Label: p.Risk, Tone: p.RiskTone, Detail: privacyDetail(agg.ProxySignalCount)},
		"blacklist":      {Label: "未检出滥用记录（P2 接入 RBL 后启用）", Tone: "good", Detail: "邮件黑名单检测将在下一阶段启用"},
	}
	return p
}

func accessNetworkKey(t string) string {
	switch t {
	case "hosting":
		return "hosting"
	case "business":
		return "business"
	case "isp", "mobile":
		return "isp"
	case "residential":
		return "residential"
	default:
		return "unknown"
	}
}

func accessNetworkLabel(t string) string {
	switch t {
	case "hosting":
		return "数据中心"
	case "business":
		return "商业网络"
	case "isp":
		return "运营商网络"
	case "mobile":
		return "移动网络"
	case "residential":
		return "住宅网络"
	default:
		return "未知"
	}
}

func privacyKey(signal bool, count int) string {
	if !signal {
		return "clean"
	}
	if count >= 2 {
		return "proxy_confirmed"
	}
	return "proxy_suspected"
}

func privacyConfidence(count int) string {
	if count >= 2 {
		return "high"
	}
	return "low"
}

func privacyDetail(count int) string {
	switch count {
	case 0:
		return "未检出代理/VPN/Tor 信号"
	case 1:
		return "仅 1 个检测源命中，需要其他来源复核"
	default:
		return fmt.Sprintf("%d 个检测源一致命中", count)
	}
}

func reputationKey(signalRisk int) string {
	switch {
	case signalRisk >= 20:
		return "high"
	case signalRisk >= 10:
		return "medium"
	default:
		return "low"
	}
}

func reputationLabel(signalRisk int) string {
	switch {
	case signalRisk >= 20:
		return "存在明显风险信号"
	case signalRisk >= 10:
		return "存在风险信号"
	default:
		return "信誉良好"
	}
}

func reputationTone(signalRisk int) string {
	if signalRisk >= 20 {
		return "bad"
	}
	if signalRisk >= 10 {
		return "warn"
	}
	return "good"
}

func reputationDetail(signalRisk int) string {
	switch {
	case signalRisk >= 20:
		return "多源检出代理或滥用相关信号"
	case signalRisk >= 10:
		return "检出少量风险信号，需复核"
	default:
		return "公开数据源未检出明显风险"
	}
}

func classificationSourceDetail(agg *Aggregated) string {
	parts := []string{}
	if agg.CountrySources > 0 {
		parts = append(parts, fmt.Sprintf("%d个定位源国家一致", agg.CountryMajority))
	}
	if agg.ASNSources > 0 {
		parts = append(parts, fmt.Sprintf("%d个源 ASN 一致", agg.ASNMajority))
	}
	if !agg.RDAPRegistered {
		parts = append(parts, "RIR 未提供注册国，无法区分原生IP和广播IP")
	}
	if len(parts) == 0 {
		return "数据源不足，无法判断来源"
	}
	return strings.Join(parts, "；")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ============ CLI 格式化输出 ============

func formatPurityReport(r *PurityReport) string {
	var b strings.Builder
	b.WriteString("IP 纯净度检测: " + r.IP + "\n")
	b.WriteString(strings.Repeat("─", 46) + "\n")
	rblText := "未检出"
	if !r.RBL.Probed {
		rblText = "未启用"
	} else if r.RBL.QueryLimited {
		rblText = "查询受限"
	} else if r.RBL.ListedCount > 0 || r.RBL.NetworkListedCount > 0 {
		rblText = fmt.Sprintf("%d 命中 / %d 网段命中", r.RBL.ListedCount, r.RBL.NetworkListedCount)
	}
	latencyText := "—"
	if r.Stability.Probed && r.Stability.P95LatencyMs > 0 {
		latencyText = fmt.Sprintf("%.2f ms（成功 %.0f%%）", r.Stability.P95LatencyMs, r.Stability.SuccessRate*100)
	}
	rows := [][2]string{
		{"综合质量分", fmt.Sprintf("%d / 100  (纯净度85%% 网络10%% 数据5%%)", r.Score)},
		{"IP 纯净度", fmt.Sprintf("%d · %s（%s）", r.Purity.Score, r.Purity.Label, r.Purity.ConfidenceLabel)},
		{"IP 类型", fmt.Sprintf("%s（%d/%d 类型源一致）", r.IPType, r.Profile.TypedSources, r.Profile.SourceCount)},
		{"归属地", strings.TrimSpace(r.Country + " " + r.Region + " " + r.City)},
		{"ASN", r.AsOrganization},
		{"公开风险", fmt.Sprintf("%d", r.Purity.SignalRisk)},
		{"归属风险", fmt.Sprintf("%d", r.Purity.IdentityRisk)},
		{"RBL 黑名单", rblText},
		{"P95 延迟", latencyText},
		{"覆盖扣分", fmt.Sprintf("%d（覆盖上限 %d）", r.Purity.UncertaintyDeduction, r.Purity.CoverageCeiling)},
		{"数据来源", r.Source},
		{"建议", r.Recommendation},
	}
	if r.CountryCode != "" {
		rows[3] = [2]string{"归属地", strings.TrimSpace(r.Country+" "+r.Region+" "+r.City) + " (" + r.CountryCode + ")"}
	}
	maxW := 0
	for _, row := range rows {
		if w := displayWidth(row[0]); w > maxW {
			maxW = w
		}
	}
	for _, row := range rows {
		if row[1] == "" {
			continue
		}
		b.WriteString(padKey(row[0], maxW+2) + ": " + row[1] + "\n")
	}
	b.WriteString(strings.Repeat("─", 46) + "\n")
	b.WriteString("原因:\n")
	for _, reason := range r.MainReasons {
		b.WriteString("  · " + reason + "\n")
	}
	b.WriteString(strings.Repeat("─", 46))
	return b.String()
}

// ============ 批量检测 ============

type purityCheckRequest struct {
	IPs []string `json:"ips"`
}

type purityCheckResponse struct {
	OK          bool            `json:"ok"`
	RunID       string          `json:"run_id"`
	InputErrors []inputError    `json:"input_errors"`
	RunErrors   []string        `json:"run_errors"`
	Reports     []*PurityReport `json:"reports"`
}

type inputError struct {
	Input string `json:"input"`
	Error string `json:"error"`
}

const maxBatchIPs = 10

func newRunID() string {
	return "web-" + time.Now().UTC().Format("20060102T150405.000") + "Z"
}

// purityCheck 批量检测（内部并发 3）
func purityCheck(ips []string) purityCheckResponse {
	resp := purityCheckResponse{OK: true, RunID: newRunID()}
	var valid []string
	seen := map[string]bool{}
	for _, raw := range ips {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		if net.ParseIP(ip) == nil {
			resp.InputErrors = append(resp.InputErrors, inputError{Input: raw, Error: "invalid IP address"})
			continue
		}
		if !seen[ip] {
			seen[ip] = true
			valid = append(valid, ip)
		}
	}
	if len(valid) == 0 {
		resp.OK = false
		return resp
	}
	if len(valid) > maxBatchIPs {
		valid = valid[:maxBatchIPs]
	}

	sem := make(chan struct{}, 3)
	type result struct {
		ip  string
		rep *PurityReport
	}
	results := make([]result, len(valid))
	var wg sync.WaitGroup
	for i, ip := range valid {
		wg.Add(1)
		go func(idx int, ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			agg, rbl, stab := queryPurityInputs(ip)
			if agg == nil {
				results[idx] = result{ip, nil}
				return
			}
			rep := buildPurityReport(ip, agg, rbl, stab)
			results[idx] = result{ip, rep}
		}(i, ip)
	}
	wg.Wait()

	// 按分数降序排 rank
	var reports []*PurityReport
	for _, res := range results {
		if res.rep != nil {
			reports = append(reports, res.rep)
		} else {
			resp.RunErrors = append(resp.RunErrors, res.ip+": 数据源全部不可用")
		}
	}
	sortReportsByScoreDesc(reports)
	for i, rep := range reports {
		rep.Rank = i + 1
		rep.Name = fmt.Sprintf("ip-%d-%s", i+1, strings.ReplaceAll(rep.IP, ":", "_"))
	}
	resp.Reports = reports
	return resp
}

// sortReportsByScoreDesc 按综合分降序（同分按 IP 稳定排序）
func sortReportsByScoreDesc(reports []*PurityReport) {
	sort.SliceStable(reports, func(i, j int) bool {
		if reports[i].Score != reports[j].Score {
			return reports[i].Score > reports[j].Score
		}
		return reports[i].IP < reports[j].IP
	})
}

// stabilityToDetail 稳定性结果 → 响应结构
func stabilityToDetail(s *StabilityResult) StabilityDetail {
	if s == nil {
		return StabilityDetail{Probed: false}
	}
	return StabilityDetail{
		SuccessRate:  s.SuccessRate,
		AvgLatencyMs: s.AvgLatencyMs,
		P50LatencyMs: s.P50LatencyMs,
		P95LatencyMs: s.P95LatencyMs,
		TimeoutCount: s.TimeoutCount,
		Probed:       s.Probed,
	}
}

// rblToDetail RBL 结果 → 响应结构
func rblToDetail(r *RBLResult) RBLDetail {
	if r == nil {
		return RBLDetail{RiskLevel: "unknown", Probed: false}
	}
	return RBLDetail{
		ListedCount:        r.ListedCount,
		NetworkListedCount: r.NetworkListedCount,
		RiskLevel:          r.RiskLevel,
		QueryLimited:       r.QueryLimited,
		Probed:             r.Probed,
	}
}

// queryPurityInputs 并行查询三大数据源（聚合 / RBL / 稳定性），各自带缓存
func queryPurityInputs(ip string) (*Aggregated, *RBLResult, *StabilityResult) {
	var agg *Aggregated
	var rbl *RBLResult
	var stab *StabilityResult
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); agg = queryAggregated(ip) }()
	go func() { defer wg.Done(); rbl = queryRBL(ip) }()
	go func() { defer wg.Done(); stab = queryStability(ip) }()
	wg.Wait()
	return agg, rbl, stab
}
