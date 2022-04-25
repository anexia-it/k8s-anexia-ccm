package address

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	"go.anx.io/go-anxcloud/pkg/client"
	"go.anx.io/go-anxcloud/pkg/ipam"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	lbaasExternalIPFamiliesAnnotation = "lbaas.anx.io/external-ip-families"

	prefixCacheTimeout = 2 * time.Minute
)

var (
	errInvalidIPFamiliesAnnotation = fmt.Errorf("invalid IP family in annotation %v", lbaasExternalIPFamiliesAnnotation)
	errFamilyMismatch              = errors.New("requested family does not match prefix family")
)

// Manager allocates external IP addresses for services
type Manager interface {
	AllocateAddresses(ctx context.Context, svc *v1.Service) ([]string, error)
}

// NewWithPrefixes creates a new Manager instance for a list of Prefix identifiers
func NewWithPrefixes(ctx context.Context, apiClient api.API, legacyClient client.Client, prefixes []string) (Manager, error) {
	m := newMgr(ctx, apiClient, legacyClient)
	m.fixedPrefixes = make([]*prefix, 0, len(prefixes))

	for _, prefix := range prefixes {
		p, err := newPrefix(ctx, m.ipam.Prefix(), prefix)
		if err != nil {
			return nil, err
		}

		m.fixedPrefixes = append(m.fixedPrefixes, p)
	}

	return m, nil
}

// NewWithPrefixAutodiscovery creates a new Manager instance doing Prefix autodiscovery
func NewWithPrefixAutodiscovery(ctx context.Context, apiClient api.API, legacyClient client.Client, prefixTag string) Manager {
	m := newMgr(ctx, apiClient, legacyClient)
	m.prefixAutodiscover = &prefixTag

	return m
}

type mgr struct {
	api    api.API
	ipam   ipam.API
	logger logr.Logger

	// autodiscover is enabled when prefixAutodiscover is not nil
	prefixAutodiscover *string

	// statically configured prefixes
	fixedPrefixes []*prefix

	prefixCache          []*prefix
	prefixCacheTimestamp time.Time
}

func newMgr(ctx context.Context, apiClient api.API, legacyClient client.Client) *mgr {
	m := mgr{
		api:    apiClient,
		logger: logr.FromContextOrDiscard(ctx),
	}

	m.ipam = ipam.NewAPI(legacyClient)

	return &m
}

func (m *mgr) AllocateAddresses(ctx context.Context, svc *v1.Service) ([]string, error) {
	logger := logr.FromContextOrDiscard(ctx)

	families, err := serviceAddressFamilies(svc)
	if err != nil {
		return nil, err
	}

	currentAddresses := make(map[v1.IPFamily][]net.IP)

	for _, a := range serviceAddresses(svc) {
		ip := net.ParseIP(a)

		fam := v1.IPv6Protocol
		if ip.To4() != nil {
			fam = v1.IPv4Protocol
		}

		if _, ok := currentAddresses[fam]; !ok {
			currentAddresses[fam] = make([]net.IP, 0, 1)
		}

		currentAddresses[fam] = append(currentAddresses[fam], ip)
	}

	ret := make([]string, 0, len(families))

	for _, fam := range families {
		addresses, ok := currentAddresses[fam]

		if !ok {
			m.logger.V(1).Info("No addresses for IP family allocated yet", "family", fam)

			addr, err := m.allocateAddress(ctx, svc, fam)
			if err != nil {
				return nil, fmt.Errorf("error allocating address for family %q: %w", fam, err)
			}

			addresses = []net.IP{addr}
		}

		if len(addresses) > 1 {
			logger.Error(nil, "Multiple Ingress IPs of the same family on LoadBalancerStatus - this should not happen; still doing our best", "family", fam)
		}

		for _, a := range addresses {
			ret = append(ret, a.String())
		}
	}

	return ret, nil
}

func (m *mgr) prefixes(ctx context.Context) ([]*prefix, error) {
	if m.prefixCache != nil && m.prefixCacheTimestamp.Add(prefixCacheTimeout).After(time.Now()) {
		return m.prefixCache, nil
	}

	ret := make([]*prefix, 0)

	if m.fixedPrefixes != nil && len(m.fixedPrefixes) > 0 {
		ret = append(ret, m.fixedPrefixes...)
	}

	if m.prefixAutodiscover != nil {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		var oc types.ObjectChannel
		err := m.api.List(ctx, &corev1.Resource{Tags: []string{*m.prefixAutodiscover}}, api.ObjectChannel(&oc), api.FullObjects(true))
		if err != nil {
			return nil, fmt.Errorf("error listing resources with tag: %w", err)
		}

		for retriever := range oc {
			var res corev1.Resource
			err := retriever(&res)
			if err != nil {
				return nil, fmt.Errorf("error retrieving resource with tag: %w", err)
			}

			p, err := newPrefix(ctx, m.ipam.Prefix(), res.Identifier)
			if err != nil {
				m.logger.Error(err, "Retrieving prefix failed, doing my best continuing", "identifier", res.Identifier)
				continue
			}

			m.logger.V(2).Info("Found LoadBalancer Prefix via auto discovery", "identifier", p.identifier)
			ret = append(ret, p)
		}
	}

	m.prefixCache = ret
	m.prefixCacheTimestamp = time.Now()

	return ret, nil
}

func (m *mgr) allocateAddress(ctx context.Context, svc *v1.Service, fam v1.IPFamily) (net.IP, error) {
	log := logr.FromContextOrDiscard(ctx)

	prefixes, err := m.prefixes(ctx)
	if err != nil {
		return nil, err
	}

	// for every prefix, try to allocate an address from it, returning the first that works
	for _, p := range prefixes {
		if p.family == fam {
			ip, err := p.allocateAddress(ctx, fam)
			if err != nil {
				return nil, err
			}

			return ip, nil
		}
	}

	// When we got here, it means none of the available prefixes could allocate an address for us, meaning
	// we need a new prefix - but this is NotYetImplemented for Anexia Kubernetes Service MVP
	log.Info("no configured prefix was able to allocate an IP")
	return nil, cloudprovider.NotImplemented
}

func serviceAddressFamilies(svc *v1.Service) ([]v1.IPFamily, error) {
	families := svc.Spec.IPFamilies

	if externalFamiliesAnnotation, ok := svc.Annotations[lbaasExternalIPFamiliesAnnotation]; ok {
		familyStrings := strings.Split(externalFamiliesAnnotation, ",")
		families = make([]v1.IPFamily, 0, len(familyStrings))

		validFamilies := []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}

		for _, fam := range familyStrings {
			valid := false

			for _, validFam := range validFamilies {
				if fam == string(validFam) {
					valid = true
					break
				}
			}

			if !valid {
				return nil, fmt.Errorf("%w: %v is not a valid IPFamily", errInvalidIPFamiliesAnnotation, fam)
			}

			families = append(families, v1.IPFamily(fam))
		}
	}

	return families, nil
}

func serviceAddresses(svc *v1.Service) []string {
	status := svc.Status.LoadBalancer

	if status.Ingress == nil || len(status.Ingress) == 0 {
		return []string{}
	}

	ret := make([]string, 0, len(status.Ingress))

	for _, ing := range status.Ingress {
		if ing.IP != "" {
			ret = append(ret, ing.IP)
		}
	}

	return ret
}
