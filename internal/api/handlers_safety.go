package api

import (
	"net/http"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

func (s *Server) handleListBaselines(w http.ResponseWriter, r *http.Request) {
	baselines, err := s.svc.ListBaselines(queryInt64(r, "line_id", 0))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, baselines)
}

func (s *Server) handleUpsertBaseline(w http.ResponseWriter, r *http.Request) {
	var in model.TensionBaseline
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	baseline, err := s.svc.UpsertBaseline(&in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, baseline)
}

// assessmentRequest 评估请求体（可指定线路，缺省评估全部）。
type assessmentRequest struct {
	LineID int64 `json:"line_id"`
}

func (s *Server) handleRunAssessment(w http.ResponseWriter, r *http.Request) {
	var req assessmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	if req.LineID > 0 {
		assessment, err := s.svc.RunAssessment(req.LineID)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, assessment)
		return
	}
	lines, err := s.svc.ListLines()
	if err != nil {
		fail(w, err)
		return
	}
	results := make([]model.SafetyAssessment, 0, len(lines))
	for _, line := range lines {
		assessment, aErr := s.svc.RunAssessment(line.ID)
		if aErr != nil {
			continue
		}
		results = append(results, assessment)
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleListAssessments(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListAssessments(queryInt64(r, "line_id", 0), int(queryInt64(r, "limit", 20)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// alertAction 告警操作请求体。
type alertAction struct {
	By   string `json:"by"`
	Note string `json:"note"`
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := s.svc.ListAlerts(r.URL.Query().Get("status"), int(queryInt64(r, "limit", 100)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

func (s *Server) handleAckAlert(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	var req alertAction
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	alert, err := s.svc.AckAlert(id, req.By)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alert)
}

func (s *Server) handleCloseAlert(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	var req alertAction
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	alert, err := s.svc.CloseAlert(id, req.Note)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alert)
}
