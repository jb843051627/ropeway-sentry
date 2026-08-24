package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// UpsertBaseline 写入张力基线（同线路同传感器同生效起点覆盖）。
func (s *Store) UpsertBaseline(b *model.TensionBaseline) error {
	res, err := s.db.Exec(`INSERT INTO tension_baselines(line_id,sensor_code,expected_n,tolerance_n,temp_coeff_n,ambient_temp_c,valid_from,valid_to,created_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(line_id,sensor_code,valid_from) DO UPDATE SET
			expected_n=excluded.expected_n, tolerance_n=excluded.tolerance_n,
			temp_coeff_n=excluded.temp_coeff_n, ambient_temp_c=excluded.ambient_temp_c,
			valid_to=excluded.valid_to`,
		b.LineID, b.SensorCode, b.ExpectedN, b.ToleranceN, b.TempCoeffN, b.AmbientTempC,
		formatTime(b.ValidFrom), formatTime(b.ValidTo), formatTime(time.Now().UTC()))
	if err != nil {
		return mapConstraint(err, "baseline sensor")
	}
	b.ID, err = res.LastInsertId()
	return err
}

// ListBaselines 查询线路基线列表。
func (s *Store) ListBaselines(lineID int64) ([]model.TensionBaseline, error) {
	rows, err := s.db.Query(`SELECT id,line_id,sensor_code,expected_n,tolerance_n,temp_coeff_n,ambient_temp_c,valid_from,valid_to,created_at
		FROM tension_baselines WHERE line_id=? ORDER BY sensor_code, valid_from DESC`, lineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.TensionBaseline{}
	for rows.Next() {
		baseline, err := scanBaseline(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, baseline)
	}
	return out, rows.Err()
}

// ActiveBaselineForSensor 取某时刻生效的基线；多条时取最近创建的一条。
func (s *Store) ActiveBaselineForSensor(lineID int64, sensorCode string, at time.Time) (model.TensionBaseline, error) {
	row := s.db.QueryRow(`SELECT id,line_id,sensor_code,expected_n,tolerance_n,temp_coeff_n,ambient_temp_c,valid_from,valid_to,created_at
		FROM tension_baselines
		WHERE line_id=? AND sensor_code=? AND valid_from<=? AND valid_to>=?
		ORDER BY created_at DESC LIMIT 1`,
		lineID, sensorCode, formatTime(at), formatTime(at))
	baseline, err := scanBaseline(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return baseline, model.ErrNotFound
	}
	return baseline, err
}

func scanBaseline(scan func(dest ...any) error) (model.TensionBaseline, error) {
	var (
		b         model.TensionBaseline
		validFrom string
		validTo   string
		createdAt string
	)
	if err := scan(&b.ID, &b.LineID, &b.SensorCode, &b.ExpectedN, &b.ToleranceN, &b.TempCoeffN,
		&b.AmbientTempC, &validFrom, &validTo, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return b, model.ErrNotFound
		}
		return b, err
	}
	b.ValidFrom = parseTime(validFrom)
	b.ValidTo = parseTime(validTo)
	b.CreatedAt = parseTime(createdAt)
	return b, nil
}

// InsertAssessment 落库一次安全评估。
func (s *Store) InsertAssessment(a *model.SafetyAssessment) error {
	res, err := s.db.Exec(`INSERT INTO safety_assessments(line_id,wind_score,tension_score,structure_score,integrity_rate,level,icing_active,notes,assessed_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		a.LineID, a.WindScore, a.TensionScore, a.StructureScore, a.IntegrityRate,
		a.Level, boolToInt(a.IcingActive), a.Notes, formatTime(a.AssessedAt))
	if err != nil {
		return err
	}
	a.ID, err = res.LastInsertId()
	return err
}

// ListAssessments 查询线路历史评估。
func (s *Store) ListAssessments(lineID int64, limit int) ([]model.SafetyAssessment, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,line_id,wind_score,tension_score,structure_score,integrity_rate,level,icing_active,notes,assessed_at
		FROM safety_assessments WHERE line_id=? ORDER BY assessed_at DESC LIMIT ?`, lineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.SafetyAssessment{}
	for rows.Next() {
		var (
			a          model.SafetyAssessment
			level      string
			icing      int64
			assessedAt string
		)
		if err := rows.Scan(&a.ID, &a.LineID, &a.WindScore, &a.TensionScore, &a.StructureScore,
			&a.IntegrityRate, &level, &icing, &a.Notes, &assessedAt); err != nil {
			return nil, err
		}
		parsed, err := model.ParseLineStatus(level)
		if err != nil {
			return nil, err
		}
		a.Level = parsed
		a.IcingActive = intToBool(icing)
		a.AssessedAt = parseTime(assessedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

// LatestAssessment 取线路最近一次评估。
func (s *Store) LatestAssessment(lineID int64) (model.SafetyAssessment, error) {
	list, err := s.ListAssessments(lineID, 1)
	if err != nil {
		return model.SafetyAssessment{}, err
	}
	if len(list) == 0 {
		return model.SafetyAssessment{}, model.ErrNotFound
	}
	return list[0], nil
}
