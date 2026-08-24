package service

import (
	"fmt"
	"sort"

	"github.com/jb843051627/ropeway-sentry/internal/cache"
	"github.com/jb843051627/ropeway-sentry/internal/ingest"
	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/validation"
)

// cacheSnapshot 把心跳模型转成缓存快照。
func cacheSnapshot(hb model.SensorHeartbeat, batchID int64) cache.SensorSnapshot {
	return cache.SensorSnapshot{
		SensorID: hb.SensorID,
		Code:     hb.Code,
		Kind:     hb.Kind,
		Value:    hb.Value,
		Quality:  hb.Quality,
		SeenAt:   hb.SeenAt,
		BatchID:  batchID,
	}
}

// IngestBatch 批量遥测入库链，顺序即设计文档约定：
//  1. 结构校验 → 2. 线路存在 → 3. 校验和重算比对 → 4. 时间窗
//     （未来/过期拒收）→ 5. 窗口内逐点过滤 → 6. 归属与启用校验 →
//  7. 量程三级 good/suspect/rejected → 8. INSERT OR IGNORE 幂等落库并计数 →
//  9. 心跳与快照刷新（异步管道）→ 10. 阈值越限告警。
func (s *Service) IngestBatch(in model.BatchInput) (model.BatchResult, error) {
	result := model.BatchResult{QualityHits: map[model.Quality]int64{}, Notes: []string{}}
	var tally validation.QualityTally
	if err := in.ValidateBatch(); err != nil {
		s.reject()
		return result, err
	}
	line, err := s.store.GetLineByCode(in.LineCode)
	if err != nil {
		s.reject()
		return result, fmt.Errorf("line %s: %w", in.LineCode, err)
	}
	if err := validation.VerifyChecksum(in.Points, in.Checksum); err != nil {
		s.reject()
		return result, err
	}
	now := s.clock.Now()
	if err := validation.CheckWindow(in.WindowStart, in.WindowEnd, now, s.window); err != nil {
		s.reject()
		return result, err
	}
	kept, dropped := validation.FilterBatchPoints(in.Points, in.WindowStart, in.WindowEnd)
	for _, note := range dropped {
		result.Notes = append(result.Notes, "dropped: "+note)
	}
	if len(kept) == 0 {
		s.reject()
		return result, fmt.Errorf("%w: all points outside declared window", model.ErrEmptyBatch)
	}

	type resolved struct {
		sensor model.RopeSensor
		spec   validation.RangeSpec
	}
	sensors := make(map[string]resolved, len(kept))
	for _, p := range kept {
		if _, ok := sensors[p.SensorCode]; ok {
			continue
		}
		sensor, err := s.store.GetSensorByCode(p.SensorCode)
		if err != nil {
			s.reject()
			return result, fmt.Errorf("sensor %s: %w", p.SensorCode, err)
		}
		if err := validation.CheckSensorOwnership(sensor, line.ID); err != nil {
			s.reject()
			return result, err
		}
		sensors[p.SensorCode] = resolved{sensor: sensor, spec: validation.FromSensor(sensor)}
	}

	batch := model.TelemetryBatch{
		LineID:      line.ID,
		WindowStart: in.WindowStart,
		WindowEnd:   in.WindowEnd,
		PointCount:  len(kept),
		Checksum:    in.Checksum,
	}
	if err := s.store.InsertBatch(&batch); err != nil {
		return result, err
	}
	rows := make([]storePointRow, 0, len(kept))
	for _, p := range kept {
		entry := sensors[p.SensorCode]
		quality := validation.Grade(entry.spec, p.Value)
		tally.Add(quality)
		rows = append(rows, storePointRow{
			SensorID: entry.sensor.ID,
			Seq:      p.Seq,
			TakenAt:  p.TakenAt,
			Value:    p.Value,
			Quality:  quality,
		})
	}
	inserted, err := s.insertPoints(batch.ID, rows)
	if err != nil {
		return result, err
	}
	dupes := int64(len(rows)) - inserted
	result.BatchID = batch.ID
	result.Accepted = true
	result.Inserted = inserted
	result.Duplicate = dupes
	result.QualityHits = tallyToMap(tally)
	s.countAccepted(inserted, dupes)

	// 心跳/快照刷新：每个传感器取窗口内最新一点。
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].TakenAt.Before(kept[j].TakenAt) })
	touched := map[string]int{}
	for idx, p := range kept {
		entry := sensors[p.SensorCode]
		if prev, ok := touched[p.SensorCode]; ok && kept[prev].TakenAt.After(p.TakenAt) {
			continue
		}
		touched[p.SensorCode] = idx
		quality := validation.Grade(entry.spec, p.Value)
		if quality == model.QualityRejected {
			continue // rejected 点不推进心跳，避免影响热路径快照
		}
		s.touchHeartbeat(entry.sensor, p.Value, quality, p.TakenAt, batch.ID)
		if s.pipeline != nil {
			ingest.EnqueueSnapshotRefresh(s.pipeline, s.store, s.cache, cacheSnapshot(
				model.SensorHeartbeat{SensorID: entry.sensor.ID, Code: entry.sensor.Code, Kind: entry.sensor.Kind},
				batch.ID))
		}
	}

	alerts, err := s.evaluateThresholds(line.ID, now)
	if err == nil {
		result.AlertsNew = alerts
	}
	return result, nil
}

// GetBatch 查询批次元数据。
func (s *Service) GetBatch(id int64) (model.TelemetryBatch, error) {
	return s.store.GetBatch(id)
}

// RecomputeChecksum 服务端校验和重算接口：不落库、无副作用。
func (s *Service) RecomputeChecksum(points []model.TelemetryPointInput) model.ChecksumResult {
	return model.ChecksumResult{Checksum: validation.ComputeChecksum(points), Points: len(points)}
}
