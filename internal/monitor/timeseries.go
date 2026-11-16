package monitor

import (
	"sync"
	"time"
)

// defaultTSCapacity is the number of data points stored per app (24h at 15s intervals).
const defaultTSCapacity = 5760

// TimePoint holds a single metrics snapshot for one app.
type TimePoint struct {
	Timestamp  time.Time `json:"timestamp"`
	CPU        float64   `json:"cpu"`
	Memory     int64     `json:"memory"`
	NetworkIn  int64     `json:"network_in"`
	NetworkOut int64     `json:"network_out"`
}

// ringBuffer is a fixed-capacity circular buffer of TimePoints.
type ringBuffer struct {
	data  []TimePoint
	cap   int
	write int
	count int
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		data: make([]TimePoint, capacity),
		cap:  capacity,
	}
}

func (rb *ringBuffer) append(p TimePoint) {
	rb.data[rb.write] = p
	rb.write = (rb.write + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}
}

// query returns all points with timestamps after the cutoff, oldest first.
func (rb *ringBuffer) query(cutoff time.Time) []TimePoint {
	if rb.count == 0 {
		return nil
	}

	start := 0
	if rb.count == rb.cap {
		start = rb.write // oldest element in a full buffer
	}

	var result []TimePoint
	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.cap
		if rb.data[idx].Timestamp.After(cutoff) {
			result = append(result, rb.data[idx])
		}
	}
	return result
}

// TimeSeriesStore keeps in-memory ring buffers of metrics per app.
type TimeSeriesStore struct {
	mu       sync.RWMutex
	apps     map[string]*ringBuffer
	capacity int
}

// NewTimeSeriesStore creates a store with the given per-app capacity.
func NewTimeSeriesStore(capacity int) *TimeSeriesStore {
	if capacity <= 0 {
		capacity = defaultTSCapacity
	}
	return &TimeSeriesStore{
		apps:     make(map[string]*ringBuffer),
		capacity: capacity,
	}
}

// Record appends a data point for the given app.
func (ts *TimeSeriesStore) Record(appID string, point TimePoint) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	rb, ok := ts.apps[appID]
	if !ok {
		rb = newRingBuffer(ts.capacity)
		ts.apps[appID] = rb
	}
	rb.append(point)
}

// Query returns all data points for the given app within the specified duration.
func (ts *TimeSeriesStore) Query(appID string, period time.Duration) []TimePoint {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	rb, ok := ts.apps[appID]
	if !ok {
		return nil
	}

	cutoff := time.Now().Add(-period)
	return rb.query(cutoff)
}

// TimeSeriesResponse is the JSON response for the timeseries endpoint.
type TimeSeriesResponse struct {
	AppID  string      `json:"app_id"`
	Period string      `json:"period"`
	Points []TimePoint `json:"points"`
}
