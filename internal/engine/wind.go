// Package engine 承载全部纯判定逻辑：风载、张力、断丝、倾斜、
// 开放等级合成与季节结冰收紧规则。不依赖存储与网络，便于独立验证。
package engine

import "fmt"

// beaufortBounds 蒲福风级上边界（m/s），索引即风级，最后一档为 12 级开放上界。
var beaufortBounds = [13]float64{
	0.3, 1.6, 3.4, 5.5, 8.0, 10.8, 13.9, 17.2, 20.8, 24.5, 28.5, 32.7, 1 << 30,
}

// beaufortNames 各风级中文描述。
var beaufortNames = [13]string{
	"无风", "软风", "轻风", "微风", "和风", "清劲风",
	"强风", "疾风", "大风", "烈风", "狂风", "暴风", "飓风",
}

// BeaufortScale 将风速（m/s）映射为蒲福风级 0-12。
func BeaufortScale(windMS float64) int {
	scale := 0
	for i, bound := range beaufortBounds {
		if windMS < bound {
			scale = i
			break
		}
		scale = i
	}
	return scale
}

// BeaufortName 返回风级中文描述；越界返回占位文本。
func BeaufortName(scale int) string {
	if scale < 0 || scale > 12 {
		return "未知"
	}
	return beaufortNames[scale]
}

// WindVerdict 风载维度判定结论。
type WindVerdict struct {
	SpeedMS    float64 `json:"speed_ms"`
	Scale      int     `json:"beaufort_scale"`
	Name       string  `json:"beaufort_name"`
	Restricted bool    `json:"restricted"`
	Critical   bool    `json:"critical"`
	Detail     string  `json:"detail"`
}

// WindThresholds 风载判据：达到 restrictedScale 收紧运行，
// 达到 criticalScale 触发 closed 级处置。
type WindThresholds struct {
	RestrictedScale int
	CriticalScale   int
}

// DefaultWindThresholds 客运索道常用判据：8 级大风限行，10 级烈风停运。
func DefaultWindThresholds() WindThresholds {
	return WindThresholds{RestrictedScale: 8, CriticalScale: 10}
}

// EvaluateWind 综合蒲福分级与阈值给出风载结论。
func EvaluateWind(windMS float64, th WindThresholds) WindVerdict {
	scale := BeaufortScale(windMS)
	v := WindVerdict{SpeedMS: windMS, Scale: scale, Name: BeaufortName(scale)}
	switch {
	case scale >= th.CriticalScale:
		v.Critical = true
		v.Detail = fmt.Sprintf("wind scale %d (%s) >= critical %d", scale, v.Name, th.CriticalScale)
	case scale >= th.RestrictedScale:
		v.Restricted = true
		v.Detail = fmt.Sprintf("wind scale %d (%s) >= restricted %d", scale, v.Name, th.RestrictedScale)
	default:
		v.Detail = fmt.Sprintf("wind scale %d (%s) within limits", scale, v.Name)
	}
	return v
}
