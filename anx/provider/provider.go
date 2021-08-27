package provider

import (
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	anexia "github.com/anexia-it/go-anxcloud/pkg"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"io"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type Provider interface {
	anexia.API
	Config() *configuration.ProviderConfig
}

type anxProvider struct {
	anexia.API
	config              *configuration.ProviderConfig
	instanceManager     instanceManager
	loadBalancerManager loadBalancerManager
}

func newAnxProvider(config configuration.ProviderConfig) (*anxProvider, error) {
	client, err := client.New(client.TokenFromString(config.Token))
	if err != nil {
		return nil, fmt.Errorf("could not create anexia client. %w", err)
	}

	return &anxProvider{
		API:    anexia.NewAPI(client),
		config: &config,
	}, nil
}

func (a *anxProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	a.instanceManager = instanceManager{a}
	a.loadBalancerManager = loadBalancerManager{a}
	klog.Infof("Running with customer prefix '%s'", a.config.CustomerID)
}

func (a anxProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return a.loadBalancerManager, true
}

func (a anxProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (a anxProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
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
