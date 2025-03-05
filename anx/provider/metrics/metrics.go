package metrics

import (
	"fmt"
	anxclient "go.anx.io/go-anxcloud/pkg/client"
	"path"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"

	k8smetrics "k8s.io/component-base/metrics"
)

const FeatureEnabled = 1
const FeatureDisabled = 0

const fqCollectorName = "cloud_provider_anexia"

var (
	constLabels = prometheus.Labels{"collector": "anexia-provider-collector"}

	descProviderBuild = prometheus.NewDesc(getFQMetricName("provider_build"),
		"information about the build version of a specific provider", []string{"name", "version"}, constLabels)

	descProviderFeatures = prometheus.NewDesc(getFQMetricName("feature"), "provider features and their state",
		[]string{"name"}, constLabels)
)

type ProviderMetrics struct {
	m                                     *sync.RWMutex
	Name                                  string
	providerVersion                       prometheus.Metric
	ReconciliationTotalDuration           *k8smetrics.HistogramVec
	ReconciliationCreateErrorsTotal       *k8smetrics.CounterVec
	ReconciliationDeleteRetriesTotal      *k8smetrics.CounterVec
	ReconciliationDeleteErrorsTotal       *k8smetrics.CounterVec
	ReconciliationCreatedTotal            *k8smetrics.CounterVec
	ReconciliationDeletedTotal            *k8smetrics.CounterVec
	ReconciliationCreateResources         *k8smetrics.HistogramVec
	ReconciliationPendingResources        *k8smetrics.GaugeVec
	ReconciliationRetrievedResourcesTotal *k8smetrics.CounterVec
	featureState                          map[string]prometheus.Metric
	descriptions                          []*prometheus.Desc
	HttpRequestDuration                   *prometheus.HistogramVec
	HttpClientRequestCount                *k8smetrics.CounterVec
	HttpClientRequestInFlight             *k8smetrics.GaugeVec
	HttpClientRequestDuration             *k8smetrics.HistogramVec
}

func getCounterOpts(metricName string, helpMessage string) *k8smetrics.CounterOpts {
	return &k8smetrics.CounterOpts{
		Name: getFQMetricName(metricName),
		Help: helpMessage,
	}
}

func getHistogramOpts(metricName string, helpMessage string) *k8smetrics.HistogramOpts {
	return &k8smetrics.HistogramOpts{
		Name:    getFQMetricName(metricName),
		Help:    helpMessage,
		Buckets: prometheus.ExponentialBuckets(2, 2, 10),
	}
}

