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

var (
	errLoadBalancerNotRegistered = errors.New("specified load balancer is not registered for stateRetriever")
)

// stateRetriever provides the remote state for multiple loadbalancers of a service
type stateRetriever interface {
	// FilteredState returns the loadbalancer state filtered by loadbalancer ID
	FilteredState(string) (*remoteLoadBalancerState, error)

	// Done is called to tell the StateRetriever that reconciliation has finished for a single load balancer
	// it MUST be called excactly once per load balancer
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

type stateRetrieverImpl struct {
	tags   []string
	api    api.API
	logger logr.Logger

	loadBalancers        map[string]*stateRetrieverLoadBalancerContainer
	loadBalancersMapLock sync.Mutex

	loadBalancersReadyWaitGroup sync.WaitGroup
	err                         error
}

type stateRetrieverLoadBalancerContainer struct {
	lock           sync.Mutex
	updateComplete chan interface{}

	state *remoteLoadBalancerState
}

// newStateRetriever creates StateRetriever for service
func newStateRetriever(ctx context.Context, a api.API, serviceUID string, lbIdentifiers []string) stateRetriever {
	sr := &stateRetrieverImpl{
		api:           a,
		loadBalancers: make(map[string]*stateRetrieverLoadBalancerContainer),
		logger:        logr.FromContextOrDiscard(ctx),
		tags:          []string{fmt.Sprintf("anxccm-svc-uid=%v", serviceUID)},
	}

	for _, lbIdentifier := range lbIdentifiers {
		sr.loadBalancers[lbIdentifier] = &stateRetrieverLoadBalancerContainer{
			updateComplete: make(chan interface{}),
			state:          &remoteLoadBalancerState{},
		}
	}

	sr.loadBalancersReadyWaitGroup.Add(len(lbIdentifiers))

	go sr.updateLoop(ctx)

	return sr
}

func (r *stateRetrieverImpl) Done(lbID string) error {
	r.loadBalancersMapLock.Lock()
	defer r.loadBalancersMapLock.Unlock()

	if _, ok := r.loadBalancers[lbID]; !ok {
		return errLoadBalancerNotRegistered
	}

	close(r.loadBalancers[lbID].updateComplete)
	delete(r.loadBalancers, lbID)
	r.loadBalancersReadyWaitGroup.Done()

	return nil
}

// FilteredState returns the loadbalancer state filtered by loadbalancer ID
func (r *stateRetrieverImpl) FilteredState(lbIdentifier string) (*remoteLoadBalancerState, error) {
	loadBalancer, err := r.getLoadBalancer(lbIdentifier)
	if err != nil {
		return nil, err
	}

	// lock FilteredState for lbIdentifier
	loadBalancer.lock.Lock()
	defer loadBalancer.lock.Unlock()

	r.loadBalancersReadyWaitGroup.Done()

	// wait until all registered lb's called FilteredState
	<-loadBalancer.updateComplete

	if r.err != nil {
		return nil, r.err
	}

	return loadBalancer.state, nil
}

func (r *stateRetrieverImpl) updateLoop(ctx context.Context) {
	waitChannel := make(chan interface{})
	defer close(waitChannel)

	for {
		go func() { // make waitgroup "selectable" (allows updateLoop to be cancelable via ctx)
			r.loadBalancersReadyWaitGroup.Wait()
			waitChannel <- nil
		}()

		select {
		case <-waitChannel:
			break
		case <-ctx.Done():
			r.err = ctx.Err()
			logr.FromContextOrDiscard(ctx).Error(ctx.Err(), "LoadBalancer state retriever for didn't finish properly")
			for _, lb := range r.loadBalancers {
				close(lb.updateComplete)
			}
			return
		}

		r.loadBalancersMapLock.Lock()
		if len(r.loadBalancers) == 0 {
			// all done
			return
		}

		r.update(ctx)

		r.loadBalancersReadyWaitGroup.Add(len(r.loadBalancers))

		for _, lb := range r.loadBalancers {
			lb.updateComplete <- nil
		}
		r.loadBalancersMapLock.Unlock()
	}
}

func (r *stateRetrieverImpl) update(ctx context.Context) {
	for lb := range r.loadBalancers {
		r.loadBalancers[lb].state = &remoteLoadBalancerState{
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

func (r *stateRetrieverImpl) getLoadBalancer(lbIdentifier string) (*stateRetrieverLoadBalancerContainer, error) {
	r.loadBalancersMapLock.Lock()
	defer r.loadBalancersMapLock.Unlock()
	loadBalancer, ok := r.loadBalancers[lbIdentifier]
	if !ok {
		return nil, errLoadBalancerNotRegistered
	}
	return loadBalancer, nil
}

func (r *stateRetrieverImpl) setLoadBalancerState(lbID string, setter func(state *remoteLoadBalancerState)) {
	if lb, ok := r.loadBalancers[lbID]; ok {
		setter(lb.state)
	}
}

type objectWithStateRetriever interface {
	types.Object
	lbaasv1.StateRetriever
}

func (r *stateRetrieverImpl) sortObjectIntoStateArray(lbID string, o objectWithStateRetriever) {
	r.setLoadBalancerState(lbID, func(state *remoteLoadBalancerState) {
		if o.StateFailure() {
			state.existingFailed = append(r.loadBalancers[lbID].state.existingFailed, o)
		} else if o.StateProgressing() {
			state.existingProgressing = append(r.loadBalancers[lbID].state.existingProgressing, o)
		}
	})
}

type fetcher func(identifier string) error

func (r *stateRetrieverImpl) frontendResourceFetcher(ctx context.Context) fetcher {
	return func(identifier string) error {
		frontend := &lbaasv1.Frontend{Identifier: identifier}
		if err := r.api.Get(ctx, frontend); err != nil {
			return err
		}

		lbID := frontend.LoadBalancer.Identifier

		r.setLoadBalancerState(lbID, func(state *remoteLoadBalancerState) {
			state.frontends = append(state.frontends, frontend)
			r.sortObjectIntoStateArray(lbID, frontend)
		})

		return nil
	}
}

func (r *stateRetrieverImpl) backendResourceFetcher(ctx context.Context) fetcher {
	return func(identifier string) error {
		backend := &lbaasv1.Backend{Identifier: identifier}
		if err := r.api.Get(ctx, backend); err != nil {
			return err
		}

		lbID := backend.LoadBalancer.Identifier
		r.setLoadBalancerState(lbID, func(state *remoteLoadBalancerState) {
			state.backends = append(state.backends, backend)
			r.sortObjectIntoStateArray(lbID, backend)
		})

		return nil
	}
}

func (r *stateRetrieverImpl) bindResourceFetcher(ctx context.Context, allBinds []*lbaasv1.Bind) fetcher {
	return func(identifier string) error {
		bind := &lbaasv1.Bind{Identifier: identifier}
		if err := r.api.Get(ctx, bind); err != nil {
			return err
		}

		allBinds = append(allBinds, bind)

		return nil
	}
}

func (r *stateRetrieverImpl) serverResourceFetcher(ctx context.Context, allServers []*lbaasv1.Server) fetcher {
	return func(identifier string) error {
		server := &lbaasv1.Server{Identifier: identifier}
		if err := r.api.Get(ctx, server); err != nil {
			return err
		}

		allServers = append(allServers, server)

		return nil
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

		frontendResourceTypeIdentifier: r.frontendResourceFetcher(ctx),
		backendResourceTypeIdentifier:  r.backendResourceFetcher(ctx),
		bindResourceTypeIdentifier:     r.bindResourceFetcher(ctx, allBinds),
		serverResourceTypeIdentifier:   r.serverResourceFetcher(ctx, allServers),
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

	for lbID := range r.loadBalancers {
		r.loadBalancers[lbID].state.binds, err = r.filterBinds(lbID, allBinds)
		if err != nil {
			return err
		}

		r.loadBalancers[lbID].state.servers, err = r.filterServers(lbID, allServers)
		if err != nil {
			return err
		}
	}

	for lbID, lb := range r.loadBalancers {
		r.logger.V(1).Info(
			fmt.Sprintf("retrieved resources for lb %q", lbID),
			"num-frontends", len(lb.state.frontends),
			"num-binds", len(lb.state.binds),
			"num-backends", len(lb.state.backends),
			"num-servers", len(lb.state.servers),
		)
	}

	return nil
}
