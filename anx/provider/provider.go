package provider

import (
	"encoding/json"
	"fmt"
	anexia "github.com/anexia-it/go-anxcloud/pkg"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"io"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const cloudProviderName = "anx"

var (
	cloudProviderScheme = fmt.Sprintf("%s://", cloudProviderName)
)

type providerConfig struct {
}

type Provider interface {
	anexia.API
}

type anxProvider struct {
	anexia.API
	config          providerConfig
	instanceManager instanceManager
}

func newAnxProvider(config providerConfig) (*anxProvider, error) {
	client, err := client.New(client.AuthFromEnv(false))
	if err != nil {
		return nil, fmt.Errorf("could not create anexia client. %w", err)
	}
	return &anxProvider{
		API:    anexia.NewAPI(client),
		config: config,
	}, nil
}

func (a *anxProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	a.instanceManager = instanceManager{a}
}

func (a anxProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
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
	return cloudProviderName
}

func (a anxProvider) HasClusterID() bool {
	return true
}

func registerCloudProvider() {
	cloudprovider.RegisterCloudProvider("anx", func(configReader io.Reader) (cloudprovider.Interface, error) {
		if configReader == nil {
			klog.Info("no configuration was provided for the anx cloud-provider")
			return newAnxProvider(providerConfig{})
		}

		config, err := io.ReadAll(configReader)
		if err != nil {
			return nil, err
		}
		var providerConfig providerConfig
		err = json.Unmarshal(config, &providerConfig)
		if err != nil {
			return nil, err
		}

		provider, err := newAnxProvider(providerConfig)
		if err != nil {
			return nil, err
		}
		return provider, nil
	})
}

func init() {
	registerCloudProvider()
}
