package engine

import (
	"fmt"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// WireSample 断丝滑动窗口的单个样本：时刻与累计断丝计数。
type WireSample struct {
	At         time.Time
	Cumulative int
}

// WireRate 在窗口内按首尾样本计算断丝增速（根/小时）。
// 样本不足两枚或时间跨度为零时返回 0（信息不足不作判定）。
func WireRate(samples []WireSample, window time.Duration, now time.Time) (float64, bool) {
	cutoff := now.Add(-window)
	firstIdx := -1
	last := WireSample{}
	for i, s := range samples {
		if s.At.Before(cutoff) {
			continue
		}
		if firstIdx == -1 {
			firstIdx = i
		}
		if s.At.After(last.At) || last.At.IsZero() {
			last = s
		}
	}
	if firstIdx == -1 {
		return 0, false
	}
	first := samples[firstIdx]
	spanHours := last.At.Sub(first.At).Hours()
	if spanHours <= 0 {
		return 0, false
	}
	delta := float64(last.Cumulative - first.Cumulative)
	if delta < 0 {
		// 计数回退说明传感器复位，本窗不计速率。
		return 0, true
	}
	return delta / spanHours, true
}

// WireVerdict 断丝速率判定结论。
type WireVerdict struct {
	RatePerHour float64 `json:"rate_per_hour"`
	Limit       float64 `json:"limit_per_hour"`
	Cumulative  int     `json:"cumulative"`
	Critical    bool    `json:"critical"`
	Suspect     bool    `json:"suspect"`
	Detail      string  `json:"detail"`
}

// WireThresholds 断丝判据：累计绝对数 + 滑动窗口速率双门限。
type WireThresholds struct {
	AbsoluteLimit    int
	RateLimitPerHour float64
}

// DefaultWireThresholds 常用判据：累计 20 根或 6 根/小时进入处置流程。
func DefaultWireThresholds() WireThresholds {
	return WireThresholds{AbsoluteLimit: 20, RateLimitPerHour: 6}
}

// EvaluateWires 结合累计计数与滑动窗口速率给出断丝结论。
func EvaluateWires(samples []WireSample, latestCumulative int, window time.Duration, now time.Time, th WireThresholds) WireVerdict {
	v := WireVerdict{Limit: th.RateLimitPerHour, Cumulative: latestCumulative}
	rate, ok := WireRate(samples, window, now)
	if !ok {
		v.Detail = "insufficient samples for rate estimation"
	} else {
		v.RatePerHour = rate
	}
	switch {
	case latestCumulative >= th.AbsoluteLimit:
		v.Critical = true
		v.Detail = fmt.Sprintf("cumulative %d >= absolute limit %d", latestCumulative, th.AbsoluteLimit)
	case v.RatePerHour > th.RateLimitPerHour:
		v.Critical = true
		v.Detail = fmt.Sprintf("rate %.2f/h exceeds limit %.2f/h", v.RatePerHour, th.RateLimitPerHour)
	case v.RatePerHour > th.RateLimitPerHour/2:
		v.Suspect = true
		v.Detail = fmt.Sprintf("rate %.2f/h above half of limit %.2f/h", v.RatePerHour, th.RateLimitPerHour)
	default:
		v.Detail = fmt.Sprintf("cumulative %d, rate %.2f/h nominal", latestCumulative, v.RatePerHour)
	}
	return v
}

// PointsToWireSamples 把断丝传感器的遥测点转换为滑动窗口样本，
// 只接受 good/suspect 质量；rejected 点会影响计数因此剔除。
func PointsToWireSamples(points []model.StoredPoint) []WireSample {
	out := make([]WireSample, 0, len(points))
	for _, p := range points {
		if p.Quality == model.QualityRejected {
			continue
		}
		out = append(out, WireSample{At: p.TakenAt, Cumulative: int(p.Value)})
	}
	return out
}
