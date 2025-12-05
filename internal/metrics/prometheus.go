package metrics

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// PrometheusExporter exports metrics in Prometheus text format
func (m *Metrics) PrometheusFormat() string {
	var sb strings.Builder

	// Counters
	sb.WriteString("# HELP orders_received_total Total number of orders received\n")
	sb.WriteString("# TYPE orders_received_total counter\n")
	sb.WriteString(fmt.Sprintf("orders_received_total %d\n\n", atomic.LoadUint64(&m.OrdersReceived)))

	sb.WriteString("# HELP orders_matched_total Total number of orders matched\n")
	sb.WriteString("# TYPE orders_matched_total counter\n")
	sb.WriteString(fmt.Sprintf("orders_matched_total %d\n\n", atomic.LoadUint64(&m.OrdersMatched)))

	sb.WriteString("# HELP orders_cancelled_total Total number of orders cancelled\n")
	sb.WriteString("# TYPE orders_cancelled_total counter\n")
	sb.WriteString(fmt.Sprintf("orders_cancelled_total %d\n\n", atomic.LoadUint64(&m.OrdersCancelled)))

	sb.WriteString("# HELP trades_executed_total Total number of trades executed\n")
	sb.WriteString("# TYPE trades_executed_total counter\n")
	sb.WriteString(fmt.Sprintf("trades_executed_total %d\n\n", atomic.LoadUint64(&m.TradesExecuted)))

	// Throughput gauge
	sb.WriteString("# HELP throughput_orders_per_sec Current throughput in orders per second\n")
	sb.WriteString("# TYPE throughput_orders_per_sec gauge\n")
	sb.WriteString(fmt.Sprintf("throughput_orders_per_sec %.2f\n\n", m.Throughput()))

	// Latency histogram
	p50, p99, p999 := m.Percentiles()
	sb.WriteString("# HELP http_request_duration_ms HTTP request latencies in milliseconds\n")
	sb.WriteString("# TYPE http_request_duration_ms summary\n")
	sb.WriteString(fmt.Sprintf("http_request_duration_ms{quantile=\"0.5\"} %.3f\n", p50))
	sb.WriteString(fmt.Sprintf("http_request_duration_ms{quantile=\"0.99\"} %.3f\n", p99))
	sb.WriteString(fmt.Sprintf("http_request_duration_ms{quantile=\"0.999\"} %.3f\n\n", p999))

	return sb.String()
}
