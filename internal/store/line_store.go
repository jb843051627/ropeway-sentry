package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// CreateLine 新建线路，返回自增 ID。
func (s *Store) CreateLine(l *model.RopewayLine) error {
	now := time.Now().UTC()
	res, err := s.db.Exec(`INSERT INTO ropeway_lines(code,name,length_m,tower_count,rated_speed_ms,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		l.Code, l.Name, l.LengthM, l.TowerCount, l.RatedSpeedMS, model.LineOpen, formatTime(now), formatTime(now))
	if err != nil {
		return mapConstraint(err, "line code")
	}
	l.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	l.Status = model.LineOpen
	l.CreatedAt, l.UpdatedAt = now, now
	return nil
}

// GetLine 按 ID 查询线路。
func (s *Store) GetLine(id int64) (model.RopewayLine, error) {
	row := s.db.QueryRow(`SELECT id,code,name,length_m,tower_count,rated_speed_ms,status,created_at,updated_at
		FROM ropeway_lines WHERE id=?`, id)
	return scanLine(row.Scan)
}

// GetLineByCode 按业务编码查询线路。
func (s *Store) GetLineByCode(code string) (model.RopewayLine, error) {
	row := s.db.QueryRow(`SELECT id,code,name,length_m,tower_count,rated_speed_ms,status,created_at,updated_at
		FROM ropeway_lines WHERE code=?`, code)
	return scanLine(row.Scan)
}

// ListLines 返回全部线路，按 ID 升序。
func (s *Store) ListLines() ([]model.RopewayLine, error) {
	rows, err := s.db.Query(`SELECT id,code,name,length_m,tower_count,rated_speed_ms,status,created_at,updated_at
		FROM ropeway_lines ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.RopewayLine{}
	for rows.Next() {
		line, err := scanLine(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}

// UpdateLineStatus 更新开放等级（状态机校验在 service 层完成）。
func (s *Store) UpdateLineStatus(id int64, status model.LineStatus, at time.Time) error {
	res, err := s.db.Exec(`UPDATE ropeway_lines SET status=?, updated_at=? WHERE id=?`, status, formatTime(at), id)
	if err != nil {
		return err
	}
	return requireAffected(res, model.ErrNotFound)
}

// CountTowersForLine 统计线路下支架数量，用于回填 tower_count。
func (s *Store) CountTowersForLine(lineID int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM towers WHERE line_id=?`, lineID).Scan(&n)
	return n, err
}

// RefreshTowerCount 把支架统计值同步进线路档案。
func (s *Store) RefreshTowerCount(lineID int64) error {
	n, err := s.CountTowersForLine(lineID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE ropeway_lines SET tower_count=? WHERE id=?`, n, lineID)
	return err
}

func scanLine(scan func(dest ...any) error) (model.RopewayLine, error) {
	var (
		l         model.RopewayLine
		status    string
		createdAt string
		updatedAt string
	)
	if err := scan(&l.ID, &l.Code, &l.Name, &l.LengthM, &l.TowerCount, &l.RatedSpeedMS, &status, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return l, model.ErrNotFound
		}
		return l, err
	}
	parsed, err := model.ParseLineStatus(status)
	if err != nil {
		return l, fmt.Errorf("line %d: %w", l.ID, err)
	}
	l.Status = parsed
	l.CreatedAt = parseTime(createdAt)
	l.UpdatedAt = parseTime(updatedAt)
	return l, nil
}

// mapConstraint 将唯一键冲突翻译为 ErrConflict。
func mapConstraint(err error, what string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w: duplicate %s", model.ErrConflict, what)
	}
	return err
}
