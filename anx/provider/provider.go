package provider

import (
	"context"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	anexia "go.anx.io/go-anxcloud/pkg"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	anxClient "go.anx.io/go-anxcloud/pkg/client"
	"go.anx.io/go-anxcloud/pkg/core/resource"
	"io"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sort"
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

func (a *anxProvider) Replication() (sync.LoadBalancerReplicationManager, bool) {
	if a.Config().SecondaryLoadBalancersIdentifiers != nil && a.Config().LoadBalancerIdentifier != "" {
		a.loadBalancerManager.notify = make(chan struct{}, 10)
		return a.loadBalancerManager, true
	}

	return nil, false
}

func (a *anxProvider) Initialize(builder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	a.instanceManager = instanceManager{a}
	if a.Config().AutoDiscoverLoadBalancer {
		balancer, secondaryLoadBalancers, err := autoDiscoverLoadBalancer(a, stop)
		if err != nil {
			panic(fmt.Errorf("unable to autodiscover loadbalancer to configure"))
		}

		klog.Infof("discovered load balancer '%s'", balancer)
		if secondaryLoadBalancers != nil {
			klog.Infof("discovered load balancers for replication %v", secondaryLoadBalancers)
		}

		a.config.SecondaryLoadBalancersIdentifiers = secondaryLoadBalancers
		a.config.LoadBalancerIdentifier = balancer
	}

	a.loadBalancerManager = loadBalancerManager{Provider: a, notify: nil}
	klog.Infof("Running with customer prefix '%s'", a.config.CustomerID)
}

func autoDiscoverLoadBalancer(a *anxProvider, stop <-chan struct{}) (string, []string, error) {
	newAPI, err := api.NewAPI(api.WithClientOptions(anxClient.Logger(klogr.New()), anxClient.TokenFromString(a.Config().Token)))
	if err != nil {
		return "", nil, err
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
	}, api.Paged(1, 100, &pageIter))

	if err != nil {
		return "", nil, fmt.Errorf("unable to autodisover load balancer by tag '%s': %w", tag, err)
	}

	var infos []resource.Info
	pageIter.Next(&infos)
	if pageIter.TotalPages() > 1 {
		klog.Errorf("too many load balancers were discovered currently only 100")
	}

	var identifiers []string

	for _, info := range infos {
		identifiers = append(identifiers, info.Identifier)
	}
	sort.Strings(identifiers)

	var secondaryLoadBalancers []string
	secondaryLoadBalancers = append(secondaryLoadBalancers, identifiers[1:]...)

	primaryLB := identifiers[0]
	err = newAPI.Get(ctx, resource.Info{Identifier: primaryLB})
	if err != nil {
		klog.Errorf("checking if load balancer '%s' exists returned an error: %s", primaryLB,
			err.Error())
		return "", nil, err
	}

	return primaryLB, secondaryLoadBalancers, nil
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
