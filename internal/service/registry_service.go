package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// CreateLine 新建线路（初始固定 open）。
func (s *Service) CreateLine(in *model.RopewayLine) (model.RopewayLine, error) {
	if err := in.Validate(); err != nil {
		return model.RopewayLine{}, err
	}
	if _, err := s.store.GetLineByCode(in.Code); err == nil {
		return model.RopewayLine{}, fmt.Errorf("%w: line code %s exists", model.ErrConflict, in.Code)
	}
	if err := s.store.CreateLine(in); err != nil {
		return model.RopewayLine{}, err
	}
	s.cache.PutLineStatus(in.ID, in.Status)
	return *in, nil
}

// ListLines 列出全部线路。
func (s *Service) ListLines() ([]model.RopewayLine, error) {
	return s.store.ListLines()
}

// GetLine 查询单条线路。
func (s *Service) GetLine(id int64) (model.RopewayLine, error) {
	return s.store.GetLine(id)
}

// TransitionLine 人工驱动开放状态机：
//   - 迁移必须落在合法边内；
//   - 进入 maintenance 需要已激活的维护锁；
//   - 线路存在未关闭 critical 告警时禁止回到 open。
func (s *Service) TransitionLine(lineID int64, to string) (model.RopewayLine, error) {
	target, err := model.ParseLineStatus(to)
	if err != nil {
		return model.RopewayLine{}, err
	}
	line, err := s.store.GetLine(lineID)
	if err != nil {
		return line, err
	}
	if !model.CanTransition(line.Status, target) {
		return line, fmt.Errorf("%w: %s → %s", model.ErrInvalidTransition, line.Status, target)
	}
	now := s.clock.Now()
	if target == model.LineMaintenance {
		if _, err := s.store.ActiveHoldForLine(lineID); err != nil {
			return line, fmt.Errorf("%w: enter maintenance requires an active hold", model.ErrConflict)
		}
	}
	if target == model.LineOpen && line.Status == model.LineClosed {
		n, err := s.store.CountOpenCriticalForLine(lineID)
		if err != nil {
			return line, err
		}
		if n > 0 {
			return line, fmt.Errorf("%w: %d critical alerts still open", model.ErrConflict, n)
		}
	}
	if err := s.store.UpdateLineStatus(lineID, target, now); err != nil {
		return line, err
	}
	s.cache.PutLineStatus(lineID, target)
	return s.store.GetLine(lineID)
}

// CreateTower 新建支架并回填线路统计。
func (s *Service) CreateTower(t *model.Tower) (model.Tower, error) {
	if err := t.Validate(); err != nil {
		return model.Tower{}, err
	}
	if _, err := s.store.GetLine(t.LineID); err != nil {
		return model.Tower{}, fmt.Errorf("line %d: %w", t.LineID, err)
	}
	if _, err := s.store.GetTowerByCode(t.LineID, t.Code); err == nil {
		return model.Tower{}, fmt.Errorf("%w: tower %s exists on line %d", model.ErrConflict, t.Code, t.LineID)
	}
	if err := s.store.CreateTower(t); err != nil {
		return model.Tower{}, err
	}
	return s.store.GetTower(t.ID)
}

// ListTowers 列出支架；lineID<=0 为全部。
func (s *Service) ListTowers(lineID int64) ([]model.Tower, error) {
	return s.store.ListTowers(lineID)
}

// CreateSensor 登记传感器档案，基线与容差在此固化。
func (s *Service) CreateSensor(sen *model.RopeSensor) (model.RopeSensor, error) {
	if err := sen.Validate(); err != nil {
		return model.RopeSensor{}, err
	}
	if _, err := s.store.GetLine(sen.LineID); err != nil {
		return model.RopeSensor{}, fmt.Errorf("line %d: %w", sen.LineID, err)
	}
	if sen.TowerID > 0 {
		tower, err := s.store.GetTower(sen.TowerID)
		if err != nil {
			return model.RopeSensor{}, fmt.Errorf("tower %d: %w", sen.TowerID, err)
		}
		if tower.LineID != sen.LineID {
			return model.RopeSensor{}, fmt.Errorf("%w: tower %d belongs to line %d", model.ErrOrphanSensor, sen.TowerID, tower.LineID)
		}
	}
	if err := s.store.CreateSensor(sen); err != nil {
		return model.RopeSensor{}, err
	}
	return s.store.GetSensor(sen.ID)
}

// ListSensors 列出传感器；lineID<=0 为全部。
func (s *Service) ListSensors(lineID int64) ([]model.RopeSensor, error) {
	return s.store.ListSensors(lineID)
}

// GetSensor 查询单个传感器。
func (s *Service) GetSensor(id int64) (model.RopeSensor, error) {
	return s.store.GetSensor(id)
}

// SetSensorEnabled 停用/启用传感器；停用后入库链拒收其数据点。
func (s *Service) SetSensorEnabled(id int64, enabled bool) error {
	if _, err := s.store.GetSensor(id); err != nil {
		return err
	}
	return s.store.SetSensorEnabled(id, enabled)
}

// SensorSnapshotView 缓存快照的只读视图，供 API 使用。
type SensorSnapshotView struct {
	Sensor  model.SensorHeartbeat `json:"sensor"`
	Cached  bool                  `json:"cached"`
	FreshIn time.Duration         `json:"-"`
}

// LatestSnapshot 查询传感器最新读数：优先缓存，回落数据库心跳表。
func (s *Service) LatestSnapshot(sensorID int64) (SensorSnapshotView, error) {
	sensor, err := s.store.GetSensor(sensorID)
	if err != nil {
		return SensorSnapshotView{}, err
	}
	hb, hbErr := s.store.LatestHeartbeat(sensorID)
	view := SensorSnapshotView{Sensor: hb}
	if hbErr != nil {
		view.Sensor = model.SensorHeartbeat{SensorID: sensor.ID, Code: sensor.Code, Kind: sensor.Kind}
		return view, nil
	}
	if s.cache != nil {
		if snap, ok := s.cache.SensorSnapshotByID(sensorID); ok {
			view.Cached = true
			view.Sensor.Value = snap.Value
			view.Sensor.SeenAt = snap.SeenAt
		}
	}
	return view, nil
}

// touchHeartbeat 心跳更新 + 快照刷新的公共入口。
func (s *Service) touchHeartbeat(sensor model.RopeSensor, value float64, quality model.Quality, at time.Time, batchID int64) {
	hb := model.SensorHeartbeat{SensorID: sensor.ID, Code: sensor.Code, Kind: sensor.Kind, Value: value, Quality: quality, SeenAt: at}
	_ = s.store.UpsertHeartbeat(hb)
	if s.cache != nil {
		s.cache.PutSensorSnapshot(cacheSnapshot(hb, batchID))
	}
}
