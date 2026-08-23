package engine

import (
	"fmt"
	"strings"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// Level 单维度的三档结论。
type Level string

// 三档维度结论。
const (
	LevelOk         Level = "ok"
	LevelRestricted Level = "restricted"
	LevelCritical   Level = "critical"
)

// Dimension 参与开放等级合成的单一维度。
type Dimension struct {
	Name   string `json:"name"`
	Level  Level  `json:"level"`
	Detail string `json:"detail"`
}

// Synthesize 开放等级合成规则（优先级从高到低）：
//  1. 任一维度 critical（critical 风速 / 张力 critical / 结构 red）→ closed；
//  2. 维护 active 锁存在 → maintenance（同线路至多一把，互斥语义由 service 保证）;
//  3. 任一维度 restricted → restricted；
//  4. 其余 → open。
func Synthesize(dims []Dimension, holdActive bool) model.LineStatus {
	for _, d := range dims {
		if d.Level == LevelCritical {
			return model.LineClosed
		}
	}
	for _, d := range dims {
		if d.Level == LevelRestricted {
			return model.LineRestricted
		}
	}
	if holdActive {
		return model.LineMaintenance
	}
	return model.LineOpen
}

// Explain 输出合成过程的人类可读说明，写入评估备注。
func Explain(dims []Dimension, holdActive bool, level model.LineStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "synthesized %s from", level)
	for _, d := range dims {
		fmt.Fprintf(&b, " %s=%s", d.Name, d.Level)
	}
	if holdActive {
		b.WriteString(" hold=active")
	}
	return b.String()
}

// WorstDimension 返回等级最高的维度名，便于告警定位。
func WorstDimension(dims []Dimension) (string, Level) {
	worstName := ""
	worst := LevelOk
	for _, d := range dims {
		if rank(d.Level) > rank(worst) {
			worst, worstName = d.Level, d.Name
		}
	}
	return worstName, worst
}

func rank(l Level) int {
	switch l {
	case LevelCritical:
		return 2
	case LevelRestricted:
		return 1
	default:
		return 0
	}
}
