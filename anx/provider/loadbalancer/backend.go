package loadbalancer

import (
	"context"
	"go.anx.io/go-anxcloud/pkg/lbaas/backend"
	"go.anx.io/go-anxcloud/pkg/lbaas/common"
	"go.anx.io/go-anxcloud/pkg/pagination"
)

func ensureBackendInLoadBalancer(ctx context.Context, lb LoadBalancer,
	backendName string) (BackendID, error) {
	existingBackend := findBackendInLB(ctx, lb, backendName)

	// check if we need to create a backend
	if existingBackend == nil {
		createdBackend, err := createBackendForLB(ctx, lb, backendName)
		if err != nil {
			return "", err
		}

		// we can stop here
		return createdBackend, nil
	}

	return BackendID(existingBackend.Identifier), nil
}

func findBackendInLB(ctx context.Context, lb LoadBalancer, name string) *backend.Backend {
	backends, cancelFunc := pagination.AsChan(ctx, lb.Backend(), SearchParameter(name))
	defer cancelFunc()
	for elem := range backends {
		backendInfo := elem.(backend.BackendInfo)
		fetchedBackend, err := lb.Backend().GetByID(ctx, backendInfo.Identifier)
		if err != nil {
			lb.Logger.Error(err, "unable to search for backend", "name", name, "resource", "backend")
		}
		if fetchedBackend.LoadBalancer.Identifier == string(lb.State.ID) {
			return &fetchedBackend
		}
	}
	return nil
}

func createBackendForLB(ctx context.Context, lb LoadBalancer, backendName string) (BackendID, error) {
	definition := backend.Definition{
		Name:         backendName,
		State:        common.NewlyCreated,
		LoadBalancer: string(lb.State.ID),
		Mode:         common.TCP,
	}

	createdBackend, err := lb.Backend().Create(ctx, definition)
	if err != nil {
		return "", err
	}

	lb.Logger.Info("configured backend for loadbalancer", "name", backendName, "resource", "backend")
	return BackendID(createdBackend.Identifier), nil
}

func deleteBackendFromLB(ctx context.Context, g LoadBalancer, name string) error {
	backend := findBackendInLB(ctx, g, name)
	if backend == nil {
		return nil
	}
	return g.Backend().DeleteByID(ctx, backend.Identifier)
}
