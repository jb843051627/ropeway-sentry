package validation

import (
	"math"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// RangeSpec 单传感器的软硬两级量程：
// 硬区间之外直接 rejected；软区间之外 suspect；其余 good。
type RangeSpec struct {
	SoftMin float64
	SoftMax float64
	HardMin float64
	HardMax float64
}

// FromSensor 由传感器档案推导量程判定参数。
func FromSensor(s model.RopeSensor) RangeSpec {
	return RangeSpec{
		SoftMin: s.SoftMin,
		SoftMax: s.SoftMax,
		HardMin: s.HardMin,
		HardMax: s.HardMax,
	}
}

// Grade 三级量程判定。rejected 不视为错误，而是数据质量结论之一，
// 调用方仍可决定是否落库（本服务选择落库并标记质量）。
func Grade(spec RangeSpec, value float64) model.Quality {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return model.QualityRejected
	}
	if value < spec.HardMin || value > spec.HardMax {
		return model.QualityRejected
	}
	if value < spec.SoftMin || value > spec.SoftMax {
		return model.QualitySuspect
	}
	return model.QualityGood
}

// QualityTally 三级质量计数器（实体定义在 model 包，此处为别名）。
type QualityTally = model.QualityTally
