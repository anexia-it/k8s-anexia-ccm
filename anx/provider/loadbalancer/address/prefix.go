package address

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	anxprefix "go.anx.io/go-anxcloud/pkg/ipam/prefix"
	v1 "k8s.io/api/core/v1"
)

type prefix struct {
	identifier string
	prefix     net.IPNet
	family     v1.IPFamily
}

func newPrefix(ctx context.Context, client anxprefix.API, identifier string) (*prefix, error) {
	p, err := client.Get(ctx, identifier)
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
	ip := calculateVIP(p.prefix)

	log.V(1).Info(
		"allocated external IP",
		"prefix", p.prefix.String(),
		"address", ip.String(),
	)

	return ip, nil
}
