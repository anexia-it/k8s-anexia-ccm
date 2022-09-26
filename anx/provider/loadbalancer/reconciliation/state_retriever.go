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
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"
)

var (
	// errLoadBalancerNotRegistered is returned if a loadbalancer which was not initially registered
	// is called with FilteredState() or Done().
	// If this error is returned it is almost certainly a programming error within the CCM implementation (most likely inside this package).
	errLoadBalancerNotRegistered = errors.New("specified load balancer is not registered for stateRetriever")
)

// stateRetriever provides the remote state for multiple loadbalancers of a service
type stateRetriever interface {
	// LoadBalancerState returns the loadbalancer state filtered by loadbalancer ID
	LoadBalancerState(string) (*remoteLoadBalancerState, error)

	// Done is called to tell the StateRetriever that reconciliation has finished for a single load balancer
	// it MUST be called excactly once per load balancer
	Done(string) error
}

// remoteLoadBalancerState holds the state for a single loadbalancer
type remoteLoadBalancerState struct {
	frontends []*lbaasv1.Frontend
	backends  []*lbaasv1.Backend
	binds     []*lbaasv1.Bind
	servers   []*lbaasv1.Server

	// we store existing failed Objects here
	existingFailed []types.Object

	// and store existing Objects that are not yet ready here
	existingProgressing []types.Object
}

type stateRetrieverImpl struct {
	tags   []string
	api    api.API
	logger logr.Logger

	// holds state & sync objects for loadbalancers (lb identifier is key)
	loadBalancers map[string]*stateRetrieverLoadBalancerContainer
	// mutex for loadBalancers map access
	loadBalancersMapLock sync.Mutex

	// syncs update for all loadbalancers registerd to the state retriever
	loadBalancersReadyWaitGroup sync.WaitGroup
	err                         error
}

// stateRetrieverLoadBalancerContainer holds state & sync objects for a single loadbalancer
type stateRetrieverLoadBalancerContainer struct {
	lock           sync.Mutex
	updateComplete chan interface{}

	state *remoteLoadBalancerState
}

// newStateRetriever creates StateRetriever for service
func newStateRetriever(ctx context.Context, a api.API, serviceTag string, lbIdentifiers []string) stateRetriever {
	sr := &stateRetrieverImpl{
		api:           a,
		loadBalancers: make(map[string]*stateRetrieverLoadBalancerContainer),
		logger:        logr.FromContextOrDiscard(ctx),
		tags:          []string{serviceTag},
	}

	// initialize stateRetrieverLoadBalancerContainer for each loadbalancer
	for _, lbIdentifier := range lbIdentifiers {
		sr.loadBalancers[lbIdentifier] = &stateRetrieverLoadBalancerContainer{
			updateComplete: make(chan interface{}),
			state:          &remoteLoadBalancerState{},
		}
	}

	// add all registered loadbalancers to waitgroup (used to sync state retrieving)
	sr.loadBalancersReadyWaitGroup.Add(len(lbIdentifiers))

	// start internal update loop
	go sr.updateLoop(ctx)

	return sr
}

// Done unregisters a loadbalancer from the state retriever.
// The stateRetriever won't wait for the FilteredState(lbIdentifier) call in future state retrieving iterations.
func (r *stateRetrieverImpl) Done(lbIdentifier string) error {
	r.loadBalancersMapLock.Lock()
	defer r.loadBalancersMapLock.Unlock()

	if _, ok := r.loadBalancers[lbIdentifier]; !ok {
		return errLoadBalancerNotRegistered
	}

	close(r.loadBalancers[lbIdentifier].updateComplete)
	delete(r.loadBalancers, lbIdentifier)
	r.loadBalancersReadyWaitGroup.Done()

	return nil
}

