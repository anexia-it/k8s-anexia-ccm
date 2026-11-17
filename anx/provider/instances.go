package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/utils"
	"github.com/go-logr/logr"
	vminfo "go.anx.io/go-anxcloud/pkg/vsphere/info"
	"go.anx.io/go-anxcloud/pkg/vsphere/powercontrol"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type instanceManager struct {
	Provider

	lastUnauthorizedOrForbiddenInstanceExistCall time.Time
}

var (
	errNamedVirtualMachineNotFound = errors.New("virtual machine with given name not found")
	errVirtualMachineNameNotUnique = errors.New("virtual machine name not unique")
)

// NodeAddressesByProviderID gets Node Ips for Given ProviderID,
func (i *instanceManager) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	if providerID == "" {
		return nil, errors.New("empty providerId is not allowed")
	}
	info, err := i.VSphere().Info().Get(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("could not get vm info: %w", err)
	}

	if len(info.Network) == 0 {
		klog.Infof("VM %q has no networks, no node addresses to set", providerID)
		return nil, nil
	}

	var nodeAddresses []v1.NodeAddress
	for _, network := range info.Network {
		for _, ip := range append(network.IPv4, network.IPv6...) {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				continue
			}
			if hasNodeAddress(nodeAddresses, parsed) {
				continue
			}

			addressType := v1.NodeInternalIP
			if !parsed.IsPrivate() {
				addressType = v1.NodeExternalIP
			}

			nodeAddresses = append(nodeAddresses, v1.NodeAddress{
				Type:    addressType,
				Address: ip,
			})
		}
	}

	if len(nodeAddresses) == 0 {
		klog.Warningf("VM %q has networks but no parseable IP addresses", providerID)
	}

	// ensure there is always at least one InternalIP,
	// e.g. for nodes that are only attached to public networks
	if !hasNodeAddressOfType(nodeAddresses, v1.NodeInternalIP) {
		for _, addr := range nodeAddresses {
			if addr.Type == v1.NodeExternalIP {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{
					Type:    v1.NodeInternalIP,
					Address: addr.Address,
				})
				break
			}
		}
	}

	return nodeAddresses, nil
}

func hasNodeAddress(addresses []v1.NodeAddress, ip net.IP) bool {
	for _, addr := range addresses {
		if net.ParseIP(addr.Address).Equal(ip) {
			return true
		}
	}

	return false
}

func hasNodeAddressOfType(addresses []v1.NodeAddress, addressType v1.NodeAddressType) bool {
	for _, addr := range addresses {
		if addr.Type == addressType {
			return true
		}
	}

	return false
}

func (i *instanceManager) handleUnauthorizedForbidden(err error) {
	if utils.IsUnauthorizedOrForbiddenError(err) {
		i.lastUnauthorizedOrForbiddenInstanceExistCall = time.Now()
	}
}

func (i *instanceManager) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	if i.lastUnauthorizedOrForbiddenInstanceExistCall.Add(time.Minute).After(time.Now()) {
		return false, utils.ErrUnauthorizedForbiddenBackoff
	}

	providerID, err := i.InstanceIDByNode(ctx, node)
	if errors.Is(err, errNamedVirtualMachineNotFound) {
		return false, nil
	}
	if err != nil {
		i.handleUnauthorizedForbidden(err)
		return false, err
	}

	_, err = i.VSphere().Info().Get(ctx, providerID)
	if err == nil {
		return true, nil
	}

	i.handleUnauthorizedForbidden(err)

	if utils.IsNotFoundError(err) {
		return false, nil
	}

	return false, err
}

func (i *instanceManager) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	providerID, err := i.InstanceIDByNode(ctx, node)
	if err != nil {
		return false, err
	}

	state, err := i.VSphere().PowerControl().Get(ctx, providerID)
	if err != nil {
		return false, fmt.Errorf("could not get power state of '%s': %w", providerID, err)
	}

	switch state {
	case powercontrol.OnState:
		return false, nil
	case powercontrol.OffState:
		return true, nil
	default:
		return false, fmt.Errorf("unknown power state '%s'", state)
	}
}

