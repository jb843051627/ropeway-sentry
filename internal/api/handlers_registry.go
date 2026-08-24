package api

import (
	"net/http"
	"strconv"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// queryInt64 解析查询参数为 int64，缺省返回 fallback。
func queryInt64(r *http.Request, name string, fallback int64) int64 {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func (s *Server) handleListLines(w http.ResponseWriter, r *http.Request) {
	lines, err := s.svc.ListLines()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) handleCreateLine(w http.ResponseWriter, r *http.Request) {
	var in model.RopewayLine
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	line, err := s.svc.CreateLine(&in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, line)
}

func (s *Server) handleGetLine(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	line, err := s.svc.GetLine(id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, line)
}

// transitionRequest 状态迁移请求体。
type transitionRequest struct {
	To string `json:"to"`
}

func (s *Server) handleTransitionLine(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	var req transitionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	line, err := s.svc.TransitionLine(id, req.To)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, line)
}

func (s *Server) handleListTowers(w http.ResponseWriter, r *http.Request) {
	towers, err := s.svc.ListTowers(queryInt64(r, "line_id", 0))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, towers)
}

func (s *Server) handleCreateTower(w http.ResponseWriter, r *http.Request) {
	var in model.Tower
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	tower, err := s.svc.CreateTower(&in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tower)
}

func (s *Server) handleListSensors(w http.ResponseWriter, r *http.Request) {
	sensors, err := s.svc.ListSensors(queryInt64(r, "line_id", 0))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sensors)
}

func (s *Server) handleCreateSensor(w http.ResponseWriter, r *http.Request) {
	var in model.RopeSensor
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	sensor, err := s.svc.CreateSensor(&in)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sensor)
}

func (s *Server) handleGetSensor(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	sensor, err := s.svc.GetSensor(id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sensor)
}

// enabledRequest 启停请求体。
type enabledRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSetSensorEnabled(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	var req enabledRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error()})
		return
	}
	if err := s.svc.SetSensorEnabled(id, req.Enabled); err != nil {
		fail(w, err)
		return
	}
	sensor, err := s.svc.GetSensor(id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sensor)
}
