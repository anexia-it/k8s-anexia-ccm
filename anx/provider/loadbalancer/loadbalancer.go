// Package loadbalancer wraps Anexias LBaaS service to an interface more suitable for K8s LoadBalancer usage
package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/loadbalancer/address"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/loadbalancer/discovery"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/loadbalancer/reconciliation"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
)

type mgr struct {
	logger       logr.Logger
	api          api.API
	legacyClient client.Client
	clusterName  string
	k8s          kubernetes.Interface

	addressManager address.Manager

	loadBalancers []string
	sync          *sync.Mutex

	backoffSteps int

	metrics metrics.ProviderMetrics
}

var (
	// ErrNoLoadBalancers is returned when no LBaaS LoadBalancer is configured or found via AutoDiscovery
	ErrNoLoadBalancers = errors.New("no LoadBalancers configured or found via AutoDiscovery")

	// ErrPortNameNotUnique is returned when asked to reconcile a service with non-unique port names
	ErrPortNameNotUnique = errors.New("port name not unique")

	// ErrNoUsableNodeAddress is returned when asked to reconcile a Service for set of Nodes from which at least one does not have a usable address.
	ErrNoUsableNodeAddress = errors.New("Node lacks usable address")

	// ErrSingleVIPConflict is returned when asked to provision a LoadBalancer service while another already uses the single load balancer IP usable for Anexia Kubernetes Service beta.
	ErrSingleVIPConflict = errors.New("only a single LoadBalancer can be used in Anexia Kubernetes Service beta, but found another service using the external IP already")

	// If this annotation is set, it is used instead of the external IP(s). This must be a valid hostname, otherwise it will cause issues.
	// This is to prevent hair-pinning by Kubernetes, when accessing internal services via their external URL.
	//
	// Said workaround can likely be deprecated with Kubernetes 1.32 and [KEP #1860].
	//
	// [KEP #1860]: https://github.com/kubernetes/enhancements/issues/1860
	AKEAnnotationHostname = "lbaas.anx.io/load-balancer-proxy-pass-hostname"
)

// New creates a new LoadBalancer manager for the given Anexia generic client, cluster name and identifier of the
// LBaaS LoadBalancer resource identifier to add kubernetes services to (LBaaS LoadBalancers are machines
// serving many Kubernetes LoadBalancer Services).
//
// The given overrideClusterName can be given for cases were the kubernetes controller-manager does not know it
// and there are multiple clusters running in the same Anexia customer, resulting in possibly colliding resources.
func New(config *configuration.ProviderConfig, logger logr.Logger, k8sClient kubernetes.Interface, apiClient api.API, legacyClient client.Client, providerMetrics metrics.ProviderMetrics) (cloudprovider.LoadBalancer, error) {
	m := mgr{
		api:          apiClient,
		legacyClient: legacyClient,
		k8s:          k8sClient,
		logger:       logger,
		sync:         &sync.Mutex{},
		metrics:      providerMetrics,
		backoffSteps: config.LoadBalancerBackoffSteps,
	}

	m.clusterName = config.ClusterName

	ctx := logr.NewContext(context.TODO(), logger)

	if err := m.configureLoadBalancers(ctx, config); err != nil {
		return nil, fmt.Errorf("error configuring LoadBalancers: %w", err)
	}

	if err := m.configurePrefixes(ctx, config); err != nil {
		return nil, fmt.Errorf("error configuring LoadBalancer Prefixes: %w", err)
	}

	return &m, nil
}

// handleRateLimitError is converting a rate limit error returned by the Anexia Engine into
// an [cloudproviderapi.RetryError]. This is only effective in the [EnsureLoadBalancer]
// method, where the rate limiting was most noticeable.
func handleRateLimitError(err error) error {
	var rateLimitErr api.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return cloudproviderapi.NewRetryError("rate limiting by engine", time.Until(rateLimitErr.RetryAfter))
	}

	return err
}

func (m mgr) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	_, clusterName = m.prepare(ctx, clusterName, service)
	return strings.Join([]string{service.Name, service.Namespace, clusterName}, ".")
}

func (m mgr) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	ctx, clusterName = m.prepare(ctx, clusterName, service)

	recon, externalAddresses, err := m.reconciliationForService(ctx, clusterName, service, []*v1.Node{})
	if err != nil {
		return nil, false, err
	}

	reconStatus, err := recon.Status()
	if err != nil {
		return nil, false, err
	}

	status := lbStatusFromReconcileStatus(reconStatus, service)

	created := true
	for _, ea := range externalAddresses {
		ports, ok := reconStatus[ea.String()]
		if !ok {
			created = false
			break
		}

		for _, port := range service.Spec.Ports {
			portFound := false

			for _, createdPort := range ports {
				if int32(createdPort) == port.Port {
					portFound = true
					break
				}
			}

			if !portFound {
				created = false
				break
			}
		}
	}

	return status, created, nil
}

