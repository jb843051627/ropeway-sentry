package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/store"
	"github.com/jb843051627/ropeway-sentry/internal/validation"
)

// storePointRow 落库行别名，隔离 store 细节。
type storePointRow = store.StoredPointRow

func (s *Service) insertPoints(batchID int64, rows []storePointRow) (int64, error) {
	return s.store.InsertPointsORIgnore(batchID, rows)
}

func (s *Service) reject() {
	if s.metrics != nil {
		s.metrics.Inc(metrics.BatchesRejected)
	}
}

func (s *Service) countAccepted(inserted, dupes int64) {
	if s.metrics == nil {
		return
	}
	s.metrics.Inc(metrics.BatchesAccepted)
	s.metrics.Add(metrics.PointsInserted, inserted)
	s.metrics.Add(metrics.PointsDuplicate, dupes)
}

// tallyToMap 三级计数转响应映射。
func tallyToMap(t validation.QualityTally) map[model.Quality]int64 {
	return map[model.Quality]int64{
		model.QualityGood:     t.Good,
		model.QualitySuspect:  t.Suspect,
		model.QualityRejected: t.Rejected,
	}
}

// raiseAlert 告警去重窗口内的同键告警只累加次数不新建记录。
// 返回 true 表示新建了告警。
func (s *Service) raiseAlert(lineID, sensorID int64, kind string, severity model.AlertSeverity, message string) (bool, error) {
	dedupKey := fmt.Sprintf("L%d|%s", lineID, kind)
	if sensorID > 0 {
		dedupKey = fmt.Sprintf("%s|S%d", dedupKey, sensorID)
	}
	now := s.clock.Now()
	candidate, err := s.store.FindDedupCandidate(dedupKey, now.Add(-s.params.DedupWindow))
	switch {
	case err == nil:
		return false, s.store.TouchAlert(candidate.ID, candidate.Occurrences+1, now)
	case !errors.Is(err, model.ErrNotFound):
		return false, err
	}
	alert := &model.Alert{
		LineID:       lineID,
		SensorID:     sensorID,
		DedupKey:     dedupKey,
		Kind:         kind,
		Severity:     severity,
		Message:      message,
		Status:       model.AlertOpen,
		FirstSeenAt:  now,
		LatestSeenAt: now,
	}
	if err := s.store.InsertAlert(alert); err != nil {
		return false, err
	}
	if s.metrics != nil {
		s.metrics.Inc(metrics.AlertsRaised)
	}
	log.Printf("alert raised line=%d kind=%s severity=%s", lineID, kind, severity)
	return true, nil
}

// ListAlerts 列出告警；status 为空返回全部。
func (s *Service) ListAlerts(status string, limit int) ([]model.Alert, error) {
	if status != "" {
		if _, err := parseStatusFilter(status); err != nil {
			return nil, err
		}
	}
	return s.store.ListAlerts(status, limit)
}

func parseStatusFilter(raw string) (model.AlertStatus, error) {
	switch model.AlertStatus(raw) {
	case model.AlertOpen, model.AlertAcked, model.AlertClosed:
		return model.AlertStatus(raw), nil
	default:
		return "", fmt.Errorf("unknown alert status filter %q", raw)
	}
}

// AckAlert open → acked。重复 ack 视为冲突。
func (s *Service) AckAlert(id int64, by string) (model.Alert, error) {
	alert, err := s.store.GetAlert(id)
	if err != nil {
		return alert, err
	}
	if alert.Status != model.AlertOpen {
		return alert, fmt.Errorf("%w: alert %d is %s, expected open", model.ErrConflict, id, alert.Status)
	}
	if err := s.store.AckAlert(id, by, s.clock.Now()); err != nil {
		return alert, err
	}
	return s.store.GetAlert(id)
}

// CloseAlert 关闭告警：critical 必须先 ack；非 critical 允许直接关闭。
func (s *Service) CloseAlert(id int64, note string) (model.Alert, error) {
	alert, err := s.store.GetAlert(id)
	if err != nil {
		return alert, err
	}
	if alert.Status == model.AlertClosed {
		return alert, fmt.Errorf("%w: alert %d already closed", model.ErrConflict, id)
	}
	if alert.Severity == model.SeverityCritical && alert.Status != model.AlertAcked {
		return alert, fmt.Errorf("%w: alert %d", model.ErrAckRequired, id)
	}
	if err := s.store.CloseAlert(id, note, s.clock.Now()); err != nil {
		return alert, err
	}
	if s.metrics != nil {
		s.metrics.Inc(metrics.AlertsClosed)
	}
	return s.store.GetAlert(id)
}

// evaluateThresholds 遍历线路全部启用传感器做阈值越限判定。
// 单个传感器失败不中断整批入库链，只记日志。
func (s *Service) evaluateThresholds(lineID int64, now time.Time) ([]string, error) {
	sensors, err := s.store.ListSensors(lineID)
	if err != nil {
		return nil, err
	}
	var fresh []string
	for _, sen := range sensors {
		if !sen.Enabled {
			continue
		}
		kind, err := s.checkSensorThreshold(sen, now)
		if err != nil {
			log.Printf("threshold check deferred sensor=%s: %v", sen.Code, err)
			continue
		}
		if kind != "" {
			fresh = append(fresh, kind)
		}
	}
	return fresh, nil
}

