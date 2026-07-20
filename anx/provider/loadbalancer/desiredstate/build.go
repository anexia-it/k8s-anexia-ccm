package desiredstate

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var (
	ErrServiceRequired        = errors.New("service is required")
	ErrNotLoadBalancerService = errors.New("service is not of type LoadBalancer")
	ErrServiceUIDRequired     = errors.New("service UID is required")
	ErrNoLoadBalancers        = errors.New("at least one load balancer identifier is required")
	ErrNoExternalAddresses    = errors.New("at least one external address is required")
	ErrUnsupportedProtocol    = errors.New("unsupported service protocol")
	ErrNodePortRequired       = errors.New("service port has no NodePort")
	ErrPortNameNotUnique      = errors.New("service port name is not unique")
	ErrNodeNameNotUnique      = errors.New("node name is not unique")
	ErrNoUsableNodeAddress    = errors.New("node has no usable address")
	ErrAddressFamilyNotUnique = errors.New("external address family is not unique")
)

type desiredPort struct {
	name     string
	external uint16
	internal uint16
}

type desiredNode struct {
	name    string
	address string
}

// Build creates the complete desired LBaaS resource graph without making any
// external calls or inspecting Service.Status.
func Build(input Input) (State, error) {
	if input.Service == nil {
		return State{}, ErrServiceRequired
	}
	if input.Service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return State{}, ErrNotLoadBalancerService
	}
	if input.Service.UID == "" {
		return State{}, ErrServiceUIDRequired
	}

	loadBalancers, err := loadBalancerIdentifiers(input.LoadBalancerIdentifiers)
	if err != nil {
		return State{}, err
	}
	addresses, err := externalAddresses(input.ExternalAddresses)
	if err != nil {
		return State{}, err
	}
	ports, err := servicePorts(input.Service)
	if err != nil {
		return State{}, err
	}
	nodes, err := serviceNodes(input.Nodes)
	if err != nil {
		return State{}, err
	}

	serviceName := strings.Join([]string{input.Service.Namespace, input.Service.Name}, "/")
	tags := []string{ServiceUIDTagKeyPrefix + string(input.Service.UID)}
	state := State{
		Service:                 serviceName,
		ServiceUID:              string(input.Service.UID),
		ClusterName:             input.ClusterName,
		LoadBalancerIdentifiers: loadBalancers,
		ExternalAddresses:       addresses,
		Backends:                make([]Backend, 0, len(loadBalancers)*len(ports)),
		Servers:                 make([]Server, 0, len(loadBalancers)*len(ports)*len(nodes)),
		Frontends:               make([]Frontend, 0, len(loadBalancers)*len(ports)),
		Binds:                   make([]Bind, 0, len(loadBalancers)*len(ports)*len(addresses)),
	}

	suffix := resourceName(input.Service.Name, input.Service.Namespace, input.ClusterName)
	for _, loadBalancer := range loadBalancers {
		for _, port := range ports {
			backendName := resourceName(port.name, suffix)
			backendKey := resourceKey(KindBackend, loadBalancer, backendName)
			frontendName := resourceName(port.name, suffix)
			frontendKey := resourceKey(KindFrontend, loadBalancer, frontendName)

			state.Backends = append(state.Backends, Backend{
				Key:                    backendKey,
				Name:                   backendName,
				LoadBalancerIdentifier: loadBalancer,
				Mode:                   ModeTCP,
				HealthCheck:            TCPHealthCheck,
				Tags:                   slices.Clone(tags),
			})

			for _, node := range nodes {
				serverName := resourceName(node.name, port.name, suffix)
				state.Servers = append(state.Servers, Server{
					Key:        resourceKey(KindServer, loadBalancer, serverName),
					Name:       serverName,
					BackendKey: backendKey,
					IP:         node.address,
					Port:       port.internal,
					Check:      ServerCheckEnabled,
					Tags:       slices.Clone(tags),
				})
			}

			state.Frontends = append(state.Frontends, Frontend{
				Key:                    frontendKey,
				Name:                   frontendName,
				LoadBalancerIdentifier: loadBalancer,
				DefaultBackendKey:      backendKey,
				Mode:                   ModeTCP,
				Tags:                   slices.Clone(tags),
			})

			for _, address := range addresses {
				family := "v6"
				if net.ParseIP(address).To4() != nil {
					family = "v4"
				}
				bindName := resourceName(family, port.name, suffix)
				state.Binds = append(state.Binds, Bind{
					Key:         resourceKey(KindBind, loadBalancer, bindName),
					Name:        bindName,
					FrontendKey: frontendKey,
					Address:     address,
					Port:        port.external,
					Tags:        slices.Clone(tags),
				})
			}
		}
	}

	return state, nil
}

