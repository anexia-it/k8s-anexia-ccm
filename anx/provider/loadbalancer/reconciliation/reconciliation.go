package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/wait"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"

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

	serviceUID string

	tags []string

	remoteStateSnapshot *remoteLoadBalancerState

	// information and connections gathered from existing resources

	portBackends  map[string]*lbaasv1.Backend
	portFrontends map[string]*lbaasv1.Frontend
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
) (Reconciliation, error) {
	tags := []string{
		fmt.Sprintf("anxccm-svc-uid=%v", serviceUID),
	}

	recon := reconciliation{
		ctx:    ctx,
		api:    apiClient,
		logger: logr.FromContextOrDiscard(ctx),

		serviceUID: serviceUID,

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
	retriever, done := stateRetrieverFromContextOrNew(r.ctx, r)
	defer done()

	return r.getStateDiff(retriever)
}

// Reconcile calls ReconcileCheck in a loop, every time creating and destroying resources, until reconciliation
// is done.
func (r *reconciliation) Reconcile() error {
	retriever, done := stateRetrieverFromContextOrNew(r.ctx, r)
	defer done()

	completed := false

	for !completed {
		toCreate, toDestroy, err := r.getStateDiff(retriever)
		if err != nil {
			if !errors.Is(err, ErrLBaaSResourceProgressing) {
				return err
			}

			r.logger.Info("Some existing resources are not ready to use, waiting for them to get ready", "objects", mustStringifyObjects(r.remoteStateSnapshot.existingProgressing))

			err := r.waitForResources(r.remoteStateSnapshot.existingProgressing)
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
			r.logger.V(1).Info("destroying resources", "objects", mustStringifyObjects(toDestroy))

			allowRetry := true

			for len(toDestroy) > 0 && allowRetry {
				allowRetry = false

				newToDestroy := make([]types.Object, 0, len(toDestroy))

				for _, obj := range toDestroy {
					if err := r.api.Destroy(r.ctx, obj); api.IgnoreNotFound(err) != nil {
						r.logger.Info("Destroying LBaaS resource failed, marking for retry and continuing",
							"object", mustStringifyObject(obj),
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
				r.logger.Error(ErrResourcesNotDestroyable, "Some resources could not be deleted",
					"objects", mustStringifyObjects(toDestroy),
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
			r.logger.Info("waiting for created resources to become ready", "objects", mustStringifyObjects(toCreate))

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

func (r *reconciliation) handleExistingResources() (handled bool, toCreate, toDestroy []types.Object, err error) {
	if len(r.remoteStateSnapshot.existingFailed) > 0 {
		return true, []types.Object{}, r.remoteStateSnapshot.existingFailed, nil
	}

	if len(r.remoteStateSnapshot.existingProgressing) > 0 {
		return true, nil, nil, ErrLBaaSResourceProgressing
	}

	return false, nil, nil, nil
}

func (r *reconciliation) getStateDiff(retriever stateRetriever) ([]types.Object, []types.Object, error) {
	if err := r.retrieveState(retriever); err != nil {
		return nil, nil, fmt.Errorf("error retrieving current state for reconciliation: %w", err)
	}

	if handled, toCreate, toDestroy, err := r.handleExistingResources(); handled {
		return toCreate, toDestroy, err
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
	retriever, done := stateRetrieverFromContextOrNew(r.ctx, r)
	defer done()

	if err := r.retrieveState(retriever); err != nil {
		return nil, err
	}

	ret := make(map[string][]uint16)

	for _, bind := range r.remoteStateSnapshot.binds {
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
			Steps:    math.MaxInt,
			Cap:      5 * time.Minute,
		},
		func() (done bool, err error) {
			notReady := make([]types.Object, 0, len(toCreate))
			failed := make([]types.Object, 0, len(toCreate))

			for _, obj := range toCreate {
				state, ok := obj.(lbaasv1.StateRetriever)
				if !ok {
					r.logger.Error(nil, "Object does not have state", "object", mustStringifyObject(obj))
					return false, errors.New("coding error: waitForResources called for Object not implementing lbaasv1.StateRetriever")
				}

				// if we already retrieved Objects: shortcut
				if state.StateSuccess() && !firstPass {
					continue
				}

				err := r.api.Get(r.ctx, obj)
				if err != nil {
					r.logger.Error(err, "Error retrieving current state of Object, assuming it's failed", "object", mustStringifyObject(obj))
					failed = append(failed, obj)
					continue
				}

				if state.StateProgressing() {
					notReady = append(notReady, obj)
				} else if state.StateFailure() {
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

func (r *reconciliation) retrieveState(retriever stateRetriever) error {

	r.portBackends = make(map[string]*lbaasv1.Backend)
	r.portFrontends = make(map[string]*lbaasv1.Frontend)

	var err error
	r.remoteStateSnapshot, err = retriever.FilteredState(r.lb.Identifier)
	if err != nil {
		return err
	}

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
