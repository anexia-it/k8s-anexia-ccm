package mocks

import (
	loadbalancer "go.anx.io/go-anxcloud/pkg/lbaas/loadbalancer"
)

// LoadBalancer is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/loadbalancer generic API.
type LoadBalancer = GenericResourceAPI[loadbalancer.Loadbalancer, loadbalancer.Definition]
