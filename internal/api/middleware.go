package api

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder 记录响应码供访问日志使用。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush 透传底层 Flush（CSV 流式输出需要）。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// withLogging 统一访问日志与 异常兜底。
func withLogging(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				logger.Printf("panic %s %s: %v", r.Method, r.URL.Path, p)
				writeJSON(rec, http.StatusInternalServerError, errorJSON{Error: "internal panic"})
			}
			logger.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
		}()
		next.ServeHTTP(rec, r)
	})
}
