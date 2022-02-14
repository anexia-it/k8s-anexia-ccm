package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

const FeatureEnabled = 1
const FeatureDisabled = 0

var (
	descProviderBuild = prometheus.NewDesc("provider_build",
		"information about the build version of a specific provider", []string{"name", "version"}, nil)

	descProviderFeatures = prometheus.NewDesc("feature", "provider features and their state",
		[]string{"name"}, nil)
)

type ProviderMetrics struct {
	m               *sync.RWMutex
	Name            string
	providerVersion prometheus.Metric
	featureState    map[string]prometheus.Metric
	descriptions    []*prometheus.Desc
}

// NewProviderMetrics returns a prometheus.Collector for Provider Metrics.
func NewProviderMetrics(providerName, providerVersion string) ProviderMetrics {
	description := []*prometheus.Desc{descProviderBuild, descProviderFeatures}

	versionMetric := prometheus.MustNewConstMetric(descProviderBuild, prometheus.GaugeValue,
		1, "name", providerName, "version", providerVersion)

	return ProviderMetrics{
		Name:            providerName,
		providerVersion: versionMetric,
		descriptions:    description,
	}
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

	featureState := prometheus.MustNewConstMetric(descProviderFeatures, prometheus.CounterValue, FeatureEnabled,
		"name", featureName)
	p.featureState[featureName] = featureState
}

func (p *ProviderMetrics) MarkFeatureDisabled(featureName string) {
	p.m.Lock()
	defer p.m.Unlock()

	featureState := prometheus.MustNewConstMetric(descProviderFeatures, prometheus.CounterValue, FeatureDisabled,
		"name", featureName)
	p.featureState[featureName] = featureState
}
