// Package service 编排全部业务用例：
// 遥测入库校验链、安全评估、告警状态机、维护锁互斥、CSV 导出与看板。
package service

import (
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/cache"
	"github.com/jb843051627/ropeway-sentry/internal/clock"
	"github.com/jb843051627/ropeway-sentry/internal/engine"
	"github.com/jb843051627/ropeway-sentry/internal/ingest"
	"github.com/jb843051627/ropeway-sentry/internal/metrics"
	"github.com/jb843051627/ropeway-sentry/internal/store"
	"github.com/jb843051627/ropeway-sentry/internal/validation"
)

// Params 业务可调参数，全部来自 config 装配。
type Params struct {
	DedupWindow time.Duration // 告警去重窗口
	Retention   time.Duration // 遥测批次保留窗
	WireWindow  time.Duration // 断丝滑动窗口长度
	Staleness   time.Duration // 心跳过期时长
	WarnTTL     time.Duration // warning 自动关闭时长
}

// DefaultParams 生产默认参数。
func DefaultParams() Params {
	return Params{
		DedupWindow: 15 * time.Minute,
		Retention:   48 * time.Hour,
		WireWindow:  2 * time.Hour,
		Staleness:   30 * time.Minute,
		WarnTTL:     24 * time.Hour,
	}
}

// Service 单体服务的门面：api 层只与本结构交互。
type Service struct {
	store    *store.Store
	clock    clock.Clock
	cache    *cache.Cache
	metrics  *metrics.Metrics
	pipeline *ingest.Pipeline
	params   Params
	icing    engine.IcingPolicy
	windTH   engine.WindThresholds
	wireTH   engine.WireThresholds
	window   validation.WindowRule
}

// New 组装服务。各协作者允许为 nil 以便裁剪部署（如无缓存）。
func New(st *store.Store, clk clock.Clock, c *cache.Cache, m *metrics.Metrics, p *ingest.Pipeline, params Params) *Service {
	if params.DedupWindow <= 0 {
		params.DedupWindow = DefaultParams().DedupWindow
	}
	if params.Retention <= 0 {
		params.Retention = DefaultParams().Retention
	}
	if params.WireWindow <= 0 {
		params.WireWindow = DefaultParams().WireWindow
	}
	if params.Staleness <= 0 {
		params.Staleness = DefaultParams().Staleness
	}
	if params.WarnTTL <= 0 {
		params.WarnTTL = DefaultParams().WarnTTL
	}
	icing := engine.DefaultIcingPolicy()
	return &Service{
		store:    st,
		clock:    clk,
		cache:    c,
		metrics:  m,
		pipeline: p,
		params:   params,
		icing:    icing,
		windTH:   icing.BaseWindThresholds(),
		wireTH:   engine.DefaultWireThresholds(),
		window:   validation.DefaultWindowRule(),
	}
}

// Now 暴露注入时钟的当前时刻。
func (s *Service) Now() time.Time { return s.clock.Now() }
