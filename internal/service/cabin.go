package service

import (
	"fmt"
	"sort"
)

// CabinSafeGap 安全跟驰间距：额定速度乘以 2 秒追踪距离。
func CabinSafeGap(ratedSpeedMS float64) float64 {
	return ratedSpeedMS * 2.0
}

// CabinOverspeedLimit 超速判定上限：额定速度上浮 15%。
func CabinOverspeedLimit(ratedSpeedMS float64) float64 {
	return ratedSpeedMS * 1.15
}

// CabinSampleFinding 单条定位快照的两项判定结论。
type CabinSampleFinding struct {
	CabinNo   string `json:"cabin_no"`
	Overspeed bool   `json:"overspeed"`
	ThinGap   bool   `json:"thin_gap"`
}

// ClassifyCabinSample 纯函数：对单条快照做超速与间距过小判定；
// gap<=0 表示首舱或无前车参照，不参与间距判定。
func ClassifyCabinSample(cabinNo string, speedMS, gapM, ratedSpeedMS float64) CabinSampleFinding {
	finding := CabinSampleFinding{CabinNo: cabinNo}
	if speedMS > CabinOverspeedLimit(ratedSpeedMS) {
		finding.Overspeed = true
	}
	if gapM > 0 && gapM < CabinSafeGap(ratedSpeedMS) {
		finding.ThinGap = true
	}
	return finding
}

// CabinStat 单舱聚合统计。
type CabinStat struct {
	CabinNo    string  `json:"cabin_no"`
	Samples    int64   `json:"samples"`
	MaxSpeedMS float64 `json:"max_speed_ms"`
	MinGapM    float64 `json:"min_gap_m"`
	Overspeeds int64   `json:"overspeeds"`
	ThinGaps   int64   `json:"thin_gaps"`
}

// CabinSpacingReport 线路客舱间距与速度巡检报告。
type CabinSpacingReport struct {
	LineID        int64       `json:"line_id"`
	LineCode      string      `json:"line_code"`
	RatedSpeedMS  float64     `json:"rated_speed_ms"`
	SafeGapM      float64     `json:"safe_gap_m"`
	SpeedLimitMS  float64     `json:"speed_limit_ms"`
	Samples       int64       `json:"samples"`
	MinGapM       float64     `json:"min_gap_m"`
	GapViolations int64       `json:"gap_violations"`
	Overspeeds    int64       `json:"overspeeds"`
	Cabins        []CabinStat `json:"cabins"`
	Notes         []string    `json:"notes"`
}

// CabinSpacingReport 汇总线路最近 limit 条定位快照：
// 逐条做超速/间距判定，再按舱号聚合出各舱极值与违例计数。
// limit<=0 取默认 200 条。
func (s *Service) CabinSpacingReport(lineID int64, limit int) (CabinSpacingReport, error) {
	if limit <= 0 {
		limit = 200
	}
	line, err := s.store.GetLine(lineID)
	if err != nil {
		return CabinSpacingReport{}, err
	}
	positions, err := s.store.ListCabinPositions(line.ID, limit)
	if err != nil {
		return CabinSpacingReport{}, err
	}

	report := CabinSpacingReport{
		LineID:       line.ID,
		LineCode:     line.Code,
		RatedSpeedMS: line.RatedSpeedMS,
		SafeGapM:     CabinSafeGap(line.RatedSpeedMS),
		SpeedLimitMS: CabinOverspeedLimit(line.RatedSpeedMS),
		Cabins:       []CabinStat{},
	}
	statsByCabin := map[string]*CabinStat{}
	order := []string{}
	report.MinGapM = -1
	for _, pos := range positions {
		finding := ClassifyCabinSample(pos.CabinNo, pos.SpeedMS, pos.GapToPrevM, line.RatedSpeedMS)
		report.Samples++
		stat, seen := statsByCabin[pos.CabinNo]
		if !seen {
			stat = &CabinStat{CabinNo: pos.CabinNo, MinGapM: -1}
			statsByCabin[pos.CabinNo] = stat
			order = append(order, pos.CabinNo)
		}
		stat.Samples++
		if pos.SpeedMS > stat.MaxSpeedMS {
			stat.MaxSpeedMS = pos.SpeedMS
		}
		if finding.Overspeed {
			stat.Overspeeds++
			report.Overspeeds++
		}
		if pos.GapToPrevM > 0 {
			if stat.MinGapM < 0 || pos.GapToPrevM < stat.MinGapM {
				stat.MinGapM = pos.GapToPrevM
			}
			if report.MinGapM < 0 || pos.GapToPrevM < report.MinGapM {
				report.MinGapM = pos.GapToPrevM
			}
			if finding.ThinGap {
				stat.ThinGaps++
				report.GapViolations++
			}
		}
	}
	sort.Strings(order)
	for _, cabinNo := range order {
		report.Cabins = append(report.Cabins, *statsByCabin[cabinNo])
	}
	report.Notes = summarizeSpacing(report)
	return report, nil
}

// summarizeSpacing 把报告结论压缩为人类可读备注列表。
func summarizeSpacing(report CabinSpacingReport) []string {
	notes := []string{}
	switch {
	case report.Samples == 0:
		notes = append(notes, "no cabin positions in window")
	case report.GapViolations == 0 && report.Overspeeds == 0:
		notes = append(notes, fmt.Sprintf("all %d samples within gap %.1fm and speed %.2fm/s limits",
			report.Samples, report.SafeGapM, report.SpeedLimitMS))
	default:
		if report.GapViolations > 0 {
			notes = append(notes, fmt.Sprintf("%d samples below safe gap %.1fm (worst %.1fm)",
				report.GapViolations, report.SafeGapM, report.MinGapM))
		}
		if report.Overspeeds > 0 {
			notes = append(notes, fmt.Sprintf("%d samples above speed limit %.2fm/s",
				report.Overspeeds, report.SpeedLimitMS))
		}
	}
	if len(report.Cabins) > 0 {
		worst := report.Cabins[0]
		for _, stat := range report.Cabins {
			if stat.ThinGaps > worst.ThinGaps {
				worst = stat
			}
		}
		if worst.ThinGaps > 0 {
			notes = append(notes, fmt.Sprintf("cabin %s leads with %d thin-gap hits", worst.CabinNo, worst.ThinGaps))
		}
	}
	return notes
}
