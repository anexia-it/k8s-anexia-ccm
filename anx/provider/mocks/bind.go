package mocks

import (
	bind "go.anx.io/go-anxcloud/pkg/lbaas/bind"
)

// Bind is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/bind generic API.
type Bind = GenericResourceAPI[bind.Bind, bind.Definition]
