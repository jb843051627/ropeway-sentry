package store

import (
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/model"
)

// DayQualityRow 按 UTC 日期聚合的三级质量计数。
type DayQualityRow struct {
	Date     string `json:"date"`
	Good     int64  `json:"good"`
	Suspect  int64  `json:"suspect"`
	Rejected int64  `json:"rejected"`
}

// DayPeakRow 按 UTC 日期聚合的量值峰值与样本数。
type DayPeakRow struct {
	Date      string  `json:"date"`
	PeakValue float64 `json:"peak_value"`
	Samples   int64   `json:"samples"`
}

// AlertFlowRow 按 UTC 日期聚合的告警新增/关闭流量。
type AlertFlowRow struct {
	Date   string `json:"date"`
	Opened int64  `json:"opened"`
	Closed int64  `json:"closed"`
}

// takenAtDayExpr RFC3339Nano 文本的前 10 字节即 UTC 日期。
const takenAtDayExpr = "substr(p.taken_at,1,10)"

// DailyKindQuality 按日聚合线路指定种类传感器的质量计数；
// kind 为空字符串时聚合全部种类，供完整率均值使用。
func (s *Store) DailyKindQuality(lineID int64, kind model.SensorKind, since time.Time) ([]DayQualityRow, error) {
	query := `SELECT ` + takenAtDayExpr + ` AS day,
		SUM(CASE WHEN p.quality='good' THEN 1 ELSE 0 END),
		SUM(CASE WHEN p.quality='suspect' THEN 1 ELSE 0 END),
		SUM(CASE WHEN p.quality='rejected' THEN 1 ELSE 0 END)
		FROM telemetry_points p JOIN rope_sensors s ON s.id=p.sensor_id
		WHERE s.line_id=? AND p.taken_at>=?`
	args := []any{lineID, formatTime(since)}
	if kind != "" {
		query += ` AND s.kind=?`
		args = append(args, string(kind))
	}
	query += ` GROUP BY day ORDER BY day`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DayQualityRow{}
	for rows.Next() {
		var row DayQualityRow
		if err := rows.Scan(&row.Date, &row.Good, &row.Suspect, &row.Rejected); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// DailyWindPeaks 按日聚合风速计的最大观测值（剔除 rejected 点），
// 作为当日阵风峰值参与周报 KPI。
func (s *Store) DailyWindPeaks(lineID int64, since time.Time) ([]DayPeakRow, error) {
	rows, err := s.db.Query(`SELECT `+takenAtDayExpr+` AS day, MAX(p.value), COUNT(*)
		FROM telemetry_points p JOIN rope_sensors s ON s.id=p.sensor_id
		WHERE s.line_id=? AND s.kind='wind' AND p.taken_at>=? AND p.quality!='rejected'
		GROUP BY day ORDER BY day`, lineID, formatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DayPeakRow{}
	for rows.Next() {
		var (
			row       DayPeakRow
			peakValue any
		)
		if err := rows.Scan(&row.Date, &peakValue, &row.Samples); err != nil {
			return nil, err
		}
		if v, ok := peakValue.(float64); ok {
			row.PeakValue = v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// DailyTensionExceeds 按日统计张力传感器被量程判定标记为
// suspect/rejected 的点数，作为张力偏移超限次数的分母口径。
func (s *Store) DailyTensionExceeds(lineID int64, since time.Time) ([]DayQualityRow, error) {
	rows, err := s.db.Query(`SELECT `+takenAtDayExpr+` AS day,
		SUM(CASE WHEN p.quality='suspect' THEN 1 ELSE 0 END),
		SUM(CASE WHEN p.quality='rejected' THEN 1 ELSE 0 END)
		FROM telemetry_points p JOIN rope_sensors s ON s.id=p.sensor_id
		WHERE s.line_id=? AND s.kind='tension' AND p.taken_at>=?
		GROUP BY day ORDER BY day`, lineID, formatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DayQualityRow{}
	for rows.Next() {
		var row DayQualityRow
		if err := rows.Scan(&row.Date, &row.Suspect, &row.Rejected); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// AlertDailyFlow 按日合并告警新增（first_seen_at）与关闭（closed_at）流量，
// 用于计算告警关闭率的时间序列。
func (s *Store) AlertDailyFlow(lineID int64, since time.Time) ([]AlertFlowRow, error) {
	openedByDay, err := s.alertCountByDay(`first_seen_at`, lineID, since)
	if err != nil {
		return nil, err
	}
	closedByDay, err := s.alertCountByDay(`closed_at`, lineID, since)
	if err != nil {
		return nil, err
	}
	dates := make([]string, 0, len(openedByDay)+len(closedByDay))
	for date := range openedByDay {
		dates = append(dates, date)
	}
	for date := range closedByDay {
		if _, seen := openedByDay[date]; !seen {
			dates = append(dates, date)
		}
	}
	sortStrings(dates)
	out := make([]AlertFlowRow, 0, len(dates))
	for _, date := range dates {
		out = append(out, AlertFlowRow{
			Date:   date,
			Opened: openedByDay[date],
			Closed: closedByDay[date],
		})
	}
	return out, nil
}

// alertCountByDay 对指定时间列按日聚合计数；closed_at 需排除 NULL 行。
func (s *Store) alertCountByDay(column string, lineID int64, since time.Time) (map[string]int64, error) {
	query := `SELECT substr(` + column + `,1,10) AS day, COUNT(*)
		FROM alerts WHERE line_id=? AND ` + column + `>=? AND ` + column + ` IS NOT NULL
		GROUP BY day`
	rows, err := s.db.Query(query, lineID, formatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var (
			date string
			n    int64
		)
		if err := rows.Scan(&date, &n); err != nil {
			return nil, err
		}
		out[date] = n
	}
	return out, rows.Err()
}

// sortStrings 就地升序排序日期字符串（长度固定，直接比较即可）。
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
