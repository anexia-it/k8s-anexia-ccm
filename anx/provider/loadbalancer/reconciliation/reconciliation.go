package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/wait"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"

	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

const (
	frontendResourceTypeIdentifier = "da9d14b9d95840c08213de67f9cee6e2"
	bindResourceTypeIdentifier     = "bd24def982aa478fb3352cb5f49aab47"
	backendResourceTypeIdentifier  = "33164a3066a04a52be43c607f0c5dd8c"
	serverResourceTypeIdentifier   = "01f321a4875446409d7d8469503a905f"
)

var (
	// ErrResourcesNotDestroyable is returned when Reconcile tried to Destroy a resource but Engine returned an error
	ErrResourcesNotDestroyable = errors.New("failed to Destroy some resources")

	// ErrLBaaSResourceFailed is returned when a LBaaS resource is in a not-ok state.
	ErrLBaaSResourceFailed = errors.New("LBaaS resource in failure state")
)

// Reconciliation wraps the current status and gives methods to operate on it, changing it into the desired state. Read the documentation for New for more info.
type Reconciliation interface {
	// ReconcileCheck runs every step once, collecting resources to be destroyed but stopping once any step needs
	// something to be created. It returns the resources to be created and destroyed. When both of the returned
	// arrays are empty no resources have to be created or destroyed anymore, meaning reconciliation is complete.
	//
	// Some resources need others to already exist, so creating all resources returned by ReconcileCheck
	// and then calling ReconcileCheck again will not always result in "nothing to change".
	ReconcileCheck() (toCreate []types.Object, toDestroy []types.Object, err error)

	// Reconcile calls ReconcileCheck in a loop, each time first destroying and then creating Objects until ReconcileCheck returns no operations to be done.
	// Once Reconcile returns without error, reconciliation is complete.
	Reconcile() error

	// Status returns a map with external IP address as key and array of ports as value, based on the current state in the Engine.
	Status() (map[string][]uint16, error)
}

type reconciliation struct {
	ctx    context.Context
	api    api.API
	logger logr.Logger

	resourceNameSuffix string
	lb                 lbaasv1.LoadBalancer

	externalAddresses []net.IP
	ports             map[string]Port
	targetServers     []Server

	tags []string

	// existing resources

	frontends []lbaasv1.Frontend
	backends  []lbaasv1.Backend
	binds     []lbaasv1.Bind
	servers   []lbaasv1.Server

	// information and connections gathered from existing resources

	portBackends    map[string]*lbaasv1.Backend
	portFrontends   map[string]*lbaasv1.Frontend
	publicAddresses []string
}

// New creates a new Reconcilation instance, usable to reconcile Anexia LBaaS resources for
//
//  * a single LBaaS LoadBalancer
//  * a single service UID (used for tagging the created resources and listing them)
//  * a set of external IP addresses (translated into LBaaS Binds)
//  * a set of ports (each having an internal (Kubernetes NodePort) and external (LBaaS Bind) port)
//  * a set of nodes (each translated to a LBaaS Server)
//
// Before doing anything, it will list all resources currently present in the Engine tagged with
// `anxccm-svc-ui=$serviceUID`.
//
// Reconcilation is done in steps, in order Backends, Frontends, Binds, Servers. Each step returns a list
// of create and destroy operations to do, based on the current and desired state. The methods Reconcile,
// ReconcileCheck and Status use these steps and their results in different ways.
//
// Final result is a LBaaS Frontend and Backend per port, for each Frontend one Bind per external IP
// and, also for each Frontend, a Backend per Port and Server.
func New(
	ctx context.Context,
	apiClient api.API,

	resourceNameSuffix string,
	loadBalancerIdentifier string,
	serviceUID string,

	externalAddresses []net.IP,
	ports map[string]Port,
	servers []Server,
) (Reconciliation, error) {
	tags := []string{
		fmt.Sprintf("anxccm-svc-uid=%v", serviceUID),
	}

	recon := reconciliation{
		ctx:    ctx,
		api:    apiClient,
		logger: logr.FromContextOrDiscard(ctx),

		tags:               tags,
		resourceNameSuffix: resourceNameSuffix,

		externalAddresses: externalAddresses,
		ports:             ports,
		targetServers:     servers,
	}

	recon.lb.Identifier = loadBalancerIdentifier
	if err := apiClient.Get(ctx, &recon.lb); err != nil {
		return nil, fmt.Errorf("error retrieving LBaaS LoadBalancer to attach service to: %w", err)
	}

	recon.logger.Info(
		"Reconciling LBaaS for LoadBalancer Service",
		"num-ports", len(ports),
		"num-servers", len(servers),
		"resource-tags", tags,
	)

	return &recon, nil
}

