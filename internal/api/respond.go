// Package api 提供 HTTP 路由、编解码与全部 endpoint 处理器。
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// maxBodyBytes 限制请求体大小，防御异常大包。
const maxBodyBytes = 4 << 20

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

// errorJSON 统一错误响应体。
type errorJSON struct {
	Error string `json:"error"`
}

// fail 按错误语义映射 HTTP 状态码并输出。
func fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrConflict),
		errors.Is(err, model.ErrChecksumMismatch),
		errors.Is(err, model.ErrFutureWindow),
		errors.Is(err, model.ErrExpiredWindow),
		errors.Is(err, model.ErrBadWindow),
		errors.Is(err, model.ErrDisabledSensor),
		errors.Is(err, model.ErrOrphanSensor),
		errors.Is(err, model.ErrInvalidTransition),
		errors.Is(err, model.ErrHoldMutex),
		errors.Is(err, model.ErrAckRequired),
		errors.Is(err, model.ErrEmptyBatch):
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, errorJSON{Error: err.Error()})
}

// decodeJSON 解析请求体到 out；超限与格式错误返回 400。
func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}

// pathID 解析路径参数为 int64。
func pathID(r *http.Request, name string) (int64, error) {
	raw := r.PathValue(name)
	var id int64
	if raw == "" {
		return 0, errors.New("missing path parameter " + name)
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, errors.New("path parameter " + name + " must be an integer")
		}
		id = id*10 + int64(ch-'0')
	}
	return id, nil
}
