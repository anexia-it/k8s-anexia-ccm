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
	"go.anx.io/go-anxcloud/pkg/api/mock"
	gs "go.anx.io/go-anxcloud/pkg/apis/common/gs"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
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

var _ = Describe("Engine deletion", func() {
	It("should destroy all service resources after changing to ClusterIP without a deletion timestamp", func() {
		apiClient := mock.NewMockAPI()
		lb := lbaasv1.LoadBalancer{Identifier: "test-lb"}
		apiClient.FakeExisting(&lb)
		service := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default", UID: "test-uid"},
			Spec: v1.ServiceSpec{
				Type:  v1.ServiceTypeClusterIP,
				Ports: []v1.ServicePort{{Name: "https", Port: 443, NodePort: 30443}},
			},
		}
		manager := &mgr{
			logger: klog.NewKlogr(), api: apiClient, clusterName: "test-cluster",
			addressManager: staticAddressManager{"192.0.2.10"},
			loadBalancers:  []string{lb.Identifier}, sync: &sync.Mutex{}, backoffSteps: 10,
			metrics: metrics.NewProviderMetrics("anexia", "0.0.0-unit-tests"),
		}
		tag := "anxccm-svc-uid=" + string(service.UID)
		name := "https." + manager.GetLoadBalancerName(context.Background(), "test-cluster", service)
		state := gs.HasState{State: lbaasv1.Deployed}
		backend := apiClient.FakeExisting(&lbaasv1.Backend{
			Name: name, Mode: lbaasv1.TCP, HealthCheck: `"adv_check": "tcp-check"`,
			LoadBalancer: lb, HasState: state,
		}, tag)
		frontend := apiClient.FakeExisting(&lbaasv1.Frontend{
			Name: name, Mode: lbaasv1.TCP, LoadBalancer: &lb,
			DefaultBackend: &lbaasv1.Backend{Identifier: backend}, HasState: state,
		}, tag)
		bind := apiClient.FakeExisting(&lbaasv1.Bind{
			Name: "v4." + name, Address: "192.0.2.10", Port: 443,
			Frontend: lbaasv1.Frontend{Identifier: frontend}, HasState: state,
		}, tag)
		server := apiClient.FakeExisting(&lbaasv1.Server{
			Name: name, IP: "10.0.0.1", Port: 30443,
			Backend: lbaasv1.Backend{Identifier: backend}, HasState: state,
		}, tag)

		Expect(manager.EnsureLoadBalancerDeleted(context.Background(), "test-cluster", service)).To(Succeed())
		for _, identifier := range []string{backend, frontend, bind, server} {
			Expect(apiClient.Inspect(identifier).Existing()).To(BeFalse(), identifier)
			Expect(apiClient.Inspect(identifier).DestroyedCount()).To(Equal(1), identifier)
		}
	})
})

func TestLoadBalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LBaaS operator")
}
