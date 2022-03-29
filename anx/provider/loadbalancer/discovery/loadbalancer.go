package discovery

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"

	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

// DiscoverLoadBalancers uses the given generic API client and tag to discover LBaaS LoadBalancers to use.
func DiscoverLoadBalancers(ctx context.Context, apiClient api.API, tag string) ([]string, error) {
	logger := logr.FromContextOrDiscard(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ret := make([]string, 0)

	var oc types.ObjectChannel
	err := apiClient.List(ctx, &corev1.Resource{Tags: []string{tag}}, api.ObjectChannel(&oc))
	if err != nil {
		return nil, fmt.Errorf("unable to autodiscover load balancer by tag %q: %w", tag, err)
	}

	for retriever := range oc {
		var res corev1.Resource
		err := retriever(&res)
		if err != nil {
			return nil, fmt.Errorf("error retrieving resource: %w", err)
		}

		lb := lbaasv1.LoadBalancer{Identifier: res.Identifier}
		err = apiClient.Get(ctx, &lb)
		if err != nil {
			logger.Error(err, "Error retrieving LoadBalancer, maybe something else is tagged with %q? Ignoring this one and continuing",
				"identifier", res.Identifier,
			)
			continue
		}

		logger.V(1).Info("Found LoadBalancer via auto discovery", "identifier", lb.Identifier)

		ret = append(ret, res.Identifier)
	}

	return ret, nil
}
