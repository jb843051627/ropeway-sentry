package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// ExportTelemetryCSV 以 UTC 时间导出遥测点。
// lineID>0 时限定单条线路；lineID<=0 表示跨全部线路。
// 表头固定，时间列统一 RFC3339 UTC，便于下游按字符串排序。
func (s *Service) ExportTelemetryCSV(w io.Writer, lineID int64, since time.Time, limit int) error {
	if lineID > 0 {
		if _, err := s.store.GetLine(lineID); err != nil {
			return fmt.Errorf("line %d: %w", lineID, err)
		}
	}
	points, err := s.store.RecentLinePoints(lineID, since, limit)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"batch_id", "sensor_code", "seq", "taken_at_utc", "value", "quality"}); err != nil {
		return err
	}
	for _, p := range points {
		record := []string{
			fmt.Sprintf("%d", p.BatchID),
			p.SensorCode,
			fmt.Sprintf("%d", p.Seq),
			p.TakenAt.UTC().Format(time.RFC3339Nano),
			fmt.Sprintf("%.6f", p.Value),
			string(p.Quality),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ExportAlertsCSV 以 UTC 时间导出告警流水。
func (s *Service) ExportAlertsCSV(w io.Writer, limit int) error {
	alerts, err := s.store.ListAlerts("", limit)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "line_id", "kind", "severity", "status", "occurrences",
		"first_seen_at_utc", "latest_seen_at_utc", "acked_by", "closed_at_utc", "message"}); err != nil {
		return err
	}
	for _, a := range alerts {
		closedAt := ""
		if a.ClosedAt != nil {
			closedAt = a.ClosedAt.UTC().Format(time.RFC3339Nano)
		}
		record := []string{
			fmt.Sprintf("%d", a.ID),
			fmt.Sprintf("%d", a.LineID),
			a.Kind,
			string(a.Severity),
			string(a.Status),
			fmt.Sprintf("%d", a.Occurrences),
			a.FirstSeenAt.UTC().Format(time.RFC3339Nano),
			a.LatestSeenAt.UTC().Format(time.RFC3339Nano),
			a.AckedBy,
			closedAt,
			a.Message,
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// Dashboard 看板总览：全局计数 + 各线路摘要。
type Dashboard struct {
	LinesTotal      int                  `json:"lines_total"`
	LinesByStatus   map[string]int       `json:"lines_by_status"`
	OpenWarnings    int64                `json:"open_warnings"`
	OpenCriticals   int64                `json:"open_criticals"`
	ActiveHolds     int                  `json:"active_holds"`
	Batches24h      int64                `json:"batches_24h"`
	IntegrityRate24 float64              `json:"integrity_rate_24h"`
	GeneratedAt     string               `json:"generated_at"`
	Lines           []DashboardLineEntry `json:"lines"`
}

// DashboardLineEntry 单条线路的看板行。
type DashboardLineEntry struct {
	ID        int64            `json:"id"`
	Code      string           `json:"code"`
	Name      string           `json:"name"`
	Status    model.LineStatus `json:"status"`
	Batches24 int64            `json:"batches_24h"`
	Criticals int64            `json:"critical_alerts_open"`
	HoldID    int64            `json:"active_hold_id"`
}

// Dashboard 汇总看板数据；单线查询失败不阻塞整体输出。
func (s *Service) Dashboard() (Dashboard, error) {
	now := s.clock.Now()
	since := now.Add(-24 * time.Hour)
	board := Dashboard{
		LinesByStatus: map[string]int{},
		GeneratedAt:   now.Format(time.RFC3339),
		Lines:         []DashboardLineEntry{},
	}
	lines, err := s.store.ListLines()
	if err != nil {
		return board, err
	}
	board.LinesTotal = len(lines)

	var tallyAll validationTally
	for _, line := range lines {
		board.LinesByStatus[string(line.Status)]++
		entry := DashboardLineEntry{ID: line.ID, Code: line.Code, Name: line.Name, Status: line.Status}

		batches, bErr := s.store.CountBatchesSince(line.ID, since)
		if bErr == nil {
			entry.Batches24 = batches
			board.Batches24h += batches
		}

		tally, qErr := s.store.QualityCountsSince(line.ID, since)
		if qErr == nil {
			tallyAll.add(tally)
		}

		criticals, cErr := s.store.CountOpenCriticalForLine(line.ID)
		if cErr == nil {
			entry.Criticals = criticals
		}

		if hold, hErr := s.store.ActiveHoldForLine(line.ID); hErr == nil {
			entry.HoldID = hold.ID
			board.ActiveHolds++
		}

		board.Lines = append(board.Lines, entry)
	}
	board.IntegrityRate24 = tallyAll.rate()

	warnings, criticals, err := s.store.CountOpenBySeverity()
	if err != nil {
		return board, err
	}
	board.OpenWarnings = warnings
	board.OpenCriticals = criticals
	return board, nil
}
