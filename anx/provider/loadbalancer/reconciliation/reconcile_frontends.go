package reconciliation

import (
	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

const frontendResourceTypeIdentifier = "da9d14b9d95840c08213de67f9cee6e2"

func (r *reconciliation) reconcileFrontends() (toCreate, toDestroy []types.Object, err error) {
	targetFrontends := make([]*lbaasv1.Frontend, 0, len(r.ports))
	for name := range r.ports {
		backend, ok := r.portBackends[name]
		if !ok {
			r.logger.V(2).Info("Not reconciling frontend because backend not (yet?) found",
				"port", name,
			)
			continue
		}

		targetFrontends = append(targetFrontends, &lbaasv1.Frontend{
			Name:           r.makeResourceName(name),
			Mode:           lbaasv1.TCP,
			LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: r.lb.Identifier},
			DefaultBackend: &lbaasv1.Backend{Identifier: backend.Identifier},
		})
	}

	toCreate = make([]types.Object, 0, len(targetFrontends))
	toDestroy = make([]types.Object, 0, len(r.remoteStateSnapshot.frontends))

	err = compare.Reconcile(
		targetFrontends, r.remoteStateSnapshot.frontends,
		&toCreate, &toDestroy,
		"Name", "Mode", "LoadBalancer.Identifier", "DefaultBackend.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	if len(toCreate) == 0 && len(toDestroy) == 0 {
		for name := range r.ports {
			expectedName := r.makeResourceName(name)

			for _, f := range targetFrontends {
				if f.Name == expectedName {
					r.portFrontends[name] = f
					break
				}
			}
		}
	}

	return
}
