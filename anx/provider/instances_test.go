package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/info"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/powercontrol"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/search"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"testing"
)

const nodeIdentifier = "test-ident"

func TestFetchingID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const nodeName = "test-node"

	t.Run("GetProviderIDForNode/NoProviderID", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.searchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: nodeIdentifier,
			}}, nil)

		manager := instanceManager{provider}

		providerId, err := manager.InstanceIDByNode(ctx, &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Spec: v1.NodeSpec{},
		})

		require.NoError(t, err)
		require.Equal(t, nodeIdentifier, providerId)
	})

	t.Run("GetProviderIDForNode/WithProviderID", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()

		manager := instanceManager{provider}

		providerId, err := manager.InstanceIDByNode(ctx, &v1.Node{
			Spec: v1.NodeSpec{ProviderID: fmt.Sprintf("%s%s", cloudProviderScheme, nodeIdentifier)},
		})

		require.NoError(t, err)
		require.Equal(t, nodeIdentifier, providerId)
	})

	t.Run("GetProviderIDForNode/MultipleVMs", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.searchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Name:       "VM1",
				Identifier: nodeIdentifier,
			},
			{
				Name:       "VM2",
				Identifier: "secondIdentifier",
			},
		}, nil)

		manager := instanceManager{provider}

		_, err := manager.InstanceIDByNode(ctx, &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Spec: v1.NodeSpec{},
		})

		require.Error(t, err)
	})
}

func TestInstanceExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := providerManagedNode()

	t.Run("InstanceExists", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{}, nil)
		manager := instanceManager{provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.NoError(t, err)
		require.True(t, exists, "expected instance to exist")
	})

	t.Run("InstanceDoesNotExist", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{}, &client.ResponseError{
			Response: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		})
		manager := instanceManager{provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.NoError(t, err)
		require.False(t, exists, "expected instance to exist")
	})

	t.Run("UnknownError", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{}, errors.New("unknownError"))
		manager := instanceManager{provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.Error(t, err)
		require.False(t, exists, "expected instance to exist")
	})
}

func TestInstanceShutdown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const nodeName = "test-node"
	node := providerManagedNode()

	t.Run("PoweredOn", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.searchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: nodeIdentifier,
			}}, nil)

		provider.powerControlMock.On("Get", ctx, nodeIdentifier).Return(powercontrol.OnState, nil)

		manager := instanceManager{provider}
		isShutdown, err := manager.InstanceShutdown(ctx, &node)
		require.NoError(t, err)
		require.False(t, isShutdown)
	})

	t.Run("PoweredOff", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.searchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: nodeIdentifier,
			}}, nil)

		provider.powerControlMock.On("Get", ctx, nodeIdentifier).Return(powercontrol.OffState, nil)

		manager := instanceManager{provider}
		isShutdown, err := manager.InstanceShutdown(ctx, &node)
		require.NoError(t, err)
		require.True(t, isShutdown)
	})

	t.Run("UnknownPowerState", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.searchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: nodeIdentifier,
			}}, nil)

		provider.powerControlMock.On("Get", ctx, nodeIdentifier).Return(powercontrol.State("NoExistentState"), nil)

		manager := instanceManager{provider}
		_, err := manager.InstanceShutdown(ctx, &node)
		require.Error(t, err)
	})
}

func TestInstanceTypeFromInfo(t *testing.T) {
	t.Parallel()
	instanceTypeStr := instanceType(info.Info{
		RAM: 4096,
		CPU: 5,
	})

	require.Equal(t, "C5-M4", instanceTypeStr)
}

func TestInstanceMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := providerManagedNode()

	t.Run("OneNetwork", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{
			CPU:             5,
			RAM:             4096,
			LocationCountry: "AT",
			LocationCode:    "AT04",
			Network: []info.Network{
				{
					IPv4: []string{"10.0.0.1"},
				},
			},
		}, nil)
		manager := instanceManager{provider}

		metadata, err := manager.InstanceMetadata(ctx, &node)
		require.NoError(t, err)
		require.Equal(t, metadata.InstanceType, "C5-M4")
		require.Equal(t, metadata.Zone, "AT04")
		require.Equal(t, metadata.Region, "AT")
		require.Len(t, metadata.NodeAddresses, 1)
		require.Equal(t, metadata.NodeAddresses[0].Address, "10.0.0.1")
		require.Equal(t, string(metadata.NodeAddresses[0].Type), "InternalIP")
	})

	t.Run("MultipleNetworks", func(t *testing.T) {
		t.Parallel()
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{
			CPU:             5,
			RAM:             4096,
			LocationCountry: "AT",
			LocationCode:    "AT04",
			Network: []info.Network{
				{
					IPv4: []string{"10.0.0.1"},
				},
				{
					IPv4: []string{"172.16.0.1"},
				},
			},
		}, nil)
		manager := instanceManager{provider}

		metadata, err := manager.InstanceMetadata(ctx, &node)
		require.NoError(t, err)
		require.Equal(t, metadata.InstanceType, "C5-M4")
		require.Equal(t, metadata.Zone, "AT04")
		require.Equal(t, metadata.Region, "AT")
		require.Len(t, metadata.NodeAddresses, 1)
		require.Equal(t, metadata.NodeAddresses[0].Address, "10.0.0.1")
		require.Equal(t, string(metadata.NodeAddresses[0].Type), "InternalIP")
	})

}
