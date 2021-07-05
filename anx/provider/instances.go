package provider

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

type instanceController struct {
}

func (i instanceController) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	panic("implement me")
}

func (i instanceController) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	panic("implement me")
}

func (i instanceController) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	panic("implement me")
}

func (i instanceController) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	panic("implement me")
}

func (i instanceController) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	panic("implement me")
}

func (i instanceController) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	panic("implement me")
}

func (i instanceController) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	panic("implement me")
}

func (i instanceController) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	panic("implement me")
}

func (i instanceController) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	panic("implement me")
}

func (i instanceController) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	panic("implement me")
}

func (i instanceController) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	panic("implement me")
}

func (i instanceController) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	panic("implement me")
}
