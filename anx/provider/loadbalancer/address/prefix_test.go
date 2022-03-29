package address

import (
	"context"
	"errors"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"

	anxprefix "go.anx.io/go-anxcloud/pkg/ipam/prefix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type mockAPIClient struct {
	// prefixes contains Identifier to Name
	prefixes map[string]string
}

func (m mockAPIClient) Get(ctx context.Context, identifier string) (anxprefix.Info, error) {
	if _, ok := m.prefixes[identifier]; !ok {
		return anxprefix.Info{}, errors.New("Not found")
	}

	return anxprefix.Info{
		ID:   identifier,
		Name: m.prefixes[identifier],
	}, nil
}

func (m mockAPIClient) Create(ctx context.Context, p anxprefix.Create) (anxprefix.Summary, error) {
	return anxprefix.Summary{}, errors.New("not implemented")
}

func (m mockAPIClient) List(ctx context.Context, page, limit int) ([]anxprefix.Summary, error) {
	return nil, errors.New("not implemented")
}

func (m mockAPIClient) Delete(ctx context.Context, identifier string) error {
	return errors.New("not implemented")
}

func (m mockAPIClient) Update(ctx context.Context, identifier string, update anxprefix.Update) (anxprefix.Summary, error) {
	return anxprefix.Summary{}, errors.New("not implemented")
}

var _ = Describe("prefix", func() {
	var apiClient anxprefix.API

	BeforeEach(func() {
		apiClient = mockAPIClient{
			prefixes: map[string]string{
				"v4":      "10.244.0.0/24",
				"v6":      "2001:db8::/64",
				"invalid": "newPrefixV4/28",
			},
		}
	})

	prefixTest := func(identifier string, expectedFamily v1.IPFamily) {
		Context(fmt.Sprintf("for family %v", expectedFamily), Ordered, func() {
			var p *prefix
			It("retrieves the prefix", func() {
				prfx, err := newPrefix(context.TODO(), apiClient, identifier)
				Expect(err).NotTo(HaveOccurred())

				p = prfx
				Expect(p.family).To(Equal(expectedFamily))
			})

			It("allocates an address for the family", func() {
				ip, err := p.allocateAddress(context.TODO(), expectedFamily)
				Expect(err).NotTo(HaveOccurred())

				if expectedFamily == v1.IPv4Protocol {
					Expect(ip).To(HaveLen(4))
				} else {
					Expect(ip).To(HaveLen(16))
				}
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

	prefixTest("v4", v1.IPv4Protocol)
	prefixTest("v6", v1.IPv6Protocol)
})

func TestPrefix(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "prefix test suite")
}
