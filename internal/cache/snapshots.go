package cache

import (
	"fmt"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// SensorSnapshot 传感器最新读数快照，热路径从缓存读取。
type SensorSnapshot struct {
	SensorID int64            `json:"sensor_id"`
	Code     string           `json:"code"`
	Kind     model.SensorKind `json:"kind"`
	Value    float64          `json:"value"`
	Quality  model.Quality    `json:"quality"`
	SeenAt   time.Time        `json:"seen_at"`
	BatchID  int64            `json:"batch_id"`
}

func sensorKey(sensorID int64) string { return fmt.Sprintf("sensor:%d", sensorID) }

// PutSensorSnapshot 刷新传感器快照。
func (c *Cache) PutSensorSnapshot(s SensorSnapshot) {
	c.Set(sensorKey(s.SensorID), s)
}

// SensorSnapshotByID 查询传感器快照，未命中返回 nil。
func (c *Cache) SensorSnapshotByID(sensorID int64) (*SensorSnapshot, bool) {
	raw, ok := c.Get(sensorKey(sensorID))
	if !ok {
		return nil, false
	}
	snap, ok := raw.(SensorSnapshot)
	if !ok {
		return nil, false
	}
	return &snap, true
}

func lineKey(lineID int64) string { return fmt.Sprintf("line:%d", lineID) }

// PutLineStatus 缓存线路开放等级。
func (c *Cache) PutLineStatus(lineID int64, status model.LineStatus) {
	c.SetWithTTL(lineKey(lineID), status, 10*time.Minute)
}

// LineStatusByID 查询线路等级缓存。
func (c *Cache) LineStatusByID(lineID int64) (model.LineStatus, bool) {
	raw, ok := c.Get(lineKey(lineID))
	if !ok {
		return "", false
	}
	status, ok := raw.(model.LineStatus)
	return status, ok
}
