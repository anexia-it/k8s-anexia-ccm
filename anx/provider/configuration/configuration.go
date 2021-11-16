package configuration

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
	"io"
)

type ProviderConfig struct {
	Token                    string `yaml:"anexiaToken" split_words:"true"`
	CustomerID               string `yaml:"customerID,omitempty" split_words:"true"`
	LoadBalancerIdentifier   string `yaml:"loadBalancerIdentifier,omitempty" split_words:"true"`
	AutoDiscoverLoadBalancer bool   `yaml:"AutoDiscoverLoadBalancer,omitempty" split_words:"true"`
}

const (
	CloudProviderName = "anexia"
)

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
	return providerConfig, nil
}
