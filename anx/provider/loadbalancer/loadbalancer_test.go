package loadbalancer

import (
	"testing"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
	"k8s.io/klog/v2"
)

var _ = Describe("Initialization", func() {
	It("should initialize loadbalancer", func() {
		config := configuration.ProviderConfig{
			Token:      "RANDOM_VALUE",
			CustomerID: "CUSTOMER",
		}

		logger := klog.NewKlogr()

		legacyClient, _ := client.New(client.TokenFromString(config.Token))

		genericClient, _ := api.NewAPI(
			api.WithClientOptions(
				client.TokenFromString(config.Token),
			),
			api.WithLogger(logger.WithName("go-anxcloud")),
		)

		metrics := metrics.NewProviderMetrics("anexia", "0.0.0-unit-tests")

		loadbalancer, err := New(&config, logger, nil, genericClient, legacyClient, metrics)

		Expect(loadbalancer).ToNot(BeNil())
		Expect(err).Error().ToNot(HaveOccurred())
	})
})

func TestLoadBalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LBaaS operator")
}
