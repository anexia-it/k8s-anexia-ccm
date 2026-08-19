package mocks

import (
	rule "go.anx.io/go-anxcloud/pkg/lbaas/rule"
)

// Rule is a mock for the go.anx.io/go-anxcloud/pkg/lbaas/rule generic API.
type Rule = GenericResourceAPI[rule.Rule, rule.Definition]
