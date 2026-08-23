package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// UpsertBaseline 写入张力基线（温度补偿期望区间）。
func (s *Service) UpsertBaseline(b *model.TensionBaseline) (model.TensionBaseline, error) {
	if err := b.Validate(); err != nil {
		return model.TensionBaseline{}, err
	}
	sensor, err := s.store.GetSensorByCode(b.SensorCode)
	if err != nil {
		return model.TensionBaseline{}, fmt.Errorf("sensor %s: %w", b.SensorCode, err)
	}
	if sensor.LineID != b.LineID || sensor.Kind != model.KindTension {
		return model.TensionBaseline{}, fmt.Errorf("%w: sensor %s is not a tension sensor of line %d",
			model.ErrOrphanSensor, b.SensorCode, b.LineID)
	}
	if err := s.store.UpsertBaseline(b); err != nil {
		return model.TensionBaseline{}, err
	}
	list, err := s.store.ListBaselines(b.LineID)
	if err != nil {
		return model.TensionBaseline{}, err
	}
	for _, item := range list {
		if item.SensorCode == b.SensorCode && item.ValidFrom.Equal(b.ValidFrom.UTC()) {
			return item, nil
		}
	}
	return *b, nil
}

// ListBaselines 查询线路张力基线。
func (s *Service) ListBaselines(lineID int64) ([]model.TensionBaseline, error) {
	return s.store.ListBaselines(lineID)
}

// RunAssessment 执行一次线路安全评估：
// 风载 / 张力 / 断丝 / 结构四维度 → 开放等级合成 → 落库并同步线路状态。
func (s *Service) RunAssessment(lineID int64) (model.SafetyAssessment, error) {
	line, err := s.store.GetLine(lineID)
	if err != nil {
		return model.SafetyAssessment{}, err
	}
	now := s.clock.Now()
	icing := s.activeIcing(now)
	dims := []engine.Dimension{}
	notes := []string{icing.Describe()}

	windDim, windScore, windNote := s.assessWind(line.ID, icing.WindThresholds())
	dims = append(dims, windDim)
	if windNote != "" {
		notes = append(notes, windNote)
	}

	tensionDim, tensionScore, tensionNote := s.assessTension(line.ID, now)
	dims = append(dims, tensionDim)
	if tensionNote != "" {
		notes = append(notes, tensionNote)
	}

	structureDim, structureScore, structureNote := s.assessStructure(line, icing, now)
	dims = append(dims, structureDim)
	if structureNote != "" {
		notes = append(notes, structureNote)
	}

	tally, err := s.store.QualityCountsSince(line.ID, now.Add(-24*time.Hour))
	if err != nil {
		return model.SafetyAssessment{}, err
	}

	level := engine.Synthesize(dims, false)
	assessment := &model.SafetyAssessment{
		LineID:         line.ID,
		WindScore:      windScore,
		TensionScore:   tensionScore,
		StructureScore: structureScore,
		IntegrityRate:  tally.IntegrityRate(),
		Level:          level,
		IcingActive:    icing.Active,
		Notes:          engine.Explain(dims, false, level) + " | " + strings.Join(notes, "; "),
		AssessedAt:     now,
	}
	if err := s.store.InsertAssessment(assessment); err != nil {
		return *assessment, err
	}
	if line.Status != level {
		if err := s.store.UpdateLineStatus(line.ID, level, now); err != nil {
			return *assessment, err
		}
		s.cache.PutLineStatus(line.ID, level)
	}
	if s.metrics != nil {
		s.metrics.Inc(metrics.AssessmentsRun)
	}
	return *assessment, nil
}

// ListAssessments 查询线路评估历史。
func (s *Service) ListAssessments(lineID int64, limit int) ([]model.SafetyAssessment, error) {
	return s.store.ListAssessments(lineID, limit)
}

func (s *Service) assessWind(lineID int64, th engine.WindThresholds) (engine.Dimension, float64, string) {
	sensors, err := s.store.ListSensors(lineID)
	if err != nil {
		return engine.Dimension{Name: "wind", Level: engine.LevelOk}, 100, ""
	}
	speed := -1.0
	for _, sen := range sensors {
		if sen.Kind != model.KindWind || !sen.Enabled {
			continue
		}
		hb, hbErr := s.store.LatestHeartbeat(sen.ID)
		if hbErr != nil {
			continue
		}
		if hb.Value > speed {
			speed = hb.Value
		}
	}
	if speed < 0 {
		return engine.Dimension{Name: "wind", Level: engine.LevelOk}, 100, "no wind data"
	}
	verdict := engine.EvaluateWind(speed, th)
	switch {
	case verdict.Critical:
		return engine.Dimension{Name: "wind", Level: engine.LevelCritical, Detail: verdict.Detail}, 10, verdict.Detail
	case verdict.Restricted:
		return engine.Dimension{Name: "wind", Level: engine.LevelRestricted, Detail: verdict.Detail}, 55, verdict.Detail
	default:
		return engine.Dimension{Name: "wind", Level: engine.LevelOk, Detail: verdict.Detail}, 100, verdict.Detail
	}
}

