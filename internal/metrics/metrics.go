package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	OrdersReceived  uint64
	OrdersMatched   uint64
	OrdersCancelled uint64
	TradesExecuted  uint64

	start time.Time

	// Latency histogram buckets in milliseconds:
	// 0–1, 1–2, 2–5, 5–10, 10–20, 20–50, 50–100, 100–200, 200+
	buckets [9]uint64
	mu      sync.Mutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		start: time.Now(),
	}
}

// RecordLatency tracks order processing time in ms.
func (m *Metrics) RecordLatency(ms float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch {
	case ms <= 1:
		m.buckets[0]++
	case ms <= 2:
		m.buckets[1]++
	case ms <= 5:
		m.buckets[2]++
	case ms <= 10:
		m.buckets[3]++
	case ms <= 20:
		m.buckets[4]++
	case ms <= 50:
		m.buckets[5]++
	case ms <= 100:
		m.buckets[6]++
	case ms <= 200:
		m.buckets[7]++
	default:
		m.buckets[8]++
	}
}

// Percentiles (p50, p99, p999)
func (m *Metrics) Percentiles() (p50, p99, p999 float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := uint64(0)
	for _, c := range m.buckets {
		total += c
	}
	if total == 0 {
		return 0, 0, 0
	}

	thresholds := []float64{0.50, 0.99, 0.999}
	results := make([]float64, 3)

	running := uint64(0)

	bucketBounds := []float64{1, 2, 5, 10, 20, 50, 100, 200, 300}

	idx := 0
	for i, count := range m.buckets {
		running += count
		for idx < len(thresholds) && float64(running) >= thresholds[idx]*float64(total) {
			results[idx] = bucketBounds[i]
			idx++
		}
	}

	return results[0], results[1], results[2]
}

func (m *Metrics) Throughput() float64 {
	elapsed := time.Since(m.start).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(atomic.LoadUint64(&m.OrdersReceived)) / elapsed
}
