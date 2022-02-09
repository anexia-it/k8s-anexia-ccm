package replication

import (
	"context"
	"errors"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync/components"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync/delta"
	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/lbaas/backend"
	"go.anx.io/go-anxcloud/pkg/lbaas/bind"
	"go.anx.io/go-anxcloud/pkg/lbaas/frontend"
	"go.anx.io/go-anxcloud/pkg/lbaas/loadbalancer"
	"go.anx.io/go-anxcloud/pkg/lbaas/server"
	"math/rand"
	"sync"
)

func SyncLoadBalancer(ctx context.Context, anxAPI api.API, source, target components.HashedLoadBalancer) error {
	logr.FromContextOrDiscard(ctx).Info("syncing load balancer", "target", target.Identifier, "source",
		source.Identifier)

	// calculate deltas
	backendDelta := delta.NewDelta(components.ToHasher(source.Backends), components.ToHasher(target.Backends))
	frontendDelta := delta.NewDelta(components.ToHasher(source.Frontends), components.ToHasher(target.Frontends))
	bindDelta := delta.NewDelta(components.ToHasher(source.Binds), components.ToHasher(target.Binds))
	serverDelta := delta.NewDelta(components.ToHasher(source.Servers), components.ToHasher(target.Servers))

	// delete all resources
	for _, hasher := range serverDelta.Delete {
		server := hasher.(components.HashedServer).Server
		err := anxAPI.Destroy(ctx, server)
		if err != nil {
			return err
		}
		target.Servers = components.DeleteServer(server.Identifier, target.Servers)
	}

	for _, hasher := range bindDelta.Delete {
		bind := hasher.(components.HashedBind).Bind
		err := anxAPI.Destroy(ctx, bind)
		if err != nil {
			return err
		}
		target.Binds = components.DeleteBind(bind.Identifier, target.Binds)
	}

	for _, hasher := range frontendDelta.Delete {
		frontend := hasher.(components.HashedFrontend).Frontend
		err := anxAPI.Destroy(ctx, frontend)
		if err != nil {
			return err
		}
		target.Frontends = components.DeleteFrontend(frontend.Identifier, target.Frontends)
	}

	for _, hasher := range backendDelta.Delete {
		backend := hasher.(components.HashedBackend).Backend
		err := anxAPI.Destroy(ctx, backend)
		if err != nil {
			return err
		}
		target.Backends = components.DeleteBackend(backend.Identifier, target.Backends)
	}

	for _, hasher := range backendDelta.Create {
		backend := hasher.(components.HashedBackend)
		backendClone := *backend.Backend
		backendClone.LoadBalancer.Identifier = target.Identifier
		backendClone.LoadBalancer.Name = ""
		backendClone.Identifier = ""

		err := anxAPI.Create(ctx, &backendClone)
		if err != nil {
			return err
		}
		target.Backends = append(target.Backends, components.NewHashedBackend(backendClone))
	}

	// Create frontends
	for _, hasher := range frontendDelta.Create {
		frontend := hasher.(components.HashedFrontend)
		frontendClone := *frontend.Frontend
		frontendClone.Name = fmt.Sprintf("%d.%s.lb-sync.anx.io", rand.Int63(), frontend.Frontend.Name)
		frontendClone.LoadBalancer.Identifier = target.Identifier
		frontendClone.LoadBalancer.Name = ""
		frontendClone.Identifier = ""

		var defaultBackend *components.HashedBackend
		if frontendClone.DefaultBackend != nil {
			defaultBackend = components.GetBackendByName(frontendClone.DefaultBackend.Name, target.Backends)
		}
		frontendClone.DefaultBackend = defaultBackend.Backend

		err := anxAPI.Create(ctx, &frontendClone)
		if err != nil {
			return err
		}
		target.Frontends = append(target.Frontends, components.NewHashedFrontend(frontendClone))
	}

	// Create binds
	for _, hasher := range bindDelta.Create {
		bind := hasher.(components.HashedBind)
		bindClone := *bind.Bind
		//bindClone.Name = fmt.Sprintf("%d.%s.lb-sync.anx.io", rand.Int63(), bind.Bind.Name)
		bindClone.Identifier = ""

		boundFrontend := components.FindCorrespondingFrontend(bind.Bind.Frontend.Name, source.Frontends, target.Frontends)
		if boundFrontend == nil {
			return errors.New("corresponding frontend not found")
		}
		bindClone.Frontend = *boundFrontend.Frontend

		err := anxAPI.Create(ctx, &bindClone)
		if err != nil {
			return err
		}
	}

	// Create Servers
	for _, hasher := range serverDelta.Create {
		server := hasher.(components.HashedServer)
		serverClone := *server.Server
		serverClone.Identifier = ""

		serverBackend := components.GetBackendByName(serverClone.Backend.Name, target.Backends)
		if serverBackend == nil {
			return errors.New("could not find corresponding backend in loadbalancer")
		}
		serverClone.Backend = *serverBackend.Backend

		err := anxAPI.Create(ctx, &serverClone)
		if err != nil {
			return err
		}
		target.Servers = append(target.Servers, components.NewHashedServer(serverClone))
	}

	return nil
}

