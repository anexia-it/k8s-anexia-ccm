package loadbalancer

import (
	"context"

	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/loadbalancer/await"
	v1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"go.anx.io/go-anxcloud/pkg/lbaas/common"
	"go.anx.io/go-anxcloud/pkg/lbaas/frontend"
	"go.anx.io/go-anxcloud/pkg/pagination"
	"go.anx.io/go-anxcloud/pkg/utils/param"
)

var SearchParameter = param.ParameterBuilder("search")

func createFrontendForLB(ctx context.Context, lb LoadBalancer, frontendName string) (FrontendID, error) {
	backendID := lb.State.BackendID
	definition := frontend.Definition{
		Name:           frontendName,
		DefaultBackend: string(backendID),
		Mode:           common.TCP,
		State:          common.NewlyCreated,
		LoadBalancer:   string(lb.State.ID),
	}

	createdFrontend, err := lb.Frontend().Create(ctx, definition)
	if err != nil {
		return "", err
	}
	lb.Logger.Info("configured frontend for loadbalancer", "name", frontendName, "resource", "frontend")

	// await frontend to reach any success state
	err = await.AwaitFrontendState(ctx, createdFrontend.Identifier, await.SuccessStates...)
	if err != nil {
		return "", err
	}

	return FrontendID(createdFrontend.Identifier), nil
}

func findFrontendInLB(ctx context.Context, lb LoadBalancer, name string) *frontend.Frontend {
	frontends, cancelFunc := pagination.AsChan(ctx, lb.Frontend(), SearchParameter(name))
	defer cancelFunc()

	for elem := range frontends {
		frontendInfo := elem.(frontend.FrontendInfo)
		fetchedFrontend, err := lb.Frontend().GetByID(ctx, frontendInfo.Identifier)
		if err != nil {
			lb.Logger.Error(err, "unable to find frontend in lb", "name", name, "resource", "frontend")
		}

		if fetchedFrontend.LoadBalancer.Identifier == string(lb.State.ID) {
			return &fetchedFrontend
		}
	}
	return nil
}

func ensureFrontendInLoadBalancer(ctx context.Context, lb LoadBalancer, frontendName string) (FrontendID, error) {

	existingFrontend := findFrontendInLB(ctx, lb, frontendName)

	// check if we need to create a frontend
	if existingFrontend == nil {
		createdFrontend, err := createFrontendForLB(ctx, lb, frontendName)
		if err != nil {
			return "", err
		}

		// we can stop here
		return createdFrontend, nil
	}

	return FrontendID(existingFrontend.Identifier), nil
}

func ensureFrontendDeleted(ctx context.Context, g LoadBalancer, name string) error {
	existingFrontend := findFrontendInLB(ctx, g, name)
	if existingFrontend == nil {
		return nil
	}
	err := g.Frontend().DeleteByID(ctx, existingFrontend.Identifier)
	if err != nil {
		return err
	}

	return await.Deleted(ctx, &v1.Frontend{Identifier: existingFrontend.Identifier})

}
