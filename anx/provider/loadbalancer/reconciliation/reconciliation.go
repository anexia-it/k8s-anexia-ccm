package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/wait"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"

	gs "go.anx.io/go-anxcloud/pkg/apis/common/gs"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

var (
	// ErrResourcesNotDestroyable is returned when Reconcile tried to Destroy a resource but Engine returned an error
	ErrResourcesNotDestroyable = errors.New("failed to Destroy some resources")

	// ErrLBaaSResourceProgressing is returned by ReconcileCheck() when an existing resource was retrieved as not yet ready.
	ErrLBaaSResourceProgressing = errors.New("LBaaS resource still progressing")

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

	frontends []*lbaasv1.Frontend
	backends  []*lbaasv1.Backend
	binds     []*lbaasv1.Bind
	servers   []*lbaasv1.Server

	// we store existing failed Objects here
	existingFailed []types.Object

	// and store existing Objects that are not yet ready here
	existingProgressing []types.Object

	// information and connections gathered from existing resources

	portBackends    map[string]*lbaasv1.Backend
	portFrontends   map[string]*lbaasv1.Frontend
	publicAddresses []string

	backoffSteps int

	metrics metrics.ProviderMetrics
}

// New creates a new Reconcilation instance, usable to reconcile Anexia LBaaS resources for
//
//   - a single LBaaS LoadBalancer
//   - a single service UID (used for tagging the created resources and listing them)
//   - a set of external IP addresses (translated into LBaaS Binds)
//   - a set of ports (each having an internal (Kubernetes NodePort) and external (LBaaS Bind) port)
//   - a set of nodes (each translated to a LBaaS Server)
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

	backoffSteps int,

	metrics metrics.ProviderMetrics,
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

		backoffSteps: backoffSteps,

		metrics: metrics,
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

	if len(r.existingFailed) > 0 {
		return []types.Object{}, r.existingFailed, nil
	}

	if len(r.existingProgressing) > 0 {
		return nil, nil, ErrLBaaSResourceProgressing
	}

	retToDestroy := []types.Object{}
	retToCreate := []types.Object{}

	steps := []func() ([]types.Object, []types.Object, error){
		r.reconcileBackends,
		r.reconcileFrontends,
		r.reconcileBinds,
		r.reconcileServers,
	}

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
		startTimeTotal := time.Now()
		toCreate, toDestroy, err := r.ReconcileCheck()
		if err != nil {
			if !errors.Is(err, ErrLBaaSResourceProgressing) {
				return err
			}

			r.logger.Info("Some existing resources are not ready to use, waiting for them to get ready", "objects", mustStringifyObjects(r.existingProgressing))

			err := r.waitForResources(r.existingProgressing)
			if err != nil {
				return err
			}
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
			r.metrics.ReconciliationPendingResources.WithLabelValues("lbaas", "destroy").Add(float64(len(toDestroy)))
			r.logger.V(1).Info("destroying resources", "objects", mustStringifyObjects(toDestroy))

			allowRetry := true

		outer:
			for len(toDestroy) > 0 && allowRetry {
				allowRetry = false

				newToDestroy := make([]types.Object, 0, len(toDestroy))

				for _, obj := range toDestroy {
					err := r.api.Destroy(r.ctx, obj)
					err = api.IgnoreNotFound(err) // We deliberately want to ignore not found errors, as they indicate success.

					if err != nil {
						newToDestroy = append(newToDestroy, obj)
						r.metrics.ReconciliationDeleteRetriesTotal.WithLabelValues("lbaas").Inc()

						if api.IsRateLimitError(err) {
							r.logger.Error(err, "aborting reconciliation, waiting for rate-limit to be released")
							// If we run into a rate-limiting error, abort immediately.
							break outer
						}

						r.logger.Info("Destroying LBaaS resource failed, marking for retry and continuing",
							"object", mustStringifyObject(obj),
						)
					}

					// The loadbalancer got already deleted, we're successful with this one!
					//
					// Allow retry to delete other things, in case they failed previously.
					allowRetry = true

					r.metrics.ReconciliationDeletedTotal.WithLabelValues("lbaas").Inc()
					r.metrics.ReconciliationPendingResources.WithLabelValues("lbaas", "destroy").Dec()
				}

				toDestroy = newToDestroy
			}

			if len(toDestroy) > 0 && !allowRetry {
				r.logger.Error(ErrResourcesNotDestroyable, "Some resources could not be deleted",
					"objects", mustStringifyObjects(toDestroy),
				)

				r.metrics.ReconciliationDeleteErrorsTotal.WithLabelValues("lbaas").Inc()

				return ErrResourcesNotDestroyable
			}
		} else if len(toCreate) > 0 {
			r.metrics.ReconciliationPendingResources.WithLabelValues("lbaas", "create").Add(float64(len(toCreate)))
			r.logger.V(1).Info("creating resources", "count", len(toCreate))

			for _, obj := range toCreate {
				if err := r.api.Create(r.ctx, obj); err != nil {
					// Ensure decrementing pending resources before returning to prevent leakage
					r.metrics.ReconciliationPendingResources.WithLabelValues("lbaas", "create").Dec()
					r.metrics.ReconciliationCreateErrorsTotal.WithLabelValues("lbaas").Inc()
					return fmt.Errorf("error creating LBaaS resource: %w", err)
				}

				if err := r.tagResource(obj); err != nil {
					// Ensure decrementing pending resources before returning to prevent leakage
					r.metrics.ReconciliationPendingResources.WithLabelValues("lbaas", "create").Dec()
					r.metrics.ReconciliationCreateErrorsTotal.WithLabelValues("lbaas").Inc()
					return fmt.Errorf("error tagging LBaaS resource: %w", err)
				}

				r.metrics.ReconciliationPendingResources.WithLabelValues("lbaas", "create").Dec()
				r.metrics.ReconciliationCreatedTotal.WithLabelValues("lbaas").Inc()
			}
			r.logger.Info("waiting for created resources to become ready", "objects", mustStringifyObjects(toCreate))

			startTimeCreate := time.Now()
			err := r.waitForResources(toCreate)
			if err != nil && !errors.Is(err, ErrLBaaSResourceFailed) {
				r.metrics.ReconciliationCreateErrorsTotal.WithLabelValues("lbaas").Inc()
				return err
			}

			r.metrics.ReconciliationCreateResources.WithLabelValues("lbaas").Observe(float64(time.Since(startTimeCreate).Seconds()))
		} else {
			completed = true
		}

		r.metrics.ReconciliationTotalDuration.WithLabelValues("lbaas").Observe(float64(time.Since(startTimeTotal).Seconds()))
	}

	return nil
}

