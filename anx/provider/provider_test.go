package provider

import (
	"bytes"
	"fmt"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/stretchr/testify/require"
	cloudprovider "k8s.io/cloud-provider"
	"os"
	"testing"
)

func TestNewProvider(t *testing.T) {
	provider, err := newAnxProvider(configuration.ProviderConfig{
		Token:      "RANDOME_VALUE",
		CustomerID: "CUSTOMER",
	})
	require.NoError(t, err)
	require.NotNil(t, provider)

	provider.Initialize(nil, nil)
	instances, instancesEnabled := provider.Instances()
	instancesV2, instancesV2Enabled := provider.InstancesV2()
	zones, zonesEnabled := provider.Zones()
	hasClusterID := provider.HasClusterID()
	routes, routesEnabled := provider.Routes()
	clusters, clustersEnabled := provider.Clusters()
	providerName := provider.ProviderName()
	loadbalancer, loadbalancerEnabled := provider.LoadBalancer()

	require.Nil(t, instances)
	require.NotNil(t, instancesV2)
	require.Nil(t, zones)
	require.Nil(t, routes)
	require.Nil(t, clusters)
	require.NotNil(t, loadbalancer)
	require.False(t, instancesEnabled)
	require.True(t, instancesV2Enabled)
	require.False(t, zonesEnabled)
	require.False(t, routesEnabled)
	require.True(t, hasClusterID)
	require.False(t, clustersEnabled)
	require.True(t, loadbalancerEnabled)

	require.Equal(t, configuration.CloudProviderName, providerName)
	require.IsType(t, instanceManager{}, instancesV2)
	manager := instancesV2.(instanceManager)
	require.Equal(t, provider, manager.Provider)
}

func TestRegisterCloudProvider(t *testing.T) {
	require.NoError(t, os.Setenv("ANEXIA_TOKEN", "TOKEN"))
	t.Cleanup(func() {
		require.NoError(t, os.Unsetenv("ANEXIA_TOKEN"))
	})
	provider, err := cloudprovider.GetCloudProvider("anexia", bytes.NewReader([]byte("customerID: 555")))
	require.NoError(t, err)
	require.NotNil(t, provider)
	anxProvider, ok := provider.(*anxProvider)
	require.True(t, ok)
	config := anxProvider.Config()
	require.Equal(t, "555", config.CustomerID)
	require.Equal(t, "TOKEN", config.Token)
}

func TestProviderScheme(t *testing.T) {
	t.Parallel()
	require.Equal(t, fmt.Sprintf("%s://", configuration.CloudProviderName), configuration.CloudProviderScheme)
}

func TestProviderConfig(t *testing.T) {
	t.Parallel()
	provider, err := newAnxProvider(configuration.ProviderConfig{
		Token:      "5555",
		CustomerID: "5555",
	})
	require.NoError(t, err)

	config := provider.Config()

	require.Equal(t, "5555", config.CustomerID)
}
