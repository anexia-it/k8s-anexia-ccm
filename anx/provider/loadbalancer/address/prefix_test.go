package address

import (
	"context"
	"fmt"
	"net"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"go.anx.io/go-anxcloud/pkg/api/mock"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"go.anx.io/go-anxcloud/pkg/ipam/address"
	anxprefix "go.anx.io/go-anxcloud/pkg/ipam/prefix"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/legacyapimock"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("prefix", func() {
	var a mock.API
	var c *gomock.Controller
	var ipamClient *legacyapimock.MockIPAMAPI
	var addressClient *legacyapimock.MockIPAMAddressAPI
	var prefixClient *legacyapimock.MockIPAMPrefixAPI

	BeforeEach(func() {
		c = gomock.NewController(GinkgoT())
		ipamClient = legacyapimock.NewMockIPAMAPI(c)
		addressClient = legacyapimock.NewMockIPAMAddressAPI(c)
		prefixClient = legacyapimock.NewMockIPAMPrefixAPI(c)
		ipamClient.EXPECT().Address().AnyTimes().Return(addressClient)
		ipamClient.EXPECT().Prefix().AnyTimes().Return(prefixClient)
	})

	prefixTest := func(withTaggedAddress bool, identifier string, expectedFamily v1.IPFamily, expectedPrefix, expectedAddress string) {
		Context(fmt.Sprintf("for family %v", expectedFamily), Ordered, func() {
			BeforeEach(func() {
				a = mock.NewMockAPI()
				if withTaggedAddress {
					// used to find a tagged resources identifier
					// using lbaasv1.Backend is very hacky.. but currently except the lbaas
					// resources no other resources have the ObjectCoreType defined in the mock client
					a.FakeExisting(&lbaasv1.Backend{Identifier: "test-identifier"}, "kubernetes-lb-vip-clustername")
				}
			})

			var p *prefix
			var err error
			It("retrieves the prefix and address", func() {
				prefixClient.EXPECT().Get(gomock.Any(), identifier).Return(anxprefix.Info{Name: expectedPrefix}, nil)
				if withTaggedAddress {
					addressClient.EXPECT().Get(gomock.Any(), "test-identifier").Return(address.Address{Name: expectedAddress, PrefixID: identifier}, nil)
				}

				if withTaggedAddress {
					p, err = newPrefix(context.TODO(), a, ipamClient, identifier, pointer.String("clustername"))
				} else {
					p, err = newPrefix(context.TODO(), a, ipamClient, identifier, nil)
				}
				Expect(err).NotTo(HaveOccurred())

				Expect(p.family).To(Equal(expectedFamily))
			})

			It("allocates an address for the family", func() {
				ip, err := p.allocateAddress(context.TODO(), expectedFamily)
				Expect(err).NotTo(HaveOccurred())

				Expect(ip.Equal(net.ParseIP(expectedAddress))).To(BeTrue())
			})

			It("returns the correct error when allocating for wrong family", func() {
				family := v1.IPv4Protocol
				if expectedFamily == family {
					family = v1.IPv6Protocol
				}

				_, err := p.allocateAddress(context.TODO(), family)
				Expect(err).To(MatchError(errFamilyMismatch))
			})
		})
	}

	prefixTest(true, "v4", v1.IPv4Protocol, "10.244.0.0/24", "10.244.0.5")
	prefixTest(true, "v6", v1.IPv6Protocol, "2001:db8::/64", "2001:db8::ffff:ffff:ffff:fff5")

	// test legacy code path (use address at prefix[-2])
	prefixTest(false, "v4", v1.IPv4Protocol, "10.244.0.0/24", "10.244.0.254")
	prefixTest(false, "v6", v1.IPv6Protocol, "2001:db8::/64", "2001:db8::ffff:ffff:ffff:fffe")
})

func TestPrefix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "prefix test suite")
}
