package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/kubernetes/resourceclient"
	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"
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
		Expect(&instanceManager{}).To(BeAssignableToTypeOf(instancesV2))
		manager := instancesV2.(*instanceManager)
		Expect(provider).To(Equal(manager.Provider))
	})

	It("should keep LoadBalancer support enabled when Engine reconciliation is disabled", func() {
		provider, err := newAnxProvider(configuration.ProviderConfig{
			Token:                     "RANDOME_VALUE",
			CustomerID:                "CUSTOMER",
			DisableEngineLoadBalancer: true,
		})
		Expect(err).Error().ToNot(HaveOccurred())
		Expect(provider).ToNot(BeNil())

		provider.Initialize(nil, nil)
		loadbalancer, loadbalancerEnabled := provider.LoadBalancer()

		Expect(loadbalancer).ToNot(BeNil())
		Expect(loadbalancerEnabled).To(BeTrue())
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

var _ = Describe("AKE Loadbalancer detection", func() {
	It("should apply the existing disable configuration when a resource exists", func() {
		var requestPath, requestLimit string
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestPath = request.URL.Path
			requestLimit = request.URL.Query().Get("limit")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
				"apiVersion":"ake.k8s.anx.io/v1",
				"kind":"LoadbalancerList",
				"metadata":{},
				"items":[{
					"apiVersion":"ake.k8s.anx.io/v1",
					"kind":"Loadbalancer",
					"metadata":{"name":"test","namespace":"ake-hacomaco"}
				}]
			}`))
		}))
		DeferCleanup(server.Close)

		provider := &anxProvider{
			config: &configuration.ProviderConfig{},
			logger: logr.Discard(),
		}
		provider.disableEngineLoadBalancerForAKE(staticControllerClientBuilder{
			config: &rest.Config{Host: server.URL},
		})

		Expect(provider.config.DisableEngineLoadBalancer).To(BeTrue())
		Expect(requestPath).To(Equal("/apis/ake.k8s.anx.io/v1/loadbalancers"))
		Expect(requestLimit).To(Equal("1"))
	})

	It("should not query Kubernetes when reconciliation is already disabled explicitly", func() {
		provider := &anxProvider{
			config: &configuration.ProviderConfig{DisableEngineLoadBalancer: true},
			logger: logr.Discard(),
		}

		provider.disableEngineLoadBalancerForAKE(staticControllerClientBuilder{})

		Expect(provider.config.DisableEngineLoadBalancer).To(BeTrue())
	})

	It("should detect a Loadbalancer in any namespace", func() {
		client := newAKELoadBalancerResourceClient(&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "ake.k8s.anx.io/v1",
			"kind":       "Loadbalancer",
			"metadata": map[string]interface{}{
				"name":      "b06c68f4a2154fe58d11ae12bed7039f",
				"namespace": "ake-hacomaco",
			},
		}})

		exists, err := akeLoadBalancerExists(context.Background(), client)

		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should return false when no Loadbalancer exists", func() {
		exists, err := akeLoadBalancerExists(context.Background(), newAKELoadBalancerResourceClient())

		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should treat an absent CRD as no Loadbalancer", func() {
		dynamicClient := newAKEDynamicClient()
		dynamicClient.PrependReactor("list", "loadbalancers", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(akeLoadBalancerTarget.GroupVersionResource().GroupResource(), "")
		})
		client, err := resourceclient.New(dynamicClient, akeLoadBalancerTarget)
		Expect(err).NotTo(HaveOccurred())

		exists, err := akeLoadBalancerExists(context.Background(), client)

		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should return errors other than an absent CRD", func() {
		dynamicClient := newAKEDynamicClient()
		dynamicClient.PrependReactor("list", "loadbalancers", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(
				akeLoadBalancerTarget.GroupVersionResource().GroupResource(),
				"",
				errors.New("RBAC denied"),
			)
		})
		client, err := resourceclient.New(dynamicClient, akeLoadBalancerTarget)
		Expect(err).NotTo(HaveOccurred())

		exists, err := akeLoadBalancerExists(context.Background(), client)

		Expect(exists).To(BeFalse())
		Expect(apierrors.IsForbidden(err)).To(BeTrue())
	})
})

type staticControllerClientBuilder struct {
	config *rest.Config
}

func (b staticControllerClientBuilder) Config(string) (*rest.Config, error) {
	if b.config == nil {
		return nil, errors.New("unexpected Config call")
	}
	return b.config, nil
}

func (b staticControllerClientBuilder) ConfigOrDie(string) *rest.Config {
	return b.config
}

func (staticControllerClientBuilder) Client(string) (kubernetes.Interface, error) {
	return nil, nil
}

func (staticControllerClientBuilder) ClientOrDie(string) kubernetes.Interface {
	return nil
}

func newAKELoadBalancerResourceClient(objects ...runtime.Object) *resourceclient.Client {
	client, err := resourceclient.New(newAKEDynamicClient(objects...), akeLoadBalancerTarget)
	Expect(err).NotTo(HaveOccurred())
	return client
}

func newAKEDynamicClient(objects ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			akeLoadBalancerTarget.GroupVersionResource(): "LoadbalancerList",
		},
		objects...,
	)
}

func TestProviderSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Suite")
}
