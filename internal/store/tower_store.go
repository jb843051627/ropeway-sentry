package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// requireAffected 断言写操作至少影响一行，否则返回 ErrNotFound。
func requireAffected(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}

// boolToInt 布尔转 SQLite 整型。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool 整型转布尔。
func intToBool(n int64) bool { return n != 0 }

// CreateTower 新建支架并回填线路的 tower_count。
func (s *Store) CreateTower(t *model.Tower) error {
	res, err := s.db.Exec(`INSERT INTO towers(line_id,code,height_m,position_m,tilt_limit_deg,enabled,created_at)
		VALUES(?,?,?,?,?,?,?)`,
		t.LineID, t.Code, t.HeightM, t.PositionM, t.TiltLimitDeg, boolToInt(t.Enabled), formatTime(time.Now().UTC()))
	if err != nil {
		return mapConstraint(err, "tower code")
	}
	if t.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	return s.RefreshTowerCount(t.LineID)
}

// GetTower 按 ID 查询支架。
func (s *Store) GetTower(id int64) (model.Tower, error) {
	row := s.db.QueryRow(`SELECT id,line_id,code,height_m,position_m,tilt_limit_deg,enabled,created_at
		FROM towers WHERE id=?`, id)
	return scanTower(row.Scan)
}

// GetTowerByCode 按线路内编码查询支架。
func (s *Store) GetTowerByCode(lineID int64, code string) (model.Tower, error) {
	row := s.db.QueryRow(`SELECT id,line_id,code,height_m,position_m,tilt_limit_deg,enabled,created_at
		FROM towers WHERE line_id=? AND code=?`, lineID, code)
	return scanTower(row.Scan)
}

// ListTowers 按线路查询支架；lineID<=0 返回全部。
func (s *Store) ListTowers(lineID int64) ([]model.Tower, error) {
	const base = `SELECT id,line_id,code,height_m,position_m,tilt_limit_deg,enabled,created_at FROM towers`
	var (
		rows *sql.Rows
		err  error
	)
	if lineID > 0 {
		rows, err = s.db.Query(base+` WHERE line_id=? ORDER BY position_m`, lineID)
	} else {
		rows, err = s.db.Query(base + ` ORDER BY line_id, position_m`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Tower{}
	for rows.Next() {
		tw, err := scanTower(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, tw)
	}
	return out, rows.Err()
}

func scanTower(scan func(dest ...any) error) (model.Tower, error) {
	var (
		t         model.Tower
		enabled   int64
		createdAt string
	)
	if err := scan(&t.ID, &t.LineID, &t.Code, &t.HeightM, &t.PositionM, &t.TiltLimitDeg, &enabled, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, model.ErrNotFound
		}
		return t, err
	}
	t.Enabled = intToBool(enabled)
	t.CreatedAt = parseTime(createdAt)
	return t, nil
}
