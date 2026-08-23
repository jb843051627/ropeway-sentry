package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "now": s.svc.Now().Format(time.RFC3339)})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(s.metrics.Render()))
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	board, err := s.svc.Dashboard()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, board)
}

func (s *Server) handleExportTelemetry(w http.ResponseWriter, r *http.Request) {
	lineID := queryInt64(r, "line_id", 0)
	if lineID > 0 {
		// 在写出响应头前完成存在性校验，避免半途失败只能截断。
		if _, err := s.svc.GetLine(lineID); err != nil {
			fail(w, err)
			return
		}
	}
	since := time.Now().Add(-24 * time.Hour)
	if raw := r.URL.Query().Get("hours"); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours <= 0 || hours > 24*30 {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "hours must be a positive integer <= 720"})
			return
		}
		since = s.svc.Now().Add(-time.Duration(hours) * time.Hour)
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"telemetry_line_%d.csv\"", lineID))
	if err := s.svc.ExportTelemetryCSV(w, lineID, since, int(queryInt64(r, "limit", 5000))); err != nil {
		// 响应头已发出，只能截断并记录；正常路径下错误极少发生。
		s.logger.Printf("telemetry export failed line=%d: %v", lineID, err)
	}
}

func (s *Server) handleExportAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"alerts.csv\"")
	if err := s.svc.ExportAlertsCSV(w, int(queryInt64(r, "limit", 2000))); err != nil {
		s.logger.Printf("alerts export failed: %v", err)
	}
}

// queryFloat64 解析查询参数为 float64，缺省或非法时返回 fallback。
func queryFloat64(r *http.Request, name string, fallback float64) float64 {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

func (s *Server) handleLineKPI(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	report, err := s.svc.LineWeeklyKPI(id, int(queryInt64(r, "days", 7)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleCabinReport(w http.ResponseWriter, r *http.Request) {
	report, err := s.svc.CabinSpacingReport(queryInt64(r, "line_id", 0), int(queryInt64(r, "limit", 200)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleFrostRisk(w http.ResponseWriter, r *http.Request) {
	verdict, err := s.svc.FrostOutlook(
		queryFloat64(r, "temp_c", 0),
		queryFloat64(r, "humidity_pct", 0),
		queryFloat64(r, "wind_ms", 0),
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, verdict)
}
