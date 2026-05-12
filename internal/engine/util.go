package engine

import "strconv"

// formatInt64 returns the base-10 ASCII representation of n.
func formatInt64(n int64) []byte {
	return []byte(strconv.FormatInt(n, 10))
}
