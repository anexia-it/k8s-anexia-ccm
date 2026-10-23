package mocks

import (
	frontend "go.anx.io/go-anxcloud/pkg/lbaas/frontend"
)

// Frontend is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/frontend generic API.
type Frontend = GenericResourceAPI[frontend.Frontend, frontend.Definition]
