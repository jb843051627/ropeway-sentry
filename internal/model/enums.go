package model

import "fmt"

// LineStatus 线路开放等级，合成优先级 closed > maintenance > restricted > open。
type LineStatus string

// 四档开放等级。
const (
	LineOpen        LineStatus = "open"
	LineRestricted  LineStatus = "restricted"
	LineMaintenance LineStatus = "maintenance"
	LineClosed      LineStatus = "closed"
)

// ParseLineStatus 把文本解析为 LineStatus。
func ParseLineStatus(raw string) (LineStatus, error) {
	switch LineStatus(raw) {
	case LineOpen, LineRestricted, LineMaintenance, LineClosed:
		return LineStatus(raw), nil
	default:
		return "", fmt.Errorf("unknown line status %q", raw)
	}
}

// lineTransitions 定义人工状态机的合法迁移边。
// closed 非终态：处置完成后允许复线（回 open）或先降级观察（restricted），
// 复线是否放行由 service 层结合未关闭 critical 告警二次把关。
var lineTransitions = map[LineStatus][]LineStatus{
	LineOpen:        {LineRestricted, LineMaintenance, LineClosed},
	LineRestricted:  {LineOpen, LineMaintenance, LineClosed},
	LineMaintenance: {LineOpen, LineRestricted, LineClosed},
	LineClosed:      {LineRestricted, LineOpen},
}

// CanTransition 判断 from → to 是否为合法迁移。
func CanTransition(from, to LineStatus) bool {
	if from == to {
		return false
	}
	for _, next := range lineTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// AllowedTransitions 返回当前状态的合法后继集合。
func AllowedTransitions(from LineStatus) []LineStatus {
	return lineTransitions[from]
}

// AlertSeverity 告警级别：critical 必须先 ack 才能关闭。
type AlertSeverity string

// 两档告警级别。
const (
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// ParseAlertSeverity 解析告警级别文本。
func ParseAlertSeverity(raw string) (AlertSeverity, error) {
	switch AlertSeverity(raw) {
	case SeverityWarning, SeverityCritical:
		return AlertSeverity(raw), nil
	default:
		return "", fmt.Errorf("unknown alert severity %q", raw)
	}
}

// AlertStatus 告警状态机：open → acked → closed；非 critical 允许直接 close。
type AlertStatus string

// 三态。
const (
	AlertOpen   AlertStatus = "open"
	AlertAcked  AlertStatus = "acked"
	AlertClosed AlertStatus = "closed"
)

// Quality 遥测量程三级判定结果。
type Quality string

// good / suspect / rejected。
const (
	QualityGood     Quality = "good"
	QualitySuspect  Quality = "suspect"
	QualityRejected Quality = "rejected"
)

// SensorKind 传感器种类，风速计与倾斜仪复用同一套传感器体系。
type SensorKind string

// 支持的传感器种类。
const (
	KindTension   SensorKind = "tension"   // 钢丝绳张力（N）
	KindWireBreak SensorKind = "wirebreak" // 断丝累计计数
	KindFlux      SensorKind = "flux"      // 磁通量漏磁信号
	KindWind      SensorKind = "wind"      // 风速（m/s）
	KindTilt      SensorKind = "tilt"      // 支架倾角（deg）
)

// ParseSensorKind 解析传感器种类文本。
func ParseSensorKind(raw string) (SensorKind, error) {
	switch SensorKind(raw) {
	case KindTension, KindWireBreak, KindFlux, KindWind, KindTilt:
		return SensorKind(raw), nil
	default:
		return "", fmt.Errorf("unknown sensor kind %q", raw)
	}
}

// HoldStatus 维护停机锁生命周期：planned → active → released。
type HoldStatus string

// 锁三态。
const (
	HoldPlanned  HoldStatus = "planned"
	HoldActive   HoldStatus = "active"
	HoldReleased HoldStatus = "released"
)

// InspectionKind 巡检方式。
type InspectionKind string

// 目检 / 探伤。
const (
	InspectionVisual InspectionKind = "visual"
	InspectionNDT    InspectionKind = "ndt"
)

// ParseInspectionKind 解析巡检方式。
func ParseInspectionKind(raw string) (InspectionKind, error) {
	switch InspectionKind(raw) {
	case InspectionVisual, InspectionNDT:
		return InspectionKind(raw), nil
	default:
		return "", fmt.Errorf("unknown inspection kind %q", raw)
	}
}
