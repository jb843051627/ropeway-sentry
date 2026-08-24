package api

import (
	"net/http"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

func (s *Server) handleListHolds(w http.ResponseWriter, r *http.Request) {
	holds, err := s.svc.ListHolds(queryInt64(r, "line_id", 0))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, holds)
}

// holdRequest 维护锁创建请求体。
type holdRequest struct {
	model.MaintenanceHold
}

func (s *Server) handleCreateHold(w http.ResponseWriter, r *http.Request) {
	var in holdRequest
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	hold, err := s.svc.CreateHold(&in.MaintenanceHold)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, hold)
}

func (s *Server) handleActivateHold(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	hold, err := s.svc.ActivateHold(id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hold)
}

// releaseRequest 释放请求体，note 为必填结论。
type releaseRequest struct {
	Note string `json:"note"`
}

func (s *Server) handleReleaseHold(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	var req releaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	hold, err := s.svc.ReleaseHold(id, req.Note)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hold)
}

func (s *Server) handleListInspections(w http.ResponseWriter, r *http.Request) {
	records, err := s.svc.ListInspections(queryInt64(r, "line_id", 0), int(queryInt64(r, "limit", 100)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleCreateInspection(w http.ResponseWriter, r *http.Request) {
	var in model.InspectionRecord
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	record, err := s.svc.CreateInspection(&in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}
