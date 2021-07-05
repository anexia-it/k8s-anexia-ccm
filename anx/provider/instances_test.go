package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/info"
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
		provider := getMockedAnxProvider()

		manager := instanceManager{provider}

		providerId, err := manager.InstanceIDByNode(ctx, &v1.Node{
			Spec: v1.NodeSpec{ProviderID: fmt.Sprintf("%s%s", cloudProviderScheme, nodeIdentifier)},
		})

		require.NoError(t, err)
		require.Equal(t, nodeIdentifier, providerId)
	})
}

func TestInstanceExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	node := v1.Node{
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("%s%s", cloudProviderScheme, nodeIdentifier),
		},
	}

	t.Run("InstanceExists", func(t *testing.T) {
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{}, nil)
		manager := instanceManager{provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.NoError(t, err)
		require.True(t, exists, "expected instance to exist")
	})

	t.Run("InstanceDoesNotExist", func(t *testing.T) {
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
		provider := getMockedAnxProvider()
		provider.infoMock.On("Get", ctx, nodeIdentifier).Return(info.Info{}, errors.New("unknownError"))
		manager := instanceManager{provider}
		exists, err := manager.InstanceExists(ctx, &node)

		require.Error(t, err)
		require.False(t, exists, "expected instance to exist")
	})
}
