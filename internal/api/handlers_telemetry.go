package api

import (
	"net/http"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

func (s *Server) handleIngestBatch(w http.ResponseWriter, r *http.Request) {
	var in model.BatchInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	result, err := s.svc.IngestBatch(in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRecomputeChecksum(w http.ResponseWriter, r *http.Request) {
	var in model.ChecksumInput
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.svc.RecomputeChecksum(in.Points))
}

func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	batch, err := s.svc.GetBatch(id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

// vibrationRequest 振动事件请求体（时间缺省取服务器当前）。
type vibrationRequest struct {
	model.VibrationEvent
}

func (s *Server) handleCreateVibration(w http.ResponseWriter, r *http.Request) {
	var in vibrationRequest
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	event, err := s.svc.RecordVibration(&in.VibrationEvent)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) handleListVibration(w http.ResponseWriter, r *http.Request) {
	sinceRaw := r.URL.Query().Get("since")
	since := time.Time{}
	if sinceRaw != "" {
		parsed, perr := time.Parse(time.RFC3339, sinceRaw)
		if perr != nil {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "since must be RFC3339"})
			return
		}
		since = parsed
	}
	events, err := s.svc.ListVibration(queryInt64(r, "line_id", 0), since, int(queryInt64(r, "limit", 100)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleCreateCabinPosition(w http.ResponseWriter, r *http.Request) {
	var in model.CabinPosition
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	position, err := s.svc.RecordCabinPosition(&in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, position)
}

func (s *Server) handleListCabinPositions(w http.ResponseWriter, r *http.Request) {
	positions, err := s.svc.ListCabinPositions(queryInt64(r, "line_id", 0), int(queryInt64(r, "limit", 50)))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, positions)
}
