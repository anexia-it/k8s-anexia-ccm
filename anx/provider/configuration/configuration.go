package configuration

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
	"io"
	"k8s.io/cloud-provider/options"
)

type ProviderConfig struct {
	Token                            string   `yaml:"anexiaToken" split_words:"true"`
	CustomerID                       string   `yaml:"customerID,omitempty" split_words:"true"`
	LoadBalancerIdentifier           string   `yaml:"loadBalancerIdentifier,omitempty" split_words:"true"`
	ClusterName                      string   `yaml:"clusterName,omitempty" split_words:"true"`
	AutoDiscoveryTagPrefix           string   `yaml:"autoDiscoveryTagPrefix,omitempty" split_words:"true" default:"anxkube-ccm-lb"`
	AutoDiscoverLoadBalancer         bool     `yaml:"autoDiscoverLoadBalancer,omitempty" split_words:"true"`
	SecondaryLoadBalancerIdentifiers []string `yaml:"secondaryLoadBalancersIdentifiers" split_words:"trues"`
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
