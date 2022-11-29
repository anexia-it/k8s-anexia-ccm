package reconciliation

import (
	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

const serverResourceTypeIdentifier = "01f321a4875446409d7d8469503a905f"

func (r *reconciliation) reconcileServers() (toCreate, toDestroy []types.Object, err error) {
	targetServers := make([]*lbaasv1.Server, 0, len(r.ports)*len(r.targetServers))
	for _, server := range r.targetServers {
		for portName, port := range r.ports {
			backend, ok := r.portBackends[portName]
			if !ok {
				r.logger.V(2).Info("Not reconciling server because backend not (yet?) found",
					"server", server.Name,
					"port", portName,
				)
				continue
			}

			targetServers = append(targetServers, &lbaasv1.Server{
				Name:    r.makeResourceName(server.Name, portName),
				IP:      server.Address.String(),
				Port:    int(port.Internal),
				Check:   "enabled",
				Backend: lbaasv1.Backend{Identifier: backend.Identifier},
			})
		}
	}

	toCreate = make([]types.Object, 0, len(targetServers))
	toDestroy = make([]types.Object, 0, len(r.remoteStateSnapshot.servers))

	err = compare.Reconcile(
		targetServers, r.remoteStateSnapshot.servers,
		&toCreate, &toDestroy,
		"Name", "IP", "Port", "Check", "Backend.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	return
}
