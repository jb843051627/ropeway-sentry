package service

import (
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/engine"
	"github.com/jb843051627/ropeway-sentry/internal/model"
	"github.com/jb843051627/ropeway-sentry/internal/store"
)

// maxKPIWindowDays 周报回溯天数上限，防止全表扫描。
const maxKPIWindowDays = 28

// KPIDayBucket 线路周报的单日分桶指标。
type KPIDayBucket struct {
	Date           string  `json:"date"`
	WindPeakMS     float64 `json:"wind_peak_ms"`
	TensionExceeds int64   `json:"tension_exceeds"`
	AlertCloseRate float64 `json:"alert_close_rate"`
	IntegrityRate  float64 `json:"integrity_rate"`
}

// LineKPIReport 线路周期运营安全周报：日分桶 + 汇总。
type LineKPIReport struct {
	LineID        int64          `json:"line_id"`
	LineCode      string         `json:"line_code"`
	Days          int            `json:"days"`
	GeneratedAt   time.Time      `json:"generated_at"`
	MeanWindPeak  float64        `json:"mean_wind_peak_ms"`
	TotalExceeds  int64          `json:"total_tension_exceeds"`
	CloseRate     float64        `json:"overall_alert_close_rate"`
	MeanIntegrity float64        `json:"mean_integrity_rate"`
	Buckets       []KPIDayBucket `json:"buckets"`
}

// kpiDayIndex 把聚合行按 UTC 日期组织成查找表。
func kpiDayIndexQuality(rows []store.DayQualityRow) map[string]store.DayQualityRow {
	index := make(map[string]store.DayQualityRow, len(rows))
	for _, row := range rows {
		index[row.Date] = row
	}
	return index
}

// kpiPeakIndex 按日期索引风速峰值。
func kpiPeakIndex(rows []store.DayPeakRow) map[string]float64 {
	index := make(map[string]float64, len(rows))
	for _, row := range rows {
		index[row.Date] = row.PeakValue
	}
	return index
}

// kpiFlowIndex 按日期索引告警流量。
func kpiFlowIndex(rows []store.AlertFlowRow) map[string]store.AlertFlowRow {
	index := make(map[string]store.AlertFlowRow, len(rows))
	for _, row := range rows {
		index[row.Date] = row
	}
	return index
}

// closeRatio 关闭率：closed/opened；窗口内无新增告警时视为 1（无可关闭项）。
func closeRatio(closed, opened int64) float64 {
	if opened <= 0 {
		return 1
	}
	return float64(closed) / float64(opened)
}

// LineWeeklyKPI 聚合线路近 N 天的运营安全周报：
// 按日分桶输出阵风峰值、张力偏移超限次数、告警关闭率与遥测完整率，
// 并给出周期均值汇总。days<=0 取默认 7 天，超过上限按上限截断。
func (s *Service) LineWeeklyKPI(lineID int64, days int) (LineKPIReport, error) {
	if days <= 0 {
		days = 7
	}
	if days > maxKPIWindowDays {
		days = maxKPIWindowDays
	}
	line, err := s.store.GetLine(lineID)
	if err != nil {
		return LineKPIReport{}, err
	}

	now := s.clock.Now().UTC()
	windowStart := startOfUTCDay(now).AddDate(0, 0, -(days - 1))

	allDays, err := s.store.DailyKindQuality(line.ID, "", windowStart)
	if err != nil {
		return LineKPIReport{}, err
	}
	tensionDays, err := s.store.DailyTensionExceeds(line.ID, windowStart)
	if err != nil {
		return LineKPIReport{}, err
	}
	windPeaks, err := s.store.DailyWindPeaks(line.ID, windowStart)
	if err != nil {
		return LineKPIReport{}, err
	}
	alertFlow, err := s.store.AlertDailyFlow(line.ID, windowStart)
	if err != nil {
		return LineKPIReport{}, err
	}

	integrityByDay := kpiDayIndexQuality(allDays)
	exceedsByDay := kpiDayIndexQuality(tensionDays)
	peakByDay := kpiPeakIndex(windPeaks)
	flowByDay := kpiFlowIndex(alertFlow)

	report := LineKPIReport{
		LineID:      line.ID,
		LineCode:    line.Code,
		Days:        days,
		GeneratedAt: now,
		Buckets:     make([]KPIDayBucket, 0, days),
	}
	peakSum, peakDays := 0.0, 0
	integritySum := 0.0
	var totalOpened, totalClosed int64
	for i := 0; i < days; i++ {
		date := startOfUTCDay(now).AddDate(0, 0, -(days - 1 - i)).Format("2006-01-02")
		bucket := KPIDayBucket{Date: date}
		if peak, ok := peakByDay[date]; ok && peak > bucket.WindPeakMS {
			bucket.WindPeakMS = peak
		}
		if quality, ok := exceedsByDay[date]; ok {
			bucket.TensionExceeds = quality.Suspect + quality.Rejected
		}
		if flow, ok := flowByDay[date]; ok {
			bucket.AlertCloseRate = closeRatio(flow.Closed, flow.Opened)
			totalOpened += flow.Opened
			totalClosed += flow.Closed
		} else {
			bucket.AlertCloseRate = 1
		}
		if quality, ok := integrityByDay[date]; ok {
			bucket.IntegrityRate = model.QualityTally{
				Good:     quality.Good,
				Suspect:  quality.Suspect,
				Rejected: quality.Rejected,
			}.IntegrityRate()
		} else {
			bucket.IntegrityRate = 1
		}
		if bucket.WindPeakMS > 0 {
			peakSum += bucket.WindPeakMS
			peakDays++
		}
		integritySum += bucket.IntegrityRate
		report.TotalExceeds += bucket.TensionExceeds
		report.Buckets = append(report.Buckets, bucket)
	}
	if peakDays > 0 {
		report.MeanWindPeak = peakSum / float64(peakDays)
	}
	report.MeanIntegrity = integritySum / float64(days)
	report.CloseRate = closeRatio(totalClosed, totalOpened)
	return report, nil
}

// startOfUTCDay 截取时刻所在的 UTC 零点。
func startOfUTCDay(at time.Time) time.Time {
	t := at.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// FrostOutlook 以当前季节策略评估给定气象条件下的结冰风险，
// 供运维在低温天气下决定是否提前降速或停运。
func (s *Service) FrostOutlook(tempC, humidityPct, windMS float64) (engine.FrostVerdict, error) {
	input := engine.FrostInput{TempC: tempC, HumidityPct: humidityPct, WindMS: windMS}
	if err := input.Validate(); err != nil {
		return engine.FrostVerdict{}, err
	}
	return s.icing.AssessFrost(input), nil
}
