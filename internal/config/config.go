// Package config 负责从环境变量装配进程运行参数，
// 全部字段都带默认值，保证零配置也能启动。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 汇总服务启动所需的全部配置项。
type Config struct {
	// DBPath 磁盘 SQLite 文件路径，拒绝内存库。
	DBPath string
	// Addr HTTP 监听地址。
	Addr string
	// StaticDir 内嵌值守控制台静态资源目录。
	StaticDir string
	// DedupWindow 告警去重窗口。
	DedupWindow time.Duration
	// TelemetryRetention 遥测批次时间窗回溯上限，超过视为过期拒收。
	TelemetryRetention time.Duration
	// HeartbeatStale 心跳判定的超时时长。
	HeartbeatStale time.Duration
	// WarningAutoClose warning 告警自动关闭的时长。
	WarningAutoClose time.Duration
	// WatchdogInterval 后台看护扫描间隔。
	WatchdogInterval time.Duration
	// IngestQueueSize 异步后处理队列容量。
	IngestQueueSize int
	// WireRateLimit 断丝滑动窗口速率告警阈值（根/小时）。
	WireRateLimit float64
}

// Load 读取环境变量并填充默认值。
func Load() Config {
	return Config{
		DBPath:             envString("ROPEWAY_SENTRY_DB", "data/ropeway-sentry.db"),
		Addr:               envString("ROPEWAY_SENTRY_ADDR", "127.0.0.1:8942"),
		StaticDir:          envString("ROPEWAY_SENTRY_STATIC", "web/static"),
		DedupWindow:        envDuration("ROPEWAY_SENTRY_DEDUP_WINDOW", 15*time.Minute),
		TelemetryRetention: envDuration("ROPEWAY_SENTRY_RETENTION", 48*time.Hour),
		HeartbeatStale:     envDuration("ROPEWAY_SENTRY_STALE", 30*time.Minute),
		WarningAutoClose:   envDuration("ROPEWAY_SENTRY_WARN_TTL", 24*time.Hour),
		WatchdogInterval:   envDuration("ROPEWAY_SENTRY_WATCHDOG_INTERVAL", 30*time.Second),
		IngestQueueSize:    envInt("ROPEWAY_SENTRY_QUEUE_SIZE", 256),
		WireRateLimit:      float64(envInt("ROPEWAY_SENTRY_WIRE_RATE", 6)),
	}
}

// Validate 对配置做基础一致性检查。
func (c Config) Validate() error {
	if strings.TrimSpace(c.DBPath) == "" {
		return fmt.Errorf("db path must not be empty")
	}
	if strings.Contains(c.DBPath, ":memory:") {
		return fmt.Errorf("in-memory sqlite is not allowed: %s", c.DBPath)
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("listen addr must not be empty")
	}
	if c.IngestQueueSize <= 0 {
		return fmt.Errorf("ingest queue size must be positive: %d", c.IngestQueueSize)
	}
	return nil
}

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
