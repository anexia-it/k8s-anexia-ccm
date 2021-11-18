package loadbalancer

import (
	"context"
	"errors"
	fmt "fmt"
	"github.com/anexia-it/go-anxcloud/pkg/lbaas"
	"github.com/anexia-it/go-anxcloud/pkg/lbaas/server"
	"github.com/go-logr/logr"
)

const (
	StatusNoServersAssigned       = "load balancer has no backend servers assigned"
	StatusBackendMissing          = "load balancer backend is not present"
	StatusFrontendMissing         = "load balancer frontend is not present"
	SatusBindMissing              = "load balancer frontend bind is not present"
	StatusSuccessfullyProvisioned = "load balancer successfully provisioned"
)

var (
	InvalidGroupError = errors.New("group has not been initialised correctly")
)

// state holds the current state of an LoadBalancer.
// It acts like a cache in order to not execute too many request during
// more complex reconcile operations.
type state struct {
	ID LoadBalancerID
	BackendID
	FrontendID
	BindID
}

// LoadBalancer represents an actual LoadBalancer
type LoadBalancer struct {
	lbaas.API
	State  *state
	Logger logr.Logger
}

// NodeEndpoint represents the actual K8s NodePort.
type NodeEndpoint struct {
	IP   string
	Port int32
}

// Type IDs in order to not pass strings around and prevent errors.

type LoadBalancerID string
type FrontendID string
type BackendID string
type BindID string

func NewLoadBalancer(api lbaas.API, id LoadBalancerID, logger logr.Logger) LoadBalancer {
	return LoadBalancer{
		API:    api,
		Logger: logger.WithValues("loadbalancer", id),
		State:  &state{ID: id},
	}
}

func (g LoadBalancer) EnsureLBConfig(ctx context.Context, lbName string, endpoints []NodeEndpoint) error {
	wrapErr := func(err error) error { return fmt.Errorf("unable to create loadbalancer: %w", err) }

	// ensure backend exists in every anexia load balancer of the group
	backendId, err := ensureBackendInLoadBalancer(ctx, g, lbName)
	if err != nil {
		return wrapErr(err)
	}
	g.State.BackendID = backendId

	// make sure frontends for these backends exists in every load balancer
	frontendId, err := ensureFrontendInLoadBalancer(ctx, g, lbName)
	if err != nil {
		return wrapErr(err)
	}
	g.State.FrontendID = frontendId

	bind, err := ensureFrontendBindInLoadBalancer(ctx, g, lbName)
	if err != nil {
		return wrapErr(err)
	}
	g.State.BindID = bind

	for _, endpoint := range endpoints {
		_, err := ensureBackendServerInLoadBalancer(ctx, g, lbName, endpoint)
		if err != nil {
			return wrapErr(err)
		}
	}
	servers := findServersByBackendInLB(ctx, g, lbName, "")
	if len(servers) == len(endpoints) {
		return nil
	}

	err = cleanupOldServers(ctx, g, lbName, endpoints, servers)
	if err != nil {
		return wrapErr(err)
	}

	return nil
}

func cleanupOldServers(ctx context.Context, g LoadBalancer, lbName string,
	wantedEndpoints []NodeEndpoint, exisitngServers []*server.Server) error {
	for _, server := range exisitngServers {
		var found bool
		for _, endpoint := range wantedEndpoints {
			if server.Name == getServerName(lbName, endpoint) {
				found = true
				break
			}
		}

		// when server was found we continue with next server
		if found {
			continue
		}

		// server was not found so we remove it
		err := g.Server().DeleteByID(ctx, server.Identifier)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g LoadBalancer) EnsureLBDeleted(ctx context.Context, lbName string) error {

	wrapErr := func(err error) error { return fmt.Errorf("unable to delete loadbalancer: %w", err) }

	// check if we have a frontend if yes delete it and all related to it
	frontend := findFrontendInLB(ctx, g, lbName)
	if frontend != nil {
		g.State.FrontendID = FrontendID(frontend.Identifier)
		// delete frontend bind
		err := ensureFrontendBindDeleted(ctx, g, lbName)
		if err != nil {
			return wrapErr(err)
		}

		// delete frontend
		err = ensureFrontendDeleted(ctx, g, lbName)
		if err != nil {
			return wrapErr(err)
		}
	}

	// check if we have a backend and if yes delete it and all resources related to it
	backend := findBackendInLB(ctx, g, lbName)
	if backend != nil {
		g.State.BackendID = BackendID(backend.Identifier)
		// delete all servers beforehand
		err := deleteServersFromBackendInLB(ctx, g, lbName)
		if err != nil {
			return wrapErr(err)
		}

		// delete backend
		err = deleteBackendFromLB(ctx, g, lbName)
		if err != nil {
			return wrapErr(err)
		}
	}
	return nil
}

func (g LoadBalancer) GetProvisioningState(ctx context.Context, lbName string) (bool, string) {
	backend := findBackendInLB(ctx, g, lbName)
	if backend == nil {
		return false, StatusBackendMissing
	}
	g.State.BackendID = BackendID(backend.Identifier)

	frontend := findFrontendInLB(ctx, g, lbName)
	if frontend == nil {
		return false, StatusFrontendMissing
	}
	g.State.FrontendID = FrontendID(frontend.Identifier)

	bind := findFrontendBindInLoadBalancer(ctx, g, lbName)
	if bind == nil {
		return false, SatusBindMissing
	}
	g.State.BindID = BindID(bind.Identifier)

	servers := findServersByBackendInLB(ctx, g, lbName, "")
	if len(servers) == 0 {
		return false, StatusNoServersAssigned
	}

	return true, StatusSuccessfullyProvisioned
}

// HostInformation holds the information about a host.
type HostInformation struct {
	IP       string
	Hostname string
}

func (g LoadBalancer) GetHostInformation(ctx context.Context) (HostInformation, error) {
	lbIdentifier := g.State.ID
	fetchedBalancer, err := g.LoadBalancer().GetByID(ctx, string(lbIdentifier))
	return HostInformation{
		IP:       fetchedBalancer.IpAddress,
		Hostname: "",
	}, err
}
