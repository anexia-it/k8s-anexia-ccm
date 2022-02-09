package sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync/components"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/controller/lbaas/sync/replication"
	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	managerApp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sync"
	"time"
)

type syncController struct {
	clientName string
	client     kubernetes.Interface
	provider   LoadBalancerReplicationProvider
	manager    LoadBalancerReplicationManager
	ctx        context.Context
}

var LoadBalancerSyncConstructor = func(initcontext app.ControllerInitContext,
	completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {

	return func(ctx managerApp.ControllerContext) (controller controller.Interface, enabled bool, err error) {
		return startSyncController(initcontext, ctx.Stop, completedConfig, cloud)
	}
}

type LoadBalancerReplicationManager interface {
	// GetIdentifiers is supposed to return a slice of load balancer identifiers.
	// the first identifier is considered to be the primary load balancer, while the others are considered
	// to be replication targets
	GetIdentifiers() []string

	// GetNotifyChannel returns a channel that is used by the controller to get notified whenever something was changed
	// that requires a new sync
	GetNotifyChannel() <-chan struct{}
}

type LoadBalancerReplicationProvider interface {
	cloudprovider.Interface
	Replication() (LoadBalancerReplicationManager, bool)
}

func startSyncController(ctx app.ControllerInitContext, stop <-chan struct{},
	config *cloudcontrollerconfig.CompletedConfig,
	cloud cloudprovider.Interface) (controller controller.Interface, enabled bool, err error) {

	controllerContext := logr.NewContext(context.Background(), klogr.New().WithName(ctx.ClientName))

	replicationProvider, ok := cloud.(LoadBalancerReplicationProvider)
	if !ok {
		klog.Info("The provider does not support lbaas replication. Skipping lbaas replication")
		return nil, false, nil
	}

	replicationManager, ok := replicationProvider.Replication()
	if !ok {
		klog.Info("The provider does not support lbaas replication. Skipping lbaas replication")
	}

	client := config.ClientBuilder.ClientOrDie(ctx.ClientName)
	syncController := &syncController{
		ctx:        controllerContext,
		clientName: ctx.ClientName,
		client:     client,
		provider:   replicationProvider,
		manager:    replicationManager,
	}

	// start controller until we get stopped and restart on any errors
	go func() {
		defer func() {
			logr.FromContextOrDiscard(controllerContext).Info("controller stopped")
		}()

		logr.FromContextOrDiscard(controllerContext).Info("starting initial config sync")
		identifiers := replicationManager.GetIdentifiers()
		_ = syncController.runSync(identifiers[0], identifiers[1:]...)
	loop:
		for {
			select {
			case _, ok := <-stop:
				if !ok {
					break loop
				}
			default:
				err := syncController.Run(stop)
				if err != nil {
					logr.FromContextOrDiscard(controllerContext).Error(err, "error when executing controller")
				}
				time.Sleep(10 * time.Second)
			}

		}
	}()

	return nil, true, nil
}

func (s *syncController) Run(stopChan <-chan struct{}) error {
	notify := s.manager.GetNotifyChannel()
	if notify == nil {
		return errors.New("LoadBalancerReplicationManager is not supposed to return a nil channel")
	}

	identifiers := s.manager.GetIdentifiers()
	if len(identifiers) < 1 {
		return errors.New("at least two load balancer ID's are required for the replication controller")
	}

	select {
	case _, ok := <-stopChan:
		if !ok {
			return nil
		}
	case _, ok := <-notify:
		emptyChan(notify)
		if !ok {
			return errors.New("unexpected close of notify channel by the LoadBalancerReplicationManager")
		}
		err := s.runSync(identifiers[0], identifiers[1:]...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *syncController) runSync(sourceLB string, targetLBs ...string) (ctrlError error) {
	defer func() {
		p := recover()
		if p != nil {
			ctrlError = fmt.Errorf("panic during controller execution: %v", p)
		}
	}()

	logger := logr.FromContextOrDiscard(s.ctx)

	anxClient, err := api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
	if err != nil {
		return err
	}
	var config components.HashedLoadBalancer
	var waitGroup sync.WaitGroup

	waitGroup.Add(1)

	ctx := logr.NewContext(context.Background(), logger)

	go func() {
		defer waitGroup.Done()
		var err error
		config, err = replication.FetchLoadBalancer(ctx, sourceLB, anxClient)
		if err != nil {
			logger.Error(err, "could not fetch source loadbalancers lb config")
			panic(err)
		}
	}()

	waitGroup.Wait()

	waitGroup.Add(len(targetLBs))
	for _, val := range targetLBs {
		go func(targetLB string) {
			defer waitGroup.Done()
			target, err := replication.FetchLoadBalancer(ctx, targetLB, anxClient)
			if err != nil {
				logger.Error(fmt.Errorf("could not fetch data from target loadbalancer %s: %w", targetLB, err),
					"could not fetch load balancer data")
			}
			err = replication.SyncLoadBalancer(ctx, anxClient, config, target)
			if err != nil {
				panic(err)
			}
			logger.Info("load balancer successfully synced", "load-balancer", target.Identifier)
		}(val)
	}
	waitGroup.Wait()

	return nil
}

func emptyChan(notify <-chan struct{}) {
loop:
	for {
		select {
		case <-notify:
			continue
		default:
			break loop
		}
	}
}
