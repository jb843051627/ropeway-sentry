package validation

import (
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// WindowRule 时间窗判定的参数集合。
type WindowRule struct {
	// Retention 允许回溯的历史上限，窗口起点早于 now-Retention 视为过期。
	Retention time.Duration
	// MaxSpan 单批允许覆盖的最大时间跨度。
	MaxSpan time.Duration
}

// DefaultWindowRule 给出生产默认参数。
func DefaultWindowRule() WindowRule {
	return WindowRule{Retention: 48 * time.Hour, MaxSpan: 6 * time.Hour}
}

// CheckWindow 校验批次时间窗：
// start < end；end 不得落在未来（未来过期拒收原则的“未来”分支）；
// start 不得早于保留期（过期拒收分支）；跨度不得超出单批上限。
func CheckWindow(start, end, now time.Time, rule WindowRule) error {
	if !start.Before(end) {
		return model.ErrBadWindow
	}
	if rule.Retention > 0 && start.Before(now.Add(-rule.Retention)) {
		return model.ErrExpiredWindow
	}
	if rule.MaxSpan > 0 && end.Sub(start) > rule.MaxSpan {
		return model.ErrFutureWindow
	}
	return nil
}

// InWindow 判断时刻 t 是否位于 [start,end] 闭区间内。
func InWindow(t, start, end time.Time) bool {
	return !t.Before(start) && !t.After(end)
}

// PointWithinBatch 遥测点采样时刻必须落在批次声明的窗口内，
// 否则视为越窗数据拒收该点（由调用方决定剔除或整批拒绝）。
func PointWithinBatch(takenAt, windowStart, windowEnd time.Time) bool {
	return InWindow(takenAt, windowStart, windowEnd)
}
