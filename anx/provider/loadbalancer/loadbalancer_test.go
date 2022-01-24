package loadbalancer

import (
	"context"
	tUtils "github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/test"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.anx.io/go-anxcloud/pkg/lbaas/backend"
	"go.anx.io/go-anxcloud/pkg/lbaas/bind"
	"go.anx.io/go-anxcloud/pkg/lbaas/frontend"
	"go.anx.io/go-anxcloud/pkg/lbaas/loadbalancer"
	"go.anx.io/go-anxcloud/pkg/lbaas/server"
	"testing"
)

func TestEnsureBalancer(t *testing.T) {
	ctx := context.Background()
	provider := tUtils.GetMockedAnxProvider()
	const LbIdentifier = "test"
	balancer := NewLoadBalancer(provider.LBaaS(), LbIdentifier, logr.Discard())
	const serviceName = "test-name"

	mockEmptyLBaas(ctx, provider)

	const backendIdentifier = "backendIdentifier"
	const frontendIdentifier = "frontendIdentifier"
	const bindIdentifier = "bindIdentifier"
	const serverIdentifier = "serverIdentifier"

	provider.FrontendMock.On("Create", ctx, mock.AnythingOfType("frontend.Definition")).
		Return(mockCreateFrontend(frontendIdentifier), nil)
	provider.BackendMock.On("Create", ctx, mock.AnythingOfType("backend.Definition")).
		Return(mockBackendCreate(backendIdentifier), nil)
	provider.BindMock.On("Create", ctx, mock.AnythingOfType("bind.Definition")).
		Return(mockBindCreate(bindIdentifier), nil)
	provider.ServerMock.On("Create", ctx, mock.AnythingOfType("server.Definition")).
		Return(mockServerCreate(serverIdentifier), nil)

	err := balancer.EnsureLBConfig(ctx, serviceName, []NodeEndpoint{
		{
			IP:   "8.8.8.8",
			Port: 7070,
		}, {
			IP:   "8.8.8.8",
			Port: 7070,
		},
	})

	// make sure resources where created
	provider.ServerMock.AssertNumberOfCalls(t, "Create", 2)
	provider.BackendMock.AssertNumberOfCalls(t, "Create", 1)
	provider.FrontendMock.AssertNumberOfCalls(t, "Create", 1)
	provider.BindMock.AssertNumberOfCalls(t, "Create", 1)

	require.Equal(t, backendIdentifier, string(balancer.State.BackendID))
	require.Equal(t, frontendIdentifier, string(balancer.State.FrontendID))
	require.Equal(t, bindIdentifier, string(balancer.State.BindID))
	require.NoError(t, err)

}

func mockCreateFrontend(identifier string) func(ctx context.Context, definition frontend.Definition) frontend.Frontend {
	return func(ctx context.Context, definition frontend.Definition) frontend.Frontend {
		return frontend.Frontend{
			CustomerIdentifier: "random-customer-identifier",
			ResellerIdentifier: "reseller-identifier",
			Identifier:         identifier,
			Name:               definition.Name,
			LoadBalancer: &loadbalancer.Loadbalancer{
				Identifier: definition.LoadBalancer,
				Name:       "Bruce Wayne",
			},
			DefaultBackend: &backend.Backend{
				Identifier: definition.DefaultBackend,
				Name:       definition.Name,
			},
			Mode:          definition.Mode,
			ClientTimeout: "",
		}
	}
}

func mockBackendCreate(identifier string) func(ctx context.Context, definition backend.Definition) backend.Backend {
	return func(ctx context.Context, definition backend.Definition) backend.Backend {
		return backend.Backend{
			CustomerIdentifier: "random-customer-identifier",
			ResellerIdentifier: "reseller-identifier",
			Identifier:         identifier,
			Name:               definition.Name,
			LoadBalancer: loadbalancer.Loadbalancer{
				Identifier: string(definition.LoadBalancer),
				Name:       "Bruce Wayne",
			},
			HealthCheck:   "",
			Mode:          definition.Mode,
			ServerTimeout: 0,
		}
	}
}

func mockBindCreate(identifier string) func(ctx context.Context, definition bind.Definition) bind.Bind {
	return func(ctx context.Context, definition bind.Definition) bind.Bind {
		return bind.Bind{
			CustomerIdentifier: "random-customer-identifier",
			ResellerIdentifier: "reseller-identifier",
			Identifier:         identifier,
			Name:               definition.Name,
			Frontend: frontend.Frontend{
				Identifier: definition.Frontend,
				Name:       definition.Name,
			},
			Address:            definition.Frontend,
			Port:               0,
			SSL:                false,
			SslCertificatePath: "",
		}
	}
}

func mockServerCreate(identifier string) func(ctx context.Context, definition server.Definition) server.Server {
	return func(ctx context.Context, definition server.Definition) server.Server {
		return server.Server{
			CustomerIdentifier: "random-customer-identifier",
			ResellerIdentifier: "reseller-identifier",
			Identifier:         identifier,
			Name:               definition.Name,
			IP:                 definition.IP,
			Port:               definition.Port,
			Backend: backend.Backend{
				Identifier: definition.Backend,
				Name:       definition.Name,
			},
		}
	}
}

func mockEmptyLBaas(ctx context.Context, provider tUtils.MockedProvider) {

	// no backends yet created
	provider.BackendMock.On("GetPage", ctx, 1, mock.Anything, mock.AnythingOfType("param.Parameter")).Return(backend.BackendPage{
		Page:       1,
		TotalItems: 0,
		TotalPages: 0,
		Limit:      0,
		Data:       []backend.BackendInfo{},
	}, nil)

	// no frontends yet created
	provider.FrontendMock.On("GetPage", ctx, mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.AnythingOfType("param.Parameter")).Return(
		frontend.FrontendPage{
			Page:       1,
			TotalItems: 0,
			TotalPages: 0,
			Limit:      0,
			Data:       []frontend.FrontendInfo{},
		}, nil)

	// no bind yet created
	provider.BindMock.On("GetPage", ctx, mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.AnythingOfType("param.Parameter")).Return(
		bind.BindPage{
			Page:       1,
			TotalItems: 0,
			TotalPages: 0,
			Limit:      0,
			Data:       []bind.BindInfo{},
		}, nil)

	// no server yet created
	provider.ServerMock.On("GetPage", ctx, mock.AnythingOfType("int"), mock.AnythingOfType("int"), mock.AnythingOfType("param.Parameter")).Return(
		server.ServerPage{
			Page:       1,
			TotalItems: 0,
			TotalPages: 0,
			Limit:      0,
			Data:       []server.ServerInfo{},
		}, nil)

}
