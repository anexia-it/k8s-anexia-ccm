package provider

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	cloudprovider "k8s.io/cloud-provider"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Initialization", func() {
	It("should initialize a new provider", func() {
		provider, err := newAnxProvider(configuration.ProviderConfig{
			Token:      "RANDOME_VALUE",
			CustomerID: "CUSTOMER",
		})
		Expect(err).Error().ToNot(HaveOccurred())
		Expect(provider).ToNot(BeNil())

		provider.Initialize(nil, nil)
		instances, instancesEnabled := provider.Instances()
		instancesV2, instancesV2Enabled := provider.InstancesV2()
		zones, zonesEnabled := provider.Zones()
		hasClusterID := provider.HasClusterID()
		routes, routesEnabled := provider.Routes()
		clusters, clustersEnabled := provider.Clusters()
		providerName := provider.ProviderName()
		loadbalancer, loadbalancerEnabled := provider.LoadBalancer()

		Expect(instances).To(BeNil())
		Expect(instancesV2).ToNot(BeNil())
		Expect(provider.providerMetrics).ToNot(BeNil())
		Expect(zones).To(BeNil())
		Expect(routes).To(BeNil())
		Expect(clusters).To(BeNil())
		Expect(loadbalancer).ToNot(BeNil())
		Expect(instancesEnabled).To(BeFalse())
		Expect(instancesV2Enabled).To(BeTrue())
		Expect(zonesEnabled).To(BeFalse())
		Expect(routesEnabled).To(BeFalse())
		Expect(hasClusterID).To(BeTrue())
		Expect(clustersEnabled).To(BeFalse())
		Expect(loadbalancerEnabled).To(BeTrue())

		Expect(configuration.CloudProviderName).To(Equal(providerName))
		Expect(instanceManager{}).To(BeAssignableToTypeOf(instancesV2))
		manager := instancesV2.(instanceManager)
		Expect(provider).To(Equal(manager.Provider))
	})

	It("should work with incomplete initialization", func() {
		p := &anxProvider{
			// we initialize `providerMetrics` manually because
			// `anxProvider.setupProviderMetrics()` panics when called a second time
			providerMetrics: metrics.NewProviderMetrics("anexia", Version),
		}

		loadbalancer, loadbalancerEnabled := p.LoadBalancer()
		Expect(loadbalancerEnabled).To(BeFalse())
		Expect(loadbalancer).To(BeNil())

		instancesV2, instancesV2Enabled := p.InstancesV2()
		Expect(instancesV2Enabled).To(BeFalse())
		Expect(instancesV2).To(BeNil())
	})

	Context("Register cloud provider", func() {
		BeforeEach(func() {
			Expect(os.Setenv("ANEXIA_TOKEN", "TOKEN")).Error().ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.Unsetenv("ANEXIA_TOKEN")).Error().ToNot(HaveOccurred())
		})

		It("should register cloud provider", func() {
			provider, err := cloudprovider.GetCloudProvider("anexia", bytes.NewReader([]byte("customerID: 555")))
			Expect(err).Error().ToNot(HaveOccurred())
			Expect(provider).ToNot(BeNil())

			anxProvider, ok := provider.(*anxProvider)
			Expect(ok).To(BeTrue())
			config := anxProvider.Config()
			Expect(config.CustomerID).To(Equal("555"))
			Expect(config.Token).To(Equal("TOKEN"))
		})
	})
})

var _ = Describe("Config", func() {
	It("should test provider schema", func() {
		Expect(fmt.Sprintf("%s://", configuration.CloudProviderName)).To(Equal(configuration.CloudProviderScheme))
	})

	It("should test provider config", func() {
		provider, err := newAnxProvider(configuration.ProviderConfig{
			Token:      "5555",
			CustomerID: "5555",
		})
		Expect(err).Error().ToNot(HaveOccurred())

		config := provider.Config()
		Expect(config.CustomerID).To(Equal("5555"))
	})
})

func TestProviderSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Suite")
}
