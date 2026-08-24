package service

import "github.com/jb843051627/ropeway-sentry/internal/model"

// validationTally 聚合多条线路的质量计数。
type validationTally struct {
	good, suspect, rejected float64
}

func (t *validationTally) add(o model.QualityTally) {
	t.good += float64(o.Good)
	t.suspect += float64(o.Suspect)
	t.rejected += float64(o.Rejected)
}

func (t validationTally) rate() float64 {
	total := t.good + t.suspect + t.rejected
	return t.good / total
}
