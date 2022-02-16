package discovery

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	v1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"go.anx.io/go-anxcloud/pkg/client"
	"k8s.io/klog/v2"
)

func AutoDiscoverLoadBalancer(ctx context.Context, tag string) (string, []string, error) {
	newAPI, err := api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
	if err != nil {
		return "", nil, err
	}

	var pageIter types.PageInfo
	err = newAPI.List(ctx, &corev1.Info{
		Tags: []string{tag},
	}, api.Paged(1, 100, &pageIter))

	if err != nil {
		return "", nil, fmt.Errorf("unable to autodiscover load balancer by tag '%s': %w", tag, err)
	}

	var infos []corev1.Info
	pageIter.Next(&infos)
	if pageIter.TotalPages() > 1 {
		return "", nil, errors.New("too many load balancers were discovered currently only 100")
	}

	if len(infos) == 0 {
		return "", nil, errors.New("no load balancers could be discovered")
	}

	var identifiers []string

	for _, info := range infos {
		identifiers = append(identifiers, info.Identifier)
	}
	sort.Strings(identifiers)

	var secondaryLoadBalancers []string

	if len(identifiers) > 1 {
		secondaryLoadBalancers = append(secondaryLoadBalancers, identifiers[1:]...)
	}

	primaryLB := identifiers[0]
	err = newAPI.Get(ctx, &v1.LoadBalancer{Identifier: primaryLB})
	if err != nil {
		klog.Errorf("checking if load balancer '%s' exists returned an error: %s", primaryLB,
			err.Error())
		return "", nil, err
	}

	return primaryLB, secondaryLoadBalancers, nil
}

func AutoDiscoverLoadBalancerPrefixes(ctx context.Context, tag string) ([]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	genclient, err := api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
	if err != nil {
		return nil, fmt.Errorf("error creating generic API client: %w", err)
	}

	var prefixChannel types.ObjectChannel
	if err := genclient.List(ctx, &corev1.Info{Tags: []string{tag}}, api.ObjectChannel(&prefixChannel)); err != nil {
		return nil, fmt.Errorf("error listing resources with tag: %w", err)
	}

	// most of the time we have one IPv4 and one IPv6 prefix, optimize for that case
	ret := make([]string, 0, 2)

	for retriever := range prefixChannel {
		var prefix corev1.Info
		if err := retriever(&prefix); err != nil {
			cancel()
			return nil, fmt.Errorf("error retrieving resource: %w", err)
		}

		ret = append(ret, prefix.Identifier)
	}

	return ret, nil
}
