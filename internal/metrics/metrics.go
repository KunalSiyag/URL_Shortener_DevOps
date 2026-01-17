package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	shortenRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "url_shortener_shorten_requests_total",
			Help: "Total number of shorten requests",
		},
		[]string{"status"},
	)

	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func Register() {
	prometheus.MustRegister(
		shortenRequests,
		HttpRequestDuration,
	)
}
