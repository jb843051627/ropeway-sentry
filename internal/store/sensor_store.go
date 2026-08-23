package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// CreateSensor 新建传感器档案；TowerID 为 0 视为未挂载支架（存 NULL）。
func (s *Store) CreateSensor(sen *model.RopeSensor) error {
	var towerID any
	if sen.TowerID > 0 {
		towerID = sen.TowerID
	}
	res, err := s.db.Exec(`INSERT INTO rope_sensors(line_id,tower_id,code,kind,unit,enabled,expected_value,tolerance,soft_min,soft_max,hard_min,hard_max,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sen.LineID, towerID, sen.Code, sen.Kind, sen.Unit, boolToInt(sen.Enabled),
		sen.ExpectedValue, sen.Tolerance, sen.SoftMin, sen.SoftMax, sen.HardMin, sen.HardMax,
		formatTime(time.Now().UTC()))
	if err != nil {
		return mapConstraint(err, "sensor code")
	}
	sen.ID, err = res.LastInsertId()
	return err
}

// GetSensor 按 ID 查询传感器。
func (s *Store) GetSensor(id int64) (model.RopeSensor, error) {
	row := s.db.QueryRow(sensorSelect+` WHERE id=?`, id)
	return scanSensor(row.Scan)
}

// GetSensorByCode 按编码查询传感器（批量上报按编码寻址）。
func (s *Store) GetSensorByCode(code string) (model.RopeSensor, error) {
	row := s.db.QueryRow(sensorSelect+` WHERE code=?`, code)
	return scanSensor(row.Scan)
}

// ListSensors 列出传感器；lineID<=0 返回全部。
func (s *Store) ListSensors(lineID int64) ([]model.RopeSensor, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if lineID > 0 {
		rows, err = s.db.Query(sensorSelect+` WHERE line_id=? ORDER BY code`, lineID)
	} else {
		rows, err = s.db.Query(sensorSelect + ` ORDER BY line_id, code`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.RopeSensor{}
	for rows.Next() {
		sen, err := scanSensor(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, sen)
	}
	return out, rows.Err()
}

// SetSensorEnabled 启用/停用传感器；停用后入库链会拒收其数据点。
func (s *Store) SetSensorEnabled(id int64, enabled bool) error {
	res, err := s.db.Exec(`UPDATE rope_sensors SET enabled=? WHERE id=?`, boolToInt(enabled), id)
	if err != nil {
		return err
	}
	return requireAffected(res, model.ErrNotFound)
}

const sensorSelect = `SELECT id,line_id,tower_id,code,kind,unit,enabled,expected_value,tolerance,soft_min,soft_max,hard_min,hard_max,created_at FROM rope_sensors`

func scanSensor(scan func(dest ...any) error) (model.RopeSensor, error) {
	var (
		sen       model.RopeSensor
		towerID   sql.NullInt64
		kind      string
		enabled   int64
		createdAt string
	)
	if err := scan(&sen.ID, &sen.LineID, &towerID, &sen.Code, &kind, &sen.Unit, &enabled,
		&sen.ExpectedValue, &sen.Tolerance, &sen.SoftMin, &sen.SoftMax, &sen.HardMin, &sen.HardMax, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sen, model.ErrNotFound
		}
		return sen, err
	}
	parsed, err := model.ParseSensorKind(kind)
	if err != nil {
		return sen, err
	}
	sen.Kind = parsed
	sen.Enabled = intToBool(enabled)
	sen.TowerID = towerID.Int64
	sen.CreatedAt = parseTime(createdAt)
	return sen, nil
}

// UpsertHeartbeat 覆盖式写入传感器最新心跳快照。
func (s *Store) UpsertHeartbeat(hb model.SensorHeartbeat) error {
	_, err := s.db.Exec(`INSERT INTO sensor_heartbeats(sensor_id,value,quality,seen_at) VALUES(?,?,?,?)
		ON CONFLICT(sensor_id) DO UPDATE SET value=excluded.value, quality=excluded.quality, seen_at=excluded.seen_at`,
		hb.SensorID, hb.Value, hb.Quality, formatTime(hb.SeenAt))
	return err
}

// LatestHeartbeat 读取传感器最新心跳。
func (s *Store) LatestHeartbeat(sensorID int64) (model.SensorHeartbeat, error) {
	var hb model.SensorHeartbeat
	var seenAt string
	err := s.db.QueryRow(`SELECT h.sensor_id,h.value,h.quality,h.seen_at,s.code,s.kind
		FROM sensor_heartbeats h JOIN rope_sensors s ON s.id=h.sensor_id WHERE h.sensor_id=?`, sensorID).
		Scan(&hb.SensorID, &hb.Value, &hb.Quality, &seenAt, &hb.Code, &hb.Kind)
	if errors.Is(err, sql.ErrNoRows) {
		return hb, model.ErrNotFound
	}
	if err != nil {
		return hb, err
	}
	hb.SeenAt = parseTime(seenAt)
	return hb, nil
}

// StaleSensor 描述一台心跳过期且仍启用的传感器。
type StaleSensor struct {
	SensorID int64
	Code     string
	LineID   int64
	SeenAt   time.Time
}

// ListStaleSensors 找出心跳早于 cutoff 的启用传感器（无心跳视为过期）。
func (s *Store) ListStaleSensors(cutoff time.Time) ([]StaleSensor, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.code, s.line_id,
			COALESCE(h.seen_at,'')
		FROM rope_sensors s LEFT JOIN sensor_heartbeats h ON h.sensor_id=s.id
		WHERE s.enabled=1 AND h.seen_at > ?`, formatTime(cutoff))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StaleSensor{}
	for rows.Next() {
		var (
			item   StaleSensor
			seenAt string
		)
		if err := rows.Scan(&item.SensorID, &item.Code, &item.LineID, &seenAt); err != nil {
			return nil, err
		}
		if seenAt != "" {
			item.SeenAt = parseTime(seenAt)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
