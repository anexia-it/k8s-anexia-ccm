package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-base/metrics/legacyregistry"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/kubernetes/resourceclient"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/loadbalancer"

	anexia "go.anx.io/go-anxcloud/pkg"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	featureNameLoadBalancer = "load_balancer_provisioning"
	featureNameInstancesV2  = "instances_v2"
)

var akeLoadBalancerTarget = resourceclient.Target{
	Group:    "ake.k8s.anx.io",
	Version:  "v1",
	Resource: "loadbalancers",
}

var Version = "v0.0.0-unreleased"

type Provider interface {
	anexia.API
	Config() *configuration.ProviderConfig
}

type anxProvider struct {
	anexia.API

	logger logr.Logger
	config *configuration.ProviderConfig

	genericClient api.API
	legacyClient  client.Client

	instanceManager     cloudprovider.InstancesV2
	loadBalancerManager cloudprovider.LoadBalancer

	// providerMetrics is used to collect metrics inside this provider
	providerMetrics metrics.ProviderMetrics
}

func newAnxProvider(config configuration.ProviderConfig) (*anxProvider, error) {
	// make sure that token is also set as env so various managers can create clients without using the config
	err := os.Setenv("ANEXIA_TOKEN", config.Token)
	if err != nil {
		return nil, err
	}

	logger := klog.NewKlogr()

	httpClient := http.Client{Timeout: 30 * time.Second}

	providerMetrics := setupProviderMetrics()
	legacyClient, err := client.New(
		client.TokenFromString(config.Token),
		client.WithMetricReceiver(providerMetrics.MetricReceiver),
		client.HTTPClient(&httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create legacy anexia client. %w", err)
	}

	genericClient, err := api.NewAPI(
		api.WithClientOptions(
			client.TokenFromString(config.Token),
			client.WithMetricReceiver(providerMetrics.MetricReceiver),
			client.HTTPClient(&httpClient),
		),
		api.WithLogger(logger.WithName("go-anxcloud")),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create generic anexia client. %w", err)
	}

	return &anxProvider{
		API:             anexia.NewAPI(legacyClient),
		genericClient:   genericClient,
		legacyClient:    legacyClient,
		logger:          logger.WithName("anx/provider"),
		config:          &config,
		providerMetrics: providerMetrics,
	}, nil
}

func (a *anxProvider) Initialize(builder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	a.logger.Info("Anexia provider initializing", "version", Version)

	a.initializeLoadBalancerManager(builder)
	a.instanceManager = &instanceManager{Provider: a}

	if a.config.CustomerID != "" {
		klog.Infof("running with customer prefix '%s'", a.config.CustomerID)
	} else {
		klog.Infof("running without customer prefix, will have to guess a tiny bit when matching virtual machines to Nodes")
	}
}

func (a *anxProvider) initializeLoadBalancerManager(builder cloudprovider.ControllerClientBuilder) {
	var k8sClient kubernetes.Interface

	if builder != nil {
		a.disableEngineLoadBalancerForAKE(builder)

		c, err := builder.Client("LoadBalancer")
		if err != nil {
			a.logger.Error(err, "Error creating kubernetes client for LoadBalancer manager")
		} else {
			k8sClient = c
		}
	}
	config := a.Config()
	logger := a.logger.WithName("LoadBalancer")

	if lb, err := loadbalancer.New(config, logger, k8sClient, a.genericClient, a.legacyClient, a.providerMetrics); err != nil {
		a.logger.Error(err, "Error initializing LoadBalancer manager")
	} else {
		a.loadBalancerManager = lb
	}
}

// disableEngineLoadBalancerForAKE applies the same manager configuration as
// DisableEngineLoadBalancer when an AKE Loadbalancer resource indicates that
// another controller owns the Engine LBaaS configuration.
func (a *anxProvider) disableEngineLoadBalancerForAKE(builder cloudprovider.ControllerClientBuilder) {
	if a.config.DisableEngineLoadBalancer {
		return
	}

	restConfig, err := builder.Config("LoadBalancerResourceDetection")
	if err != nil {
		a.logger.Error(err, "Unable to create Kubernetes config for AKE Loadbalancer detection; preserving configured Engine LBaaS reconciliation behavior")
		return
	}

	client, err := resourceclient.NewForConfig(restConfig, akeLoadBalancerTarget)
	if err != nil {
		a.logger.Error(err, "Unable to create dynamic Kubernetes client for AKE Loadbalancer detection; preserving configured Engine LBaaS reconciliation behavior")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exists, err := akeLoadBalancerExists(ctx, client)
	if err != nil {
		a.logger.Error(err, "Unable to detect AKE Loadbalancer resources; preserving configured Engine LBaaS reconciliation behavior")
		return
	}
	if !exists {
		return
	}

	a.config.DisableEngineLoadBalancer = true
	a.logger.Info(
		"Disabling Anexia Engine LBaaS reconciliation because an AKE Loadbalancer resource exists",
		"api-group", akeLoadBalancerTarget.Group,
		"api-version", akeLoadBalancerTarget.Version,
		"resource", akeLoadBalancerTarget.Resource,
	)
}

func akeLoadBalancerExists(ctx context.Context, client *resourceclient.Client) (bool, error) {
	exists, err := client.Any(ctx, metav1.ListOptions{}, nil)
	if apierrors.IsNotFound(err) {
		// Clusters that do not use AKE do not have this CRD installed.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check for AKE Loadbalancer resources: %w", err)
	}

	return exists, nil
}

func (a anxProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	if a.loadBalancerManager == nil {
		return nil, false
	}

	a.providerMetrics.MarkFeatureEnabled(featureNameLoadBalancer)
	return a.loadBalancerManager, true
}

func (a anxProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (a anxProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	if a.instanceManager == nil {
		return nil, false
	}

	a.providerMetrics.MarkFeatureEnabled(featureNameInstancesV2)
	return a.instanceManager, true
}

func (a anxProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (a anxProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (a anxProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (a anxProvider) ProviderName() string {
	return configuration.CloudProviderName
}

func (a anxProvider) HasClusterID() bool {
	return true
}

func (a anxProvider) Config() *configuration.ProviderConfig {
	return a.config
}

var registerOnce sync.Once

func setupProviderMetrics() metrics.ProviderMetrics {
	providerMetrics := metrics.NewProviderMetrics("anexia", Version)
	registerOnce.Do(func() {
		legacyregistry.MustRegister(&providerMetrics)
		legacyregistry.MustRegister(providerMetrics.ReconciliationTotalDuration)
		legacyregistry.MustRegister(providerMetrics.ReconciliationCreateErrorsTotal)
		legacyregistry.MustRegister(providerMetrics.ReconciliationDeleteRetriesTotal)
		legacyregistry.MustRegister(providerMetrics.ReconciliationDeleteErrorsTotal)
		legacyregistry.MustRegister(providerMetrics.ReconciliationCreatedTotal)
		legacyregistry.MustRegister(providerMetrics.ReconciliationDeletedTotal)
		legacyregistry.MustRegister(providerMetrics.ReconciliationCreateResources)
		legacyregistry.MustRegister(providerMetrics.ReconciliationPendingResources)
		legacyregistry.MustRegister(providerMetrics.ReconciliationRetrievedResourcesTotal)
		legacyregistry.MustRegister(providerMetrics.HttpClientRequestCount)
		legacyregistry.MustRegister(providerMetrics.HttpClientRequestInFlight)
	})

	providerMetrics.MarkFeatureDisabled(featureNameLoadBalancer)
	providerMetrics.MarkFeatureDisabled(featureNameInstancesV2)
	return providerMetrics
}

func registerCloudProvider() {
	cloudprovider.RegisterCloudProvider("anexia", func(configReader io.Reader) (cloudprovider.Interface, error) {
		config, err := configuration.NewProviderConfig(configReader)
		if err != nil {
			return nil, err
		}

		provider, err := newAnxProvider(config)
		if err != nil {
			return nil, err
		}
		return provider, nil
	})
}

func init() {
	registerCloudProvider()
}
