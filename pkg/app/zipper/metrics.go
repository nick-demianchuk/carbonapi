package zipper

import (
	"net/http"
	"time"

	"github.com/bookingcom/carbonapi/pkg/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics keeps all the metrics exposed on /metrics endpoint
type PrometheusMetrics struct {
	Requests                  prometheus.Counter
	Responses                 *prometheus.CounterVec
	RenderMismatches          prometheus.Counter
	RenderFixedMismatches     prometheus.Counter
	RenderMismatchedResponses prometheus.Counter
	Renders                   prometheus.Counter
	FindNotFound              prometheus.Counter
	RequestCancel             *prometheus.CounterVec

	RenderDurationExp    prometheus.Histogram
	RenderOutDurationExp *prometheus.HistogramVec
	FindDurationExp      prometheus.Histogram
	FindDurationLin      prometheus.Histogram
	FindOutDuration      *prometheus.HistogramVec

	TimeInQueueSeconds *prometheus.HistogramVec

	TLDCacheProbeReqTotal  prometheus.Counter
	TLDCacheProbeErrors    prometheus.Counter
	TLDCacheHostsPerDomain prometheus.GaugeVec

	PathCacheFilteredRequests prometheus.Counter
	BackendResponses          *prometheus.CounterVec
}

// NewPrometheusMetrics creates a set of default Prom metrics
func NewPrometheusMetrics(config cfg.Zipper, ns string) *PrometheusMetrics {
	return &PrometheusMetrics{
		Requests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "http_request_total",
				Help:      "Count of HTTP requests",
			},
		),
		Responses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "http_responses_total",
				Help:      "Count of HTTP responses, partitioned by return code and handler",
			},
			[]string{"code", "handler"},
		),
		RenderMismatchedResponses: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "render_mismatched_responses_total",
				Help:      "Count of mismatched (unfixed) render responses",
			},
		),
		RenderFixedMismatches: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "render_fixed_mismatches_total",
				Help:      "Count of fixed mismatched rendered data points",
			},
		),
		RenderMismatches: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "render_mismatches_total",
				Help:      "Count of mismatched rendered data points",
			},
		),
		Renders: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "render_total",
				Help:      "Count of rendered data points",
			},
		),
		FindNotFound: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "find_not_found",
				Help:      "Count of not-found /find responses",
			},
		),
		RequestCancel: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "request_cancel",
				Help:      "Context cancellations or incoming requests due to manual cancels or timeouts",
			},
			[]string{"handler", "cause"},
		),
		RenderDurationExp: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: ns,
				Name:      "render_request_duration_seconds_exp",
				Help:      "The duration of render requests (exponential)",
				Buckets: prometheus.ExponentialBuckets(
					config.Monitoring.RenderDurationExp.Start,
					config.Monitoring.RenderDurationExp.BucketSize,
					config.Monitoring.RenderDurationExp.BucketsNum),
			},
		),
		RenderOutDurationExp: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: ns,
				Name:      "render_outbound_request_duration_seconds_exp",
				Help:      "The durations of render requests sent to storages (exponential)",
				Buckets: prometheus.ExponentialBuckets(
					// TODO (grzkv) Do we need a separate config?
					// The buckets should be of comparable size.
					config.Monitoring.RenderDurationExp.Start,
					config.Monitoring.RenderDurationExp.BucketSize,
					config.Monitoring.RenderDurationExp.BucketsNum),
			},
			[]string{"dc", "cluster"},
		),
		FindDurationExp: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: ns,
				Name:      "find_request_duration_seconds_exp",
				Help:      "The duration of find requests (exponential)",
				Buckets: prometheus.ExponentialBuckets(
					config.Monitoring.FindDurationExp.Start,
					config.Monitoring.FindDurationExp.BucketSize,
					config.Monitoring.FindDurationExp.BucketsNum),
			},
		),
		FindDurationLin: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: ns,
				Name:      "find_request_duration_seconds_lin",
				Help:      "The duration of find requests (linear), in ms",
				Buckets: prometheus.LinearBuckets(
					config.Monitoring.FindDurationLin.Start,
					config.Monitoring.FindDurationLin.BucketSize,
					config.Monitoring.FindDurationLin.BucketsNum),
			},
		),
		FindOutDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: ns,
				Name:      "find_out_duration_seconds",
				Help:      "Duration of outgoing find requests per backend cluster.",
				Buckets: prometheus.ExponentialBuckets(
					config.Monitoring.FindOutDuration.Start,
					config.Monitoring.FindOutDuration.BucketSize,
					config.Monitoring.FindOutDuration.BucketsNum),
			},
			[]string{"cluster"},
		),
		TimeInQueueSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: ns,
				Name:      "time_in_queue",
				Help:      "Time a request spends in queue in seconds.",
				Buckets: prometheus.ExponentialBuckets(
					config.Monitoring.TimeInQueueExpHistogram.Start/1000, // converstion ms -> s
					config.Monitoring.TimeInQueueExpHistogram.BucketSize,
					config.Monitoring.TimeInQueueExpHistogram.BucketsNum),
			},
			[]string{"request"},
		),
		TLDCacheProbeReqTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "tldcache_probe_req_total",
				Help:      "The total number of find requests sent by TLD cache as probes.",
			},
		),
		TLDCacheProbeErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "tldcache_probe_errors_total",
				Help:      "The total number of failed find requests sent by TLD cache as probes.",
			},
		),
		TLDCacheHostsPerDomain: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: ns,
				Name:      "tldcache_num_hosts_per_domain",
				Help:      "The number of hosts per top-level domain.",
			},
			[]string{"domain"},
		),
		PathCacheFilteredRequests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "path_cache_filtered_requests_total",
				Help:      "The total number of requests with successful backend filter by path caches",
			},
		),
		BackendResponses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      "backend_responses_total",
				Help:      "Count of backend responses, partitioned by return code and handler",
			},
			[]string{"code", "handler"},
		),
	}
}

