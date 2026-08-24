package service

import (
	"fmt"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// RecordVibration 登记振动事件并联动告警。
func (s *Service) RecordVibration(v *model.VibrationEvent) (model.VibrationEvent, error) {
	if err := v.Validate(); err != nil {
		return *v, err
	}
	if _, err := s.store.GetLine(v.LineID); err != nil {
		return *v, fmt.Errorf("line %d: %s", v.LineID, err)
	}
	tower, err := s.store.GetTower(v.TowerID)
	if err != nil {
		return *v, fmt.Errorf("tower %d: %w", v.TowerID, err)
	}
	if tower.LineID != v.LineID {
		return *v, fmt.Errorf("%w: tower %d on line %d", model.ErrOrphanSensor, v.TowerID, tower.LineID)
	}
	if v.OccurredAt.IsZero() {
		v.OccurredAt = s.clock.Now()
	}
	if err := s.store.InsertVibration(v); err != nil {
		return *v, err
	}
	if v.Severity == model.SeverityCritical {
		_, _ = s.raiseAlert(v.LineID, 0, "vibration_critical", model.SeverityCritical,
			fmt.Sprintf("tower %s peak %.2f m/s2", tower.Code, v.PeakAccelMS2))
	}
	return *v, nil
}

// ListVibration 查询振动事件。
func (s *Service) ListVibration(lineID int64, since time.Time, limit int) ([]model.VibrationEvent, error) {
	return s.store.ListVibration(lineID, since, limit)
}

// RecordCabinPosition 记录载客舱定位并做间距安全检查。
func (s *Service) RecordCabinPosition(c *model.CabinPosition) (model.CabinPosition, error) {
	if err := c.Validate(); err != nil {
		return *c, err
	}
	line, err := s.store.GetLine(c.LineID)
	if err != nil {
		return *c, fmt.Errorf("line %d: %w", c.LineID, err)
	}
	if line.LengthM > 0 && c.SectionM > line.LengthM {
		return *c, fmt.Errorf("%w: section %.1fm beyond line length %.1fm", model.ErrConflict, c.SectionM, line.LengthM)
	}
	maxSpeed := line.RatedSpeedMS * 1.15
	if c.SpeedMS > maxSpeed {
		return *c, fmt.Errorf("%w: speed %.2f exceeds rated %.2f (+15%%)", model.ErrConflict, c.SpeedMS, line.RatedSpeedMS)
	}
	if c.RecordedAt.IsZero() {
		c.RecordedAt = s.clock.Now()
	}
	if err := s.store.InsertCabinPosition(c); err != nil {
		return *c, err
	}
	safeGap := line.RatedSpeedMS * 2.0
	if c.GapToPrevM > 0 && c.GapToPrevM < safeGap {
		_, _ = s.raiseAlert(c.LineID, 0, "cabin_gap", model.SeverityWarning,
			fmt.Sprintf("cabin %s gap %.1fm below safe %.1fm", c.CabinNo, c.GapToPrevM, safeGap))
	}
	return *c, nil
}

// ListCabinPositions 查询最近定位快照。
func (s *Service) ListCabinPositions(lineID int64, limit int) ([]model.CabinPosition, error) {
	return s.store.ListCabinPositions(lineID, limit)
}

// CreateHold 登记维护锁（planned）。
func (s *Service) CreateHold(h *model.MaintenanceHold) (model.MaintenanceHold, error) {
	if h.Reason == "" || h.Operator == "" {
		return model.MaintenanceHold{}, fmt.Errorf("hold reason and operator are required")
	}
	if _, err := s.store.GetLine(h.LineID); err != nil {
		return model.MaintenanceHold{}, fmt.Errorf("line %d: %w", h.LineID, err)
	}
	if h.PlannedAt.IsZero() {
		h.PlannedAt = s.clock.Now()
	}
	if err := s.store.CreateHold(h); err != nil {
		return model.MaintenanceHold{}, err
	}
	return *h, nil
}

// ActivateHold 激活维护锁。互斥语义：同一线路同时至多一个 active，
// 已有 active 锁时拒绝并返回 ErrHoldMutex；激活后线路切入 maintenance。
func (s *Service) ActivateHold(id int64) (model.MaintenanceHold, error) {
	hold, err := s.store.GetHold(id)
	if err != nil {
		return hold, err
	}
	if hold.Status != model.HoldPlanned {
		return hold, fmt.Errorf("%w: hold %d is %s", model.ErrConflict, id, hold.Status)
	}
	if _, err := s.store.ActiveHoldForLine(hold.LineID); err == nil {
		return hold, fmt.Errorf("%w: line %d", model.ErrHoldMutex, hold.LineID)
	} else if err != model.ErrNotFound {
		return hold, err
	}
	now := s.clock.Now()
	if err := s.store.ActivateHold(id, now); err != nil {
		return hold, err
	}
	if s.metrics != nil {
		s.metrics.Inc(metrics.HoldActivations)
	}
	line, lErr := s.store.GetLine(hold.LineID)
	if lErr == nil && line.Status != model.LineMaintenance &&
		model.CanTransition(line.Status, model.LineMaintenance) {
		_ = s.store.UpdateLineStatus(line.ID, model.LineMaintenance, now)
		s.cache.PutLineStatus(line.ID, model.LineMaintenance)
	}
	return s.store.GetHold(id)
}

// ReleaseHold 释放维护锁：active 态必须携带结论文本；
// 释放后线路等级交由下一次评估合成，不自动回 open。
func (s *Service) ReleaseHold(id int64, note string) (model.MaintenanceHold, error) {
	hold, err := s.store.GetHold(id)
	if err != nil {
		return hold, err
	}
	if hold.Status != model.HoldActive {
		return hold, fmt.Errorf("%w: hold %d is %s", model.ErrConflict, id, hold.Status)
	}
	if note == "" {
		return hold, fmt.Errorf("%w: release note (conclusion) is required", model.ErrConflict)
	}
	if err := s.store.ReleaseHold(id, note, s.clock.Now()); err != nil {
		return hold, err
	}
	return s.store.GetHold(id)
}

// ListHolds 查询维护锁。
func (s *Service) ListHolds(lineID int64) ([]model.MaintenanceHold, error) {
	return s.store.ListHolds(lineID)
}

// CreateInspection 写入巡检记录（目检/探伤）。
func (s *Service) CreateInspection(r *model.InspectionRecord) (model.InspectionRecord, error) {
	if r.LineID <= 0 || r.Conclusion == "" || r.Inspector == "" {
		return model.InspectionRecord{}, fmt.Errorf("line, conclusion and inspector are required")
	}
	if _, err := model.ParseInspectionKind(string(r.Kind)); err != nil {
		return model.InspectionRecord{}, err
	}
	if _, err := s.store.GetLine(r.LineID); err != nil {
		return model.InspectionRecord{}, fmt.Errorf("line %d: %w", r.LineID, err)
	}
	if r.TowerID > 0 {
		tower, err := s.store.GetTower(r.TowerID)
		if err != nil {
			return model.InspectionRecord{}, fmt.Errorf("tower %d: %w", r.TowerID, err)
		}
		if tower.LineID != r.LineID {
			return model.InspectionRecord{}, fmt.Errorf("%w: tower %d on line %d", model.ErrOrphanSensor, r.TowerID, tower.LineID)
		}
	}
	if r.InspectedAt.IsZero() {
		r.InspectedAt = s.clock.Now()
	}
	if err := s.store.InsertInspection(r); err != nil {
		return model.InspectionRecord{}, err
	}
	return *r, nil
}

// ListInspections 查询巡检记录。
func (s *Service) ListInspections(lineID int64, limit int) ([]model.InspectionRecord, error) {
	return s.store.ListInspections(lineID, limit)
}