// ReconcileCheck checks if any resources have to be reconciled, returning the list of resources
// to be created and destroyed. When both of the returned arrays are empty no resources have to be
// created or destroyed anymore, meaning reconciliation is done.
//
// Some resources need others to already exist, so creating all resources returned by ReconcileCheck
// and then calling ReconcileCheck again will not always result in "nothing to change".
func (r *reconciliation) ReconcileCheck() ([]types.Object, []types.Object, error) {
	if err := r.retrieveState(); err != nil {
		return nil, nil, fmt.Errorf("error retrieving current state for reconciliation: %w", err)
	}

	steps := []func() ([]types.Object, []types.Object, error){
		r.reconcileBackends,
		r.reconcileFrontends,
		r.reconcileBinds,
		r.reconcileServers,
	}

	retToDestroy := r.reconcileFailedResources()
	if len(retToDestroy) > 0 {
		return []types.Object{}, retToDestroy, nil
	}

	retToCreate := []types.Object{}

	for _, step := range steps {
		toCreate, toDestroy, err := step()

		if err != nil {
			return nil, nil, err
		}

		retToCreate = append(retToCreate, toCreate...)
		retToDestroy = append(retToDestroy, toDestroy...)
	}

	return retToCreate, retToDestroy, nil
}

// Reconcile calls ReconcileCheck in a loop, every time creating and destroying resources, until reconciliation
// is done.
func (r *reconciliation) Reconcile() error {
	completed := false

	for !completed {
		toCreate, toDestroy, err := r.ReconcileCheck()
		if err != nil {
			return err
		}

		// if there is something to destroy: destroy it and start again from retrieving the state
		// if there is nothing to destroy, but something to create: create it and start again from retrieving the state
		// if there is nothing to destroy or create: we are finished
		//
		// First destroying and then retrieving gives the Engine some time to process that, also, we'll
		// retrieve the resource if it is not yet deleted, going into a destroy loop until it's gone.
		// We might create resources clashing with others that are to be destroyed, hence only creating
		// after there is nothing left to destroy.

		if len(toDestroy) > 0 {
			r.logger.V(1).Info("destroying resources", "count", len(toDestroy))

			allowRetry := true

			for len(toDestroy) > 0 && allowRetry {
				allowRetry = false

				newToDestroy := make([]types.Object, 0, len(toDestroy))

				for _, obj := range toDestroy {
					if err := r.api.Destroy(r.ctx, obj); api.IgnoreNotFound(err) != nil {
						identifier, _ := api.GetObjectIdentifier(obj, true)

						r.logger.Info("Destroying LBaaS resource failed, marking for retry and continuing",
							"resource-id", identifier,
						)

						newToDestroy = append(newToDestroy, obj)
					} else {
						// something was deleted successfully, allow retrying to delete other things in case it failed for conflicts
						allowRetry = true
					}
				}

				toDestroy = newToDestroy
			}

			if len(toDestroy) > 0 && !allowRetry {
				toDestroyForLog := make([]string, 0, len(toDestroy))
				for _, td := range toDestroy {
					identifier, _ := api.GetObjectIdentifier(td, true)
					toDestroyForLog = append(toDestroyForLog, fmt.Sprintf("%T %v", td, identifier))
				}

				r.logger.Error(ErrResourcesNotDestroyable, "Some resources could not be deleted",
					"resources", toDestroyForLog,
				)

				return ErrResourcesNotDestroyable
			}
		} else if len(toCreate) > 0 {
			r.logger.V(1).Info("creating resources", "count", len(toCreate))

			for _, obj := range toCreate {
				if err := r.api.Create(r.ctx, obj); err != nil {
					return fmt.Errorf("error creating LBaaS resource: %w", err)
				}

				if err := r.tagResource(obj); err != nil {
					return fmt.Errorf("error tagging LBaaS resource: %w", err)
				}
			}
			r.logger.Info("waiting for created resources to become ready", "count", len(toCreate))

			err := r.waitForResources(toCreate)
			if err != nil && !errors.Is(err, ErrLBaaSResourceFailed) {
				return err
			}
		} else {
			completed = true
		}
	}

	return nil
}

