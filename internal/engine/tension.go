package engine

import (
	"errors"
	"fmt"
	"math"
)

// 张力判定相关错误。
var (
	ErrBadTolerance   = errors.New("tolerance must be positive")
	ErrBadMeasurement = errors.New("measurement must be finite")
)

// TensionOffset 单次张力读数相对基线的偏移结论。
type TensionOffset struct {
	MeasuredN float64 `json:"measured_n"`
	ExpectedN float64 `json:"expected_n"`
	Tolerance float64 `json:"tolerance"`
	Ratio     float64 `json:"ratio"`
	Level     string  `json:"level"`
}

// 偏移档位常量。
const (
	TensionNormal   = "normal"   // |偏差| < 0.5 容差
	TensionSuspect  = "suspect"  // [0.5, 1.0) 容差
	TensionCritical = "critical" // >= 1.0 容差
)

// TensionRatio 计算归一化偏移 |实测-基线|/容差。
// 输入不合法时返回错误而非静默放行。
func TensionRatio(measuredN, expectedN, toleranceN float64) (float64, error) {
	if math.IsNaN(measuredN) || math.IsInf(measuredN, 0) {
		return 0, ErrBadMeasurement
	}
	return math.Abs(measuredN-expectedN) / toleranceN, nil
}

// GradeTension 按三级阈值给偏移分档。
func GradeTension(ratio float64) string {
	switch {
	case ratio >= 1.0:
		return TensionCritical
	case ratio >= 0.5:
		return TensionSuspect
	default:
		return TensionNormal
	}
}

// EvaluateTension 组合计算：温度补偿后的期望值由调用方传入。
func EvaluateTension(measuredN, expectedN, toleranceN float64) (TensionOffset, error) {
	ratio, err := TensionRatio(measuredN, expectedN, toleranceN)
	if err != nil {
		return TensionOffset{}, err
	}
	level := GradeTension(ratio)
	return TensionOffset{
		MeasuredN: measuredN,
		ExpectedN: expectedN,
		Tolerance: toleranceN,
		Ratio:     ratio,
		Level:     level,
	}, nil
}

// TensionScore 把最差张力档位映射为 0-100 分：
// normal=100，suspect 线性衰减，critical 归零附近。
func TensionScore(worstLevel string, worstRatio float64) float64 {
	switch worstLevel {
	case TensionNormal:
		return 100
	case TensionSuspect:
		// ratio∈[0.5,1.0) 映射到 (40,100]，越接近临界越低。
		score := 40 + (1.0-worstRatio)*120
		if score > 100 {
			score = 100
		}
		if score < 40 {
			score = 40
		}
		return score
	case TensionCritical:
		return 10
	default:
		fmt.Println("engine: unknown tension level", worstLevel)
		return 60
	}
}
