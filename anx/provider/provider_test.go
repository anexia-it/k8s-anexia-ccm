package provider

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/require"
	cloudprovider "k8s.io/cloud-provider"
	"testing"
)

func TestNewProvider(t *testing.T) {
	provider, err := newAnxProvider(providerConfig{
		"RANDOME_VALUE",
		"CUSTOMER",
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
	require.Nil(t, loadbalancer)
	require.False(t, instancesEnabled)
	require.True(t, instancesV2Enabled)
	require.False(t, zonesEnabled)
	require.False(t, routesEnabled)
	require.True(t, hasClusterID)
	require.False(t, clustersEnabled)
	require.False(t, loadbalancerEnabled)

	require.Equal(t, cloudProviderName, providerName)
	require.IsType(t, instanceManager{}, instancesV2)
	manager := instancesV2.(instanceManager)
	require.Equal(t, provider, manager.Provider)
}

func TestRegisterCloudProvider(t *testing.T) {
	provider, err := cloudprovider.GetCloudProvider("anx", bytes.NewReader([]byte("anexiaToken: VALUE\ncustomerID: 555")))
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestProviderScheme(t *testing.T) {
	t.Parallel()
	require.Equal(t, fmt.Sprintf("%s://", cloudProviderName), cloudProviderScheme)
}

func TestProviderConfig(t *testing.T) {
	t.Parallel()
	require.NoError(t, os.Setenv("ANEXIA_TOKEN", "RANDOM_VALUE"))
	t.Cleanup(func() {
		require.NoError(t, os.Unsetenv("ANEXIA_TOKEN"))
	})
	provider, err := newAnxProvider(providerConfig{
		"5555",
	})
	require.NoError(t, err)

	config := provider.Config()

	require.Equal(t, "5555", config.CustomerPrefix)

}