var _engsup5902_mutex = sync.Mutex{}

func (r *reconciliation) tagResource(o types.Object) error {
	_engsup5902_mutex.Lock()
	defer _engsup5902_mutex.Unlock()

	identifier, _ := api.GetObjectIdentifier(o, true)

	for _, tag := range r.tags {
		rt := corev1.ResourceWithTag{
			Identifier: identifier,
			Tag:        tag,
		}

		if err := r.api.Create(r.ctx, &rt); err != nil {
			return err
		}
	}

	return nil
}

func (r *reconciliation) Status() (map[string][]uint16, error) {
	if err := r.retrieveState(); err != nil {
		return nil, err
	}

	ret := make(map[string][]uint16)

	for _, bind := range r.binds {
		addr := bind.Address

		if _, ok := ret[addr]; !ok {
			ret[addr] = make([]uint16, 0)
		}

		ret[addr] = append(ret[addr], uint16(bind.Port))
	}

	return ret, nil
}

func (r *reconciliation) waitForResources(toCreate []types.Object) error {
	return wait.ExponentialBackoff(
		wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   1.5,
			Jitter:   1.5,
			Steps:    math.MaxInt,
			Cap:      5 * time.Minute,
		},
		func() (done bool, err error) {
			notReady := make([]string, 0)
			failed := make([]string, 0)

			for _, obj := range toCreate {
				identifier, _ := api.GetObjectIdentifier(obj, true)

				err := r.api.Get(r.ctx, obj)
				if err != nil {
					r.logger.Error(err, "Error retrieving current state of Object", "object", identifier)
					failed = append(failed, identifier)
				}

				if state, ok := obj.(lbaasv1.StateRetriever); ok {
					if state.StateProgressing() {
						notReady = append(notReady, identifier)
					} else if state.StateFailure() {
						failed = append(failed, identifier)
					}
				} else {
					r.logger.Error(nil, "Object does not have state", "object", obj)
					return false, errors.New("coding error")
				}
			}

			if len(failed) > 0 {
				err = ErrLBaaSResourceFailed
				r.logger.Error(err, "Some object are in failure state, aborting", "objects", failed)
				return false, err
			} else if len(notReady) > 0 {
				r.logger.Info("Still waiting for created resources to become ready", "objects", notReady)
				return false, nil
			} else {
				return true, nil
			}
		},
	)
}

func (r *reconciliation) retrieveState() error {
	r.frontends = make([]lbaasv1.Frontend, 0)
	r.backends = make([]lbaasv1.Backend, 0)
	r.binds = make([]lbaasv1.Bind, 0)
	r.servers = make([]lbaasv1.Server, 0)
	r.portBackends = make(map[string]*lbaasv1.Backend)
	r.portFrontends = make(map[string]*lbaasv1.Frontend)

	if err := r.retrieveResources(); err != nil {
		return err
	}

	return nil
}

func (r *reconciliation) storePublicAddress(addr string) {
	if idx := sort.SearchStrings(r.publicAddresses, addr); idx >= len(r.publicAddresses) || r.publicAddresses[idx] != addr {
		r.publicAddresses = append(r.publicAddresses, addr)
		sort.Strings(r.publicAddresses)
	}
}

