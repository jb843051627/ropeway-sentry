// Package metrics 维护进程内计数器与仪表，并以文本格式在 /metrics 暴露。
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// 预定义指标名，避免散落魔法字符串。
const (
	BatchesAccepted  = "sentry_batches_accepted_total"
	BatchesRejected  = "sentry_batches_rejected_total"
	PointsInserted   = "sentry_points_inserted_total"
	PointsDuplicate  = "sentry_points_duplicate_total"
	AlertsRaised     = "sentry_alerts_raised_total"
	AlertsClosed     = "sentry_alerts_closed_total"
	HoldActivations  = "sentry_hold_activations_total"
	AssessmentsRun   = "sentry_assessments_run_total"
	IngestQueueDepth = "sentry_ingest_queue_depth"
	CachePurgedItems = "sentry_cache_purged_items_total"
	WatchdogTicks    = "sentry_watchdog_ticks_total"
)

// Metrics 全部指标集中在一个结构里，按名字寻址。
type Metrics struct {
	mu      sync.Mutex
	samples map[string]*sample
}

type sample struct {
	counter atomic.Int64
	gauge   atomic.Int64
	isGauge bool
	help    string
}

// New 构造并注册预定义指标。
func New() *Metrics {
	m := &Metrics{samples: make(map[string]*sample)}
	m.declare(BatchesAccepted, "telemetry batches accepted")
	m.declare(BatchesRejected, "telemetry batches rejected")
	m.declare(PointsInserted, "telemetry points inserted")
	m.declare(PointsDuplicate, "telemetry points deduplicated by INSERT OR IGNORE")
	m.declare(AlertsRaised, "alerts raised or refreshed")
	m.declare(AlertsClosed, "alerts closed")
	m.declare(HoldActivations, "maintenance holds activated")
	m.declare(AssessmentsRun, "safety assessments executed")
	m.declareGauge(IngestQueueDepth, "pending ingest post-processing tasks")
	m.declare(CachePurgedItems, "cache entries purged by janitor")
	m.declare(WatchdogTicks, "watchdog scan rounds")
	return m
}

func (m *Metrics) declare(name, help string) {
	m.samples[name] = &sample{help: help}
}

func (m *Metrics) declareGauge(name, help string) {
	m.samples[name] = &sample{help: help, isGauge: true}
}

// Inc 计数器加一。
func (m *Metrics) Inc(name string) { m.Add(name, 1) }

// Add 计数器累加。
func (m *Metrics) Add(name string, delta int64) {
	s, ok := m.samples[name]
	if !ok {
		s = &sample{help: "ad-hoc counter"}
		m.samples[name] = s
	}
	s.counter.Add(delta)
}

// SetGauge 设置仪表绝对值（如队列深度）。
func (m *Metrics) SetGauge(name string, value int64) {
	s, ok := m.samples[name]
	if !ok {
		s = &sample{help: "ad-hoc gauge", isGauge: true}
		m.samples[name] = s
	}
	s.gauge.Store(value)
}

// Snapshot 返回排序后的名称→值映射，便于测试与导出。
func (m *Metrics) Snapshot() map[string]int64 {
	out := make(map[string]int64, len(m.samples))
	names := make([]string, 0, len(m.samples))
	for name := range m.samples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := m.samples[name]
		if s.isGauge {
			out[name] = s.gauge.Load()
		} else {
			out[name] = s.counter.Load()
		}
	}
	return out
}

// Render 输出文本格式指标页（# HELP/# TYPE 行 + 样本行）。
func (m *Metrics) Render() string {
	m.mu.Lock()
	names := make([]string, 0, len(m.samples))
	type row struct {
		name  string
		help  string
		gauge bool
	}
	rows := make([]row, 0, len(m.samples))
	for name, s := range m.samples {
		names = append(names, name)
		rows = append(rows, row{name: name, help: s.help, gauge: s.isGauge})
	}
	m.mu.Unlock()
	sort.Strings(names)

	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "# HELP %s %s\n", r.name, r.help)
		kind := "counter"
		if r.gauge {
			kind = "gauge"
		}
		fmt.Fprintf(&b, "# TYPE %s %s\n", r.name, kind)
	}
	for _, name := range names {
		s := m.samples[name]
		var v int64
		if s.isGauge {
			v = s.gauge.Load()
		} else {
			v = s.counter.Load()
		}
		fmt.Fprintf(&b, "%s %d\n", name, v)
	}
	return b.String()
}
