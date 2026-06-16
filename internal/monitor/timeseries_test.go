package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeSeriesStore_RecordAndQuery(t *testing.T) {
	ts := NewTimeSeriesStore(100)
	now := time.Now()

	for i := 0; i < 5; i++ {
		ts.Record("1", TimePoint{
			Timestamp:  now.Add(time.Duration(i) * time.Minute),
			CPU:        float64(10 + i),
			Memory:     int64(1000 + i*100),
			NetworkIn:  int64(500 + i*50),
			NetworkOut: int64(200 + i*20),
		})
	}

	points := ts.Query("1", 10*time.Minute)
	require.Len(t, points, 5)
	assert.Equal(t, float64(10), points[0].CPU)
	assert.Equal(t, float64(14), points[4].CPU)
}

func TestTimeSeriesStore_QueryFiltersOldPoints(t *testing.T) {
	ts := NewTimeSeriesStore(100)
	now := time.Now()

	// Record points: 3 old, 2 recent.
	for i := 0; i < 3; i++ {
		ts.Record("1", TimePoint{
			Timestamp: now.Add(-2 * time.Hour).Add(time.Duration(i) * time.Minute),
			CPU:       float64(i),
		})
	}
	for i := 0; i < 2; i++ {
		ts.Record("1", TimePoint{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			CPU:       float64(10 + i),
		})
	}

	points := ts.Query("1", 1*time.Hour)
	require.Len(t, points, 2)
}

func TestTimeSeriesStore_QueryUnknownApp(t *testing.T) {
	ts := NewTimeSeriesStore(100)
	points := ts.Query("999", time.Hour)
	assert.Nil(t, points)
}

func TestTimeSeriesStore_RingBufferWraps(t *testing.T) {
	ts := NewTimeSeriesStore(3)
	now := time.Now()

	for i := 0; i < 5; i++ {
		ts.Record("1", TimePoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			CPU:       float64(i),
		})
	}

	// Only last 3 should remain (indices 2, 3, 4).
	points := ts.Query("1", time.Hour)
	require.Len(t, points, 3)
	assert.Equal(t, float64(2), points[0].CPU)
	assert.Equal(t, float64(4), points[2].CPU)
}

func TestNewTimeSeriesStore_DefaultCapacity(t *testing.T) {
	ts := NewTimeSeriesStore(0)
	assert.Equal(t, defaultTSCapacity, ts.capacity)
}
