package model

import (
	"fmt"
	"time"
)

// RopeSensor 钢丝绳/环境传感器。
// ExpectedValue 与 Tolerance 构成传感器自带的基线与容差；
// 软硬量程共同支撑 good / suspect / rejected 三级判定。
type RopeSensor struct {
	ID            int64      `json:"id"`
	LineID        int64      `json:"line_id"`
	TowerID       int64      `json:"tower_id"`
	Code          string     `json:"code"`
	Kind          SensorKind `json:"kind"`
	Unit          string     `json:"unit"`
	Enabled       bool       `json:"enabled"`
	ExpectedValue float64    `json:"expected_value"`
	Tolerance     float64    `json:"tolerance"`
	SoftMin       float64    `json:"soft_min"`
	SoftMax       float64    `json:"soft_max"`
	HardMin       float64    `json:"hard_min"`
	HardMax       float64    `json:"hard_max"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Validate 新建传感器的字段约束。
func (s *RopeSensor) Validate() error {
	if s.LineID <= 0 {
		return fmt.Errorf("sensor must reference a line")
	}
	if s.Code == "" {
		return fmt.Errorf("sensor code is required")
	}
	if _, err := ParseSensorKind(string(s.Kind)); err != nil {
		return err
	}
	if s.Tolerance <= 0 {
		return fmt.Errorf("tolerance must be positive")
	}
	if s.SoftMin > s.SoftMax {
		return fmt.Errorf("soft range inverted: [%.2f, %.2f]", s.SoftMin, s.SoftMax)
	}
	if s.HardMin > s.HardMax {
		return fmt.Errorf("hard range inverted: [%.2f, %.2f]", s.HardMin, s.HardMax)
	}
	if s.HardMin > s.SoftMin || s.HardMax < s.SoftMax {
		return fmt.Errorf("hard range must enclose soft range")
	}
	return nil
}

// SensorHeartbeat 传感器最近一次上报快照，用于心跳过期扫描。
type SensorHeartbeat struct {
	SensorID int64
	Code     string
	Kind     SensorKind
	Value    float64
	Quality  Quality
	SeenAt   time.Time
}
