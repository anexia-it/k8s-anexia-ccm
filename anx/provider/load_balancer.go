package provider

import (
	"context"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/loadbalancer"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/sync"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"strconv"
)

type loadBalancerManager struct {
	Provider
	notify chan struct{}

	rLock *sync.SubjectLock
}

func newLoadBalancerManager(provider Provider) loadBalancerManager {
	return loadBalancerManager{
		Provider: provider,
		notify:   nil,
		rLock:    sync.NewSubjectLock(),
	}
}

func (l *loadBalancerManager) GetLoadBalancer(ctx context.Context, clusterName string,
	service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	ctx = prepareContext(ctx)

	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	overallState := true
	portStatus := make([]v1.PortStatus, len(service.Spec.Ports))
	for i, svcPort := range service.Spec.Ports {

		lbGroup := loadbalancer.NewLoadBalancer(svcPort.Port, l.LBaaS(),
			loadbalancer.LoadBalancerID(l.Config().LoadBalancerIdentifier),
			logr.FromContextOrDiscard(ctx))

		portStatus[i].Port = svcPort.Port
		portStatus[i].Protocol = svcPort.Protocol

		// the lb name consists of the load balancer name and port
		lbFullName := fmt.Sprintf("%s.%s", strconv.Itoa(int(svcPort.Port)), lbName)
		isPresent, statusMessage := lbGroup.GetProvisioningState(ctx, lbFullName)
		overallState = overallState && isPresent

		if !isPresent {
			portStatus[i].Error = &statusMessage
		}
	}

	information, err := loadbalancer.GetHostInformation(ctx, l.LBaaS(), l.Config().LoadBalancerIdentifier)
	if err != nil {
		return nil, false, err
	}

	status, err := assembleLBStatus(ctx, information, portStatus)
	return status, overallState, err
}

func (l loadBalancerManager) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	if clusterName != "" {
		return fmt.Sprintf("%s.%s.%s", service.Name, service.Namespace, clusterName)
	}

	return fmt.Sprintf("%s.%s", service.Name, service.Namespace)
}

func (l loadBalancerManager) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service,
	nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {

	defer l.notifyOthers()
	ctx = prepareContext(ctx)

	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	l.rLock.Lock(lbName)
	defer l.rLock.Unlock(lbName)

	portStatus := make([]v1.PortStatus, len(service.Spec.Ports))
	for i, svcPort := range service.Spec.Ports {
		lbGroup := loadbalancer.NewLoadBalancer(svcPort.Port, l.LBaaS(),
			loadbalancer.LoadBalancerID(l.Config().LoadBalancerIdentifier),
			logr.FromContextOrDiscard(ctx))

		portStatus[i].Port = svcPort.Port
		portStatus[i].Protocol = svcPort.Protocol

		lbPortName := fmt.Sprintf("%s.%s", strconv.Itoa(int(svcPort.Port)), lbName)
		err := lbGroup.EnsureLBConfig(ctx, lbPortName, getNodeEndpoints(nodes, svcPort.NodePort))

		if err != nil {
			portError := err.Error()
			portStatus[i].Error = &portError
		}
	}

	information, err := loadbalancer.GetHostInformation(ctx, l.LBaaS(), l.Config().LoadBalancerIdentifier)
	if err != nil {
		return nil, err
	}
	status, err := assembleLBStatus(ctx, information, portStatus)
	if err != nil {
		return status, err
	}

	return status, nil
}

func assembleLBStatus(ctx context.Context, hostInformation loadbalancer.HostInformation,
	portStatus []v1.PortStatus) (*v1.LoadBalancerStatus, error) {

	status := &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP:       hostInformation.IP,
				Hostname: hostInformation.Hostname,
				Ports:    portStatus,
			},
		},
	}
	return status, nil
}

func (l loadBalancerManager) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	ctx = prepareContext(ctx)
	defer l.notifyOthers()

	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	l.rLock.Lock(lbName)
	defer l.rLock.Unlock(lbName)

	for _, svcPort := range service.Spec.Ports {
		lbGroup := loadbalancer.NewLoadBalancer(svcPort.Port, l.LBaaS(),
			loadbalancer.LoadBalancerID(l.Config().LoadBalancerIdentifier),
			logr.FromContextOrDiscard(ctx))

		// first we make sure that the base configuration for the lb exists
		lbPortName := fmt.Sprintf("%s.%s", strconv.Itoa(int(svcPort.Port)), lbName)
		err := lbGroup.EnsureLBConfig(ctx, lbPortName, getNodeEndpoints(nodes, svcPort.NodePort))
		if err != nil {
			return err
		}
	}

	return nil
}

func (l loadBalancerManager) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string,
	service *v1.Service) error {
	defer l.notifyOthers()
	ctx = prepareContext(ctx)

	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	l.rLock.Lock(lbName)
	defer l.rLock.Unlock(lbName)

	for _, svcPort := range service.Spec.Ports {
		lbGroup := loadbalancer.NewLoadBalancer(svcPort.Port, l.LBaaS(),
			loadbalancer.LoadBalancerID(l.Config().LoadBalancerIdentifier),
			logr.FromContextOrDiscard(ctx))

		lbPortName := fmt.Sprintf("%s.%s", strconv.Itoa(int(svcPort.Port)), lbName)
		err := lbGroup.EnsureLBDeleted(ctx, lbPortName)
		if err != nil {
			return err
		}
	}

	return nil
}

func prepareContext(ctx context.Context) context.Context {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		// logger is not set but we definitely need one
		logger = klogr.New()
		ctx = logr.NewContext(ctx, logger)
	}

	return ctx
}

func getNodeEndpoints(nodes []*v1.Node, port int32) []loadbalancer.NodeEndpoint {
	// in most cases every node will at least have one IP
	retAddresses := make([]loadbalancer.NodeEndpoint, 0, len(nodes))

	for _, node := range nodes {
		externalIP := getNodeAddressOfType(node, v1.NodeExternalIP)
		internalIP := getNodeAddressOfType(node, v1.NodeInternalIP)

		var nodeAddress string

		if internalIP != nil && internalIP.Address != "" {
			nodeAddress = internalIP.Address
		}

		// externalIP should be preferred
		if externalIP != nil && externalIP.Address != "" {
			nodeAddress = externalIP.Address
		}

		if nodeAddress != "" {
			retAddresses = append(retAddresses, loadbalancer.NodeEndpoint{
				IP:   nodeAddress,
				Port: port,
			})
		}
	}
	return retAddresses
}

func (l loadBalancerManager) GetIdentifiers() []string {
	identifiers := make([]string, 0, len(l.Config().SecondaryLoadBalancerIdentifiers)+1)
	identifiers = append(identifiers, l.Config().LoadBalancerIdentifier)
	identifiers = append(identifiers, l.Config().SecondaryLoadBalancerIdentifiers...)
	return identifiers
}

func (l loadBalancerManager) GetNotifyChannel() <-chan struct{} {
	return l.notify
}

func getNodeAddressOfType(node *v1.Node, addressType v1.NodeAddressType) *v1.NodeAddress {
	for _, address := range node.Status.Addresses {
		if address.Type == addressType {
			return &address
		}
	}
	return nil
}

func (l loadBalancerManager) notifyOthers() {
	go func() {
		select {
		case l.notify <- struct{}{}:
			klog.V(1).Info("trigger notification")
		default:
			klog.V(3).Info("notification is dropped because there are still pending events to be processed")
		}
	}()
}
