package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

// stateRetriever provides the remote state for multiple loadbalancers of a service
type stateRetriever interface {
	// FilteredState returns the loadbalancer state filtered by loadbalancer ID
	FilteredState(context.Context, string) (*remoteLoadBalancerState, error)

	// Done tells the StateRetriever that a reconciliation has finished for a single loadbalancer (also due to errors)
	Done(string) error
}

type remoteLoadBalancerState struct {
	frontends []*lbaasv1.Frontend
	backends  []*lbaasv1.Backend
	binds     []*lbaasv1.Bind
	servers   []*lbaasv1.Server

	// we store existing failed Objects here
	existingFailed []types.Object

	// and store existing Objects that are not yet ready here
	existingProgressing []types.Object

	publicAddresses []string
}

type lbSyncMeta struct {
	lock           sync.Mutex
	updateComplete chan bool
}

type stateRetrieverImpl struct {
	tags   []string
	api    api.API
	logger logr.Logger

	state map[string]*remoteLoadBalancerState

	lbSyncMetaMap     map[string]*lbSyncMeta
	lbSyncMetaMapLock sync.Mutex

	allLBsReadyToReceiveNew sync.WaitGroup
	err                     error
}

// newStateRetriever creates StateRetriever for service
func newStateRetriever(ctx context.Context, a api.API, serviceUID string, lbCount int) stateRetriever {
	sr := &stateRetrieverImpl{
		api:           a,
		lbSyncMetaMap: make(map[string]*lbSyncMeta),
		state:         make(map[string]*remoteLoadBalancerState),
		logger:        logr.FromContextOrDiscard(ctx),

		tags: []string{
			fmt.Sprintf("anxccm-svc-uid=%v", serviceUID),
		},
	}

	sr.allLBsReadyToReceiveNew.Add(lbCount)

	go sr.start(ctx)

	return sr
}

func (r *stateRetrieverImpl) Done(lbID string) error {
	r.lbSyncMetaMapLock.Lock()
	defer r.lbSyncMetaMapLock.Unlock()

	r.allLBsReadyToReceiveNew.Done()
	delete(r.lbSyncMetaMap, lbID)
	return nil
}

func (r *stateRetrieverImpl) FilteredState(ctx context.Context, lbID string) (*remoteLoadBalancerState, error) {
	r.lbSyncMetaMapLock.Lock()
	syncMeta, ok := r.lbSyncMetaMap[lbID]
	if !ok {
		syncMeta = &lbSyncMeta{
			lock:           sync.Mutex{},
			updateComplete: make(chan bool),
		}
		r.lbSyncMetaMap[lbID] = syncMeta
	}
	r.lbSyncMetaMapLock.Unlock()

	syncMeta.lock.Lock()
	defer func() {
		syncMeta.lock.Unlock()
	}()

	r.allLBsReadyToReceiveNew.Done()

	<-syncMeta.updateComplete

	if r.err != nil {
		return nil, r.err
	}

	return r.state[lbID], nil
}

func (r *stateRetrieverImpl) start(ctx context.Context) {
	for {
		r.allLBsReadyToReceiveNew.Wait()

		if len(r.lbSyncMetaMap) == 0 {
			// all done
			return
		}

		r.update(ctx)

		r.allLBsReadyToReceiveNew.Add(len(r.lbSyncMetaMap))

		for _, lbMeta := range r.lbSyncMetaMap {
			lbMeta.updateComplete <- true
		}
	}
}

func (r *stateRetrieverImpl) update(ctx context.Context) {
	r.state = make(map[string]*remoteLoadBalancerState)
	for lb := range r.lbSyncMetaMap {
		r.state[lb] = &remoteLoadBalancerState{
			frontends: make([]*lbaasv1.Frontend, 0),
			backends:  make([]*lbaasv1.Backend, 0),
			binds:     make([]*lbaasv1.Bind, 0),
			servers:   make([]*lbaasv1.Server, 0),

			existingFailed:      make([]types.Object, 0),
			existingProgressing: make([]types.Object, 0),

			publicAddresses: make([]string, 0),
		}
	}

	r.err = r.retrieveResources(ctx)
}

func (r *stateRetrieverImpl) isRegisteredLB(lbID string) bool {
	_, ok := r.state[lbID]
	return ok
}

func (r *stateRetrieverImpl) sortObjectIntoStateArray(lbID string, o types.Object) {
	if !r.isRegisteredLB(lbID) {
		return
	}

	sr, ok := o.(lbaasv1.StateRetriever)
	if !ok {
		return
	}

	if sr.StateFailure() {
		r.state[lbID].existingFailed = append(r.state[lbID].existingFailed, o)
	} else if sr.StateProgressing() {
		r.state[lbID].existingProgressing = append(r.state[lbID].existingProgressing, o)
	}
}

func (r *stateRetrieverImpl) retrieveResources(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
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

		frontendResourceTypeIdentifier: func(identifier string) error {
			frontend := &lbaasv1.Frontend{Identifier: identifier}
			if err = r.api.Get(ctx, frontend); err != nil {
				return err
			}

			lbID := frontend.LoadBalancer.Identifier
			if !r.isRegisteredLB(lbID) {
				return nil
			}
			r.state[lbID].frontends = append(r.state[lbID].frontends, frontend)
			r.sortObjectIntoStateArray(lbID, frontend)

			return nil
		},

		backendResourceTypeIdentifier: func(identifier string) error {
			backend := &lbaasv1.Backend{Identifier: identifier}
			if err = r.api.Get(ctx, backend); err != nil {
				return err
			}

			lbID := backend.LoadBalancer.Identifier
			if !r.isRegisteredLB(lbID) {
				return nil
			}
			r.state[lbID].backends = append(r.state[lbID].backends, backend)
			r.sortObjectIntoStateArray(lbID, backend)

			return nil
		},

		bindResourceTypeIdentifier: func(identifier string) error {
			bind := &lbaasv1.Bind{Identifier: identifier}
			if err = r.api.Get(ctx, bind); err != nil {
				return err
			}

			allBinds = append(allBinds, bind)

			return nil
		},

		serverResourceTypeIdentifier: func(identifier string) error {
			server := &lbaasv1.Server{Identifier: identifier}
			if err = r.api.Get(ctx, server); err != nil {
				return err
			}

			allServers = append(allServers, server)

			return nil
		},
	}

	for retriever := range oc {
		var res corev1.Resource
		if err := retriever(&res); err != nil {
			return fmt.Errorf("error retrieving resource: %w", err)
		}

		logger := logr.FromContextOrDiscard(ctx).WithValues(
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

	for lbID := range r.lbSyncMetaMap {
		r.state[lbID].binds, err = r.filterBinds(lbID, allBinds)
		if err != nil {
			return err
		}

		r.state[lbID].servers, err = r.filterServers(lbID, allServers)
		if err != nil {
			return err
		}
	}

	for lbID := range r.lbSyncMetaMap {
		r.logger.V(1).Info(
			fmt.Sprintf("retrieved resources for lb %q", lbID),
			"num-frontends", len(r.state[lbID].frontends),
			"num-binds", len(r.state[lbID].binds),
			"num-backends", len(r.state[lbID].backends),
			"num-servers", len(r.state[lbID].servers),
		)
	}

	return nil
}
