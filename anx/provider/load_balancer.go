package provider

import (
	"context"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/loadbalancer"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2/klogr"
	"strconv"
)

type loadBalancerManager struct {
	Provider
}

func (l loadBalancerManager) GetLoadBalancer(ctx context.Context, clusterName string,
	service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	ctx, err := prepareContext(ctx, l)
	if err != nil {
		return nil, false, err
	}

	lbGroup := getLBFromContext(ctx)
	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	overallState := true
	portStatus := make([]v1.PortStatus, len(service.Spec.Ports))
	for i, svcPort := range service.Spec.Ports {
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

	status, err := assembleLBStatus(ctx, lbGroup, portStatus)
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
	ctx, err := prepareContext(ctx, l)
	if err != nil {
		return nil, fmt.Errorf("could not prepare context: %w", err)
	}

	lbGroup := getLBFromContext(ctx)
	lbName := l.GetLoadBalancerName(ctx, clusterName, service)

	portStatus := make([]v1.PortStatus, len(service.Spec.Ports))
	for i, svcPort := range service.Spec.Ports {
		portStatus[i].Port = svcPort.Port
		portStatus[i].Protocol = svcPort.Protocol

		lbPortName := fmt.Sprintf("%s.%s", strconv.Itoa(int(svcPort.Port)), lbName)
		err = lbGroup.EnsureLBConfig(ctx, lbPortName, getNodeEndpoints(nodes, svcPort.NodePort))

		if err != nil {
			portError := err.Error()
			portStatus[i].Error = &portError
		}
	}

	status, err := assembleLBStatus(ctx, lbGroup, portStatus)
	if err != nil {
		return status, err
	}

	return status, nil
}

func assembleLBStatus(ctx context.Context, lbGroup *loadbalancer.LoadBalancer, portStatus []v1.PortStatus) (*v1.LoadBalancerStatus, error) {
	hostInformation, err := lbGroup.GetHostInformation(ctx)
	if err != nil {
		return nil, err
	}

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
	ctx, err := prepareContext(ctx, l)
	if err != nil {
		return fmt.Errorf("could not prepare context: %w", err)
	}

	lbGroup := getLBFromContext(ctx)
	lbName := l.GetLoadBalancerName(ctx, clusterName, service)
	for _, svcPort := range service.Spec.Ports {
		// first we make sure that the base configuration for the lb exists
		lbPortName := fmt.Sprintf("%s-%s", lbName, strconv.Itoa(int(svcPort.Port)))
		err = lbGroup.EnsureLBConfig(ctx, lbPortName, getNodeEndpoints(nodes, svcPort.NodePort))
		if err != nil {
			return err
		}
	}

	return nil
}

func (l loadBalancerManager) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string,
	service *v1.Service) error {
	ctx, err := prepareContext(ctx, l)
	if err != nil {
		return err
	}

	lb := getLBFromContext(ctx)
	lbName := l.GetLoadBalancerName(ctx, clusterName, service)
	for _, svcPort := range service.Spec.Ports {
		lbPortName := fmt.Sprintf("%s.%s", strconv.Itoa(int(svcPort.Port)), lbName)
		err := lb.EnsureLBDeleted(ctx, lbPortName)
		if err != nil {
			return err
		}
	}

	return lb.EnsureLBDeleted(ctx, lbName)
}

type lbManagerContextKey struct{}

func prepareContext(ctx context.Context, l loadBalancerManager) (context.Context, error) {
	// we already have a load balancer group in this context
	if getLBFromContext(ctx) != nil {
		return ctx, nil
	}

	logger, err := logr.FromContext(ctx)
	if err != nil {
		// logger is not set but we definitely need one
		logger = klogr.New()
		ctx = logr.NewContext(ctx, logger)
	}
	identifier := l.Config().LoadBalancerIdentifier

	group := loadbalancer.NewLoadBalancer(l.LBaaS(),
		loadbalancer.LoadBalancerID(identifier),
		logger)

	return context.WithValue(ctx, lbManagerContextKey{}, &group), nil
}

func getLBFromContext(ctx context.Context) *loadbalancer.LoadBalancer {
	group, ok := ctx.Value(lbManagerContextKey{}).(*loadbalancer.LoadBalancer)
	if !ok {
		return nil
	}
	return group
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

		// externalIP should be preffered
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

func getNodeAddressOfType(node *v1.Node, addressType v1.NodeAddressType) *v1.NodeAddress {
	for _, address := range node.Status.Addresses {
		if address.Type == addressType {
			return &address
		}
	}
	return nil
}
