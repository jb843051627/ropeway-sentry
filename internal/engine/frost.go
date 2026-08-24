package engine

import (
	"fmt"
	"math"
)

// FrostVerdict 结冰风险等级三档。
type FrostVerdictLevel string

// 低 / 中 / 高 三档结冰风险。
const (
	FrostLow      FrostVerdictLevel = "low"
	FrostModerate FrostVerdictLevel = "moderate"
	FrostHigh     FrostVerdictLevel = "high"
)

// FrostInput 结冰风险合成的气象观测输入：
// 气温、相对湿度与风速共同决定覆冰沉积概率。
type FrostInput struct {
	TempC       float64 `json:"temp_c"`
	HumidityPct float64 `json:"humidity_pct"`
	WindMS      float64 `json:"wind_ms"`
}

// Validate 输入量纲校验：温度摄氏度、湿度百分比、风速米每秒。
func (in FrostInput) Validate() error {
	if in.TempC < -60 || in.TempC > 60 {
		return fmt.Errorf("temperature %.1fC out of plausible range", in.TempC)
	}
	if in.HumidityPct < 0 || in.HumidityPct > 100 {
		return fmt.Errorf("humidity %.1f%% out of range [0,100]", in.HumidityPct)
	}
	if in.WindMS < 0 || in.WindMS > 75 {
		return fmt.Errorf("wind speed %.2f m/s out of plausible range", in.WindMS)
	}
	return nil
}

// FrostVerdict 结冰风险合成结论，Score 取值 [0,100]。
type FrostVerdict struct {
	Score float64           `json:"score"`
	Level FrostVerdictLevel `json:"level"`
	// WinterMarginApplied 冬季裕度系数是否已叠加。
	WinterMarginApplied bool   `json:"winter_margin_applied"`
	Detail              string `json:"detail"`
}

// clamp 把 v 收敛到 [lo,hi] 区间。
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// humidityFactor 湿度贡献：低于 70% 视为干燥无贡献，饱和时取满。
func humidityFactor(rh float64) float64 {
	return clamp((rh-70)/30, 0, 1)
}

// freezeBandFactor 温度贡献：以 0°C 为中心的正态型权重，
// 距冰点越远（深冻或回暖）覆冰概率越低。
func freezeBandFactor(tempC float64) float64 {
	sigma := 4.0
	return math.Exp(-(tempC * tempC) / (2 * sigma * sigma))
}

// calmWindFactor 风速贡献：静风利于过冷水附着，
// 风速越大冲刷越强；12 m/s 以上视为基本不积冰。
func calmWindFactor(windMS float64) float64 {
	return clamp(1-windMS/12, 0, 1)
}

// EvaluateFrost 结冰风险纯函数：温度+湿度+风速加权合成，
// winterMargin 为冬季裕度系数（>=1 时放大分数，提前触发高档预警）。
func EvaluateFrost(in FrostInput, winterMargin float64) FrostVerdict {
	score := 100*0.50*humidityFactor(in.HumidityPct) +
		100*0.35*freezeBandFactor(in.TempC) +
		100*0.15*calmWindFactor(in.WindMS)
	marginApplied := false
	if winterMargin > 1 {
		score *= winterMargin
		marginApplied = true
	}
	score = clamp(score, 0, 100)
	level := FrostLow
	switch {
	case score >= 65:
		level = FrostHigh
	case score >= 35:
		level = FrostModerate
	}
	detail := fmt.Sprintf("frost score %.0f (%s): temp %.1fC rh %.0f%% wind %.1fm/s",
		score, level, in.TempC, in.HumidityPct, in.WindMS)
	if marginApplied {
		detail += fmt.Sprintf(", winter margin x%.2f", winterMargin)
	}
	return FrostVerdict{Score: score, Level: level, WinterMarginApplied: marginApplied, Detail: detail}
}

// FrostAssessor 结冰风险评估接口：评估编排层依赖此抽象而非具体策略。
type FrostAssessor interface {
	AssessFrost(in FrostInput) FrostVerdict
}

// WinterFrostMargin 冬季裕度系数：结冰关注期内调整风险评分。
const WinterFrostMargin = 0.85

// AssessFrost 让季节策略实现 FrostAssessor：
// 结冰季生效时叠加冬季裕度系数，非结冰季按原始观测评分。
func (p IcingPolicy) AssessFrost(in FrostInput) FrostVerdict {
	margin := 1.0
	if p.Active {
		margin = WinterFrostMargin
	}
	verdict := EvaluateFrost(in, margin)
	if p.Active {
		verdict.Detail += " (winter season)"
	}
	return verdict
}
