package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	tUtils "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/utils"
	"github.com/stretchr/testify/require"
	"go.anx.io/go-anxcloud/pkg/client"
	"go.anx.io/go-anxcloud/pkg/vsphere/info"
	"go.anx.io/go-anxcloud/pkg/vsphere/powercontrol"
	"go.anx.io/go-anxcloud/pkg/vsphere/search"
	"go.anx.io/go-anxcloud/pkg/vsphere/vmlist"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFetchingID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const nodeName = "test-node"

	t.Run("GetProviderIDForNode/NoProviderID", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		nodeIdentifier := randomNodeIdentifier()
		provider.ProviderConfig.CustomerID = "test"
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%s-%s", "test",
			nodeName)).Return([]search.VM{
			{
				Identifier: nodeIdentifier,
			}}, nil)

		manager := instanceManager{Provider: provider}

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
		provider := tUtils.GetMockedAnxProvider()

		manager := instanceManager{Provider: provider}

		nodeIdentifier := randomNodeIdentifier()
		providerId, err := manager.InstanceIDByNode(ctx, &v1.Node{
			Spec: v1.NodeSpec{ProviderID: fmt.Sprintf("%s%s", configuration.CloudProviderScheme, nodeIdentifier)},
		})

		require.NoError(t, err)
		require.Equal(t, nodeIdentifier, providerId)
	})

	t.Run("GetProviderIDForNode/AutomaticResolverNoProviderID", func(t *testing.T) {
		t.Parallel()
		const customerPrefix = "customerPrefix"

		provider := tUtils.GetMockedAnxProvider()
		provider.ProviderConfig.CustomerID = customerPrefix

		// act like one VM is already present
		provider.VmListMock.On("Get", ctx, 1, 1).Return([]vmlist.VM{
			{Name: fmt.Sprintf("%s-nodeName", customerPrefix), Identifier: "identifier"},
		}, nil)

		nodeIndentifier := randomNodeIdentifier()
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%s-%s", customerPrefix,
			nodeName)).Return([]search.VM{
			{
				Identifier: nodeIndentifier,
			}}, nil)

		manager := instanceManager{Provider: provider}

		providerId, err := manager.InstanceIDByNode(ctx, &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Spec: v1.NodeSpec{},
		})

		require.NoError(t, err)
		require.Equal(t, nodeIndentifier, providerId)
	})

	t.Run("GetProviderIDForNode/NoProviderID/MultipleVMs/IPUnique", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		randomNodeIdentifier := randomNodeIdentifier()
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%s-%s",
			provider.Config().CustomerID, nodeName)).Return([]search.VM{
			{
				Name:       fmt.Sprintf("%s-VM", provider.Config().CustomerID),
				Identifier: randomNodeIdentifier,
			},
			{
				Name:       "test-VM",
				Identifier: "secondIdentifier",
			},
		}, nil)

		provider.InfoMock.On("Get", ctx, randomNodeIdentifier).Return(info.Info{
			Name:       fmt.Sprintf("%s-VM", provider.Config().CustomerID),
			Identifier: randomNodeIdentifier,
			Network: []info.Network{
				{
					// the IP address we are looking for
					IPv4: []string{"10.0.0.1"},
				},
			},
		}, nil)

		provider.InfoMock.On("Get", ctx, "secondIdentifier").Return(info.Info{
			Name:       "test-VM",
			Identifier: "secondIdentifier",
			Network: []info.Network{
				{
					// not the IP address we are looking for
					IPv4: []string{"10.0.0.2"},
				},
			},
		}, nil)

		manager := instanceManager{Provider: provider}

		identifier, err := manager.InstanceIDByNode(ctx, &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Spec: v1.NodeSpec{},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{Type: v1.NodeExternalDNS, Address: "test-VM"},
					{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		})

		require.Equal(t, identifier, randomNodeIdentifier)
		require.NoError(t, err)
	})

	t.Run("GetProviderIDForNode/NoProviderID/MultipleVMs/IPsNotUnique", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		randomNodeIdentifier := randomNodeIdentifier()
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%s-%s",
			provider.Config().CustomerID, nodeName)).Return([]search.VM{
			{
				Name:       fmt.Sprintf("%s-VM", provider.Config().CustomerID),
				Identifier: randomNodeIdentifier,
			},
			{
				Name:       "test-VM",
				Identifier: "secondIdentifier",
			},
		}, nil)

		provider.InfoMock.On("Get", ctx, randomNodeIdentifier).Return(info.Info{
			Name:       fmt.Sprintf("%s-VM", provider.Config().CustomerID),
			Identifier: randomNodeIdentifier,
			Network: []info.Network{
				{
					// the IP address we are looking for
					IPv4: []string{"10.0.0.1"},
				},
			},
		}, nil)

		provider.InfoMock.On("Get", ctx, "secondIdentifier").Return(info.Info{
			Name:       "test-VM",
			Identifier: "secondIdentifier",
			Network: []info.Network{
				{
					// sadly the IP address we are looking for, too
					IPv4: []string{"10.0.0.1"},
				},
			},
		}, nil)

		manager := instanceManager{Provider: provider}

		_, err := manager.InstanceIDByNode(ctx, &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
			},
			Spec: v1.NodeSpec{},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{Type: v1.NodeExternalDNS, Address: "test-VM"},
					{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		})

		require.Error(t, err)
	})
}

func TestInstanceExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	identifier := randomNodeIdentifier()
	node := tUtils.ProviderManagedNode(identifier)

	t.Run("InstanceExists", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.InfoMock.On("Get", ctx, identifier).Return(info.Info{}, nil)
		manager := instanceManager{Provider: provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.NoError(t, err)
		require.True(t, exists, "expected instance to exist")
	})

	t.Run("InstanceDoesNotExist", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.InfoMock.On("Get", ctx, identifier).Return(info.Info{}, &client.ResponseError{
			Response: &http.Response{
				StatusCode: http.StatusNotFound,
			},
		})
		manager := instanceManager{Provider: provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.NoError(t, err)
		require.False(t, exists, "expected instance to exist")
	})

	t.Run("UnknownError", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()

		provider.InfoMock.On("Get", ctx, identifier).Return(info.Info{}, errors.New("unknownError"))
		manager := instanceManager{Provider: provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.Error(t, err)
		require.False(t, exists, "expected instance to exist")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.InfoMock.On("Get", ctx, identifier).Return(info.Info{}, &client.ResponseError{
			Response: &http.Response{
				StatusCode: http.StatusUnauthorized,
			},
		})
		manager := instanceManager{Provider: provider}

		// make request -> returns unauthorized
		_, err := manager.InstanceExists(ctx, &node)
		require.Error(t, err)
		require.IsType(t, err, &client.ResponseError{})

		// recent unauthorized request -> skip
		_, err = manager.InstanceExists(ctx, &node)
		require.Error(t, err)
		require.ErrorIs(t, err, utils.ErrUnauthorizedForbiddenBackoff)

		provider.InfoMock.AssertNumberOfCalls(t, "Get", 1)

		manager.lastUnauthorizedOrForbiddenInstanceExistCall = time.Now().Add(-time.Minute)

		// unauthorized request block passed -> returns unauthorized
		_, err = manager.InstanceExists(ctx, &node)
		require.Error(t, err)
		require.IsType(t, err, &client.ResponseError{})

		provider.InfoMock.AssertNumberOfCalls(t, "Get", 2)

		manager.lastUnauthorizedOrForbiddenInstanceExistCall = time.Time{}
	})
}

func TestInstanceShutdown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const nodeName = "test-node"
	identifier := randomNodeIdentifier()
	node := tUtils.ProviderManagedNode(identifier)

	t.Run("PoweredOn", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: identifier,
			}}, nil)

		provider.PowerControlMock.On("Get", ctx, identifier).Return(powercontrol.OnState, nil)

		manager := instanceManager{Provider: provider}
		isShutdown, err := manager.InstanceShutdown(ctx, &node)
		require.NoError(t, err)
		require.False(t, isShutdown)
	})

	t.Run("PoweredOff", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: identifier,
			}}, nil)

		provider.PowerControlMock.On("Get", ctx, identifier).Return(powercontrol.OffState, nil)

		manager := instanceManager{Provider: provider}
		isShutdown, err := manager.InstanceShutdown(ctx, &node)
		require.NoError(t, err)
		require.True(t, isShutdown)
	})

	t.Run("UnknownPowerState", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.SearchMock.On("ByName", ctx, fmt.Sprintf("%%-%s", nodeName)).Return([]search.VM{
			{
				Identifier: identifier,
			}}, nil)

		provider.PowerControlMock.On("Get", ctx, identifier).Return(powercontrol.State("NoExistentState"), nil)

		manager := instanceManager{Provider: provider}
		_, err := manager.InstanceShutdown(ctx, &node)
		require.Error(t, err)
	})
}

func TestInstanceTypeFromInfo(t *testing.T) {
	t.Parallel()
	t.Run("WithDiskInfo", func(t *testing.T) {
		t.Parallel()
		instanceTypeStr := instanceType(info.Info{
			RAM: 4096,
			CPU: 5,
			DiskInfo: []info.DiskInfo{
				{
					DiskType: "ENT6",
					DiskGB:   5,
				},
				{
					DiskType: "ENT7",
					DiskGB:   100,
				},
			},
		})

		require.Equal(t, "C5-M4-ENT7", instanceTypeStr)
	})

	t.Run("WithDiskInfo but performance type missing for largest disk", func(t *testing.T) {
		t.Parallel()
		instanceTypeStr := instanceType(info.Info{
			RAM: 4096,
			CPU: 5,
			DiskInfo: []info.DiskInfo{
				{
					DiskType: "ENT6",
					DiskGB:   5,
				},
				{
					DiskGB: 100,
				},
			},
		})

		require.Equal(t, "C5-M4-ENT6", instanceTypeStr)
	})

	t.Run("WithDiskInfo but performance type missing", func(t *testing.T) {
		t.Parallel()
		instanceTypeStr := instanceType(info.Info{
			RAM: 4096,
			CPU: 5,
			DiskInfo: []info.DiskInfo{
				{
					DiskGB: 5,
				},
				{
					DiskGB: 100,
				},
			},
		})

		require.Equal(t, "C5-M4", instanceTypeStr)
	})

	t.Run("NoDiskInfo", func(t *testing.T) {
		t.Parallel()
		instanceTypeStr := instanceType(info.Info{
			RAM: 4096,
			CPU: 5,
		})

		require.Equal(t, "C5-M4", instanceTypeStr)
	})

}

func TestInstanceMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	identifier := randomNodeIdentifier()
	node := tUtils.ProviderManagedNode(identifier)

	t.Run("OneNetwork", func(t *testing.T) {
		t.Parallel()
		provider := tUtils.GetMockedAnxProvider()
		provider.InfoMock.On("Get", ctx, identifier).Return(info.Info{
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
		manager := instanceManager{Provider: provider}

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
		provider := tUtils.GetMockedAnxProvider()
		provider.InfoMock.On("Get", ctx, identifier).Return(info.Info{
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
		manager := instanceManager{Provider: provider}

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

func randomNodeIdentifier() string {
	return fmt.Sprintf("test-ident-%s", strconv.Itoa(rand.Intn(math.MaxInt)))
}
