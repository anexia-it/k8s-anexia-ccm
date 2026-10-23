package loadbalancer

import (
	"context"
	"sync"
	"testing"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type staticAddressManager []string

func (m staticAddressManager) AllocateAddresses(context.Context, *v1.Service) ([]string, error) {
	return m, nil
}

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

var _ = Describe("Disabled Engine reconciliation", func() {
	It("should immediately return the desired service status without an Engine client", func() {
		manager := &mgr{
			logger:                    klog.NewKlogr(),
			disableEngineLoadBalancer: true,
			addressManager:            staticAddressManager{"192.0.2.10"},
			sync:                      &sync.Mutex{},
		}
		service := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: v1.ServiceSpec{
				Type:  v1.ServiceTypeLoadBalancer,
				Ports: []v1.ServicePort{{Name: "https", Port: 443}},
			},
		}

		status, err := manager.EnsureLoadBalancer(context.Background(), "test-cluster", service, []*v1.Node{{}})

		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(&v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{
			IP: "192.0.2.10",
			Ports: []v1.PortStatus{{
				Port:     443,
				Protocol: v1.ProtocolTCP,
			}},
		}}}))

		currentStatus, exists, err := manager.GetLoadBalancer(context.Background(), "test-cluster", service)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())
		Expect(currentStatus).To(Equal(status))
	})

	It("should treat deletion as an immediate no-op", func() {
		manager := &mgr{
			logger:                    klog.NewKlogr(),
			disableEngineLoadBalancer: true,
			addressManager:            staticAddressManager{"192.0.2.10"},
			sync:                      &sync.Mutex{},
		}

		Expect(manager.EnsureLoadBalancerDeleted(context.Background(), "test-cluster", &v1.Service{})).To(Succeed())
	})
})

func TestLoadBalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LBaaS operator")
}