func setReconcileMetrics(providerMetrics *ProviderMetrics) {
	providerMetrics.ReconciliationTotalDuration = k8smetrics.NewHistogramVec(
		getHistogramOpts("reconcile_total_duration_seconds", "Histogram of times spent for one total reconciliation"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationCreateErrorsTotal = k8smetrics.NewCounterVec(
		getCounterOpts("reconcile_create_errors_total", "Counter of errors while creating resources in a reconciliation"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationDeleteRetriesTotal = k8smetrics.NewCounterVec(
		getCounterOpts("reconcile_delete_retries_total", "Counter of retries while deleting resources in a reconciliation"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationDeleteErrorsTotal = k8smetrics.NewCounterVec(
		getCounterOpts("reconcile_delete_errors_total", "Counter of errors while deleting resources in a reconciliation"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationCreatedTotal = k8smetrics.NewCounterVec(
		getCounterOpts("reconcile_created_total", "Counter of total created resources"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationDeletedTotal = k8smetrics.NewCounterVec(
		getCounterOpts("reconcile_deleted_total", "Counter of total deleted resources"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationCreateResources = k8smetrics.NewHistogramVec(
		getHistogramOpts("reconcile_create_resources_duration_seconds", "Histogram of times spent waiting for resources to become ready after creation"),
		[]string{"service"},
	)

	providerMetrics.ReconciliationPendingResources = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
		Name: getFQMetricName("reconcile_resources_pending"),
		Help: "Gauge of pending creation or deletion operations of resources"},
		[]string{"service", "operation"},
	)

	providerMetrics.ReconciliationRetrievedResourcesTotal = k8smetrics.NewCounterVec(&k8smetrics.CounterOpts{
		Name: getFQMetricName("reconcile_retrieved_resources_total"),
		Help: "Counter of total numbers of resources retrieved grouped by type"},
		[]string{"service", "type"},
	)

	providerMetrics.HttpClientRequestDuration = k8smetrics.NewHistogramVec(
		getHistogramOpts("http_client_request_duration_seconds", "Duration from sending a request to Anexia Engine to retrieving the response in seconds"),
		[]string{"resource", "method"},
	)

	providerMetrics.HttpClientRequestCount = k8smetrics.NewCounterVec(&k8smetrics.CounterOpts{
		Name: getFQMetricName("http_client_requests_total"),
		Help: "Total amount of requests sent to Anexia Engine"},
		[]string{"resource", "method", "status"},
	)

	providerMetrics.HttpClientRequestInFlight = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
		Name: getFQMetricName("http_client_requests_in_flight"),
		Help: "Amount of requests sent to Anexia Engine currently waiting for response"},
		[]string{"resource", "method"},
	)
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

func (p *ProviderMetrics) MetricReceiver(metrics map[anxclient.Metric]float64, labels map[anxclient.MetricLabel]string) {
	var resource, method, status string

	for label, value := range labels {
		switch label {
		case anxclient.MetricLabelResource:
			resource = filterResourceLabel(value)
		case anxclient.MetricLabelMethod:
			method = value
		case anxclient.MetricLabelStatus:
			status = value
		}
	}

	for metric, value := range metrics {
		switch metric {
		case anxclient.MetricRequestDuration:
			p.HttpClientRequestDuration.WithLabelValues(resource, method).Observe(value)
		case anxclient.MetricRequestCount:
			p.HttpClientRequestCount.WithLabelValues(resource, method, status).Add(value)
		case anxclient.MetricRequestInflight:
			p.HttpClientRequestInFlight.WithLabelValues(resource, method).Add(value)
		}
	}
}

// filterResourceLabel takes the resource label given to the MetricReceiver by go-anxcloud client and tries to
// prevent swamping Prometheus with high-cardinality labels by
//   - removing the /api/ prefix (not high-cardinality relevant, but still nice)
//   - checking if the second to last path element ends with ".json", truncating the last path element in this case
//   - for metrics we do not care for the exact resource but the type of resource
//   - "it takes X seconds to retrieve VM infos"
//   - some resource-specific handling
//
// Having this here is if course not ideal, but it's the least-invasive way to add metrics to go-anxcloud and use
// them here. Once we have the new generic client in go-anxcloud for everything, this will get better as we then
// just generate metrics by Object type and Operation, not by URL.
func filterResourceLabel(resource string) string {
	resource = strings.TrimPrefix(resource, "/api/")

	if identifierRemoved := path.Base(path.Dir(resource)); strings.HasSuffix(identifierRemoved, ".json") {
		resource = identifierRemoved
	}

	// the vsphere info API endpoint is at "vsphere/v1/info.json/$identifier/info" for some reason, so the
	// identifier stripping above does not catch it
	const vsphereInfo = "vsphere/v1/info.json"
	if strings.HasPrefix(resource, vsphereInfo+"/") {
		resource = vsphereInfo
	}

	// the vsphere provisioning API endpoint is at
	// "vsphere/v1/provisioning/vm.json/$location/$template_type/$template", which again prevents the identifier
	// stripping above from catching it
	const vsphereProvisioning = "vsphere/v1/provisioning/vm.json"
	if strings.HasPrefix(resource, vsphereProvisioning+"/") {
		resource = vsphereProvisioning
	}

	return resource
}
