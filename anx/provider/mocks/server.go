package mocks

import (
	server "go.anx.io/go-anxcloud/pkg/lbaas/server"
)

// Server is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/server generic API.
type Server = GenericResourceAPI[server.Server, server.Definition]
