package reconciliation

import (
	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

const backendResourceTypeIdentifier = "33164a3066a04a52be43c607f0c5dd8c"

func (r *reconciliation) reconcileBackends() (toCreate, toDestroy []types.Object, err error) {
	targetBackends := make([]*lbaasv1.Backend, 0, len(r.ports))
	for name := range r.ports {
		targetBackends = append(targetBackends, &lbaasv1.Backend{
			Name:         r.makeResourceName(name),
			LoadBalancer: lbaasv1.LoadBalancer{Identifier: r.lb.Identifier},
			Mode:         lbaasv1.TCP,
			HealthCheck:  `"adv_check": "tcp-check"`,
		})
	}

	toCreate = make([]types.Object, 0, len(targetBackends))
	toDestroy = make([]types.Object, 0, len(r.remoteStateSnapshot.backends))

	err = compare.Reconcile(
		targetBackends, r.remoteStateSnapshot.backends,
		&toCreate, &toDestroy,
		"Name", "Mode", "HealthCheck", "LoadBalancer.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	if len(toCreate) == 0 && len(toDestroy) == 0 {
		for name := range r.ports {
			expectedName := r.makeResourceName(name)

			for _, b := range targetBackends {
				if b.Name == expectedName {
					r.portBackends[name] = b
					break
				}
			}
		}
	}

	return
}