// LoadBalancerState returns the loadbalancer state filtered by loadbalancer ID
func (r *stateRetrieverImpl) LoadBalancerState(lbIdentifier string) (*remoteLoadBalancerState, error) {
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

// updateLoop is an internal update loop called exactly once for each state retriever instance.
// It is responsible for waiting for all loadbalancers to be ready for the next state retrival,
// updating the state and finally start over until all loadbalancers are Done().
func (r *stateRetrieverImpl) updateLoop(ctx context.Context) {
	waitChannel := make(chan interface{})
	defer close(waitChannel)

	for {
		go func() { // make waitgroup "selectable" (allows updateLoop to be cancelable via ctx)
			r.loadBalancersReadyWaitGroup.Wait()
			waitChannel <- nil
		}()

		select {
		case <-waitChannel: // all loadbalancers ready or done
			break
		case <-ctx.Done(): // context expired
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

		// retrieve remote state
		r.update(ctx)

		r.loadBalancersReadyWaitGroup.Add(len(r.loadBalancers))

		for _, lb := range r.loadBalancers {
			lb.updateComplete <- nil
		}
		r.loadBalancersMapLock.Unlock()
	}
}

// update clears the previous loadbalancer states and retrieves the current engine state
func (r *stateRetrieverImpl) update(ctx context.Context) {
	for lb := range r.loadBalancers {
		r.loadBalancers[lb].state = &remoteLoadBalancerState{
			frontends: make([]*lbaasv1.Frontend, 0),
			backends:  make([]*lbaasv1.Backend, 0),
			binds:     make([]*lbaasv1.Bind, 0),
			servers:   make([]*lbaasv1.Server, 0),

			existingFailed:      make([]types.Object, 0),
			existingProgressing: make([]types.Object, 0),
		}
	}

	r.err = r.retrieveResources(ctx)
}

// getLoadBalancer returns the stateRetrieverLoadBalancerContainer for a lbIdentifier or an error if not present
func (r *stateRetrieverImpl) getLoadBalancer(lbIdentifier string) (*stateRetrieverLoadBalancerContainer, error) {
	r.loadBalancersMapLock.Lock()
	defer r.loadBalancersMapLock.Unlock()
	loadBalancer, ok := r.loadBalancers[lbIdentifier]
	if !ok {
		return nil, errLoadBalancerNotRegistered
	}
	return loadBalancer, nil
}

// setLoadBalancerState prevents setting a non-registered loadbalancer state by only calling the setter if the
// lbIdentifier is present
func (r *stateRetrieverImpl) setLoadBalancerState(lbIdentifier string, setter func(state *remoteLoadBalancerState)) {
	if lb, ok := r.loadBalancers[lbIdentifier]; ok {
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

type typedRetriever func(identifier string) error
type objectCreater[T types.Object] func(identifier string) T

// genericResourceRetriever returns a typed retriever used for lbaasv1.Backend and lbaasv1.Frontend
func genericResourceRetriever[T objectWithStateRetriever](ctx context.Context, r *stateRetrieverImpl, oc objectCreater[T]) typedRetriever {
	return func(identifier string) error {
		o := oc(identifier)

		if err := r.api.Get(ctx, o); err != nil {
			return err
		}

		var lbID string
		switch x := any(o).(type) {
		case *lbaasv1.Frontend:
			lbID = x.LoadBalancer.Identifier
		case *lbaasv1.Backend:
			lbID = x.LoadBalancer.Identifier
		}

		r.setLoadBalancerState(lbID, func(state *remoteLoadBalancerState) {
			switch x := any(o).(type) {
			case *lbaasv1.Frontend:
				state.frontends = append(state.frontends, x)
			case *lbaasv1.Backend:
				state.backends = append(state.backends, x)
			}
			r.sortObjectIntoStateArray(lbID, o)
		})

		return nil
	}
}

// genericResourceRetriever returns a typed retriever used for lbaasv1.Bind and lbaasv1.Server
func bindAndServerResourceRetriever[T types.Object](ctx context.Context, r *stateRetrieverImpl, all *[]T, oc objectCreater[T]) typedRetriever {
	return func(identifier string) error {
		o := oc(identifier)
		if err := r.api.Get(ctx, o); err != nil {
			return err
		}

		*all = append(*all, o)

		return nil
	}
}

func (r *stateRetrieverImpl) filterBindsAndServers(allBinds []*lbaasv1.Bind, allServers []*lbaasv1.Server) error {
	var err error
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

	return nil
}

func (r *stateRetrieverImpl) filterBinds(lbID string, allBinds []*lbaasv1.Bind) ([]*lbaasv1.Bind, error) {
	ret := make([]*lbaasv1.Bind, 0, len(allBinds))

	// Binds and Servers are filtered for our LoadBalancer here, after we hopefully retrieved their Frontends and Backends already
	for _, bind := range allBinds {
		idx, err := compare.Search(lbaasv1.Frontend{Identifier: bind.Frontend.Identifier}, r.loadBalancers[lbID].state.frontends, "Identifier")
		if err != nil {
			return nil, fmt.Errorf("error checking if Binds belongs to one of our frontends: %w", err)
		} else if idx != -1 {
			ret = append(ret, bind)
			r.sortObjectIntoStateArray(lbID, bind)
		}
	}

	return ret, nil
}

func (r *stateRetrieverImpl) filterServers(lbID string, allServers []*lbaasv1.Server) ([]*lbaasv1.Server, error) {
	ret := make([]*lbaasv1.Server, 0, len(allServers))

	for _, server := range allServers {
		idx, err := compare.Search(lbaasv1.Backend{Identifier: server.Backend.Identifier}, r.loadBalancers[lbID].state.backends, "Identifier")
		if err != nil {
			return nil, fmt.Errorf("error checking if Server belongs to one of our frontends: %w", err)
		} else if idx != -1 {
			ret = append(ret, server)
			r.sortObjectIntoStateArray(lbID, server)
		}
	}

	return ret, nil
}

func createTypedRetrievers(ctx context.Context, r *stateRetrieverImpl, allBinds *[]*lbaasv1.Bind, allServers *[]*lbaasv1.Server) map[string]typedRetriever {
	return map[string]typedRetriever{
		frontendResourceTypeIdentifier: genericResourceRetriever(ctx, r, func(identifier string) *lbaasv1.Frontend { return &lbaasv1.Frontend{Identifier: identifier} }),
		backendResourceTypeIdentifier:  genericResourceRetriever(ctx, r, func(identifier string) *lbaasv1.Backend { return &lbaasv1.Backend{Identifier: identifier} }),
		bindResourceTypeIdentifier:     bindAndServerResourceRetriever(ctx, r, allBinds, func(identifier string) *lbaasv1.Bind { return &lbaasv1.Bind{Identifier: identifier} }),
		serverResourceTypeIdentifier:   bindAndServerResourceRetriever(ctx, r, allServers, func(identifier string) *lbaasv1.Server { return &lbaasv1.Server{Identifier: identifier} }),
	}
}

// retrieveResource retrieves a single resource
func retrieveResource(ctx context.Context, retriever types.ObjectRetriever, typedRetrievers map[string]typedRetriever) error {
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

	return nil
}

// retrieveResources retrieves all resources for a service
func (r *stateRetrieverImpl) retrieveResources(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var oc types.ObjectChannel
	err := r.api.List(ctx, &corev1.Resource{Tags: r.tags}, api.ObjectChannel(&oc), api.FullObjects(true))
	if err != nil {
		var he api.HTTPError

		var retErr error
		// error 422 is returned when nothing is tagged with the searched-for tag
		if !(errors.As(err, &he) && he.StatusCode() == 422) {
			retErr = fmt.Errorf("error retrieving resources: %w", err)
		}

		return retErr
	}

	allBinds := make([]*lbaasv1.Bind, 0)
	allServers := make([]*lbaasv1.Server, 0)

	typedRetrievers := createTypedRetrievers(ctx, r, &allBinds, &allServers)

	// frontends and backends are filtered for our LoadBalancer here already
	for retriever := range oc {
		if err := retrieveResource(ctx, retriever, typedRetrievers); err != nil {
			return fmt.Errorf("error retrieving resource: %w", err)
		}
	}

	if err := r.filterBindsAndServers(allBinds, allServers); err != nil {
		return fmt.Errorf("error filtering binds and servers: %w", err)
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
