package sync

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	managerApp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

type syncController struct {
	clientName string
	client     kubernetes.Interface
	provider   LoadBalancerReplicationProvider
	manager    LoadBalancerReplicationManager
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
}

type LoadBalancerReplicationProvider interface {
	cloudprovider.Interface
	Replication() (LoadBalancerReplicationManager, bool)
}

func startSyncController(ctx app.ControllerInitContext, stop <-chan struct{},
	config *cloudcontrollerconfig.CompletedConfig,
	cloud cloudprovider.Interface) (controller controller.Interface, enabled bool, err error) {

	logr.NewContext(context.Background(), klogr.New().WithName(ctx.ClientName))

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
		clientName: ctx.ClientName,
		client:     client,
		provider:   replicationProvider,
		manager:    replicationManager,
	}

	go syncController.Run(stop)

	return nil, true, nil
}

func (s *syncController) Run(stopChan <-chan struct{}) {
	<-stopChan
}
