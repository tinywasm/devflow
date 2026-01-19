package devflow

import (
	"strconv"
	"strings"
)

// CompareVersions compares two semantic version strings (e.g., "v1.2.3").
// It returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
// It handles "v" prefix gracefully.
func CompareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			// Parse logic that handles suffixes like "-beta" if needed,
			// but for this task basic numeric comparison is prioritized.
			// We split by non-numeric to get the main number.
			fields := strings.FieldsFunc(parts1[i], isNotDigit)
			if len(fields) > 0 {
				n1, _ = strconv.Atoi(fields[0])
			}
		}
		if i < len(parts2) {
			fields := strings.FieldsFunc(parts2[i], isNotDigit)
			if len(fields) > 0 {
				n2, _ = strconv.Atoi(fields[0])
			}
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}

func isNotDigit(r rune) bool {
	return r < '0' || r > '9'
}
