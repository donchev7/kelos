package usage

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	upsertsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kelos_usage_collector_upserts_total",
			Help: "Total usage collector PostgreSQL upsert attempts",
		},
		[]string{"resource", "result"},
	)
	parseErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kelos_usage_collector_parse_errors_total",
			Help: "Total usage collector parse errors",
		},
		[]string{"kind"},
	)
	syncResourcesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kelos_usage_collector_synced_resources_total",
			Help: "Total Kubernetes resources processed by the usage collector",
		},
		[]string{"resource"},
	)
)

func init() {
	metrics.Registry.MustRegister(upsertsTotal, parseErrorsTotal, syncResourcesTotal)
}
