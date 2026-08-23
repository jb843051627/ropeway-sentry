package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// InsertVibration 登记振动事件。
func (s *Store) InsertVibration(v *model.VibrationEvent) error {
	res, err := s.db.Exec(`INSERT INTO vibration_events(line_id,tower_id,peak_accel_ms2,freq_band_hz,duration_ms,severity,occurred_at)
		VALUES(?,?,?,?,?,?,?)`,
		v.LineID, v.TowerID, v.PeakAccelMS2, v.FreqBandHz, v.DurationMS, v.Severity, formatTime(v.OccurredAt))
	if err != nil {
		return mapConstraint(err, "vibration tower")
	}
	v.ID, err = res.LastInsertId()
	return err
}

// ListVibration 查询线路振动事件；since 为零值时不限时间。
func (s *Store) ListVibration(lineID int64, since time.Time, limit int) ([]model.VibrationEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT id,line_id,tower_id,peak_accel_ms2,freq_band_hz,duration_ms,severity,occurred_at
		FROM vibration_events WHERE line_id=?`
	args := []any{lineID}
	if !since.IsZero() {
		query += ` AND occurred_at>=?`
		args = append(args, formatTime(since))
	}
	query += ` ORDER BY occurred_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.VibrationEvent{}
	for rows.Next() {
		var (
			v          model.VibrationEvent
			severity   string
			occurredAt string
		)
		if err := rows.Scan(&v.ID, &v.LineID, &v.TowerID, &v.PeakAccelMS2, &v.FreqBandHz, &v.DurationMS, &severity, &occurredAt); err != nil {
			return nil, err
		}
		parsed, err := model.ParseAlertSeverity(severity)
		if err != nil {
			return nil, err
		}
		v.Severity = parsed
		v.OccurredAt = parseTime(occurredAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

// WorstVibrationSince 返回窗口内最严重事件的级别；无事件返回 ok=false。
func (s *Store) WorstVibrationSince(lineID int64, since time.Time) (model.AlertSeverity, bool, error) {
	var severity string
	err := s.db.QueryRow(`SELECT severity FROM vibration_events
		WHERE line_id=? AND occurred_at>=? ORDER BY CASE severity WHEN 'critical' THEN 0 ELSE 1 END LIMIT 1`,
		lineID, formatTime(since)).Scan(&severity)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	parsed, err := model.ParseAlertSeverity(severity)
	if err != nil {
		return "", false, err
	}
	return parsed, true, nil
}

// InsertCabinPosition 记录载客舱定位快照。
func (s *Store) InsertCabinPosition(c *model.CabinPosition) error {
	res, err := s.db.Exec(`INSERT INTO cabin_positions(line_id,cabin_no,section_m,speed_ms,gap_to_prev_m,recorded_at)
		VALUES(?,?,?,?,?,?)`,
		c.LineID, c.CabinNo, c.SectionM, c.SpeedMS, c.GapToPrevM, formatTime(c.RecordedAt))
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

// ListCabinPositions 取线路最近的定位快照（按 recorded_at 倒序）。
func (s *Store) ListCabinPositions(lineID int64, limit int) ([]model.CabinPosition, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id,line_id,cabin_no,section_m,speed_ms,gap_to_prev_m,recorded_at
		FROM cabin_positions WHERE line_id=? ORDER BY recorded_at DESC LIMIT ?`, lineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.CabinPosition{}
	for rows.Next() {
		var (
			c          model.CabinPosition
			recordedAt string
		)
		if err := rows.Scan(&c.ID, &c.LineID, &c.CabinNo, &c.SectionM, &c.SpeedMS, &c.GapToPrevM, &recordedAt); err != nil {
			return nil, err
		}
		c.RecordedAt = parseTime(recordedAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// MinCabinGapSince 返回窗口内观测到的最小车厢间距。
func (s *Store) MinCabinGapSince(lineID int64, since time.Time) (float64, bool, error) {
	var gap float64
	err := s.db.QueryRow(`SELECT MIN(gap_to_prev_m) FROM cabin_positions WHERE line_id=? AND recorded_at>=?`,
		lineID, formatTime(since)).Scan(&gap)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return gap, true, nil
}
