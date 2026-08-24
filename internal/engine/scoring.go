package engine

import (
	"fmt"
	"strings"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// StructureInputs 结构维度评估的输入集合。
type StructureInputs struct {
	WorstTiltRatio  float64             // 全线最差倾斜占比（无数据为 0）
	WorstVibration  model.AlertSeverity // 窗口内最重振动级别（空表示无事件）
	VibrationCount  int                 // 窗口内振动事件数
	MinCabinGapM    float64             // 窗口内最小车厢间距（0 表示无观测）
	RatedSpeedMS    float64             // 线路额定速度，用于间距判据
	InspectionCount int                 // 窗口内巡检次数（正向项）
}

// StructureVerdict 结构维度结论。
type StructureVerdict struct {
	Score  float64 `json:"score"`
	Level  Level   `json:"level"`
	Detail string  `json:"detail"`
}

// EvaluateStructure 综合倾斜、振动、车间距与巡检覆盖给出结构分。
// 判据：
//   - 倾斜占比 ≥0.8 → restricted；≥1.2 → critical；
//   - critical 振动 ≥3 次 → critical；1-2 次 → restricted；
//   - 车间距低于额定速度对应安全距离（speed×2 秒）→ restricted。
func EvaluateStructure(in StructureInputs) StructureVerdict {
	level := LevelOk
	details := []string{}

	switch {
	case in.WorstTiltRatio >= 1.2:
		level = LevelCritical
		details = append(details, fmt.Sprintf("worst tilt ratio %.2f", in.WorstTiltRatio))
	case in.WorstTiltRatio >= 0.8:
		if level == LevelOk {
			level = LevelRestricted
		}
		details = append(details, fmt.Sprintf("worst tilt ratio %.2f", in.WorstTiltRatio))
	}

	switch vibrationLoad(in) {
	case 2:
		level = LevelCritical
		details = append(details, "repeated critical vibration")
	case 1:
		if level == LevelOk {
			level = LevelRestricted
		}
		details = append(details, "critical vibration observed")
	case 0:
		if in.VibrationCount > 0 && level == LevelOk {
			level = LevelRestricted
			details = append(details, "warning vibration events")
		}
	}

	if in.RatedSpeedMS > 0 && in.MinCabinGapM > 0 {
		safeGap := in.RatedSpeedMS * 2.0
		if in.MinCabinGapM < safeGap {
			if level == LevelOk {
				level = LevelRestricted
			}
			details = append(details, fmt.Sprintf("min cabin gap %.1fm below safe %.1fm", in.MinCabinGapM, safeGap))
		}
	}

	score := structureScore(level, in.InspectionCount)
	return StructureVerdict{Score: score, Level: level, Detail: strings.Join(details, "; ")}
}

// vibrationLoad 把振动事件压缩成三档负载：2=重复 critical，1=critical，0=warning 及以下。
func vibrationLoad(in StructureInputs) int {
	if in.WorstVibration == "" {
		return -1
	}
	if in.WorstVibration != model.SeverityCritical {
		return 0
	}
	if in.VibrationCount >= 3 {
		return 2
	}
	return 1
}

func structureScore(level Level, inspections int) float64 {
	base := map[Level]float64{
		LevelCritical:   15,
		LevelRestricted: 55,
		LevelOk:         95,
	}[level]
	bonus := float64(inspections) * 1.5
	if bonus > 5 {
		bonus = 5
	}
	score := base + bonus
	if score > 100 {
		score = 100
	}
	return score
}
