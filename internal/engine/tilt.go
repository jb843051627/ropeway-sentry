package engine

import (
	"fmt"
)

// TiltVerdict 支架倾角越限结论。
type TiltVerdict struct {
	TowerID  int64   `json:"tower_id"`
	Code     string  `json:"code"`
	AngleDeg float64 `json:"angle_deg"`
	LimitDeg float64 `json:"limit_deg"`
	Ratio    float64 `json:"ratio"`
	Critical bool    `json:"critical"`
	Suspect  bool    `json:"suspect"`
	Detail   string  `json:"detail"`
}

// EvaluateTilt 支架倾斜越限判定：
// 占比（实测角/限值）≥0.8 进入受限区，≥1.2 判 critical。
// limitDeg 已含结冰季裕度收紧，由 IcingPolicy.TiltLimit 计算。
func EvaluateTilt(towerID int64, code string, angleDeg, limitDeg float64) (TiltVerdict, error) {
	if limitDeg <= 0 {
		return TiltVerdict{}, fmt.Errorf("tilt limit must be positive for tower %d", towerID)
	}
	ratio := angleDeg / limitDeg
	v := TiltVerdict{
		TowerID:  towerID,
		Code:     code,
		AngleDeg: angleDeg,
		LimitDeg: limitDeg,
		Ratio:    ratio,
	}
	switch {
	case ratio >= 1.2:
		v.Critical = true
		v.Detail = fmt.Sprintf("tilt %.2fdeg exceeds %.2fdeg limit by %.0f%%", angleDeg, limitDeg, (ratio-1)*100)
	case ratio >= 0.8:
		v.Suspect = true
		v.Detail = fmt.Sprintf("tilt %.2fdeg approaching %.2fdeg limit (%.0f%%)", angleDeg, limitDeg, ratio*100)
	default:
		v.Detail = fmt.Sprintf("tilt %.2fdeg within limit %.2fdeg", angleDeg, limitDeg)
	}
	return v, nil
}

// WorstTiltRatio 从一组结论中取最大占比；空集合返回 0。
func WorstTiltRatio(verdicts []TiltVerdict) float64 {
	worst := 0.0
	for _, v := range verdicts {
		if v.Ratio > worst {
			worst = v.Ratio
		}
	}
	return worst
}