var _engsup5902_mutex = sync.Mutex{}

func (r *reconciliation) tagResource(o types.Object) error {
	_engsup5902_mutex.Lock()
	defer _engsup5902_mutex.Unlock()

	identifier, _ := types.GetObjectIdentifier(o, true)

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
	// we want to retrieve every Object in this loop at least once, to deal with "defaults-to-success" and similar things.
	firstPass := true

	return wait.ExponentialBackoff(
		wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   1.5,
			Jitter:   1.5,
			Steps:    r.backoffSteps,
			Cap:      5 * time.Minute,
		},
		func() (done bool, err error) {
			notReady := make([]types.Object, 0, len(toCreate))
			failed := make([]types.Object, 0, len(toCreate))

			for _, obj := range toCreate {
				state, ok := obj.(gs.StateRetriever)
				if !ok {
					r.logger.Error(nil, "Object does not have state", "object", mustStringifyObject(obj))
					return false, errors.New("coding error: waitForResources called for Object not implementing lbaasv1.StateRetriever")
				}

				// if we already retrieved Objects: shortcut
				if state.StateOK() && !firstPass {
					continue
				}

				err := r.api.Get(r.ctx, obj)
				if err != nil {
					r.logger.Error(err, "Error retrieving current state of Object, assuming it's failed", "object", mustStringifyObject(obj))
					failed = append(failed, obj)
					continue
				}

				if state.StatePending() {
					notReady = append(notReady, obj)
				} else if state.StateError() {
					failed = append(failed, obj)
				}
			}

			firstPass = false

			if len(failed) > 0 {
				err = ErrLBaaSResourceFailed
				r.logger.Error(err, "Some object are in failure state, aborting", "objects", mustStringifyObjects(failed))
				return false, err
			} else if len(notReady) > 0 {
				r.logger.Info("Still waiting for created resources to become ready", "objects", mustStringifyObjects(notReady))
				return false, nil
			} else {
				return true, nil
			}
		},
	)
}