func (i *instanceManager) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	providerID, err := i.InstanceIDByNode(ctx, node)
	if err != nil {
		return nil, err
	}

	nodeAddresses, err := i.NodeAddressesByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	info, err := i.VSphere().Info().Get(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    providerID,
		InstanceType:  instanceType(info),
		NodeAddresses: nodeAddresses,
		Zone:          info.LocationCode,
		Region:        info.LocationCountry,
	}, nil
}

// try to fetch the correct instance by name, first by using the configured prefix (or a
// wildcard when not configured) and, when none found this way, tries the name as-is without
// any prefix
func (i *instanceManager) instancesByName(ctx context.Context, name string) ([]string, error) {
	logger := logr.FromContextOrDiscard(ctx).WithValues("nodeName", name)

	namePrefix := i.Config().CustomerID
	if namePrefix == "" {
		namePrefix = "%"
		logger.Info("Customer ID prefix not configured, using a wildcard", "prefix", namePrefix)
	} else {
		logger.V(0).Info("Listing VMs with configured prefix", "prefix", namePrefix)
	}

	vms, err := i.VSphere().Search().ByName(ctx, fmt.Sprintf("%s-%s", namePrefix, name))

	if err == nil && len(vms) == 0 {
		logger.V(0).Info("Didn't find any VM by name with prefix, retrying without prefix", "prefix", namePrefix)
		vms, err = i.VSphere().Search().ByName(ctx, name)
	}

	if err != nil {
		return nil, fmt.Errorf("error listing VMs by name: %w", err)
	}

	ret := make([]string, 0, len(vms))
	for _, vm := range vms {
		ret = append(ret, vm.Identifier)
	}

	return ret, nil
}

func (i *instanceManager) filterInstances(ctx context.Context, node *v1.Node, vms []string) []string {
	logger := logr.FromContextOrDiscard(ctx).WithValues("nodeName", node.Name)

	nodeIPs := nodeInternalIPs(node)

	filtered := make([]string, 0, len(vms))
nextVM:
	for _, vm := range vms {
		fullVM, err := i.VSphere().Info().Get(ctx, vm)
		if err != nil {
			logger.Error(err, "Error retrieving full VM details", "identifier", vm)
			continue
		}

		for _, network := range fullVM.Network {
			for _, ip := range append(network.IPv4, network.IPv6...) {
				for _, nodeIP := range nodeIPs {
					if nodeIP.Equal(net.ParseIP(ip)) {
						filtered = append(filtered, fullVM.Identifier)
						continue nextVM
					}
				}
			}
		}
	}

	return filtered
}

func (i *instanceManager) InstanceIDByNode(ctx context.Context, node *v1.Node) (string, error) {
	if node.Spec.ProviderID != "" {
		return strings.TrimPrefix(node.Spec.ProviderID, configuration.CloudProviderScheme), nil
	}

	logger := logr.FromContextOrDiscard(ctx).WithValues("nodeName", node.Name)

	vms, err := i.instancesByName(ctx, node.Name)
	if err != nil {
		return "", err
	}

	if len(vms) > 1 {
		logger.Info("Found multiple VMs matching node.Name, filtering by IPs now")

		vms = i.filterInstances(ctx, node, vms)

		if len(vms) > 1 {
			logger.Info("Found multiple VMs matching node.Name and having the expected IP address - giving up")
			return "", errVirtualMachineNameNotUnique
		}
	}

	if len(vms) == 1 {
		return vms[0], nil
	}

	return "", errNamedVirtualMachineNotFound
}

func instanceType(info vminfo.Info) string {
	cores := info.CPU
	ram := info.RAM / 1024
	var largestDisk *vminfo.DiskInfo
	for _, diskInfo := range info.DiskInfo {
		// skip if DiskType is zero-val as this will result in a trailing `-`
		// which is not allowed for `metadata.labels` values
		if diskInfo.DiskType != "" && (largestDisk == nil || largestDisk.DiskGB < diskInfo.DiskGB) {
			largestDisk = &diskInfo
		}
	}
	if largestDisk != nil {
		return fmt.Sprintf("C%d-M%d-%s", cores, ram, largestDisk.DiskType)
	}
	return fmt.Sprintf("C%d-M%d", cores, ram)
}

func nodeInternalIPs(node *v1.Node) []net.IP {
	ret := make([]net.IP, 0, len(node.Status.Addresses))

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			ip := net.ParseIP(addr.Address)
			if ip != nil {
				ret = append(ret, ip)
			}
		}
	}

	return ret
}