func FetchLoadBalancer(ctx context.Context, lbID string, anxAPI api.API) (components.HashedLoadBalancer, error) {
	logr.FromContextOrDiscard(ctx).Info("fetching load balancer configuration", "load-balancer", lbID)
	f := frontend.Frontend{
		LoadBalancer: &loadbalancer.Loadbalancer{Identifier: lbID},
	}
	var iter types.ObjectChannel
	err := anxAPI.List(context.Background(), &f,
		api.ObjectChannel(&iter),
		api.FullObjects(true))

	if err != nil {
		return components.HashedLoadBalancer{},
			fmt.Errorf("could not list frontends from load balancer '%s' %w", lbID, err)
	}

	hashedFrontends := make([]components.HashedFrontend, 0, 5)
	for receiver := range iter {
		var f frontend.Frontend
		err := receiver(&f)
		if err != nil {
			return components.HashedLoadBalancer{}, fmt.Errorf("error when iterating over fetched frontends %w", err)
		}
		hashedFrontends = append(hashedFrontends, components.NewHashedFrontend(f))
	}

	b := backend.Backend{
		LoadBalancer: loadbalancer.Loadbalancer{Identifier: lbID},
	}

	iter = nil
	err = anxAPI.List(ctx, &b, api.ObjectChannel(&iter), api.FullObjects(true))
	if err != nil {
		return components.HashedLoadBalancer{},
			fmt.Errorf("error when fetching backends from loadbalancer '%s' %w", lbID, err)
	}

	hashedBackends := make([]components.HashedBackend, 0, 5)
	for receiver := range iter {
		var b backend.Backend
		err := receiver(&b)
		if err != nil {
			return components.HashedLoadBalancer{}, fmt.Errorf("error when iterating over fetched backends %w", err)
		}
		hashedBackends = append(hashedBackends, components.NewHashedBackend(b))
	}

	// fetch servers for every backend we got so far.
	wg := sync.WaitGroup{}
	mutex := sync.Mutex{}
	hashedServers := make([]components.HashedServer, 0, len(hashedBackends))
	wg.Add(len(hashedBackends))
	for _, loopBackend := range hashedBackends {
		go func(baseBackend components.HashedBackend) {
			defer wg.Done()
			s := server.Server{
				Backend: backend.Backend{Identifier: baseBackend.Backend.Identifier},
			}
			var iter types.ObjectChannel
			err := anxAPI.List(ctx, &s, api.ObjectChannel(&iter), api.FullObjects(true))
			if err != nil {
				panic(err)
			}
			for receiver := range iter {
				var s server.Server
				err := receiver(&s)
				if err != nil {
					panic(err)
				}
				hashedServer := components.NewHashedServer(s)
				mutex.Lock()
				hashedServers = append(hashedServers, hashedServer)
				mutex.Unlock()
			}
		}(loopBackend)
	}
	wg.Wait()

	// fetch binds for every frontend we got so far
	wg = sync.WaitGroup{}
	hashedBinds := make([]components.HashedBind, 0, len(hashedFrontends))
	wg.Add(len(hashedFrontends))
	for _, loopFrontend := range hashedFrontends {
		go func(baseFrontend components.HashedFrontend) {
			defer wg.Done()
			fb := bind.Bind{
				Frontend: frontend.Frontend{Identifier: baseFrontend.Frontend.Identifier},
			}
			var iter types.ObjectChannel
			err = anxAPI.List(ctx, &fb, api.ObjectChannel(&iter), api.FullObjects(true))
			if err != nil {
				panic(err)
			}
			for receiver := range iter {
				var fb bind.Bind
				err := receiver(&fb)
				if err != nil {
					panic(err)
				}
				hashedBind := components.NewHashedBind(fb)
				mutex.Lock()
				hashedBinds = append(hashedBinds, hashedBind)
				mutex.Unlock()
			}

		}(loopFrontend)
	}
	wg.Wait()

	return components.NewHashedLoadBalancer(lbID, hashedFrontends, hashedBackends, hashedServers, hashedBinds), nil
}