func (r *reconciliation) retrieveResources() error {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	var oc types.ObjectChannel
	err := r.api.List(ctx, &corev1.Resource{Tags: r.tags}, api.ObjectChannel(&oc), api.FullObjects(true))
	if err != nil {
		var he api.HTTPError

		// error 422 is returned when nothing is tagged with the searched-for tag
		if !(errors.As(err, &he) && he.StatusCode() == 422) {
			return fmt.Errorf("error retrieving resources: %w", err)
		}

		return nil
	}

	typedRetrievers := map[string]func(identifier string) error{
		// frontends and backends are filtered for our LoadBalancer here already

		frontendResourceTypeIdentifier: func(identifier string) (err error) {
			frontend := lbaasv1.Frontend{Identifier: identifier}

			if err = r.api.Get(ctx, &frontend); err == nil && frontend.LoadBalancer.Identifier == r.lb.Identifier {
				r.frontends = append(r.frontends, frontend)
			}
			return
		},

		backendResourceTypeIdentifier: func(identifier string) (err error) {
			backend := lbaasv1.Backend{Identifier: identifier}
			if err = r.api.Get(ctx, &backend); err == nil && backend.LoadBalancer.Identifier == r.lb.Identifier {
				r.backends = append(r.backends, backend)
			}
			return
		},

		bindResourceTypeIdentifier: func(identifier string) (err error) {
			bind := lbaasv1.Bind{Identifier: identifier}
			if err = r.api.Get(ctx, &bind); err == nil {
				r.binds = append(r.binds, bind)
			}

			if bind.Address != "" {
				r.storePublicAddress(bind.Address)
			}
			return
		},

		serverResourceTypeIdentifier: func(identifier string) (err error) {
			server := lbaasv1.Server{Identifier: identifier}
			if err = r.api.Get(ctx, &server); err == nil {
				r.servers = append(r.servers, server)
			}
			return
		},
	}

	for retriever := range oc {
		var res corev1.Resource
		if err := retriever(&res); err != nil {
			return fmt.Errorf("error retrieving resource: %w", err)
		}

		logger := r.logger.WithValues(
			"resource-identifier", res.Identifier,
			"resource-name", res.Name,
		)

		if typedRetriever, ok := typedRetrievers[res.Type.Identifier]; ok {
			err := typedRetriever(res.Identifier)
			if err != nil {
				return fmt.Errorf("error retrieving typed resource: %w", err)
			}
		} else {
			logger.Info(
				"retrieved resource of unknown type, did someone else use our tag? Ignoring it",
				"resource-type-name", res.Type.Name,
				"resource-type-id", res.Type.Identifier,
			)
		}
	}

	// Binds and Servers are filtered for our LoadBalancer here, after we hopefully retrieved their Frontends and Backends already
	allBinds := r.binds
	allServers := r.servers
	r.binds = make([]lbaasv1.Bind, 0, len(allBinds))
	r.servers = make([]lbaasv1.Server, 0, len(allServers))

	for _, bind := range allBinds {
		idx, err := compare.Search(lbaasv1.Frontend{Identifier: bind.Frontend.Identifier}, r.frontends, "Identifier")
		if err != nil {
			return fmt.Errorf("error checking if Binds belongs to one of our frontends: %w", err)
		} else if idx != -1 {
			r.binds = append(r.binds, bind)
		}
	}

	for _, server := range allServers {
		idx, err := compare.Search(lbaasv1.Backend{Identifier: server.Backend.Identifier}, r.backends, "Identifier")
		if err != nil {
			return fmt.Errorf("error checking if Server belongs to one of our frontends: %w", err)
		} else if idx != -1 {
			r.servers = append(r.servers, server)
		}
	}

	r.logger.V(1).Info(
		"retrieved resources",
		"num-frontends", len(r.frontends),
		"num-binds", len(r.binds),
		"num-backends", len(r.backends),
		"num-servers", len(r.servers),
	)
	return nil
}

func (r *reconciliation) makeResourceName(parts ...string) string {
	validParts := make([]string, 0, len(parts)+1)
	for _, part := range append(parts, r.resourceNameSuffix) {
		if part != "" {
			validParts = append(validParts, part)
		}
	}

	return strings.Join(validParts, ".")
}

func (r *reconciliation) reconcileFailedResources() (toDestroy []types.Object) {
	toDestroy = make([]types.Object, 0, len(r.backends)+len(r.frontends)+len(r.binds)+len(r.servers))

	for _, backend := range r.backends {
		if backend.StateFailure() {
			r.logger.Info("Scheduling failed LBaaS Backend for Destroy", "identifier", backend.Identifier)
			toDestroy = append(toDestroy, &backend)
		}
	}

	for _, frontend := range r.frontends {
		if frontend.StateFailure() {
			r.logger.Info("Scheduling failed LBaaS Frontend for Destroy", "identifier", frontend.Identifier)
			toDestroy = append(toDestroy, &frontend)
		}
	}

	for _, bind := range r.binds {
		if bind.StateFailure() {
			r.logger.Info("Scheduling failed LBaaS Bind for Destroy", "identifier", bind.Identifier)
			toDestroy = append(toDestroy, &bind)
		}
	}

	for _, server := range r.servers {
		if server.StateFailure() {
			r.logger.Info("Scheduling failed LBaaS Server for Destroy", "identifier", server.Identifier)
			toDestroy = append(toDestroy, &server)
		}
	}

	return
}

