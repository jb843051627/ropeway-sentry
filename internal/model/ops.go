package model

import "time"

// SafetyAssessment 线路安全评估结论：三维度评分 + 完整率 → 开放等级。
type SafetyAssessment struct {
	ID             int64      `json:"id"`
	LineID         int64      `json:"line_id"`
	WindScore      float64    `json:"wind_score"`
	TensionScore   float64    `json:"tension_score"`
	StructureScore float64    `json:"structure_score"`
	IntegrityRate  float64    `json:"integrity_rate"`
	Level          LineStatus `json:"level"`
	IcingActive    bool       `json:"icing_active"`
	Notes          string     `json:"notes"`
	AssessedAt     time.Time  `json:"assessed_at"`
}

// DimensionScore 单维度归一化分（0-100，越高越安全）。
type DimensionScore struct {
	Name  string
	Value float64
}

// ClampScore 把分数限制到 [0,100] 区间。
func ClampScore(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	default:
		return v
	}
}

// Alert 告警记录，含去重键与 ack → close 状态机字段。
type Alert struct {
	ID           int64         `json:"id"`
	LineID       int64         `json:"line_id"`
	SensorID     int64         `json:"sensor_id"`
	DedupKey     string        `json:"dedup_key"`
	Kind         string        `json:"kind"`
	Severity     AlertSeverity `json:"severity"`
	Message      string        `json:"message"`
	Status       AlertStatus   `json:"status"`
	Occurrences  int64         `json:"occurrences"`
	FirstSeenAt  time.Time     `json:"first_seen_at"`
	LatestSeenAt time.Time     `json:"latest_seen_at"`
	AckedBy      string        `json:"acked_by"`
	AckedAt      *time.Time    `json:"acked_at"`
	ClosedAt     *time.Time    `json:"closed_at"`
	CloseNote    string        `json:"close_note"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// MaintenanceHold 维护停机互斥锁：同一线路同时至多一个 active。
type MaintenanceHold struct {
	ID          int64      `json:"id"`
	LineID      int64      `json:"line_id"`
	Reason      string     `json:"reason"`
	Operator    string     `json:"operator"`
	Status      HoldStatus `json:"status"`
	PlannedAt   time.Time  `json:"planned_at"`
	ActivatedAt *time.Time `json:"activated_at"`
	ReleasedAt  *time.Time `json:"released_at"`
	ReleaseNote string     `json:"release_note"`
	CreatedAt   time.Time  `json:"created_at"`
}

// InspectionRecord 巡检记录：目检/探伤结论与处置建议。
type InspectionRecord struct {
	ID             int64          `json:"id"`
	LineID         int64          `json:"line_id"`
	TowerID        int64          `json:"tower_id"`
	Kind           InspectionKind `json:"kind"`
	Conclusion     string         `json:"conclusion"`
	Recommendation string         `json:"recommendation"`
	Inspector      string         `json:"inspector"`
	InspectedAt    time.Time      `json:"inspected_at"`
}
