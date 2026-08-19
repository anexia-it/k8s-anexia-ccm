package mocks

import (
	backend "go.anx.io/go-anxcloud/pkg/lbaas/backend"
)

// Backend is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/backend generic API.
type Backend = GenericResourceAPI[backend.Backend, backend.Definition]
