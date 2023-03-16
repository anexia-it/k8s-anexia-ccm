package metrics

import (
	"fmt"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"

	k8smetrics "k8s.io/component-base/metrics"
)

const FeatureEnabled = 1
const FeatureDisabled = 0

const fqCollectorName = "cloud_provider_anexia"

var (
	constLabels          = prometheus.Labels{"collector": "anexia-provider-collector"}
	constLabelsReconcile = prometheus.Labels{"service": "lbass"}

	descProviderBuild = prometheus.NewDesc(getFQMetricName("provider_build"),
		"information about the build version of a specific provider", []string{"name", "version"}, constLabels)

	descProviderFeatures = prometheus.NewDesc(getFQMetricName("feature"), "provider features and their state",
		[]string{"name"}, constLabels)
)

type ProviderMetrics struct {
	m                                     *sync.RWMutex
	Name                                  string
	providerVersion                       prometheus.Metric
	ReconciliationTotalDuration           prometheus.Histogram
	ReconciliationCreateErrorsTotal       prometheus.Counter
	ReconciliationDeleteRetriesTotal      prometheus.Counter
	ReconciliationDeleteErrorsTotal       prometheus.Counter
	ReconciliationCreatedTotal            prometheus.Counter
	ReconciliationDeletedTotal            prometheus.Counter
	ReconciliationCreateResources         prometheus.Histogram
	ReconciliationPendingResources        *k8smetrics.GaugeVec
	ReconciliationRetrievedResourcesTotal *k8smetrics.CounterVec
	featureState                          map[string]prometheus.Metric
	descriptions                          []*prometheus.Desc
}

func getCounterOpts(metricName string, helpMessage string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Name:        getFQMetricName(metricName),
		Help:        helpMessage,
		ConstLabels: constLabelsReconcile,
	}
}

func getHistogramOpts(metricName string, helpMessage string) prometheus.HistogramOpts {
	return prometheus.HistogramOpts{
		Name:        getFQMetricName(metricName),
		Help:        helpMessage,
		ConstLabels: constLabelsReconcile,
		Buckets:     prometheus.ExponentialBuckets(2, 2, 10),
	}
}

func setReconcileMetrics(providerMetrics *ProviderMetrics) {
	providerMetrics.ReconciliationTotalDuration = prometheus.NewHistogram(
		getHistogramOpts("reconcile_total_duration_seconds", "Histogram of times spent for one total reconciliation"),
	)

	providerMetrics.ReconciliationCreateErrorsTotal = prometheus.NewCounter(
		getCounterOpts("reconcile_create_errors_total", "Counter of errors while creating resources in a reconciliation"),
	)

	providerMetrics.ReconciliationDeleteRetriesTotal = prometheus.NewCounter(
		getCounterOpts("reconcile_delete_retries_total", "Counter of retries while deleting resources in a reconciliation"),
	)

	providerMetrics.ReconciliationDeleteErrorsTotal = prometheus.NewCounter(
		getCounterOpts("reconcile_delete_errors_total", "Counter of errors while deleting resources in a reconciliation"),
	)

	providerMetrics.ReconciliationCreatedTotal = prometheus.NewCounter(
		getCounterOpts("reconcile_created_total", "Counter of total created resources"),
	)

	providerMetrics.ReconciliationDeletedTotal = prometheus.NewCounter(
		getCounterOpts("reconcile_deleted_total", "Counter of total deleted resources"),
	)

	providerMetrics.ReconciliationCreateResources = prometheus.NewHistogram(
		getHistogramOpts("reconcile_create_resources_duration_seconds", "Histogram of times spent waiting for resources to become ready after creation"),
	)

	providerMetrics.ReconciliationPendingResources = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
		Name:        getFQMetricName("reconcile_resources_pending"),
		Help:        "Gauge of pending creation or deletion operations of resources",
		ConstLabels: constLabelsReconcile,
	}, []string{"operation"})

	providerMetrics.ReconciliationRetrievedResourcesTotal = k8smetrics.NewCounterVec(&k8smetrics.CounterOpts{
		Name:        getFQMetricName("reconcile_retrieved_resources_total"),
		Help:        "Counter of total numbers of resources retrieved grouped by type",
		ConstLabels: constLabelsReconcile,
	}, []string{"type"})
}

// NewProviderMetrics returns a prometheus.Collector for Provider Metrics.
func NewProviderMetrics(providerName, providerVersion string) ProviderMetrics {
	description := []*prometheus.Desc{
		descProviderBuild,
		descProviderFeatures,
	}

	versionMetric := prometheus.MustNewConstMetric(descProviderBuild, prometheus.CounterValue,
		1, providerName, providerVersion)

	providerMetrics := ProviderMetrics{
		providerVersion: versionMetric,
		descriptions:    description,
		m:               &sync.RWMutex{},
		featureState:    map[string]prometheus.Metric{},
	}

	setReconcileMetrics(&providerMetrics)

	return providerMetrics
}

func (p *ProviderMetrics) Describe(descs chan<- *prometheus.Desc) {
	for _, description := range p.descriptions {
		descs <- description
	}
}

func (p *ProviderMetrics) Collect(metrics chan<- prometheus.Metric) {
	metrics <- p.providerVersion
	metrics <- p.ReconciliationTotalDuration
	metrics <- p.ReconciliationCreateErrorsTotal
	metrics <- p.ReconciliationDeleteRetriesTotal
	metrics <- p.ReconciliationDeleteErrorsTotal
	metrics <- p.ReconciliationCreatedTotal
	metrics <- p.ReconciliationDeletedTotal
	metrics <- p.ReconciliationCreateResources

	p.m.RLock()
	defer p.m.RUnlock()
	for _, counter := range p.featureState {
		metrics <- counter
	}
}

func (p *ProviderMetrics) MarkFeatureEnabled(featureName string) {
	p.m.Lock()
	defer p.m.Unlock()

	featureState := prometheus.MustNewConstMetric(descProviderFeatures, prometheus.CounterValue, FeatureEnabled, featureName)
	p.featureState[featureName] = featureState
}

func (p *ProviderMetrics) MarkFeatureDisabled(featureName string) {
	p.m.Lock()
	defer p.m.Unlock()

	featureState := prometheus.MustNewConstMetric(descProviderFeatures, prometheus.CounterValue, FeatureDisabled, featureName)
	p.featureState[featureName] = featureState
}

func (p *ProviderMetrics) Create(_ *semver.Version) bool {
	return true
}

func (p *ProviderMetrics) ClearState() {}

func (p *ProviderMetrics) FQName() string {
	return fqCollectorName
}

func getFQMetricName(metricName string) string {
	return fmt.Sprintf("%s_%s", fqCollectorName, metricName)
}
