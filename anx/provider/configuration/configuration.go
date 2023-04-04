package configuration

import (
	"fmt"
	"io"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
	"k8s.io/cloud-provider/options"
)

type ProviderConfig struct {
	Token                  string `yaml:"anexiaToken" split_words:"true"`
	CustomerID             string `yaml:"customerID,omitempty" split_words:"true"`
	ClusterName            string `yaml:"clusterName,omitempty" split_words:"true"`
	AutoDiscoveryTagPrefix string `yaml:"autoDiscoveryTagPrefix,omitempty" split_words:"true" default:"anxkube-ccm-lb"`

	// if ccm shall discover $LoadBalancerIdentifier, $SecondaryLoadBalancerIdentifiers and $LoadBalancerPrefixIdentifiers via tag "$AutoDiscoveryTagPrefix-$ClusterName"
	AutoDiscoverLoadBalancer bool `yaml:"autoDiscoverLoadBalancer,omitempty" split_words:"true"`

	// the LBaaS LoadBalancer resource to configure for LoadBalancer Services
	LoadBalancerIdentifier string `yaml:"loadBalancerIdentifier,omitempty" split_words:"true"`

	// identifiers of LBaaS LoadBalancer resources to keep in sync with $LoadBalancerIdentifier
	SecondaryLoadBalancerIdentifiers []string `yaml:"secondaryLoadBalancersIdentifiers" split_words:"true"`

	// lists the identifiers of prefixes from which external IPs for LoadBalancer Services can be allocated
	LoadBalancerPrefixIdentifiers []string `yaml:"loadBalancerPrefixIdentifiers,omitempty" split_words:"true"`

	// defines the number of retries to wait for LoadBalancer resources to be ready
	LoadBalancerBackoffSteps int `yaml:"loadBalancerBackoffSteps" default:"30"`
}

const (
	CloudProviderName = "anexia"
)

// managerOptions is used to capture the configuration values that are added via cmd flags
var managerOptions *options.CloudControllerManagerOptions

func GetManagerOptions() (*options.CloudControllerManagerOptions, error) {
	if managerOptions != nil {
		return managerOptions, nil
	}

	var err error
	managerOptions, err = options.NewCloudControllerManagerOptions()
	return managerOptions, err
}

var (
	CloudProviderScheme = fmt.Sprintf("%s://", CloudProviderName)
)

func NewProviderConfig(configReader io.Reader) (ProviderConfig, error) {
	var providerConfig ProviderConfig
	if configReader != nil {
		config, err := io.ReadAll(configReader)
		if err != nil {
			return ProviderConfig{}, err
		}
		err = yaml.Unmarshal(config, &providerConfig)
		if err != nil {
			return ProviderConfig{}, err
		}
	}

	err := envconfig.Process("ANEXIA", &providerConfig)
	if err != nil {
		return ProviderConfig{}, err
	}

	err = applyCliFlagsToProviderConfig(&providerConfig)

	return providerConfig, err
}

func applyCliFlagsToProviderConfig(providerConfig *ProviderConfig) error {
	managerOptions, err := GetManagerOptions()
	if err != nil {
		return err
	}
	sharedConfigClusterName := managerOptions.KubeCloudShared.ClusterName

	// kubernetes is a fallback name we don't want to use
	if sharedConfigClusterName != "kubernetes" {
		providerConfig.ClusterName = sharedConfigClusterName
	}

	return nil
}
