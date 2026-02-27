package address

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	testhelper "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test"
	"go.anx.io/go-anxcloud/pkg/api"
	corev1 "go.anx.io/go-anxcloud/pkg/apis/core/v1"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"go.anx.io/go-anxcloud/pkg/ipam/address"
	anxprefix "go.anx.io/go-anxcloud/pkg/ipam/prefix"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/apimock"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/legacyapimock"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("prefix", func() {
	var a *testhelper.FakeAPI
	var c *gomock.Controller
	var ipamClient *legacyapimock.MockIPAMAPI
	var addressClient *legacyapimock.MockIPAMAddressAPI
	var prefixClient *legacyapimock.MockIPAMPrefixAPI
	var genericClient *apimock.MockAPI

	BeforeEach(func() {
		c = gomock.NewController(GinkgoT())
		ipamClient = legacyapimock.NewMockIPAMAPI(c)
		addressClient = legacyapimock.NewMockIPAMAddressAPI(c)
		prefixClient = legacyapimock.NewMockIPAMPrefixAPI(c)
		genericClient = apimock.NewMockAPI(c)
		ipamClient.EXPECT().Address().AnyTimes().Return(address.API(addressClient))
		ipamClient.EXPECT().Prefix().AnyTimes().Return(anxprefix.API(prefixClient))
	})

	prefixTest := func(withTaggedAddress bool, identifier string, expectedFamily v1.IPFamily, expectedPrefix, expectedAddress string) {
		Context(fmt.Sprintf("for family %v", expectedFamily), Ordered, func() {
			BeforeEach(func() {
				a = testhelper.NewFakeAPI(c)
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
					p, err = newPrefix(context.TODO(), a, ipamClient, identifier, ptr.To("clustername"))
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

	fallbackPrefixTest := func(identifier string, expectedFamily v1.IPFamily, expectedPrefix, expectedAddress string) {
		Context(fmt.Sprintf("for family %v", expectedFamily), Ordered, func() {
			BeforeEach(func() {
				a = testhelper.NewFakeAPI(c)
			})

			var p *prefix
			var err error
			It("retrieves the prefix and address", func() {
				prefixClient.EXPECT().Get(gomock.Any(), identifier).Return(anxprefix.Info{Name: expectedPrefix}, nil)

				p, err = newPrefix(context.TODO(), a, ipamClient, identifier, ptr.To("clustername"))

				Expect(err).NotTo(HaveOccurred())

				Expect(p.family).To(Equal(expectedFamily))
			})

			It("allocates an address for the family", func() {
				ip, err := p.allocateAddress(context.TODO(), expectedFamily)
				Expect(err).NotTo(HaveOccurred())

				Expect(ip.Equal(net.ParseIP(expectedAddress))).To(BeTrue())
			})
		})
	}

	fallbackPrefixTest("v4", v1.IPv4Protocol, "10.244.0.0/24", "10.244.0.254")
	fallbackPrefixTest("v6", v1.IPv6Protocol, "2001:db8::/64", "2001:db8::ffff:ffff:ffff:fffe")

	Context("discoverVIP", func() {
		p := &prefix{}

		It("returns an error when listing resources with provided tags resulted in an error other than 422 HTTP response", func() {
			genericClient.EXPECT().List(gomock.Any(), &corev1.Resource{Tags: []string{"vip-discovery-tag"}}, gomock.Any()).
				Return(api.NewHTTPError(http.StatusInternalServerError, "GET", nil, nil))

			addr, err := p.discoverVIP(context.TODO(), genericClient, ipamClient, "vip-discovery-tag")
			Expect(err).To(HaveOccurred())
			Expect(addr).To(BeNil())
		})

		It("returns no error when listing resources with provided tags resulted in a 422 response error", func() {
			genericClient.EXPECT().List(gomock.Any(), &corev1.Resource{Tags: []string{"vip-discovery-tag"}}, gomock.Any()).
				Return(api.NewHTTPError(http.StatusUnprocessableEntity, "GET", nil, nil))

			addr, err := p.discoverVIP(context.TODO(), genericClient, ipamClient, "vip-discovery-tag")
			Expect(err).To(BeNil())
			Expect(addr).To(BeNil())
		})
	})
})

func TestPrefix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "prefix test suite")
}
