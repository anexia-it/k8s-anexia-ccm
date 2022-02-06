package loadbalancer

import (
	"context"
	v1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"go.anx.io/go-anxcloud/pkg/lbaas/bind"
	"go.anx.io/go-anxcloud/pkg/lbaas/common"
	"go.anx.io/go-anxcloud/pkg/pagination"
)

func createBind(ctx context.Context, lb LoadBalancer, bindName string) (BindID, error) {
	bind := getBindDefinition(bindName, lb.State)

	err := lb.GenericAPI.Create(ctx, &bind)
	if err != nil {
		return "", err
	}
	lb.Logger.Info("Bind created for loadbalancer", "name", bindName, "resource", "bind")
	return BindID(bind.Identifier), nil
}

func getBindDefinition(bindName string, state *state) v1.Bind {
	return v1.Bind{
		Name:  bindName,
		State: common.NewlyCreated,
		Port:  int(state.Port),
		Frontend: v1.Frontend{
			Identifier: string(state.FrontendID),
		},
	}
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
		bind := getBindDefinition(bindName, lb.State)
		bind.State = common.Updating
		err := lb.GenericAPI.Update(ctx, &bind)
		return BindID(bind.Identifier), err
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
