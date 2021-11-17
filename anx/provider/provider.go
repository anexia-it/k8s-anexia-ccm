package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	anexia "github.com/anexia-it/go-anxcloud/pkg"
	"github.com/anexia-it/go-anxcloud/pkg/api"
	"github.com/anexia-it/go-anxcloud/pkg/api/types"
	anxClient "github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/core/resource"
	"io"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"time"
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
	client, err := anxClient.New(anxClient.TokenFromString(config.Token))
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
	if a.Config().AutoDiscoverLoadBalancer {
		balancer, err := autoDiscoverLoadBalancer(a, stop)
		if err != nil {
			panic(fmt.Errorf("unable to autodiscover loadbalancer to configure"))
		}
		a.config.LoadBalancerIdentifier = balancer
	}
	a.loadBalancerManager = loadBalancerManager{a}
	klog.Infof("Running with customer prefix '%s'", a.config.CustomerID)
}

func autoDiscoverLoadBalancer(a *anxProvider, stop <-chan struct{}) (string, error) {
	newAPI, err := api.NewAPI(api.WithClientOptions(anxClient.TokenFromString(a.Config().Token)))
	if err != nil {
		return "", err
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFunc()
	go func() {
		select {
		case <-stop:
			cancelFunc()
		case <-time.After(1 * time.Minute):
		}
	}()

	tag := fmt.Sprintf("%s-%s", a.Config().AutoDiscoveryTagPrefix, a.Config().ClusterName)
	var pageIter types.PageInfo
	err = newAPI.List(ctx, &resource.Info{
		Tags: []string{tag},
	}, api.Paged(1, 1, &pageIter))

	var infos []resource.Info
	pageIter.Next(&infos)
	if pageIter.TotalPages() > 1 {
		return "", errors.New("more than one resource was marked with LoadBalancer discovery tag")
	}

	if len(infos) != 1 {
		return "", fmt.Errorf("expected one resource to be tagged with '%s'", a.Config().ClusterName)
	}

	taggedResource := infos[0]
	err = newAPI.Get(ctx, &taggedResource)
	if err != nil {
		return "", err
	}

	return taggedResource.Identifier, nil
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
