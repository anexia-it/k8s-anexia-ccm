package reconciliation

import (
	"fmt"
	"sort"

	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

const bindResourceTypeIdentifier = "bd24def982aa478fb3352cb5f49aab47"

func (r *stateRetrieverImpl) storePublicAddress(lbID, addr string) {
	if idx := sort.SearchStrings(r.loadBalancers[lbID].state.publicAddresses, addr); idx >= len(r.loadBalancers[lbID].state.publicAddresses) || r.loadBalancers[lbID].state.publicAddresses[idx] != addr {
		r.loadBalancers[lbID].state.publicAddresses = append(r.loadBalancers[lbID].state.publicAddresses, addr)
		sort.Strings(r.loadBalancers[lbID].state.publicAddresses)
	}
}

func (r *stateRetrieverImpl) filterBinds(lbID string, allBinds []*lbaasv1.Bind) ([]*lbaasv1.Bind, error) {
	ret := make([]*lbaasv1.Bind, 0, len(allBinds))

	// Binds and Servers are filtered for our LoadBalancer here, after we hopefully retrieved their Frontends and Backends already
	for _, bind := range allBinds {
		idx, err := compare.Search(lbaasv1.Frontend{Identifier: bind.Frontend.Identifier}, r.loadBalancers[lbID].state.frontends, "Identifier")
		if err != nil {
			return nil, fmt.Errorf("error checking if Binds belongs to one of our frontends: %w", err)
		} else if idx != -1 {
			ret = append(ret, bind)
			r.sortObjectIntoStateArray(lbID, bind)

			if bind.Address != "" {
				r.storePublicAddress(lbID, bind.Address)
			}
		}
	}

	return ret, nil
}

func (r *reconciliation) reconcileBinds() (toCreate, toDestroy []types.Object, err error) {
	targetBinds := make([]*lbaasv1.Bind, 0, len(r.externalAddresses)*len(r.ports))
	for _, a := range r.externalAddresses {
		fam := "v6"

		if a.To4() != nil {
			fam = "v4"
		}

		for name, port := range r.ports {
			frontend, ok := r.portFrontends[name]
			if !ok {
				r.logger.V(2).Info("Not reconciling bind because frontend not (yet?) found",
					"address", a,
					"port", name,
				)
				continue
			}
			targetBinds = append(targetBinds, &lbaasv1.Bind{
				Name:     r.makeResourceName(fam, name),
				Address:  a.String(),
				Port:     int(port.External),
				Frontend: lbaasv1.Frontend{Identifier: frontend.Identifier},
			})
		}
	}

	toCreate = make([]types.Object, 0, len(targetBinds))
	toDestroy = make([]types.Object, 0, len(r.remoteStateSnapshot.binds))

	err = compare.Reconcile(
		targetBinds, r.remoteStateSnapshot.binds,
		&toCreate, &toDestroy,
		"Name", "Address", "Port", "Frontend.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	return
}
