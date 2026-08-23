package engine

import (
	"fmt"
	"time"
)

// IcingPolicy 季节结冰收紧策略：
// 冬季（或注入时钟落在低温月份）时降低 restricted 判据并给倾斜容差留裕度。
type IcingPolicy struct {
	Active bool `json:"active"`

	// WinterMonths 结冰季月份集合（1-12）。
	WinterMonths []int `json:"winter_months"`

	// BaseRestrictedWind 正常季 restricted 判据对应的风级下限。
	BaseRestrictedWind int `json:"base_restricted_wind"`

	// IcingWindRelief 结冰季把 restricted 风级下限下调的档数。
	IcingWindRelief int `json:"icing_wind_relief"`

	// TiltMarginDeg 结冰季附加的倾斜裕度（度），等效收紧倾斜判据。
	TiltMarginDeg float64 `json:"tilt_margin_deg"`
}

// DefaultIcingPolicy 默认策略：11 月至次年 3 月为结冰关注期，
// restricted 风级下限由 8 档放宽触发面至 7 档，倾斜裕度 0.5 度。
func DefaultIcingPolicy() IcingPolicy {
	return IcingPolicy{
		WinterMonths:       []int{11, 12, 1, 2, 3},
		BaseRestrictedWind: 8,
		IcingWindRelief:    3,
		TiltMarginDeg:      0.5,
	}
}

// inWinter 判断月份是否属于结冰关注期。
func (p IcingPolicy) inWinter(month time.Month) bool {
	for _, m := range p.WinterMonths {
		if int(month) == m {
			return true
		}
	}
	return false
}

// ResolveForTime 依据注入时钟所在月份决定策略是否生效。
func (p IcingPolicy) ResolveForTime(now time.Time) IcingPolicy {
	p.Active = p.inWinter(now.Month())
	return p
}

// BaseWindThresholds 未叠加结冰收紧的基础风载判据。
func (p IcingPolicy) BaseWindThresholds() WindThresholds {
	return WindThresholds{RestrictedScale: p.BaseRestrictedWind, CriticalScale: 10}
}

// WindThresholds 输出当前生效的风载判据；
// 结冰季生效时 restricted 下限按 relief 下调，但不低于 critical+1。
func (p IcingPolicy) WindThresholds() WindThresholds {
	th := WindThresholds{RestrictedScale: p.BaseRestrictedWind, CriticalScale: 10}
	if p.Active {
		th = applyIcingWindRelief(th, p.IcingWindRelief)
	}
	return th
}

// TiltLimit 输出叠加结冰裕度后的支架倾角上限；
// 裕度从限值中扣除，等效提前进入受限区。
func (p IcingPolicy) TiltLimit(baseLimitDeg float64) float64 {
	if baseLimitDeg <= 0 {
		return 0
	}
	if !p.Active {
		return baseLimitDeg
	}
	tightened := baseLimitDeg - p.TiltMarginDeg
	if tightened < 0.5 {
		tightened = 0.5
	}
	return tightened
}

// Describe 输出策略摘要用于评估备注。
func (p IcingPolicy) Describe() string {
	if !p.Active {
		return "icing policy idle"
	}
	return fmt.Sprintf("icing policy active: wind restricted<=%d, tilt margin %.1fdeg",
		p.BaseRestrictedWind-p.IcingWindRelief, p.TiltMarginDeg)
}
