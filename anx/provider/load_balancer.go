package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/loadbalancer"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/sync"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

type loadBalancerManager struct {
	Provider
	k8sClient kubernetes.Interface

	prefixes []lbmgrPrefix

	notify chan struct{}

	rLock *sync.SubjectLock
}

type lbmgrPrefix struct {
	prefix     net.IPNet
	family     v1.IPFamily
	identifier string
}

const (
	lbaasExternalIPFamiliesAnnotation = "lbaas.anx.io/external-ip-families"
)

var (
	errSingleVIPConflict           = errors.New("only a single LoadBalancer can be used in Anexia Kubernetes Service beta, but found another service using the external IP already")
	errFamilyMismatch              = errors.New("requested family does not match prefix family")
	errInvalidIPFamiliesAnnotation = errors.New(fmt.Sprintf("invalid IP family in annotation %v", lbaasExternalIPFamiliesAnnotation))
)

func newLoadBalancerManager(provider Provider, clientBuilder cloudprovider.ControllerClientBuilder) loadBalancerManager {
	prefixes := retrieveLoadBalancerPrefixes(provider)

	lbmgr := loadBalancerManager{
		Provider: provider,
		prefixes: prefixes,
		notify:   nil,
		rLock:    sync.NewSubjectLock(),
	}

	if clientBuilder != nil {
		if c, err := clientBuilder.Client("LoadBalancerManager"); err != nil {
			klog.ErrorS(err, "error creating kubernetes client for LoadBalancerManager")
		} else {
			lbmgr.k8sClient = c
		}
	}

	return lbmgr
}

func retrieveLoadBalancerPrefixes(provider Provider) []lbmgrPrefix {
	prefixes := make([]lbmgrPrefix, 0, len(provider.Config().LoadBalancerPrefixIdentifiers))

	log := klogr.New()

	for _, prefixIdentifier := range provider.Config().LoadBalancerPrefixIdentifiers {
		log := log.WithValues("prefixIdentifier", prefixIdentifier)

		p, err := provider.IPAM().Prefix().Get(context.TODO(), prefixIdentifier)
		if err != nil {
			log.Error(err, "error retrieving external prefix for LoadBalancer")
		}

		log = log.WithValues("prefix", p.Name)

		_, prefix, err := net.ParseCIDR(p.Name)
		if err != nil {
			log.Error(err, "error parsing external prefix for LoadBalancer")
			continue
		}

		fam := v1.IPv4Protocol

		if prefix.IP.To4() == nil {
			fam = v1.IPv6Protocol
		}

		// XXX: checking if we already have a prefix of the same family configured
		// This is for Anexia Kubernetes Service MVP, where only a single VIP is configured on the LoadBalancers
		// and we figure it out from the prefix - hence only one per family is allowed for now.
		for _, existingPrefix := range prefixes {
			if existingPrefix.family == fam {
				log.Error(
					errors.New("only one prefix for each v4 and v6 is allowed for now"),
					"Got another prefix for the same family, skipping this one",
					"existingPrefix", existingPrefix.prefix.String(), "existingPrefixIdentifier", existingPrefix.identifier,
				)

				continue
			}
		}

		prefixes = append(prefixes, lbmgrPrefix{
			identifier: prefixIdentifier,
			prefix:     *prefix,
			family:     fam,
		})
	}

	for _, p := range prefixes {
		klog.InfoS("using prefix for external LoadBalancer IPs",
			"prefixIdentifier", p.identifier,
			"prefix", p.prefix.String(),
			"family", p.family,
		)
	}

	return prefixes
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

	status, err := l.assembleLBStatus(ctx, service, portStatus)
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

	status, err := l.assembleLBStatus(ctx, service, portStatus)
	if err != nil {
		return status, err
	}

	return status, nil
}

