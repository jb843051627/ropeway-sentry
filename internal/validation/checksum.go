// Package validation 实现遥测入库链中的纯规则校验：
// 批次校验和重算、时间窗合法性、传感器归属与启用、量程三级判定。
package validation

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strings"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// CanonicalLine 生成单点的规范化字符串，作为校验和的最小单元。
// 字段顺序固定：sensor|seq|taken_at(RFC3339Nano)|value(科学计数固定精度)。
func CanonicalLine(p model.TelemetryPointInput) string {
	return fmt.Sprintf("%s|%d|%s|%.6f", p.SensorCode, p.Seq, p.TakenAt.UTC().Format("2006-01-02T15:04:05.000000000Z"), p.Value)
}

// CanonicalPayload 把整批点按稳定顺序拼接为待摘要文本。
func CanonicalPayload(points []model.TelemetryPointInput) string {
	sorted := make([]model.TelemetryPointInput, len(points))
	copy(sorted, points)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SensorCode != sorted[j].SensorCode {
			return sorted[i].SensorCode < sorted[j].SensorCode
		}
		return sorted[i].Seq < sorted[j].Seq
	})
	lines := make([]string, len(sorted))
	for i, p := range sorted {
		lines[i] = CanonicalLine(p)
	}
	return strings.Join(lines, "\n")
}

// ComputeChecksum 服务端可重算接口：对规范化载荷做 CRC32-IEEE。
func ComputeChecksum(points []model.TelemetryPointInput) uint32 {
	return crc32.ChecksumIEEE([]byte(CanonicalPayload(points)))
}

// VerifyChecksum 比对客户端声明值与服务端重算值。
func VerifyChecksum(points []model.TelemetryPointInput, expected uint32) error {
	actual := ComputeChecksum(points)
	if actual != expected {
		return fmt.Errorf("%w: expected %d got %d", model.ErrChecksumMismatch, expected, actual)
	}
	return nil
}
