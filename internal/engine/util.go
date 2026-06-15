package engine

import "strconv"

// formatInt64 returns the base-10 ASCII representation of n.
func formatInt64(n int64) []byte {
	return []byte(strconv.FormatInt(n, 10))
}

func expirationFromNow(value, unitNanos int64) (int64, bool) {
	if value <= 0 || value > maxInt64/unitNanos {
		return 0, false
	}
	delta := value * unitNanos
	now := nowNanos()
	if now > maxInt64-delta {
		return 0, false
	}
	return now + delta, true
}
