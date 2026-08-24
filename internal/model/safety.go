package model

import (
	"fmt"
	"time"
)

// VibrationEvent 振动事件：由支架振动监测触发或人工登记。
type VibrationEvent struct {
	ID           int64         `json:"id"`
	LineID       int64         `json:"line_id"`
	TowerID      int64         `json:"tower_id"`
	PeakAccelMS2 float64       `json:"peak_accel_ms2"`
	FreqBandHz   float64       `json:"freq_band_hz"`
	DurationMS   int64         `json:"duration_ms"`
	Severity     AlertSeverity `json:"severity"`
	OccurredAt   time.Time     `json:"occurred_at"`
}

// Validate 登记振动事件时的字段约束。
func (v *VibrationEvent) Validate() error {
	if v.LineID <= 0 || v.TowerID <= 0 {
		return fmt.Errorf("vibration event must reference line and tower")
	}
	if v.PeakAccelMS2 <= 0 {
		return fmt.Errorf("peak accel must be positive")
	}
	if v.FreqBandHz <= 0 {
		return fmt.Errorf("freq band must be positive")
	}
	if v.DurationMS <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if _, err := ParseAlertSeverity(string(v.Severity)); err != nil {
		return err
	}
	return nil
}

// CabinPosition 载客舱实时定位快照。
type CabinPosition struct {
	ID         int64     `json:"id"`
	LineID     int64     `json:"line_id"`
	CabinNo    string    `json:"cabin_no"`
	SectionM   float64   `json:"section_m"`
	SpeedMS    float64   `json:"speed_ms"`
	GapToPrevM float64   `json:"gap_to_prev_m"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Validate 定位快照字段约束；间距过小在 service 层结合额定速度判定。
func (c *CabinPosition) Validate() error {
	if c.LineID <= 0 {
		return fmt.Errorf("cabin position must reference a line")
	}
	if c.CabinNo == "" {
		return fmt.Errorf("cabin number is required")
	}
	if c.SectionM < 0 {
		return fmt.Errorf("section must not be negative")
	}
	if c.SpeedMS < 0 {
		return fmt.Errorf("speed must not be negative")
	}
	return nil
}

// TensionBaseline 张力基线：按温度补偿的期望区间，优先级高于传感器自带容差。
type TensionBaseline struct {
	ID           int64     `json:"id"`
	LineID       int64     `json:"line_id"`
	SensorCode   string    `json:"sensor_code"`
	ExpectedN    float64   `json:"expected_n"`
	ToleranceN   float64   `json:"tolerance_n"`
	TempCoeffN   float64   `json:"temp_coeff_n_per_c"`
	AmbientTempC float64   `json:"ambient_temp_c"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidTo      time.Time `json:"valid_to"`
	CreatedAt    time.Time `json:"created_at"`
}

// Validate 基线字段约束。
func (b *TensionBaseline) Validate() error {
	if b.LineID <= 0 {
		return fmt.Errorf("baseline must reference a line")
	}
	if b.SensorCode == "" {
		return fmt.Errorf("baseline must reference a sensor")
	}
	if b.ExpectedN <= 0 {
		return fmt.Errorf("expected tension must be positive")
	}
	if !b.ValidFrom.Before(b.ValidTo) {
		return fmt.Errorf("validity window start must precede end")
	}
	return nil
}

// EffectiveExpected 计算温度补偿后的期望张力：
// 每偏离标定温度 1 摄氏度，期望值平移 TempCoeffN 牛顿。
func (b *TensionBaseline) EffectiveExpected() float64 {
	return b.ExpectedN + b.TempCoeffN*(b.AmbientTempC-20.0)
}

// Covers 判断基线在某时刻是否生效。
func (b *TensionBaseline) Covers(at time.Time) bool {
	return !at.Before(b.ValidFrom) && !at.After(b.ValidTo)
}
