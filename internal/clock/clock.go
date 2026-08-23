// Package clock 提供可注入的时间源。
// 业务侧（季节结冰规则、心跳过期、去重窗口）一律通过 Clock 取当前时间，
// 便于在测试或演练场景中拨动手动时钟驱动时间相关判据。
package clock

import (
	"sync"
	"time"
)

// Clock 是全项目唯一的时间读取接口。
type Clock interface {
	Now() time.Time
}

// System 返回真实 UTC 时间。
type System struct{}

// Now 实现 Clock。
func (System) Now() time.Time { return time.Now().UTC() }

// Manual 是可手动推进的时钟，供注入场景使用。
type Manual struct {
	mu   sync.Mutex
	curr time.Time
}

// NewManual 以给定初始时刻构造手动时钟（统一转成 UTC）。
func NewManual(start time.Time) *Manual {
	return &Manual{curr: start.UTC()}
}

// Now 实现 Clock。
func (m *Manual) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.curr
}

// Set 直接把时钟拨到指定时刻。
func (m *Manual) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.curr = t.UTC()
}

// Advance 向前推进一段时间；负值会被忽略以避免时间倒流。
func (m *Manual) Advance(d time.Duration) {
	if d <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.curr = m.curr.Add(d)
}

// Freeze 返回当前时刻的副本快照，防止外部修改内部状态。
func (m *Manual) Freeze() time.Time {
	return m.Now()
}