func metricsServer(app *App, serve bool) *http.Server {
	prometheus.MustRegister(app.Metrics.Requests)
	prometheus.MustRegister(app.Metrics.Responses)
	prometheus.MustRegister(app.Metrics.Renders)
	prometheus.MustRegister(app.Metrics.RenderMismatches)
	prometheus.MustRegister(app.Metrics.RenderFixedMismatches)
	prometheus.MustRegister(app.Metrics.RenderMismatchedResponses)
	prometheus.MustRegister(app.Metrics.FindNotFound)
	prometheus.MustRegister(app.Metrics.RequestCancel)
	prometheus.MustRegister(app.Metrics.RenderDurationExp)
	prometheus.MustRegister(app.Metrics.RenderOutDurationExp)
	prometheus.MustRegister(app.Metrics.FindDurationExp)
	prometheus.MustRegister(app.Metrics.FindDurationLin)
	prometheus.MustRegister(app.Metrics.FindOutDuration)
	prometheus.MustRegister(app.Metrics.TimeInQueueSeconds)

	prometheus.MustRegister(app.Metrics.TLDCacheHostsPerDomain)
	prometheus.MustRegister(app.Metrics.TLDCacheProbeErrors)
	prometheus.MustRegister(app.Metrics.TLDCacheProbeReqTotal)

	prometheus.MustRegister(app.Metrics.PathCacheFilteredRequests)
	prometheus.MustRegister(app.Metrics.BackendResponses)

	writeTimeout := app.Config.Timeouts.Global
	if writeTimeout < 30*time.Second {
		writeTimeout = time.Minute
	}

	if serve {
		r := initMetricHandlers()
		s := &http.Server{
			Handler:      r,
			ReadTimeout:  1 * time.Second,
			WriteTimeout: writeTimeout,
		}

		s.Addr = app.Config.ListenInternal

		return s
	} else {
		return nil
	}

}
