package provider

import (
	"context"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/metrics"
	"io"
	"k8s.io/component-base/metrics/legacyregistry"
	"os"
	"time"

	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/discovery"

	anexia "go.anx.io/go-anxcloud/pkg"
	anxClient "go.anx.io/go-anxcloud/pkg/client"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
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
	config              *configuration.ProviderConfig
	client              anxClient.Client
	instanceManager     instanceManager
	loadBalancerManager loadBalancerManager

	// providerMetrics is used to collect metrics inside this provider
	providerMetrics metrics.ProviderMetrics
}

func newAnxProvider(config configuration.ProviderConfig) (*anxProvider, error) {
	// make sure that token is also set as env so various managers can create clients without using the config
	err := os.Setenv("ANEXIA_TOKEN", config.Token)
	if err != nil {
		return nil, err
	}

	client, err := anxClient.New(anxClient.TokenFromString(config.Token))
	if err != nil {
		return nil, fmt.Errorf("could not create anexia client. %w", err)
	}

	return &anxProvider{
		API:    anexia.NewAPI(client),
		client: client,
		config: &config,
	}, nil
}

func (a *anxProvider) Replication() (sync.LoadBalancerReplicationManager, bool) {
	const featureName = "load_balancer_config_replication"
	if a.isLBaaSReplicationEnabled() {
		a.providerMetrics.MarkFeatureEnabled(featureName)
	} else {
		a.providerMetrics.MarkFeatureDisabled(featureName)
	}

	return a.loadBalancerManager, a.isLBaaSReplicationEnabled()
}

func (a *anxProvider) Initialize(builder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.Infof("Anexia provider version %s", Version)

	a.setupProviderMetrics()

	a.instanceManager = instanceManager{a}
	if a.Config().AutoDiscoverLoadBalancer {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancelFunc()
		go func() {
			select {
			case <-stop:
				cancelFunc()
			case <-time.After(1 * time.Minute):
			}
		}()

		autodiscoverLBsTag := fmt.Sprintf("%s-%s", a.Config().AutoDiscoveryTagPrefix, a.Config().ClusterName)
		autodiscoverLBPrefixesTag := fmt.Sprintf("kubernetes-lb-prefix-%s", a.Config().ClusterName)

		balancer, secondaryLoadBalancers, err := discovery.AutoDiscoverLoadBalancer(ctx, autodiscoverLBsTag)
		if err != nil {
			klog.Errorf("Configured to autodiscover LoadBalancers, but discovery failed", "error", err)
		} else {
			a.config.LoadBalancerIdentifier = balancer
			klog.Infof("discovered load balancer '%s'", balancer)

			if secondaryLoadBalancers != nil {
				a.config.SecondaryLoadBalancerIdentifiers = secondaryLoadBalancers
				klog.Infof("discovered load balancers for replication %v", secondaryLoadBalancers)
			}

			prefixes, err := discovery.AutoDiscoverLoadBalancerPrefixes(ctx, autodiscoverLBPrefixesTag)
			if err != nil {
				klog.Errorf("Configured to autodiscover LoadBalancers, but discovering external prefixes failed", "error", err)
			} else {
				a.config.LoadBalancerPrefixIdentifiers = prefixes
			}
		}
	}

	a.loadBalancerManager = newLoadBalancerManager(a, builder)

	if a.isLBaaSReplicationEnabled() {
		a.loadBalancerManager.notify = make(chan struct{}, 1)
	}

	klog.Infof("running with customer prefix '%s'", a.config.CustomerID)
}

func (a *anxProvider) isLBaaSReplicationEnabled() bool {
	return len(a.Config().SecondaryLoadBalancerIdentifiers) != 0 && a.Config().LoadBalancerIdentifier != ""
}

func (a anxProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	a.providerMetrics.MarkFeatureEnabled(featureNameLoadBalancer)
	return &a.loadBalancerManager, true
}

func (a anxProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (a anxProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
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
