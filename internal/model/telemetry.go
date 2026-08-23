package model

import (
	"fmt"
	"time"
)

// TelemetryPointInput 批量遥测中的单点输入。
type TelemetryPointInput struct {
	SensorCode string    `json:"sensor"`
	Seq        int64     `json:"seq"`
	TakenAt    time.Time `json:"taken_at"`
	Value      float64   `json:"value"`
}

// BatchInput 遥测批量上报载荷，Checksum 由客户端按规范算法计算。
type BatchInput struct {
	LineCode    string                `json:"line"`
	WindowStart time.Time             `json:"window_start"`
	WindowEnd   time.Time             `json:"window_end"`
	Checksum    uint32                `json:"checksum"`
	Points      []TelemetryPointInput `json:"points"`
}

// ValidateBatch 结构层面的快速校验（时间窗与归属之外的规则在 validation 包完成）。
func (b *BatchInput) ValidateBatch() error {
	if b == nil || len(b.Points) == 0 {
		return ErrEmptyBatch
	}
	if b.LineCode == "" {
		return fmt.Errorf("line code is required")
	}
	for i, p := range b.Points {
		if p.SensorCode == "" {
			return fmt.Errorf("point %d: sensor code is required", i)
		}
		if p.Seq < 0 {
			return fmt.Errorf("point %d: negative seq %d", i, p.Seq)
		}
	}
	return nil
}

// StoredPoint 已落库的遥测点（含三级量程判定结果）。
type StoredPoint struct {
	ID         int64
	BatchID    int64
	SensorID   int64
	SensorCode string
	Seq        int64
	TakenAt    time.Time
	Value      float64
	Quality    Quality
}

// TelemetryBatch 批次元数据记录。
type TelemetryBatch struct {
	ID          int64     `json:"id"`
	LineID      int64     `json:"line_id"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	PointCount  int       `json:"point_count"`
	Checksum    uint32    `json:"checksum"`
	ReceivedAt  time.Time `json:"received_at"`
}

// BatchResult 单批入库链的处理结论，直接作为 API 响应返回。
type BatchResult struct {
	BatchID     int64             `json:"batch_id"`
	Accepted    bool              `json:"accepted"`
	Inserted    int64             `json:"inserted"`
	Duplicate   int64             `json:"duplicate"`
	QualityHits map[Quality]int64 `json:"quality_hits"`
	AlertsNew   []string          `json:"alerts_new"`
	Notes       []string          `json:"notes"`
}

// QualityTally 三级质量计数器，跨包共享的统计结构。
type QualityTally struct {
	Good     int64
	Suspect  int64
	Rejected int64
}

// Add 累加一次判定结果。
func (t *QualityTally) Add(q Quality) {
	switch q {
	case QualityGood:
		t.Good++
	case QualitySuspect:
		t.Suspect++
	default:
		t.Rejected++
	}
}

// IntegrityRate 计算完整率：good 点占比；无样本时返回 1（不惩罚）。
func (t QualityTally) IntegrityRate() float64 {
	total := t.Good + t.Suspect + t.Rejected
	if total == 0 {
		return 1
	}
	return float64(t.Good) / float64(total)
}

// ChecksumInput 校验和重算接口的请求体。
type ChecksumInput struct {
	Points []TelemetryPointInput `json:"points"`
}

// ChecksumResult 校验和重算接口的响应体。
type ChecksumResult struct {
	Checksum uint32 `json:"checksum"`
	Points   int    `json:"points"`
}
