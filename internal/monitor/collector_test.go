package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcCPUPercent(t *testing.T) {
	tests := []struct {
		name     string
		stats    *dockerStatsAPI
		expected float64
	}{
		{
			name: "normal usage",
			stats: func() *dockerStatsAPI {
				s := &dockerStatsAPI{}
				s.CPUStats.CPUUsage.TotalUsage = 200
				s.CPUStats.SystemCPUUsage = 1000
				s.CPUStats.OnlineCPUs = 2
				s.PrecpuStats.CPUUsage.TotalUsage = 100
				s.PrecpuStats.SystemCPUUsage = 500
				return s
			}(),
			expected: 40.0, // (100/500) * 2 * 100
		},
		{
			name: "zero delta system",
			stats: func() *dockerStatsAPI {
				s := &dockerStatsAPI{}
				s.CPUStats.CPUUsage.TotalUsage = 100
				s.CPUStats.SystemCPUUsage = 500
				s.PrecpuStats.CPUUsage.TotalUsage = 100
				s.PrecpuStats.SystemCPUUsage = 500
				return s
			}(),
			expected: 0.0,
		},
		{
			name: "single core fallback",
			stats: func() *dockerStatsAPI {
				s := &dockerStatsAPI{}
				s.CPUStats.CPUUsage.TotalUsage = 50
				s.CPUStats.SystemCPUUsage = 200
				s.CPUStats.OnlineCPUs = 0
				s.PrecpuStats.CPUUsage.TotalUsage = 0
				s.PrecpuStats.SystemCPUUsage = 0
				return s
			}(),
			expected: 25.0, // (50/200) * 1 * 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcCPUPercent(tt.stats)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestCalcMemoryUsage(t *testing.T) {
	tests := []struct {
		name     string
		stats    *dockerStatsAPI
		expected int64
	}{
		{
			name: "usage minus cache",
			stats: func() *dockerStatsAPI {
				s := &dockerStatsAPI{}
				s.MemoryStats.Usage = 1024
				s.MemoryStats.Stats = map[string]int64{"cache": 256}
				return s
			}(),
			expected: 768,
		},
		{
			name: "no cache stat",
			stats: func() *dockerStatsAPI {
				s := &dockerStatsAPI{}
				s.MemoryStats.Usage = 1024
				s.MemoryStats.Stats = map[string]int64{}
				return s
			}(),
			expected: 1024,
		},
		{
			name: "cache exceeds usage returns usage",
			stats: func() *dockerStatsAPI {
				s := &dockerStatsAPI{}
				s.MemoryStats.Usage = 100
				s.MemoryStats.Stats = map[string]int64{"cache": 200}
				return s
			}(),
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcMemoryUsage(tt.stats)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalcNetwork(t *testing.T) {
	stats := &dockerStatsAPI{}
	stats.Networks = map[string]struct {
		RxBytes int64 `json:"rx_bytes"`
		TxBytes int64 `json:"tx_bytes"`
	}{
		"eth0": {RxBytes: 100, TxBytes: 200},
		"eth1": {RxBytes: 50, TxBytes: 75},
	}

	rx, tx := calcNetwork(stats)
	assert.Equal(t, int64(150), rx)
	assert.Equal(t, int64(275), tx)
}

func TestCalcNetworkEmpty(t *testing.T) {
	stats := &dockerStatsAPI{}
	rx, tx := calcNetwork(stats)
	assert.Equal(t, int64(0), rx)
	assert.Equal(t, int64(0), tx)
}

func TestParseDockerCLIStats(t *testing.T) {
	raw := dockerStats{
		CPUPerc:  "25.50%",
		MemUsage: "100MiB / 1GiB",
		NetIO:    "1.5kB / 2.3kB",
	}

	stats, err := parseDockerCLIStats(raw)
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// CPU should be set up so calcCPUPercent returns ~25.5.
	cpu := calcCPUPercent(stats)
	assert.InDelta(t, 25.5, cpu, 0.1)

	// Memory should be ~100 MiB.
	mem := calcMemoryUsage(stats)
	assert.InDelta(t, 100*1024*1024, float64(mem), float64(1024*1024))
}