// checkSensorThreshold 按传感器种类分发到对应判据；返回新建告警的键（无则空）。
func (s *Service) checkSensorThreshold(sen model.RopeSensor, now time.Time) (string, error) {
	hb, err := s.store.LatestHeartbeat(sen.ID)
	if err != nil {
		return "", err
	}
	switch sen.Kind {
	case model.KindWind:
		return s.checkWind(sen, hb.Value, now)
	case model.KindTension:
		return s.checkTension(sen, hb.Value, now)
	case model.KindWireBreak:
		return s.checkWires(sen, hb.Value, now)
	case model.KindTilt:
		return s.checkTilt(sen, hb.Value, now)
	default:
		return "", nil // flux 只参与质量统计，无独立阈值
	}
}

func (s *Service) checkWind(sen model.RopeSensor, speed float64, now time.Time) (string, error) {
	verdict := engine.EvaluateWind(speed, s.activeIcing(now).WindThresholds())
	switch {
	case verdict.Critical:
		created, err := s.raiseAlert(sen.LineID, sen.ID, "wind_critical", model.SeverityCritical, verdict.Detail)
		if created {
			return "wind_critical", err
		}
		return "", err
	case verdict.Restricted:
		created, err := s.raiseAlert(sen.LineID, sen.ID, "wind_restricted", model.SeverityWarning, verdict.Detail)
		if created {
			return "wind_restricted", err
		}
		return "", err
	default:
		return "", nil
	}
}

// tensionDetail 把张力偏移结论格式化为告警文本。
func tensionDetail(o engine.TensionOffset) string {
	return fmt.Sprintf("tension %.0fN vs baseline %.0fN (ratio %.2f, level %s)", o.MeasuredN, o.ExpectedN, o.Ratio, o.Level)
}

func (s *Service) checkTension(sen model.RopeSensor, measuredN float64, now time.Time) (string, error) {
	expected, tolerance, err := s.baselineFor(sen, now)
	if err != nil {
		return "", err
	}
	offset, err := engine.EvaluateTension(measuredN, expected, tolerance)
	if err != nil {
		return "", err
	}
	detail := tensionDetail(offset)
	switch offset.Level {
	case engine.TensionCritical:
		created, err := s.raiseAlert(sen.LineID, sen.ID, "tension_offset", model.SeverityCritical, detail)
		if created {
			return "tension_offset", err
		}
		return "", err
	case engine.TensionSuspect:
		created, err := s.raiseAlert(sen.LineID, sen.ID, "tension_drift", model.SeverityWarning, detail)
		if created {
			return "tension_drift", err
		}
		return "", err
	default:
		return "", nil
	}
}

func (s *Service) checkWires(sen model.RopeSensor, latestCumulative float64, now time.Time) (string, error) {
	points, err := s.store.RecentSensorPoints(sen.ID, now.Add(-s.params.WireWindow), 500)
	if err != nil {
		return "", err
	}
	if len(points) < 2 {
		return "", nil
	}
	samples := engine.PointsToWireSamples(points)
	verdict := engine.EvaluateWires(samples, int(latestCumulative), s.params.WireWindow, now, s.wireTH)
	if !verdict.Critical {
		return "", nil
	}
	created, err := s.raiseAlert(sen.LineID, sen.ID, "wire_growth", model.SeverityCritical, verdict.Detail)
	if created {
		return "wire_growth", err
	}
	return "", err
}

func (s *Service) checkTilt(sen model.RopeSensor, angleDeg float64, now time.Time) (string, error) {
	if sen.TowerID <= 0 {
		return "", errors.New("tilt sensor not bound to a tower")
	}
	tower, err := s.store.GetTower(sen.TowerID)
	if err != nil {
		return "", err
	}
	limit := s.activeIcing(now).TiltLimit(tower.TiltLimitDeg)
	verdict, err := engine.EvaluateTilt(tower.ID, tower.Code, angleDeg, limit)
	if err != nil {
		return "", err
	}
	switch {
	case verdict.Critical:
		created, err := s.raiseAlert(sen.LineID, sen.ID, "tilt_exceed", model.SeverityCritical, verdict.Detail)
		if created {
			return "tilt_exceed", err
		}
		return "", err
	case verdict.Suspect:
		created, err := s.raiseAlert(sen.LineID, sen.ID, "tilt_watch", model.SeverityWarning, verdict.Detail)
		if created {
			return "tilt_watch", err
		}
		return "", err
	default:
		return "", nil
	}
}

// baselineFor 取传感器生效基线：优先张力基线表（含温度补偿），回落档案容差。
func (s *Service) baselineFor(sen model.RopeSensor, now time.Time) (expected, tolerance float64, err error) {
	baseline, bErr := s.store.ActiveBaselineForSensor(sen.LineID, sen.Code, now)
	if bErr == nil {
		return baseline.EffectiveExpected(), baseline.ToleranceN, nil
	}
	if !errors.Is(bErr, model.ErrNotFound) {
		return 0, 0, bErr
	}
	return sen.ExpectedValue, sen.Tolerance, nil
}

// activeIcing 依据注入时钟解析结冰季策略。
func (s *Service) activeIcing(now time.Time) engine.IcingPolicy {
	return s.icing.ResolveForTime(now)
}