func (m mgr) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	m.sync.Lock()
	defer m.sync.Unlock()

	ctx, clusterName = m.prepare(ctx, clusterName, service)

	recon, _, err := m.reconciliationForService(ctx, clusterName, service, nodes)
	if err != nil {
		return nil, handleRateLimitError(err)
	}

	if err := recon.Reconcile(); err != nil {
		return nil, handleRateLimitError(err)
	}

	status, err := m.reconciliationStatus(recon, service)
	if err != nil {
		return nil, handleRateLimitError(err)
	}

	return status, nil
}

func (m mgr) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	_, err := m.EnsureLoadBalancer(ctx, clusterName, service, nodes)
	return err
}

func (m mgr) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	_, err := m.EnsureLoadBalancer(ctx, clusterName, service, []*v1.Node{})
	return err
}

func (m *mgr) configureLoadBalancers(ctx context.Context, config *configuration.ProviderConfig) error {
	if config.AutoDiscoverLoadBalancer {
		tag := fmt.Sprintf("%s-%s", config.AutoDiscoveryTagPrefix, m.clusterName)
		lbs, err := discovery.DiscoverLoadBalancers(ctx, m.api, tag)
		if err != nil {
			return err
		}

		m.loadBalancers = lbs
	} else {
		m.loadBalancers = []string{config.LoadBalancerIdentifier}
	}

	if len(m.loadBalancers) == 0 {
		return ErrNoLoadBalancers
	}

	return nil
}

func (m *mgr) configurePrefixes(ctx context.Context, config *configuration.ProviderConfig) error {
	if prefixes := config.LoadBalancerPrefixIdentifiers; len(prefixes) > 0 {
		am, err := address.NewWithPrefixes(ctx, m.api, m.legacyClient, prefixes)
		if err != nil {
			return err
		}

		m.addressManager = am
	} else if config.AutoDiscoverLoadBalancer {
		m.addressManager = address.NewWithAutoDiscovery(ctx, m.api, m.legacyClient, m.clusterName)
	}

	return nil
}

// prepare extends the context with a logger and checks if the cluster name is overriden for this manager.
func (m mgr) prepare(ctx context.Context, clusterName string, svc *v1.Service) (context.Context, string) {
	logger := m.logger.WithValues(
		"service-uid", svc.ObjectMeta.UID,
		"service-name", svc.Name,
		"service-namespace", svc.Namespace,
		"cluster-name", m.clusterName,
	)

	if m.clusterName != "" {
		logger = logger.WithValues(
			"k8s-cluster-name", clusterName,
		)
	}

	return logr.NewContext(ctx, logger), m.clusterName
}

func (m mgr) reconciliationStatus(recon reconciliation.Reconciliation, service *v1.Service) (*v1.LoadBalancerStatus, error) {
	status, err := recon.Status()
	if err != nil {
		return nil, err
	}

	m.logger.V(2).Info("Reconcilation completed", "recon-status", status)

	return lbStatusFromReconcileStatus(status, service), nil
}

func (m mgr) reconciliationForService(ctx context.Context, clusterName string, svc *v1.Service, nodes []*v1.Node) (reconciliation.Reconciliation, []net.IP, error) {
	var ports map[string]reconciliation.Port
	var servers []reconciliation.Server
	var externalAddresses []net.IP

	if svc.DeletionTimestamp == nil {
		ports = make(map[string]reconciliation.Port, len(svc.Spec.Ports))
		for _, port := range svc.Spec.Ports {
			if prevPort, ok := ports[port.Name]; ok {
				m.logger.Error(
					ErrPortNameNotUnique, "Port name not unique",
					"port-name", port.Name,
					"previous-port", prevPort.External,
					"current-port", port.Port,
				)
				return nil, nil, ErrPortNameNotUnique
			}

			ports[port.Name] = reconciliation.Port{
				Internal: uint16(port.NodePort),
				External: uint16(port.Port),
			}
		}

		servers = make([]reconciliation.Server, 0, len(nodes))
		for _, node := range nodes {
			addr, err := getNodeEndpointAddress(node)
			if err != nil {
				return nil, nil, fmt.Errorf("error retrieving node endpoint address for node %q: %w", node.Name, err)
			}

			servers = append(servers, reconciliation.Server{
				Name:    node.Name,
				Address: addr,
			})
		}

		ea, err := m.addressManager.AllocateAddresses(ctx, svc)
		if err != nil {
			return nil, nil, err
		}

		externalAddresses = make([]net.IP, 0, len(ea))
		for _, a := range ea {
			ip := net.ParseIP(a)
			if ip.IsUnspecified() {
				continue
			}

			if err := m.checkIPCollision(ctx, ip, svc); err != nil {
				return nil, nil, err
			}

			externalAddresses = append(externalAddresses, ip)
		}
	} else {
		ports = make(map[string]reconciliation.Port)
		servers = make([]reconciliation.Server, 0)
		externalAddresses = make([]net.IP, 0)
	}

	mrecon := reconciliation.Multi()
	for _, lb := range m.loadBalancers {
		ctx := logr.NewContext(
			ctx,
			logr.FromContextOrDiscard(ctx).WithValues(
				"loadbalancer", lb,
			),
		)

		recon, err := reconciliation.New(
			ctx,
			m.api,

			m.GetLoadBalancerName(ctx, clusterName, svc),
			lb,
			string(svc.UID),

			externalAddresses,
			ports,
			servers,

			m.backoffSteps,

			m.metrics,
		)
		if err != nil {
			return nil, nil, err
		}

		mrecon.Add(recon)
	}

	return mrecon, externalAddresses, nil
}