func (l loadBalancerManager) assembleLBStatus(ctx context.Context, svc *v1.Service, portStatus []v1.PortStatus) (*v1.LoadBalancerStatus, error) {
	log := logr.FromContextOrDiscard(ctx).WithValues(
		"service", fmt.Sprintf("%s/%s", svc.Namespace, svc.Name),
	)

	families := svc.Spec.IPFamilies

	if externalFamiliesAnnotation, ok := svc.Annotations[lbaasExternalIPFamiliesAnnotation]; ok {
		familyStrings := strings.Split(externalFamiliesAnnotation, ",")
		families = make([]v1.IPFamily, 0, len(familyStrings))

		validFamilies := []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}

		for _, fam := range familyStrings {
			valid := false

			for _, validFam := range validFamilies {
				if fam == string(validFam) {
					valid = true
					break
				}
			}

			if !valid {
				return nil, fmt.Errorf("%w: %v is not a valid IPFamily", errInvalidIPFamiliesAnnotation, fam)
			}

			families = append(families, v1.IPFamily(fam))
		}
	}

	status := svc.Status.LoadBalancer

	if status.Ingress == nil || len(status.Ingress) < len(svc.Spec.IPFamilies) {
		newIPs := make([]net.IP, 0)

		if status.Ingress == nil {
			status.Ingress = make([]v1.LoadBalancerIngress, 0)
		}

	familyLoop:
		for _, fam := range families {
			log := log.WithValues("family", fam)
			ctx := logr.NewContext(ctx, log)

			for _, allocated := range status.Ingress {
				if allocated.IP == "" {
					continue
				}

				ip := net.ParseIP(allocated.IP)

				if (fam == v1.IPv4Protocol && len(ip) == 4) ||
					(fam == v1.IPv6Protocol && len(ip) == 16) {
					continue familyLoop
				}
			}

			externalIP, err := l.allocateExternalIP(ctx, fam)
			if err != nil {
				return nil, err
			}

			// XXX: check if no other service already has the same IP
			// this, again, is only relevant until we actually allocate new IPs and not use the single one available for use.
			err = l.checkIPCollision(ctx, externalIP, svc)
			if err != nil {
				return nil, err
			}

			newIPs = append(newIPs, externalIP)
		}

		for _, extIP := range newIPs {
			status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{
				IP:    extIP.String(),
				Ports: portStatus,
			})
		}
	}

	return &status, nil
}

// checkIPCollision looks at every LoadBalancer service in the cluster (except the given one) and checks if it uses the given IP already.
func (l loadBalancerManager) checkIPCollision(ctx context.Context, ip net.IP, svc *v1.Service) error {
	log := logr.FromContextOrDiscard(ctx)

	if l.k8sClient != nil {
		svcList, err := l.k8sClient.CoreV1().Services("").List(ctx, metav1.ListOptions{})
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
					log.Error(errSingleVIPConflict, "external IP collision detected")
					return errSingleVIPConflict
				}
			}
		}
	} else {
		log.Error(nil, "no usable kubernetes client to check for external IP collisions")
	}

	return nil
}

func (l loadBalancerManager) allocateExternalIP(ctx context.Context, fam v1.IPFamily) (net.IP, error) {
	log := logr.FromContextOrDiscard(ctx)

	// for every prefix, try to allocate an address from it, returning the first that works
	for _, p := range l.prefixes {
		if p.family == fam {
			ip, err := p.allocateAddress(ctx, fam)
			if err != nil {
				return nil, err
			}

			return ip, nil
		}
	}

	// When we got here, it means none of the available prefixes could allocate an address for us, meaning
	// we need a new prefix - but this is NotYetImplemented for Anexia Kubernetes Service MVP
	log.Info("no configured prefix was able to allocate an IP")
	return nil, cloudprovider.NotImplemented
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
	_, err := logr.FromContext(ctx)
	if err != nil {
		// logger is not set but we definitely need one
		logger := klogr.New()
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
	select {
	case l.notify <- struct{}{}:
		klog.V(1).Info("trigger notification")
	default:
		klog.V(3).Info("notification is dropped because there are still pending events to be processed")
	}
}

func (lp lbmgrPrefix) allocateAddress(ctx context.Context, fam v1.IPFamily) (net.IP, error) {
	if fam != lp.family {
		return nil, errFamilyMismatch
	}

	log := logr.FromContextOrDiscard(ctx).WithValues(
		"prefix", lp.prefix.String(),
		"prefix-identifier", lp.identifier,
	)
	ctx = logr.NewContext(ctx, log)

	// XXX: replace this with IPAM address allocation logic once we can add and remove LoadBalancer IPs
	// See SYSENG-918 for more info.
	ip := calculateVIP(lp.prefix)

	log.V(1).Info(
		"allocated external IP",
		"prefix", lp.prefix.String(),
		"address", ip.String(),
	)

	return ip, nil
}
