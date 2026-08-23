package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// InsertBatch 落库批次元数据，返回自增批次 ID。
// 幂等语义：同线路同校验和的批次只保留首条记录，重复提交复用原 ID，
// 后续 INSERT OR IGNORE 即可将整批计为 duplicate。
func (s *Store) InsertBatch(b *model.TelemetryBatch) error {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO telemetry_batches(line_id,window_start,window_end,point_count,checksum,received_at)
		VALUES(?,?,?,?,?,?)`,
		b.LineID, formatTime(b.WindowStart), formatTime(b.WindowEnd), b.PointCount, b.Checksum, formatTime(time.Now().UTC()))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		existing := s.db.QueryRow(`SELECT id FROM telemetry_batches WHERE line_id=? AND checksum=?`, b.LineID, b.Checksum)
		var id int64
		if err := existing.Scan(&id); err != nil {
			return err
		}
		b.ID = id
		return nil
	}
	b.ID, err = res.LastInsertId()
	return err
}

// GetBatch 按 ID 查询批次元数据。
func (s *Store) GetBatch(id int64) (model.TelemetryBatch, error) {
	var (
		b           model.TelemetryBatch
		windowStart string
		windowEnd   string
		receivedAt  string
	)
	err := s.db.QueryRow(`SELECT id,line_id,window_start,window_end,point_count,checksum,received_at
		FROM telemetry_batches WHERE id=?`, id).
		Scan(&b.ID, &b.LineID, &windowStart, &windowEnd, &b.PointCount, &b.Checksum, &receivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, model.ErrNotFound
	}
	if err != nil {
		return b, err
	}
	b.WindowStart = parseTime(windowStart)
	b.WindowEnd = parseTime(windowEnd)
	b.ReceivedAt = parseTime(receivedAt)
	return b, nil
}

// StoredPointRow INSERT OR IGNORE 所需的最小点集。
type StoredPointRow struct {
	SensorID int64
	Seq      int64
	TakenAt  time.Time
	Value    float64
	Quality  model.Quality
}

// InsertPointsORIgnore 幂等写入遥测点：
// 唯一键 (batch_id,sensor_id,seq) 冲突时忽略，返回实际新插入行数。
func (s *Store) InsertPointsORIgnore(batchID int64, points []StoredPointRow) (int64, error) {
	var inserted int64
	err := s.Transaction(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`INSERT OR IGNORE INTO telemetry_points(batch_id,sensor_id,seq,taken_at,value,quality,inserted_at)
			VALUES(?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		now := formatTime(time.Now().UTC())
		for _, p := range points {
			res, err := stmt.Exec(batchID, p.SensorID, p.Seq, formatTime(p.TakenAt), p.Value, p.Quality, now)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			inserted += n
		}
		return nil
	})
	return inserted, err
}

// CountPointsForBatch 统计批次当前落库点数（含历史重复提交）。
func (s *Store) CountPointsForBatch(batchID int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM telemetry_points WHERE batch_id=?`, batchID).Scan(&n)
	return n, err
}

// RecentSensorPoints 取传感器 since 之后的点，按时间升序，用于断丝滑动窗口。
func (s *Store) RecentSensorPoints(sensorID int64, since time.Time, limit int) ([]model.StoredPoint, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.Query(`SELECT p.id,p.batch_id,p.sensor_id,s.code,p.seq,p.taken_at,p.value,p.quality
		FROM telemetry_points p JOIN rope_sensors s ON s.id=p.sensor_id
		WHERE p.sensor_id=? AND p.taken_at>=? ORDER BY p.taken_at LIMIT ?`,
		sensorID, formatTime(since), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.StoredPoint{}
	for rows.Next() {
		var (
			p       model.StoredPoint
			takenAt string
			quality string
		)
		if err := rows.Scan(&p.ID, &p.BatchID, &p.SensorID, &p.SensorCode, &p.Seq, &takenAt, &p.Value, &quality); err != nil {
			return nil, err
		}
		p.TakenAt = parseTime(takenAt)
		p.Quality = model.Quality(quality)
		out = append(out, p)
	}
	return out, rows.Err()
}

// QualityCountsSince 汇总线路在 since 之后各质量等级的点数。
func (s *Store) QualityCountsSince(lineID int64, since time.Time) (model.QualityTally, error) {
	var tally model.QualityTally
	rows, err := s.db.Query(`SELECT p.quality, COUNT(*) FROM telemetry_points p
		JOIN rope_sensors s ON s.id=p.sensor_id
		WHERE s.line_id=? AND p.taken_at>=? GROUP BY p.quality`, lineID, formatTime(since))
	if err != nil {
		return tally, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			quality string
			count   int64
		)
		if err := rows.Scan(&quality, &count); err != nil {
			return tally, err
		}
		switch model.Quality(quality) {
		case model.QualityGood:
			tally.Good += count
		case model.QualitySuspect:
			tally.Suspect += count
		default:
			tally.Rejected += count
		}
	}
	return tally, rows.Err()
}

// CountBatchesSince 统计线路自 since 以来的批次数。
func (s *Store) CountBatchesSince(lineID int64, since time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM telemetry_batches WHERE line_id=? AND received_at>=?`,
		lineID, formatTime(since)).Scan(&n)
	return n, err
}

// RecentLinePoints 取线路 since 之后的遥测点（含传感器编码），按时间升序，
// 供 UTC CSV 导出使用；lineID<=0 表示跨全部线路。
func (s *Store) RecentLinePoints(lineID int64, since time.Time, limit int) ([]model.StoredPoint, error) {
	if limit <= 0 {
		limit = 5000
	}
	query := `SELECT p.id,p.batch_id,p.sensor_id,s.code,p.seq,p.taken_at,p.value,p.quality
		FROM telemetry_points p JOIN rope_sensors s ON s.id=p.sensor_id
		WHERE p.taken_at>=?`
	args := []any{formatTime(since)}
	if lineID > 0 {
		query += ` AND s.line_id=?`
		args = append(args, lineID)
	}
	query += ` ORDER BY p.taken_at LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.StoredPoint{}
	for rows.Next() {
		var (
			p       model.StoredPoint
			takenAt string
			quality string
		)
		if err := rows.Scan(&p.ID, &p.BatchID, &p.SensorID, &p.SensorCode, &p.Seq, &takenAt, &p.Value, &quality); err != nil {
			return nil, err
		}
		p.TakenAt = parseTime(takenAt)
		p.Quality = model.Quality(quality)
		out = append(out, p)
	}
	return out, rows.Err()
}
