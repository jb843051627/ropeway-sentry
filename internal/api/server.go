package api

import (
	"log"
	"net/http"
	"os"

	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/service"
)

// Server 聚合服务门面、指标器与静态资源目录。
type Server struct {
	svc       *service.Service
	metrics   *metrics.Metrics
	staticDir string
	logger    *log.Logger
}

// NewServer 构造 Server；staticDir 为空时跳过静态挂载。
func NewServer(svc *service.Service, m *metrics.Metrics, staticDir string) *Server {
	return &Server{svc: svc, metrics: m, staticDir: staticDir, logger: log.New(os.Stdout, "api ", log.LstdFlags)}
}

// Handler 组装全量路由（Go 1.22 增强模式 ServeMux）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	mux.HandleFunc("GET /api/lines", s.handleListLines)
	mux.HandleFunc("POST /api/lines", s.handleCreateLine)
	mux.HandleFunc("GET /api/lines/{id}", s.handleGetLine)
	mux.HandleFunc("POST /api/lines/{id}/transition", s.handleTransitionLine)

	mux.HandleFunc("GET /api/towers", s.handleListTowers)
	mux.HandleFunc("POST /api/towers", s.handleCreateTower)

	mux.HandleFunc("GET /api/sensors", s.handleListSensors)
	mux.HandleFunc("POST /api/sensors", s.handleCreateSensor)
	mux.HandleFunc("GET /api/sensors/{id}", s.handleGetSensor)
	mux.HandleFunc("POST /api/sensors/{id}/enabled", s.handleSetSensorEnabled)

	mux.HandleFunc("POST /api/telemetry/batches", s.handleIngestBatch)
	mux.HandleFunc("POST /api/telemetry/checksum", s.handleRecomputeChecksum)
	mux.HandleFunc("GET /api/telemetry/batches/{id}", s.handleGetBatch)

	mux.HandleFunc("POST /api/vibration", s.handleCreateVibration)
	mux.HandleFunc("GET /api/vibration", s.handleListVibration)
	mux.HandleFunc("POST /api/cabin-positions", s.handleCreateCabinPosition)
	mux.HandleFunc("GET /api/cabin-positions", s.handleListCabinPositions)

	mux.HandleFunc("GET /api/baselines", s.handleListBaselines)
	mux.HandleFunc("PUT /api/baselines", s.handleUpsertBaseline)
	mux.HandleFunc("POST /api/assessments", s.handleRunAssessment)
	mux.HandleFunc("GET /api/assessments", s.handleListAssessments)

	mux.HandleFunc("GET /api/alerts", s.handleListAlerts)
	mux.HandleFunc("POST /api/alerts/{id}/ack", s.handleAckAlert)
	mux.HandleFunc("POST /api/alerts/{id}/close", s.handleCloseAlert)

	mux.HandleFunc("GET /api/holds", s.handleListHolds)
	mux.HandleFunc("POST /api/holds", s.handleCreateHold)
	mux.HandleFunc("POST /api/holds/{id}/activate", s.handleActivateHold)
	mux.HandleFunc("POST /api/holds/{id}/release", s.handleReleaseHold)

	mux.HandleFunc("GET /api/inspections", s.handleListInspections)
	mux.HandleFunc("POST /api/inspections", s.handleCreateInspection)

	mux.HandleFunc("GET /export/telemetry.csv", s.handleExportTelemetry)
	mux.HandleFunc("GET /export/alerts.csv", s.handleExportAlerts)
	mux.HandleFunc("GET /api/dashboard", s.handleDashboard)

	mux.HandleFunc("GET /api/kpi/lines/{id}", s.handleLineKPI)
	mux.HandleFunc("GET /api/cabins/report", s.handleCabinReport)
	mux.HandleFunc("GET /api/frost-risk", s.handleFrostRisk)

	if s.staticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(s.staticDir)))
	}

	return withLogging(s.logger, mux)
}