func (s *Service) assessTension(lineID int64, now time.Time) (engine.Dimension, float64, string) {
	sensors, err := s.store.ListSensors(lineID)
	if err != nil {
		return engine.Dimension{Name: "tension", Level: engine.LevelOk}, 100, ""
	}
	worstLevel := engine.TensionNormal
	worstRatio := 0.0
	found := false
	for _, sen := range sensors {
		if sen.Kind != model.KindTension || !sen.Enabled {
			continue
		}
		hb, hbErr := s.store.LatestHeartbeat(sen.ID)
		if hbErr != nil {
			continue
		}
		expected, tolerance, baseErr := s.baselineFor(sen, now)
		if baseErr != nil {
			continue
		}
		offset, evalErr := engine.EvaluateTension(hb.Value, expected, tolerance)
		if evalErr != nil {
			continue
		}
		found = true
		if offset.Ratio > worstRatio {
			worstRatio = offset.Ratio
			worstLevel = offset.Level
		} else if offset.Level == engine.TensionCritical && worstLevel != engine.TensionCritical {
			worstLevel = engine.TensionCritical
		}
	}
	if !found {
		return engine.Dimension{Name: "tension", Level: engine.LevelOk}, 100, "no tension data"
	}
	dim := engine.Dimension{Name: "tension"}
	switch worstLevel {
	case engine.TensionCritical:
		dim.Level = engine.LevelCritical
	case engine.TensionSuspect:
		dim.Level = engine.LevelRestricted
	default:
		dim.Level = engine.LevelOk
	}
	note := fmt.Sprintf("worst tension ratio %.2f (%s)", worstRatio, worstLevel)
	dim.Detail = note
	return dim, engine.TensionScore(worstLevel, worstRatio), note
}

func (s *Service) assessStructure(line model.RopewayLine, icing engine.IcingPolicy, now time.Time) (engine.Dimension, float64, string) {
	inputs := engine.StructureInputs{RatedSpeedMS: line.RatedSpeedMS}

	tiltVerdicts := []engine.TiltVerdict{}
	towers, err := s.store.ListTowers(line.ID)
	if err == nil {
		sensors, _ := s.store.ListSensors(line.ID)
		tiltByTower := map[int64]float64{}
		for _, sen := range sensors {
			if sen.Kind != model.KindTilt || !sen.Enabled || sen.TowerID <= 0 {
				continue
			}
			hb, hbErr := s.store.LatestHeartbeat(sen.ID)
			if hbErr != nil {
				continue
			}
			if angle, ok := tiltByTower[sen.TowerID]; ok && angle >= hb.Value {
				continue
			}
			tiltByTower[sen.TowerID] = hb.Value
		}
		for _, tw := range towers {
			angle, ok := tiltByTower[tw.ID]
			if !ok {
				continue
			}
			verdict, vErr := engine.EvaluateTilt(tw.ID, tw.Code, angle, icing.TiltLimit(tw.TiltLimitDeg))
			if vErr == nil {
				tiltVerdicts = append(tiltVerdicts, verdict)
			}
		}
	}
	inputs.WorstTiltRatio = engine.WorstTiltRatio(tiltVerdicts)

	sev, hasVibration, err := s.store.WorstVibrationSince(line.ID, now.Add(-24*time.Hour))
	if err == nil && hasVibration {
		inputs.WorstVibration = sev
		events, _ := s.store.ListVibration(line.ID, now.Add(-24*time.Hour), 500)
		inputs.VibrationCount = len(events)
	}

	gap, hasGap, err := s.store.MinCabinGapSince(line.ID, now.Add(-time.Hour))
	if err == nil && hasGap {
		inputs.MinCabinGapM = gap
	}

	inspections, _ := s.store.CountInspectionsSince(line.ID, now.Add(-30*24*time.Hour))
	inputs.InspectionCount = int(inspections)

	verdict := engine.EvaluateStructure(inputs)
	level := engine.LevelOk
	switch verdict.Level {
	case engine.LevelCritical:
		level = engine.LevelCritical
	case engine.LevelRestricted:
		level = engine.LevelRestricted
	}
	return engine.Dimension{Name: "structure", Level: level, Detail: verdict.Detail}, verdict.Score, verdict.Detail
}