func (r *reconciliation) reconcileBackends() (toCreate, toDestroy []types.Object, err error) {
	targetBackends := make([]lbaasv1.Backend, 0, len(r.ports))
	for name := range r.ports {
		targetBackends = append(targetBackends, lbaasv1.Backend{
			Name:         r.makeResourceName(name),
			LoadBalancer: lbaasv1.LoadBalancer{Identifier: r.lb.Identifier},
			Mode:         lbaasv1.TCP,
		})
	}

	toCreate = make([]types.Object, 0, len(targetBackends))
	toDestroy = make([]types.Object, 0, len(r.backends))

	err = compare.Reconcile(
		targetBackends, r.backends,
		&toCreate, &toDestroy,
		"Name", "Mode", "LoadBalancer.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	if len(toCreate) == 0 && len(toDestroy) == 0 {
		for name := range r.ports {
			expectedName := r.makeResourceName(name)

			for _, b := range targetBackends {
				if b.Name == expectedName {
					r.portBackends[name] = &b
					break
				}
			}
		}
	}

	return
}

func (r *reconciliation) reconcileFrontends() (toCreate, toDestroy []types.Object, err error) {
	targetFrontends := make([]lbaasv1.Frontend, 0, len(r.ports))
	for name := range r.ports {
		backend, ok := r.portBackends[name]
		if !ok {
			r.logger.V(2).Info("Not reconciling frontend because backend not (yet?) found",
				"port", name,
			)
			continue
		}

		targetFrontends = append(targetFrontends, lbaasv1.Frontend{
			Name:           r.makeResourceName(name),
			Mode:           lbaasv1.TCP,
			LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: r.lb.Identifier},
			DefaultBackend: &lbaasv1.Backend{Identifier: backend.Identifier},
		})
	}

	toCreate = make([]types.Object, 0, len(targetFrontends))
	toDestroy = make([]types.Object, 0, len(r.frontends))

	err = compare.Reconcile(
		targetFrontends, r.frontends,
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
					r.portFrontends[name] = &f
					break
				}
			}
		}
	}

	return
}

func (r *reconciliation) reconcileBinds() (toCreate, toDestroy []types.Object, err error) {
	targetBinds := make([]lbaasv1.Bind, 0, len(r.externalAddresses)*len(r.ports))
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
			targetBinds = append(targetBinds, lbaasv1.Bind{
				Name:     r.makeResourceName(fam, name),
				Address:  a.String(),
				Port:     int(port.External),
				Frontend: lbaasv1.Frontend{Identifier: frontend.Identifier},
			})
		}
	}

	toCreate = make([]types.Object, 0, len(targetBinds))
	toDestroy = make([]types.Object, 0, len(r.binds))

	err = compare.Reconcile(
		targetBinds, r.binds,
		&toCreate, &toDestroy,
		"Name", "Address", "Port", "Frontend.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	return
}

func (r *reconciliation) reconcileServers() (toCreate, toDestroy []types.Object, err error) {
	targetServers := make([]lbaasv1.Server, 0, len(r.ports)*len(r.targetServers))
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

			targetServers = append(targetServers, lbaasv1.Server{
				Name:    r.makeResourceName(server.Name, portName),
				IP:      server.Address.String(),
				Port:    int(port.Internal),
				Check:   "enabled",
				Backend: lbaasv1.Backend{Identifier: backend.Identifier},
			})
		}
	}

	toCreate = make([]types.Object, 0, len(targetServers))
	toDestroy = make([]types.Object, 0, len(r.servers))

	err = compare.Reconcile(
		targetServers, r.servers,
		&toCreate, &toDestroy,
		"Name", "IP", "Port", "Check", "Backend.Identifier",
	)
	if err != nil {
		return nil, nil, err
	}

	return
}
