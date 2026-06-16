package monitor

import (
	"strconv"
	"strings"
)

// parsePercent parses a percentage string like "0.50%" into a float64.
func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseMemUsage parses a memory usage string like "100MiB / 1GiB" and returns
// the used bytes (the part before the slash).
func parseMemUsage(s string) int64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 0 {
		return 0
	}
	return parseByteSize(strings.TrimSpace(parts[0]))
}

// parseNetIO parses a network I/O string like "1.5kB / 2.3kB" and returns
// (rx_bytes, tx_bytes).
func parseNetIO(s string) (rx, tx int64) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) < 2 {
		return 0, 0
	}
	return parseByteSize(strings.TrimSpace(parts[0])),
		parseByteSize(strings.TrimSpace(parts[1]))
}

// parseByteSize converts a human-readable byte size (e.g., "1.5GiB", "100MB",
// "2.3kB") into bytes.
func parseByteSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	multiplier := int64(1)
	lower := strings.ToLower(s)

	switch {
	case strings.HasSuffix(lower, "gib"):
		multiplier = 1 << 30
		s = s[:len(s)-3]
	case strings.HasSuffix(lower, "mib"):
		multiplier = 1 << 20
		s = s[:len(s)-3]
	case strings.HasSuffix(lower, "kib"):
		multiplier = 1 << 10
		s = s[:len(s)-3]
	case strings.HasSuffix(lower, "gb"):
		multiplier = 1e9
		s = s[:len(s)-2]
	case strings.HasSuffix(lower, "mb"):
		multiplier = 1e6
		s = s[:len(s)-2]
	case strings.HasSuffix(lower, "kb"):
		multiplier = 1e3
		s = s[:len(s)-2]
	case strings.HasSuffix(lower, "b"):
		s = s[:len(s)-1]
	}

	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return int64(v * float64(multiplier))
}
