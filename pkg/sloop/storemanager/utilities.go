package storemanager

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
