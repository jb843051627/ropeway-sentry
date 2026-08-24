package model

import (
	"errors"
	"fmt"
	"time"
)

// 业务层通用错误，api 层据此映射 HTTP 状态码。
var (
	ErrNotFound          = errors.New("record not found")
	ErrConflict          = errors.New("conflicting state")
	ErrChecksumMismatch  = errors.New("batch checksum mismatch")
	ErrFutureWindow      = errors.New("telemetry window ends in the future")
	ErrExpiredWindow     = errors.New("telemetry window is older than retention")
	ErrBadWindow         = errors.New("telemetry window start must precede end")
	ErrDisabledSensor    = errors.New("sensor is disabled")
	ErrOrphanSensor      = errors.New("sensor does not belong to the line")
	ErrInvalidTransition = errors.New("illegal line status transition")
	ErrHoldMutex         = errors.New("another active hold already locks the line")
	ErrAckRequired       = errors.New("critical alert must be acknowledged before closing")
	ErrEmptyBatch        = errors.New("telemetry batch has no points")
)

// RopewayLine 客运索道线路。
type RopewayLine struct {
	ID           int64      `json:"id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	LengthM      float64    `json:"length_m"`
	TowerCount   int        `json:"tower_count"`
	RatedSpeedMS float64    `json:"rated_speed_ms"`
	Status       LineStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Validate 新建线路时的字段约束。
func (l *RopewayLine) Validate() error {
	if l.Code == "" || l.Name == "" {
		return fmt.Errorf("line code and name are required")
	}
	if l.LengthM <= 0 {
		return fmt.Errorf("line length must be positive")
	}
	if l.TowerCount < 2 {
		return fmt.Errorf("line needs at least two towers")
	}
	if l.RatedSpeedMS <= 0 || l.RatedSpeedMS > 12 {
		return fmt.Errorf("rated speed out of plausible range: %.2f", l.RatedSpeedMS)
	}
	return nil
}

// Tower 沿线支架。
type Tower struct {
	ID           int64     `json:"id"`
	LineID       int64     `json:"line_id"`
	Code         string    `json:"code"`
	HeightM      float64   `json:"height_m"`
	PositionM    float64   `json:"position_m"`
	TiltLimitDeg float64   `json:"tilt_limit_deg"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

// Validate 新建支架时的字段约束。
func (t *Tower) Validate() error {
	if t.LineID <= 0 {
		return fmt.Errorf("tower must reference a line")
	}
	if t.Code == "" {
		return fmt.Errorf("tower code is required")
	}
	if t.HeightM <= 0 {
		return fmt.Errorf("tower height must be positive")
	}
	if t.PositionM < 0 {
		return fmt.Errorf("tower position must not be negative")
	}
	if t.TiltLimitDeg <= 0 || t.TiltLimitDeg > 15 {
		return fmt.Errorf("tilt limit out of plausible range: %.2f", t.TiltLimitDeg)
	}
	return nil
}
