package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/utils"
	vminfo "github.com/anexia-it/go-anxcloud/pkg/vsphere/info"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere/powercontrol"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"strings"
)

type instanceManager struct {
	Provider
}

func (i instanceManager) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	if providerID == "" {
		return nil, errors.New("empty providerId is not allowed")
	}
	info, err := i.VSphere().Info().Get(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("could not get vm infoMock: %w", err)
	}

	if len(info.Network) == 0 {
		return nil, nil
	}

	nodeAddresses := make([]v1.NodeAddress, 0, len(info.Network))
	if len(info.Network) > 1 {
		klog.Warningf("found multiple networks for VM '%s'. This can potentially break stuff. Since only the first one"+
			"will be used", providerID)
	}
	for _, ip := range info.Network[0].IPv4 {
		nodeAddresses = append(nodeAddresses, v1.NodeAddress{
			Type:    "InternalIP",
			Address: ip,
		})
	}

	return nodeAddresses, nil
}

func (i instanceManager) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	providerID, err := i.InstanceIDByNode(ctx, node)
	if err != nil {
		return false, err
	}

	_, err = i.VSphere().Info().Get(ctx, providerID)

	if err == nil {
		return true, nil
	}

	if utils.IsNotFoundError(err) {
		return false, nil
	}

	return false, err
}

func (i instanceManager) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
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
		return false, fmt.Errorf("unkown power state '%s'", state)
	}
}

func (i instanceManager) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
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

func (i instanceManager) InstanceIDByNode(ctx context.Context, node *v1.Node) (string, error) {
	if node.Spec.ProviderID != "" {
		return strings.TrimPrefix(node.Spec.ProviderID, cloudProviderScheme), nil
	}
	vms, err := i.VSphere().Search().ByName(ctx, fmt.Sprintf("%s-%s", i.Config().CustomerPrefix, node.Name))
	if err != nil {
		return "", err
	}
	if len(vms) != 1 {
		return "", fmt.Errorf("expected 1 VM with the name '%s', but found %d", node.Name, len(vms))
	}

	return vms[0].Identifier, nil
}

func instanceType(info vminfo.Info) string {
	cores := info.CPU
	ram := info.RAM / 1024
	var largestDisk *vminfo.DiskInfo
	for _, diskInfo := range info.DiskInfo {
		if largestDisk == nil || largestDisk.DiskGB < diskInfo.DiskGB {
			largestDisk = &diskInfo
		}
	}
	if largestDisk != nil {
		return fmt.Sprintf("C%d-M%d-%s", cores, ram, largestDisk.DiskType)
	}
	return fmt.Sprintf("C%d-M%d", cores, ram)
}
