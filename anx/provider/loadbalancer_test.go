//go:build integration

package provider

import (
	"context"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	"github.com/stretchr/testify/require"
	"go.anx.io/go-anxcloud/pkg"
	"go.anx.io/go-anxcloud/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestIntegrationTestLB(t *testing.T) {
	const clusterName = "test-k8s-cluster"
	client, err := client.New(client.TokenFromEnv(false))
	require.NoError(t, err)
	require.NotNil(t, client)

	provider := &anxProvider{
		API: pkg.NewAPI(client),
		config: &configuration.ProviderConfig{
			LoadBalancerIdentifier: "285b954fdf2a449c8fdae01cc6074025",
		},
	}

	manager := loadBalancerManager{provider}
	ctx := context.Background()

	t.Run("Create loadbalancer", func(t *testing.T) {
		balancer, err := manager.EnsureLoadBalancer(ctx, clusterName, getTestService(), []*v1.Node{
			getNodeWithAddress("8.8.8.8"),
			getNodeWithAddress("9.9.9.9"),
		})
		require.NoError(t, err)
		require.NotNil(t, balancer)
	})

	t.Run("EnsureLB exists", func(t *testing.T) {
		state, isPresent, err := manager.GetLoadBalancer(ctx, clusterName, getTestService())
		require.NoError(t, err)
		require.NotNil(t, state)
		require.True(t, isPresent)
	})

	t.Run("Make sure nodes get deleted", func(t *testing.T) {
		balancer, err := manager.EnsureLoadBalancer(ctx, clusterName, getTestService(), []*v1.Node{
			getNodeWithAddress("8.8.8.8"),
		})

		require.NoError(t, err)
		require.NotNil(t, balancer)
	})

	t.Run("Make sure loadbalancer can be deleted", func(t *testing.T) {
		err = manager.EnsureLoadBalancerDeleted(ctx, clusterName, getTestService())
		require.NoError(t, err)
	})
}

func getTestService() *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "TestService",
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Protocol:   "TCP",
				Port:       8080,
				TargetPort: intstr.IntOrString{IntVal: 5},
				NodePort:   5000,
			}},
		},
	}
}

func getNodeWithAddress(ip string) *v1.Node {
	return &v1.Node{
		Status: v1.NodeStatus{
			Addresses: []v1.NodeAddress{
				{
					Type:    "ExternalIP",
					Address: ip,
				},
			},
		},
	}
}
