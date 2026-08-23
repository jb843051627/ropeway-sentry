package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// CreateHold 登记维护锁（planned 态）。
func (s *Store) CreateHold(h *model.MaintenanceHold) error {
	res, err := s.db.Exec(`INSERT INTO maintenance_holds(line_id,reason,operator,status,planned_at,created_at)
		VALUES(?,?,?,?,?,?)`,
		h.LineID, h.Reason, h.Operator, model.HoldPlanned, formatTime(h.PlannedAt), formatTime(time.Now().UTC()))
	if err != nil {
		return mapConstraint(err, "hold")
	}
	h.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	h.Status = model.HoldPlanned
	return nil
}

// GetHold 按 ID 查询维护锁。
func (s *Store) GetHold(id int64) (model.MaintenanceHold, error) {
	row := s.db.QueryRow(holdSelect+` WHERE id=?`, id)
	hold, err := scanHold(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return hold, model.ErrNotFound
	}
	return hold, err
}

// ListHolds 查询维护锁；lineID<=0 返回全部。
func (s *Store) ListHolds(lineID int64) ([]model.MaintenanceHold, error) {
	query := holdSelect
	args := []any{}
	if lineID > 0 {
		query += ` WHERE line_id=?`
		args = append(args, lineID)
	}
	query += ` ORDER BY id DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MaintenanceHold{}
	for rows.Next() {
		hold, err := scanHold(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, hold)
	}
	return out, rows.Err()
}

// ActiveHoldForLine 返回线路上唯一的 active 锁；不存在返回 ErrNotFound。
func (s *Store) ActiveHoldForLine(lineID int64) (model.MaintenanceHold, error) {
	row := s.db.QueryRow(holdSelect+` WHERE line_id=? AND status='active' ORDER BY activated_at DESC LIMIT 1`, lineID)
	hold, err := scanHold(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return hold, model.ErrNotFound
	}
	return hold, err
}

// ActivateHold planned → active。
func (s *Store) ActivateHold(id int64, at time.Time) error {
	res, err := s.db.Exec(`UPDATE maintenance_holds SET status='active', activated_at=? WHERE id=? AND status='planned'`,
		formatTime(at), id)
	if err != nil {
		return err
	}
	return requireAffected(res, model.ErrConflict)
}

// ReleaseHold active → released，必须携带结论文本。
func (s *Store) ReleaseHold(id int64, note string, at time.Time) error {
	res, err := s.db.Exec(`UPDATE maintenance_holds SET status='released', released_at=?, release_note=?
		WHERE id=? AND status='active'`, formatTime(at), note, id)
	if err != nil {
		return err
	}
	return requireAffected(res, model.ErrConflict)
}

// InsertInspection 写入巡检记录。
func (s *Store) InsertInspection(r *model.InspectionRecord) error {
	res, err := s.db.Exec(`INSERT INTO inspection_records(line_id,tower_id,kind,conclusion,recommendation,inspector,inspected_at)
		VALUES(?,?,?,?,?,?,?)`,
		r.LineID, r.TowerID, r.Kind, r.Conclusion, r.Recommendation, r.Inspector, formatTime(r.InspectedAt))
	if err != nil {
		return mapConstraint(err, "inspection tower")
	}
	r.ID, err = res.LastInsertId()
	return err
}

// ListInspections 查询巡检记录；lineID<=0 返回全部。
func (s *Store) ListInspections(lineID int64, limit int) ([]model.InspectionRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT id,line_id,tower_id,kind,conclusion,recommendation,inspector,inspected_at FROM inspection_records`
	args := []any{}
	if lineID > 0 {
		query += ` WHERE line_id=?`
		args = append(args, lineID)
	}
	query += ` ORDER BY inspected_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.InspectionRecord{}
	for rows.Next() {
		var (
			r           model.InspectionRecord
			towerID     sql.NullInt64
			kind        string
			inspectedAt string
		)
		if err := rows.Scan(&r.ID, &r.LineID, &towerID, &kind, &r.Conclusion, &r.Recommendation, &r.Inspector, &inspectedAt); err != nil {
			return nil, err
		}
		parsed, err := model.ParseInspectionKind(kind)
		if err != nil {
			return nil, err
		}
		r.Kind = parsed
		r.TowerID = towerID.Int64
		r.InspectedAt = parseTime(inspectedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountInspectionsSince 窗口内巡检次数（结构维度参考项）。
func (s *Store) CountInspectionsSince(lineID int64, since time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM inspection_records WHERE line_id=? AND inspected_at>=?`,
		lineID, formatTime(since)).Scan(&n)
	return n, err
}

const holdSelect = `SELECT id,line_id,reason,operator,status,planned_at,activated_at,released_at,release_note,created_at FROM maintenance_holds`

func scanHold(scan func(dest ...any) error) (model.MaintenanceHold, error) {
	var (
		h           model.MaintenanceHold
		status      string
		plannedAt   string
		activatedAt sql.NullString
		releasedAt  sql.NullString
		createdAt   string
	)
	if err := scan(&h.ID, &h.LineID, &h.Reason, &h.Operator, &status, &plannedAt, &activatedAt, &releasedAt, &h.ReleaseNote, &createdAt); err != nil {
		return h, err
	}
	switch model.HoldStatus(status) {
	case model.HoldPlanned, model.HoldActive, model.HoldReleased:
		h.Status = model.HoldStatus(status)
	default:
		return h, errors.New("unknown hold status " + status)
	}
	h.PlannedAt = parseTime(plannedAt)
	h.ActivatedAt = nullTimePtr(activatedAt)
	h.ReleasedAt = nullTimePtr(releasedAt)
	h.CreatedAt = parseTime(createdAt)
	return h, nil
}
