package loadbalancer

import (
	"context"
	"github.com/anexia-it/go-anxcloud/pkg/lbaas/bind"
	"github.com/anexia-it/go-anxcloud/pkg/lbaas/common"
	"github.com/anexia-it/go-anxcloud/pkg/pagination"
)

func createBind(ctx context.Context, lb LoadBalancer, bindName string) (BindID, error) {
	definition := getBindDefinition(bindName, lb.State)

	createdBind, err := lb.Bind().Create(ctx, definition)
	if err != nil {
		return "", err
	}
	lb.Logger.Info("Bind created for loadbalancer", "name", bindName, "resource", "bind")
	return BindID(createdBind.Identifier), nil
}

func getBindDefinition(bindName string, state *state) bind.Definition {
	definition := bind.Definition{
		Name:     bindName,
		State:    common.NewlyCreated,
		Frontend: string(state.FrontendID),
	}
	return definition
}

func findFrontendBindInLoadBalancer(ctx context.Context, lb LoadBalancer, name string) *bind.Bind {
	binds, cancelFunc := pagination.AsChan(ctx, lb.Bind(), SearchParameter(name))
	defer cancelFunc()
	for elem := range binds {
		bindInfo := elem.(bind.BindInfo)
		fetchedBind, err := lb.Bind().GetByID(ctx, bindInfo.Identifier)
		if err != nil {
			lb.Logger.Error(err, "unable to find frontend bind in loadbalancer",
				"name", name, "resource", "bind")
		}

		if fetchedBind.Frontend.Identifier == string(lb.State.FrontendID) {
			return &fetchedBind
		}
	}
	return nil
}

func ensureFrontendBindInLoadBalancer(ctx context.Context, lb LoadBalancer, bindName string) (BindID, error) {
	existingBind := findFrontendBindInLoadBalancer(ctx, lb, bindName)
	// check if we need to create bind
	if existingBind == nil {
		createBind, err := createBind(ctx, lb, bindName)
		return createBind, err
	}

	if existingBind.Frontend.Identifier == string(lb.State.FrontendID) {
		lb.Logger.Info("frontend changed", "name", bindName, "resource", "bind")
		updatedBind, err := lb.Bind().Update(ctx, existingBind.Identifier, getBindDefinition(bindName, lb.State))
		return BindID(updatedBind.Identifier), err
	}

	return BindID(existingBind.Identifier), nil
}

func ensureFrontendBindDeleted(ctx context.Context, g LoadBalancer, name string) error {
	exisitngBind := findFrontendBindInLoadBalancer(ctx, g, name)

	if exisitngBind == nil {
		return nil
	}

	return g.Bind().DeleteByID(ctx, exisitngBind.Identifier)
}
