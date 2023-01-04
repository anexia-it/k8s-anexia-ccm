package address

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	"go.anx.io/go-anxcloud/pkg/ipam"
	v1 "k8s.io/api/core/v1"
)

type prefix struct {
	identifier string
	prefix     net.IPNet
	family     v1.IPFamily
	addresses  []net.IP
}

func newPrefix(ctx context.Context, apiclient api.API, ipamClient ipam.API, identifier string, autoDiscoveryName *string) (*prefix, error) {
	p, err := ipamClient.Prefix().Get(ctx, identifier)
	if err != nil {
		return nil, err
	}

	_, n, err := net.ParseCIDR(p.Name)
	if err != nil {
		return nil, err
	}

	ret := prefix{
		identifier: identifier,
		prefix:     *n,
		family:     v1.IPv6Protocol,
	}

	if n.IP.To4() != nil {
		ret.family = v1.IPv4Protocol
	}

	if autoDiscoveryName != nil {
		tag := fmt.Sprintf("kubernetes-lb-vip-%s", *autoDiscoveryName)
		vip, err := ret.discoverVIP(ctx, apiclient, ipamClient, tag)
		if err != nil {
			return nil, fmt.Errorf("error discovering VIP: %w", err)
		}

		if vip == nil {
			// Fall back for backwards compatibility - remove once there are no autodiscovery clusters without tagged VIPs
			vip = calculateVIP(ret.prefix)
			logger := logr.FromContextOrDiscard(ctx)
			logger.Info("Could not auto discover VIP, falling back to calculated VIP", "prefix-identifier", ret.identifier, "vip", vip.String())
		}

		ret.addresses = []net.IP{vip}
	} else {
		ret.addresses = []net.IP{calculateVIP(ret.prefix)}
	}

	return &ret, nil
}

func (p prefix) allocateAddress(ctx context.Context, fam v1.IPFamily) (net.IP, error) {
	if fam != p.family {
		return nil, errFamilyMismatch
	}

	log := logr.FromContextOrDiscard(ctx).WithValues(
		"prefix", p.prefix.String(),
		"prefix-identifier", p.identifier,
	)

	// XXX: replace this with IPAM address allocation logic once we can add and remove LoadBalancer IPs
	// See SYSENG-918 for more info.
	ip := p.addresses[0]

	log.V(1).Info(
		"allocated external IP",
		"prefix", p.prefix.String(),
		"address", ip.String(),
	)

	return ip, nil
}

func (p prefix) discoverVIP(ctx context.Context, apiClient api.API, ipamClient ipam.API, tag string) (net.IP, error) {
	logger := logr.FromContextOrDiscard(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var oc types.ObjectChannel
	err := apiClient.List(ctx, &corev1.Resource{Tags: []string{tag}}, api.ObjectChannel(&oc))
	if err != nil {
		httpError := api.HTTPError{}
		// nothing is tagged with autodiscover tag -> no VIP found, but also no error
		if errors.As(err, &httpError) && httpError.StatusCode() == http.StatusUnprocessableEntity {
			err = nil
		} else {
			err = fmt.Errorf("unable to autodiscover VIP address by tag %q: %w", tag, err)
		}

		return nil, err
	}

	for retriever := range oc {
		var res corev1.Resource
		err := retriever(&res)
		if err != nil {
			return nil, fmt.Errorf("error retrieving resource: %w", err)
		}

		address, err := ipamClient.Address().Get(ctx, res.Identifier)
		if err != nil {
			logger.Error(err, "Error retrieving Address, maybe something else is tagged with %q? Ignoring this one and continuing",
				"identifier", res.Identifier,
			)
			continue
		}

		// If the address we discovered is not from the prefix for which we are allocating, skip it
		if address.PrefixID != p.identifier {
			continue
		}

		logger.V(1).Info("Found VIP Address via auto discovery", "identifier", address.ID)
		return net.ParseIP(address.Name), err
	}

	// no VIP found, but also no error
	return nil, nil

}
