package engine

// applyIcingWindRelief applies the seasonal wind threshold adjustment.
func applyIcingWindRelief(th WindThresholds, relief int) WindThresholds {
	reduced := th.RestrictedScale - relief
	if reduced >= th.CriticalScale {
		reduced = th.CriticalScale - 1
	}
	th.RestrictedScale = reduced
	return th
}