func (r *reconciliation) retrieveState() error {
	r.frontends = make([]*lbaasv1.Frontend, 0)
	r.backends = make([]*lbaasv1.Backend, 0)
	r.binds = make([]*lbaasv1.Bind, 0)
	r.servers = make([]*lbaasv1.Server, 0)
	r.portBackends = make(map[string]*lbaasv1.Backend)
	r.portFrontends = make(map[string]*lbaasv1.Frontend)

	r.existingFailed = make([]types.Object, 0)
	r.existingProgressing = make([]types.Object, 0)

	if err := r.retrieveResources(); err != nil {
		return err
	}

	return nil
}

func (r *reconciliation) sortObjectIntoStateArray(o types.Object) {
	sr, ok := o.(gs.StateRetriever)
	if !ok {
		return
	}

	if sr.StateError() {
		r.existingFailed = append(r.existingFailed, o)
	} else if sr.StatePending() {
		r.existingProgressing = append(r.existingProgressing, o)
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

	allBinds := make([]*lbaasv1.Bind, 0)
	allServers := make([]*lbaasv1.Server, 0)

	typedRetrievers := map[string]func(identifier string) error{
		// frontends and backends are filtered for our LoadBalancer here already

		frontendResourceTypeIdentifier: func(identifier string) (err error) {
			frontend := &lbaasv1.Frontend{Identifier: identifier}
			if err = r.api.Get(ctx, frontend); err == nil && frontend.LoadBalancer.Identifier == r.lb.Identifier {
				r.frontends = append(r.frontends, frontend)
				r.sortObjectIntoStateArray(frontend)
			}
			return
		},

		backendResourceTypeIdentifier: func(identifier string) (err error) {
			backend := &lbaasv1.Backend{Identifier: identifier}
			if err = r.api.Get(ctx, backend); err == nil && backend.LoadBalancer.Identifier == r.lb.Identifier {
				r.backends = append(r.backends, backend)
				r.sortObjectIntoStateArray(backend)
			}
			return
		},

		bindResourceTypeIdentifier: func(identifier string) (err error) {
			bind := &lbaasv1.Bind{Identifier: identifier}
			if err = r.api.Get(ctx, bind); err == nil {
				allBinds = append(allBinds, bind)
			}
			return
		},

		serverResourceTypeIdentifier: func(identifier string) (err error) {
			server := &lbaasv1.Server{Identifier: identifier}
			if err = r.api.Get(ctx, server); err == nil {
				allServers = append(allServers, server)
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

	r.binds, err = r.filterBinds(allBinds)
	if err != nil {
		return err
	}

	r.servers, err = r.filterServers(allServers)
	if err != nil {
		return err
	}

	r.logger.V(1).Info(
		"retrieved resources",
		"num-frontends", len(r.frontends),
		"num-binds", len(r.binds),
		"num-backends", len(r.backends),
		"num-servers", len(r.servers),
	)

	r.metrics.ReconciliationRetrievedResourcesTotal.WithLabelValues("lbaas", "frontend").Add(float64(len(r.frontends)))
	r.metrics.ReconciliationRetrievedResourcesTotal.WithLabelValues("lbaas", "bind").Add(float64(len(r.binds)))
	r.metrics.ReconciliationRetrievedResourcesTotal.WithLabelValues("lbaas", "backend").Add(float64(len(r.backends)))
	r.metrics.ReconciliationRetrievedResourcesTotal.WithLabelValues("lbaas", "server").Add(float64(len(r.servers)))

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
