// ropeway-sentry 客运索道钢丝绳与支架安全监测后端。
//
// 单体单进程：磁盘 SQLite（modernc 驱动）+ HTTP API + 内嵌值守控制台。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/api"
	"github.com/jb843051627/ropeway-sentry/internal/cache"
	"github.com/jb843051627/ropeway-sentry/internal/clock"
	"github.com/jb843051627/ropeway-sentry/internal/config"
	"github.com/jb843051627/ropeway-sentry/internal/ingest"
	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/service"
	"github.com/jb843051627/ropeway-sentry/internal/store"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	m := metrics.New()
	snapshotCache := cache.New(5 * time.Minute)
	stopJanitor := snapshotCache.StartJanitor(time.Minute)
	defer func() { <-stopJanitor }()

	pipeline := ingest.New(cfg.IngestQueueSize, m, log.Default())
	defer pipeline.Close()

	params := service.Params{
		DedupWindow: cfg.DedupWindow,
		Retention:   cfg.TelemetryRetention,
		WireWindow:  2 * time.Hour,
		Staleness:   cfg.HeartbeatStale,
		WarnTTL:     cfg.WarningAutoClose,
	}
	svc := service.New(st, clock.System{}, snapshotCache, m, pipeline, params)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	watchdogDone := svc.StartWatchdog(ctx, cfg.WatchdogInterval)

	server := api.NewServer(svc, m, cfg.StaticDir)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("ropeway-sentry listening on %s (db=%s)", cfg.Addr, cfg.DBPath)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
	<-watchdogDone
}
