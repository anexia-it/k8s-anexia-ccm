package provider

import (
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type providerConfig struct {
}

type anxProvider struct {
	config *providerConfig
}

func (a anxProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (a anxProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (a anxProvider) Instances() (cloudprovider.Instances, bool) {
	return instanceController{}, false
}

func (a anxProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
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
	return "anx"
}

func (a anxProvider) HasClusterID() bool {
	return true
}

func init() {
	cloudprovider.RegisterCloudProvider("anx", func(configReader io.Reader) (cloudprovider.Interface, error) {
		if configReader == nil {
			klog.Info("no configuration was provided for the anx cloud-provider")
			return anxProvider{}, nil
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

		return anxProvider{config: &providerConfig}, nil
	})
}
