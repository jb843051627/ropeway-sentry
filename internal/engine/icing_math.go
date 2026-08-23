package engine

// applyIcingWindRelief 结冰季放宽限行触发面：restricted 风级下限按 relief 下调，
// 即限行更容易触发。下调后不得低于 1（蒲福风级最低有效档），且天然保持 restricted < critical。
func applyIcingWindRelief(th WindThresholds, relief int) WindThresholds {
	reduced := th.RestrictedScale - relief
	if reduced < 1 {
		reduced = 1
	}
	th.RestrictedScale = reduced
	return th
}
