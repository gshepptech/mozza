package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePercent(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.50%", 0.50},
		{"100%", 100.0},
		{"0%", 0.0},
		{"25.75%", 25.75},
		{"  3.14%  ", 3.14},
		{"", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parsePercent(tt.input)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestParseMemUsage(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"100MiB / 1GiB", 100 * 1024 * 1024},
		{"1GiB / 2GiB", 1024 * 1024 * 1024},
		{"500kB / 1MB", 500 * 1000},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMemUsage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseNetIO(t *testing.T) {
	tests := []struct {
		input      string
		expectedRx int64
		expectedTx int64
	}{
		{"1.5kB / 2.3kB", 1500, 2300},
		{"100MB / 200MB", 100_000_000, 200_000_000},
		{"0B / 0B", 0, 0},
		{"bad", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rx, tx := parseNetIO(tt.input)
			assert.Equal(t, tt.expectedRx, rx)
			assert.Equal(t, tt.expectedTx, tx)
		})
	}
}

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1GiB", 1 << 30},
		{"1MiB", 1 << 20},
		{"1KiB", 1 << 10},
		{"1GB", 1_000_000_000},
		{"1MB", 1_000_000},
		{"1kB", 1_000},
		{"1024B", 1024},
		{"0", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseByteSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
