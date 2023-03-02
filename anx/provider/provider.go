package provider

import (
	"fmt"
	"io"
	"os"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-base/metrics/legacyregistry"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/loadbalancer"

	anexia "go.anx.io/go-anxcloud/pkg"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

const (
	featureNameLoadBalancer = "load_balancer_provisioning"
	featureNameInstancesV2  = "instances_v2"
)

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

	logger := klogr.NewWithOptions()

	legacyClient, err := client.New(client.TokenFromString(config.Token))
	if err != nil {
		return nil, fmt.Errorf("could not create legacy anexia client. %w", err)
	}

	genericClient, err := api.NewAPI(
		api.WithClientOptions(
			client.TokenFromString(config.Token),
		),
		api.WithLogger(logger.WithName("go-anxcloud")),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create generic anexia client. %w", err)
	}

	return &anxProvider{
		API:           anexia.NewAPI(legacyClient),
		genericClient: genericClient,
		legacyClient:  legacyClient,
		logger:        logger.WithName("anx/provider"),
		config:        &config,
	}, nil
}

func (a *anxProvider) Initialize(builder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	a.logger.Info("Anexia provider initializing", "version", Version)

	a.setupProviderMetrics()

	a.initializeLoadBalancerManager(builder)
	a.instanceManager = instanceManager{a}

	if a.config.CustomerID != "" {
		klog.Infof("running with customer prefix '%s'", a.config.CustomerID)
	} else {
		klog.Infof("running without customer prefix, will have to guess a tiny bit when matching virtual machines to Nodes")
	}
}

func (a *anxProvider) initializeLoadBalancerManager(builder cloudprovider.ControllerClientBuilder) {
	var k8sClient kubernetes.Interface

	if builder != nil {
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

func (a *anxProvider) setupProviderMetrics() {
	a.providerMetrics = metrics.NewProviderMetrics("anexia", Version)
	legacyregistry.MustRegister(&a.providerMetrics)
	legacyregistry.MustRegister(a.providerMetrics.ReconciliationPendingResources)
	legacyregistry.MustRegister(a.providerMetrics.ReconciliationRetrievedResourcesTotal)

	a.providerMetrics.MarkFeatureDisabled(featureNameLoadBalancer)
	a.providerMetrics.MarkFeatureDisabled(featureNameInstancesV2)
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