func loadBalancerIdentifiers(input []string) ([]string, error) {
	ret := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, identifier := range input {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			return nil, fmt.Errorf("%w: empty identifier", ErrNoLoadBalancers)
		}
		if _, ok := seen[identifier]; ok {
			continue
		}
		seen[identifier] = struct{}{}
		ret = append(ret, identifier)
	}
	if len(ret) == 0 {
		return nil, ErrNoLoadBalancers
	}
	sort.Strings(ret)
	return ret, nil
}

func externalAddresses(input []string) ([]string, error) {
	ret := make([]string, 0, len(input))
	families := make(map[string]string, 2)
	for _, value := range input {
		ip := net.ParseIP(value)
		if ip == nil || ip.IsUnspecified() {
			return nil, fmt.Errorf("invalid external address %q", value)
		}

		family := "IPv6"
		if ip.To4() != nil {
			family = "IPv4"
			ip = ip.To4()
		}
		if existing, ok := families[family]; ok {
			return nil, fmt.Errorf("%w: %s and %s are both %s", ErrAddressFamilyNotUnique, existing, ip.String(), family)
		}
		families[family] = ip.String()
		ret = append(ret, ip.String())
	}
	if len(ret) == 0 {
		return nil, ErrNoExternalAddresses
	}
	sort.Slice(ret, func(i, j int) bool {
		left, right := net.ParseIP(ret[i]), net.ParseIP(ret[j])
		leftV4, rightV4 := left.To4() != nil, right.To4() != nil
		if leftV4 != rightV4 {
			return leftV4
		}
		return ret[i] < ret[j]
	})
	return ret, nil
}

func servicePorts(service *corev1.Service) ([]desiredPort, error) {
	ret := make([]desiredPort, 0, len(service.Spec.Ports))
	seen := make(map[string]struct{}, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		if port.Protocol != corev1.ProtocolTCP {
			return nil, fmt.Errorf("%w: port %q uses %s", ErrUnsupportedProtocol, port.Name, port.Protocol)
		}
		if port.NodePort <= 0 {
			return nil, fmt.Errorf("%w: port %q", ErrNodePortRequired, port.Name)
		}
		if _, ok := seen[port.Name]; ok {
			return nil, fmt.Errorf("%w: %q", ErrPortNameNotUnique, port.Name)
		}
		seen[port.Name] = struct{}{}
		ret = append(ret, desiredPort{
			name:     port.Name,
			external: uint16(port.Port),
			internal: uint16(port.NodePort),
		})
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].name != ret[j].name {
			return ret[i].name < ret[j].name
		}
		return ret[i].external < ret[j].external
	})
	return ret, nil
}

func serviceNodes(input []*corev1.Node) ([]desiredNode, error) {
	ret := make([]desiredNode, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, node := range input {
		if node == nil {
			return nil, errors.New("node is nil")
		}
		if _, ok := seen[node.Name]; ok {
			return nil, fmt.Errorf("%w: %q", ErrNodeNameNotUnique, node.Name)
		}
		seen[node.Name] = struct{}{}

		address, err := nodeEndpointAddress(node)
		if err != nil {
			return nil, fmt.Errorf("node %q: %w", node.Name, err)
		}
		ret = append(ret, desiredNode{name: node.Name, address: address})
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].name < ret[j].name })
	return ret, nil
}

func nodeEndpointAddress(node *corev1.Node) (string, error) {
	var internalIP, externalIP net.IP
	for _, address := range node.Status.Addresses {
		ip := net.ParseIP(address.Address)
		if ip == nil || ip.IsUnspecified() {
			continue
		}
		switch address.Type {
		case corev1.NodeInternalIP:
			internalIP = ip
		case corev1.NodeExternalIP:
			externalIP = ip
		}
	}
	if externalIP != nil {
		return externalIP.String(), nil
	}
	if internalIP != nil {
		return internalIP.String(), nil
	}
	return "", ErrNoUsableNodeAddress
}

func resourceName(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, ".")
}

func resourceKey(kind Kind, loadBalancer, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, loadBalancer, name)
}
