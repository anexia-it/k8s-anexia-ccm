package provider

import (
	"fmt"
	anexia "github.com/anexia-it/go-anxcloud/pkg"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
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

type provider struct {
	anexia.API
	config          providerConfig
	instanceManager instanceManager
}

func newAnxProvider(config providerConfig) (*provider, error) {
	client, err := client.New(client.AuthFromEnv(false))
	if err != nil {
		return nil, fmt.Errorf("could not create anexia client. %w", err)
	}
	return &provider{
		API:    anexia.NewAPI(client),
		config: config,
	}, nil
}

func (a *provider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	a.instanceManager = instanceManager{a}
}

func (a provider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (a provider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (a provider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return a.instanceManager, true
}

func (a provider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (a provider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (a provider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (a provider) ProviderName() string {
	return cloudProviderName
}

func (a provider) HasClusterID() bool {
	return true
}

func init() {
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