// checkIPCollision looks at every LoadBalancer service in the cluster (except the given one) and checks if it uses the given IP already.
func (m mgr) checkIPCollision(ctx context.Context, ip net.IP, svc *v1.Service) error {
	log := logr.FromContextOrDiscard(ctx)

	if m.k8s != nil {
		svcList, err := m.k8s.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error listing services to check if the VIP is already in use: %w", err)
		}

		for _, s := range svcList.Items {
			if s.Namespace == svc.Namespace && s.Name == svc.Name {
				continue
			}

			if s.Spec.Type != v1.ServiceTypeLoadBalancer {
				continue
			}

			log := log.WithValues(
				"other-service", fmt.Sprintf("%s/%s", s.Namespace, s.Name),
			)

			for _, ingress := range s.Status.LoadBalancer.Ingress {
				svcIP := net.ParseIP(ingress.IP)
				if svcIP.Equal(ip) {
					log.Error(ErrSingleVIPConflict, "external IP collision detected")
					return ErrSingleVIPConflict
				}
			}
		}
	} else {
		log.Error(nil, "no usable kubernetes client to check for external IP collisions")
	}

	return nil
}

// lbStatusFromReconcileStatus crafts a [v1.LoadBalancerStatus] out of the given service and status mapping.
// [ipPortMap] is a map that maps from each IP address to the given ports.
//
// Right now, only TCP ports are supported and are therefore hardcoded.
func lbStatusFromReconcileStatus(ipPortMap map[string][]uint16, service *v1.Service) *v1.LoadBalancerStatus {
	// First, we're constructing a slice of unique portNumbers.
	// This is intentionally a int32 slice to avoid the casting at a later point.
	var portNumbers []int32
	for _, ipPorts := range ipPortMap {
		for _, p := range ipPorts {
			portNumbers = append(portNumbers, int32(p))
		}
	}

	// After we constructed our slice of port numbers, we remove any duplicate elements from it.
	slices.Sort(portNumbers)                  // sort the slice, so that compact finds duplicates
	portNumbers = slices.Compact(portNumbers) // remove any duplicates

	// To make use of the port numbers, we have to build a slice out of it
	// that is compatible with our status.
	var ports []v1.PortStatus
	for _, p := range portNumbers {
		// Since slices.Compact fills duplicates with the zero value, we skip them.
		if p == 0 {
			continue
		}

		ports = append(ports, v1.PortStatus{
			Port:     p,
			Protocol: v1.ProtocolTCP,
		})
	}

	// For the case that the annotation was set, we short-circuit and return the one ingress immediately.
	// This is important, because otherwise we could return the same hostname multiple times for a dual-stack service.
	if hostname := strings.ToLower(service.Annotations[AKEAnnotationHostname]); hostname != "" {
		return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{
			Hostname: hostname,
			Ports:    ports,
		}}}
	}

	// Otherwise, we construct a IP -> port mapping for each individiual IP.
	var ingresses []v1.LoadBalancerIngress
	for externalIP := range ipPortMap {
		ingresses = append(ingresses, v1.LoadBalancerIngress{
			IP:    externalIP,
			Ports: ports,
		})
	}

	return &v1.LoadBalancerStatus{Ingress: ingresses}
}

func getNodeEndpointAddress(n *v1.Node) (net.IP, error) {
	// XXX: assumes a node has one internal and one external IP, does funny things when a nodes has multiple of a given type
	var internalIP, externalIP net.IP

	for _, addr := range n.Status.Addresses {
		ip := net.ParseIP(addr.Address)
		if ip.IsUnspecified() {
			continue
		}

		if addr.Type == v1.NodeInternalIP {
			internalIP = ip
		} else if addr.Type == v1.NodeExternalIP {
			externalIP = ip
		}
	}

	if len(externalIP) != 0 {
		return externalIP, nil
	} else if len(internalIP) != 0 {
		return internalIP, nil
	} else {
		return nil, ErrNoUsableNodeAddress
	}
}
