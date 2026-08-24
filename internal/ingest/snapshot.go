package ingest

import (
	"context"

	"github.com/jb843051627/ropeway-sentry/internal/cache"
	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// HeartbeatSource 抽象“读传感器最新心跳”的能力，避免与 store 直接耦合。
type HeartbeatSource interface {
	LatestHeartbeat(sensorID int64) (model.SensorHeartbeat, error)
}

// EnqueueSnapshotRefresh 把“按传感器回读心跳并刷新缓存快照”包装成管道任务。
// 失败时走 Retry 短退避，仍失败则放弃（下次上报会覆盖）。
func EnqueueSnapshotRefresh(p *Pipeline, src HeartbeatSource, c *cache.Cache, snap cache.SensorSnapshot) {
	if p == nil || c == nil {
		return
	}
	p.Submit(Task{
		BatchID: snap.BatchID,
		Run: func(ctx context.Context) error {
			err := Retry(ctx, 3, func() error {
				hb, err := src.LatestHeartbeat(snap.SensorID)
				if err != nil {
					return err
				}
				snap.Value = hb.Value
				snap.Quality = hb.Quality
				snap.SeenAt = hb.SeenAt
				return nil
			})
			if err != nil {
				return err
			}
			c.PutSensorSnapshot(snap)
			return nil
		},
	})
}
