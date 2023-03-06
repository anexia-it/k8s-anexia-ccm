package loadbalancer

import (
	"testing"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
	"k8s.io/klog/v2/klogr"
)

var _ = Describe("Initialization", func() {
	It("should initialize loadbalancer", func() {
		config := configuration.ProviderConfig{
			Token:      "RANDOM_VALUE",
			CustomerID: "CUSTOMER",
		}

		logger := klogr.NewWithOptions()

		legacyClient, _ := client.New(client.TokenFromString(config.Token))

		genericClient, _ := api.NewAPI(
			api.WithClientOptions(
				client.TokenFromString(config.Token),
			),
			api.WithLogger(logger.WithName("go-anxcloud")),
		)

		loadbalancer, err := New(&config, logger, nil, genericClient, legacyClient)

		Expect(loadbalancer).ToNot(BeNil())
		Expect(err).Error().ToNot(HaveOccurred())
	})
})

func TestLoadBalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LBaaS operator")
}
