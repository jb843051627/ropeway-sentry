package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// StartWatchdog 启动后台看护协程：
// 周期扫描心跳过期的启用传感器并告警；自动关闭超时未处置的 warning。
// 通过 ctx 取消退出，返回等待退出的 done 通道。
func (s *Service) StartWatchdog(ctx context.Context, interval time.Duration) (done <-chan struct{}) {
	if interval <= 0 {
		interval = time.Minute
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.watchdogTick()
			}
		}
	}()
	return finished
}

// watchdogTick 单轮扫描：心跳过期 → 告警过期 → 指标回填。
func (s *Service) watchdogTick() {
	now := s.clock.Now()
	if s.metrics != nil {
		s.metrics.Inc(metrics.WatchdogTicks)
	}

	stale, err := s.store.ListStaleSensors(now.Add(-s.params.Staleness))
	if err != nil {
		log.Printf("watchdog: stale scan failed: %v", err)
	} else {
		for _, item := range stale {
			message := "heartbeat stale"
			if !item.SeenAt.IsZero() {
				message = fmt.Sprintf("heartbeat stale since %s", item.SeenAt.UTC().Format(time.RFC3339))
			}
			if _, rErr := s.raiseAlert(item.LineID, item.SensorID, "heartbeat_lost", model.SeverityWarning, message); rErr != nil {
				log.Printf("watchdog: raise heartbeat alert sensor=%s: %v", item.Code, rErr)
			}
		}
	}

	closed, err := s.store.AutoCloseStaleWarnings(now.Add(-s.params.WarnTTL), now)
	if err != nil {
		log.Printf("watchdog: auto-close failed: %v", err)
		return
	}
	if closed > 0 && s.metrics != nil {
		s.metrics.Add(metrics.AlertsClosed, closed)
	}
}
