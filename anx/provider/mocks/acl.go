package mocks

import (
	acl "go.anx.io/go-anxcloud/pkg/lbaas/acl"
)

// ACL is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/acl generic API.
type ACL = GenericResourceAPI[acl.ACL, acl.Definition]
