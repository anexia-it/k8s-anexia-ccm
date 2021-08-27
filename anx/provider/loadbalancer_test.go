package provider

import (
	"context"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	"github.com/anexia-it/go-anxcloud/pkg"
	"github.com/anexia-it/go-anxcloud/pkg/client"
	"github.com/stretchr/testify/require"
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
		API:    pkg.NewAPI(client),
		config: &configuration.ProviderConfig{},
	}

	manager := loadBalancerManager{provider}
	ctx := context.Background()

	balancer, err := manager.EnsureLoadBalancer(ctx, clusterName, &v1.Service{
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
	}, []*v1.Node{})

	require.NoError(t, err)
	require.NotNil(t, balancer)
}
