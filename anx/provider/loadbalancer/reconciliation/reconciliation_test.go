package reconciliation

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/test"
	"github.com/golang/mock/gomock"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	gs "go.anx.io/go-anxcloud/pkg/apis/common/gs"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	. "go.anx.io/go-anxcloud/pkg/api/mock/matcher"
	kubemetrics "k8s.io/component-base/metrics"
)

const (
	testClusterName            = "testcluster"
	testLoadBalancerIdentifier = "testLoadBalancerEngineIdentifier"
)

var _ = Describe("reconcile", func() {
	var apiClient *test.FakeAPI
	var mockCtrl *gomock.Controller

	var recon *reconciliation

	// these are configured in BeforeEach for different tests
	var svcUID string
	var externalAddresses []net.IP
	var ports map[string]Port
	var servers []Server

	var providerMetrics metrics.ProviderMetrics
	var kubeRegistry kubemetrics.KubeRegistry

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		apiClient = test.NewFakeAPI(mockCtrl)
		apiClient.SetPreCreateHook(func(ctx context.Context, a api.API, o types.Object) {
			if server, ok := o.(*lbaasv1.Server); ok {
				server.State.Type = gs.StateTypeOK
			}
		})

		apiClient.FakeExisting(&lbaasv1.LoadBalancer{
			Identifier: testLoadBalancerIdentifier,
		})

		svcUID = rand.String(32)

		externalAddresses = []net.IP{
			net.ParseIP("8.8.8.8"),
			net.ParseIP("2001:4860:4860::8888"),
		}

		ports = map[string]Port{
			"http": {
				Internal: 42037,
				External: 80,
			},

			"https": {
				Internal: 37042,
				External: 443,
			},
		}

		servers = []Server{
			{
				Name:    "test-server-01",
				Address: net.ParseIP("10.244.0.4"),
			},
			{
				Name:    "test-server-02",
				Address: net.ParseIP("8.8.8.8"),
			},
		}
	})

	JustBeforeEach(func() {
		ctx := context.TODO()

		providerMetrics = metrics.NewProviderMetrics("anexia", "0.0.0-unit-tests")
		kubeRegistry = kubemetrics.NewKubeRegistry()
		kubeRegistry.MustRegister(providerMetrics.ReconciliationPendingResources)

		r, err := New(ctx, apiClient, testClusterName, testLoadBalancerIdentifier, svcUID, externalAddresses, ports, servers, 10, providerMetrics)
		Expect(err).NotTo(HaveOccurred())

		recon = r.(*reconciliation)
	})

	Context("with existing resources but none matching our tag", func() {
		JustBeforeEach(func() {
			apiClient.FakeExisting(&lbaasv1.Frontend{Name: "foo"})
			apiClient.FakeExisting(&lbaasv1.Bind{Name: "foo"})
			apiClient.FakeExisting(&lbaasv1.Backend{Name: "foo"})
			apiClient.FakeExisting(&lbaasv1.Server{Name: "foo"})

			err := recon.retrieveState()
			Expect(err).NotTo(HaveOccurred())
		})

		It("retrieves no resources", func() {
			Expect(recon.frontends).To(HaveLen(0))
			Expect(recon.binds).To(HaveLen(0))
			Expect(recon.backends).To(HaveLen(0))
			Expect(recon.servers).To(HaveLen(0))
		})
	})

	Context("with some existing but invalid resources", func() {
		var expectDestroyFrontends []string
		var expectDestroyBinds []string
		var expectDestroyBackends []string
		var expectDestroyServers []string

		JustBeforeEach(func() {
			expectDestroyFrontends = []string{
				apiClient.FakeExisting(&lbaasv1.Frontend{
					Name: "foo",
					LoadBalancer: &lbaasv1.LoadBalancer{
						Identifier: testLoadBalancerIdentifier,
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			expectDestroyBinds = []string{
				apiClient.FakeExisting(&lbaasv1.Bind{
					Name: "foo",
					Frontend: lbaasv1.Frontend{
						Identifier: expectDestroyFrontends[0],
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			expectDestroyBackends = []string{
				apiClient.FakeExisting(&lbaasv1.Backend{
					Name: "foo",
					LoadBalancer: lbaasv1.LoadBalancer{
						Identifier: testLoadBalancerIdentifier,
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			expectDestroyServers = []string{
				apiClient.FakeExisting(&lbaasv1.Server{
					Name: "foo",
					Backend: lbaasv1.Backend{
						Identifier: expectDestroyBackends[0],
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			err := recon.retrieveState()
			Expect(err).NotTo(HaveOccurred())
		})

		It("retrieves everything", func() {
			Expect(recon.frontends).To(HaveLen(1))
			Expect(recon.binds).To(HaveLen(1))
			Expect(recon.backends).To(HaveLen(1))
			Expect(recon.servers).To(HaveLen(1))
		})

		type reconcileFunction func() ([]types.Object, []types.Object, error)
		reconcileFrontends := func() reconcileFunction { return recon.reconcileFrontends }
		reconcileBinds := func() reconcileFunction { return recon.reconcileBinds }
		reconcileBackends := func() reconcileFunction { return recon.reconcileBackends }
		reconcileServers := func() reconcileFunction { return recon.reconcileServers }

		DescribeTable("it destroys the invalid resources",
			func(r func() reconcileFunction, expStrings *[]string) {
				_, toDestroy, err := r()()
				Expect(err).NotTo(HaveOccurred())

				exp := make([]interface{}, len(*expStrings))
				for i, identifier := range *expStrings {
					exp[i] = identifier
				}

				Expect(toDestroy).To(WithTransform(
					func(o []types.Object) []string {
						ret := make([]string, 0, len(o))
						for _, e := range o {
							identifier, err := types.GetObjectIdentifier(e, true)
							Expect(err).NotTo(HaveOccurred())
							ret = append(ret, identifier)
						}
						return ret
					},
					ContainElements(exp...),
				))
			},
			Entry("frontends", reconcileFrontends, &expectDestroyFrontends),
			Entry("binds", reconcileBinds, &expectDestroyBinds),
			Entry("backends", reconcileBackends, &expectDestroyBackends),
			Entry("servers", reconcileServers, &expectDestroyServers),
		)
	})

	Context("with some existing and valid resources", func() {
		var httpBackendIdentifier string
		var httpsBackendIdentifier string
		var httpFrontendIdentifier string
		var httpsFrontendIdentifier string

		JustBeforeEach(func() {
			httpBackendIdentifier = apiClient.FakeExisting(&lbaasv1.Backend{
				Name:         "http." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				HasState:     gs.HasState{State: lbaasv1.NewlyCreated},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsBackendIdentifier = apiClient.FakeExisting(&lbaasv1.Backend{
				Name:         "https." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				HasState:     gs.HasState{State: lbaasv1.NewlyCreated},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpFrontendIdentifier = apiClient.FakeExisting(&lbaasv1.Frontend{
				Name:           "http." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpBackendIdentifier},
				HasState:       gs.HasState{State: lbaasv1.NewlyCreated},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsFrontendIdentifier = apiClient.FakeExisting(&lbaasv1.Frontend{
				Name:           "https." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				HasState:       gs.HasState{State: lbaasv1.NewlyCreated},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.http." + testClusterName,
				Address:  "8.8.8.8",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.https." + testClusterName,
				Address:  "8.8.8.8",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.http." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.https." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Server{
				Name:     "https.invalid-server." + testClusterName,
				IP:       "10.244.1.1",
				Port:     4223,
				Check:    "disabled",
				Backend:  lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			err := recon.retrieveState()
			Expect(err).NotTo(HaveOccurred())
		})

		It("retrieves everything", func() {
			Expect(recon.frontends).To(HaveLen(2))
			Expect(recon.binds).To(HaveLen(4))
			Expect(recon.backends).To(HaveLen(2))
			Expect(recon.servers).To(HaveLen(1))
		})

		It("should fail to wait for the resources to get ready due to low backoff steps", func() {
			ctx := context.TODO()

			metrics := metrics.NewProviderMetrics("anexia", "0.0.0-unit-tests")

			// Override reconciliation with only 1 backoff step
			r, err := New(ctx, apiClient, testClusterName, testLoadBalancerIdentifier, svcUID, externalAddresses, ports, servers, 1, metrics)
			Expect(err).NotTo(HaveOccurred())

			recon = r.(*reconciliation)

			err = recon.Reconcile()

			Expect(wait.Interrupted(err)).To(BeTrue())
		})

		It("waits for the resources to get ready", func() {
			const waitTime = 4 * time.Second

			time.AfterFunc(waitTime, func() {
				GinkgoRecover()

				objects := []types.Object{
					&lbaasv1.Backend{Identifier: httpBackendIdentifier},
					&lbaasv1.Backend{Identifier: httpsBackendIdentifier},
					&lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
					&lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
				}

				for _, o := range objects {
					err := apiClient.Get(context.TODO(), o)
					Expect(err).NotTo(HaveOccurred())

					switch obj := o.(type) {
					case *lbaasv1.Backend:
						obj.State = lbaasv1.Deployed
					case *lbaasv1.Frontend:
						obj.State = lbaasv1.Deployed
					}

					err = apiClient.Update(context.TODO(), o)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			timeStart := time.Now()
			timeEnd := timeStart.Add(waitTime)
			err := recon.Reconcile()

			Expect(err).NotTo(HaveOccurred())
			Expect(time.Now()).To(BeTemporally("~", timeEnd, waitTime+1*time.Second))
		})

		It("accepts the existing resources as already correct", func() {
			toCreate, toDestroy, err := recon.reconcileBackends()
			Expect(err).NotTo(HaveOccurred())
			Expect(toCreate).To(HaveLen(0))
			Expect(toDestroy).To(HaveLen(0))

			toCreate, toDestroy, err = recon.reconcileFrontends()
			Expect(err).NotTo(HaveOccurred())
			Expect(toCreate).To(HaveLen(0))
			Expect(toDestroy).To(HaveLen(0))

			toCreate, toDestroy, err = recon.reconcileBinds()
			Expect(err).NotTo(HaveOccurred())
			Expect(toCreate).To(HaveLen(0))
			Expect(toDestroy).To(HaveLen(0))
		})

		It("creates the correct server entries", func() {
			_, _, err := recon.reconcileBackends()
			Expect(err).NotTo(HaveOccurred())
			_, _, err = recon.reconcileFrontends()
			Expect(err).NotTo(HaveOccurred())
			_, _, err = recon.reconcileBinds()
			Expect(err).NotTo(HaveOccurred())

			toCreate, toDestroy, err := recon.reconcileServers()
			Expect(err).NotTo(HaveOccurred())
			Expect(toCreate).To(HaveLen(4))
			Expect(toDestroy).To(HaveLen(1))

			Expect(toDestroy[0].(*lbaasv1.Server).Name).To(Equal("https.invalid-server." + testClusterName))
			Expect(toDestroy[0].(*lbaasv1.Server).IP).To(Equal("10.244.1.1"))
			Expect(toDestroy[0].(*lbaasv1.Server).Port).To(Equal(4223))
			Expect(toDestroy[0].(*lbaasv1.Server).Check).To(Equal("disabled"))
			Expect(toDestroy[0].(*lbaasv1.Server).Backend.Identifier).To(Equal(httpsBackendIdentifier))

			expected := []lbaasv1.Server{
				{
					Name:    "test-server-01.http." + testClusterName,
					IP:      "10.244.0.4",
					Port:    42037,
					Check:   "enabled",
					Backend: lbaasv1.Backend{Identifier: httpBackendIdentifier},
				}, {
					Name:    "test-server-01.https." + testClusterName,
					IP:      "10.244.0.4",
					Port:    37042,
					Check:   "enabled",
					Backend: lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				}, {
					Name:    "test-server-02.http." + testClusterName,
					IP:      "8.8.8.8", // we prefer ExternalIP over InternalIP
					Port:    42037,
					Check:   "enabled",
					Backend: lbaasv1.Backend{Identifier: httpBackendIdentifier},
				}, {
					Name:    "test-server-02.https." + testClusterName,
					IP:      "8.8.8.8", // we prefer ExternalIP over InternalIP
					Port:    37042,
					Check:   "enabled",
					Backend: lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				},
			}

			for _, newObject := range toCreate {
				found := false
				for _, exp := range expected {
					if newServer := newObject.(*lbaasv1.Server); newServer.Name == exp.Name {
						found = true
						Expect(newServer.IP).To(Equal(exp.IP))
						Expect(newServer.Port).To(Equal(exp.Port))
						Expect(newServer.Check).To(Equal(exp.Check))
						Expect(newServer.Backend.Identifier).To(Equal(exp.Backend.Identifier))
					}
				}
				Expect(found).To(BeTrue())
			}
		})
	})

	Context("with all resources already existing", func() {
		var httpBackendIdentifier string
		var httpsBackendIdentifier string

		JustBeforeEach(func() {
			httpBackendIdentifier = apiClient.FakeExisting(&lbaasv1.Backend{
				Name:         "http." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				HasState:     gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsBackendIdentifier = apiClient.FakeExisting(&lbaasv1.Backend{
				Name:         "https." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				HasState:     gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpFrontendIdentifier := apiClient.FakeExisting(&lbaasv1.Frontend{
				Name:           "http." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpBackendIdentifier},
				HasState:       gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsFrontendIdentifier := apiClient.FakeExisting(&lbaasv1.Frontend{
				Name:           "https." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				HasState:       gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.http." + testClusterName,
				Address:  "8.8.8.8",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.https." + testClusterName,
				Address:  "8.8.8.8",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.http." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.https." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Server{
				Name:     "test-server-01.http." + testClusterName,
				IP:       "10.244.0.4",
				Port:     42037,
				Check:    "enabled",
				Backend:  lbaasv1.Backend{Identifier: httpBackendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Server{
				Name:     "test-server-01.https." + testClusterName,
				IP:       "10.244.0.4",
				Port:     37042,
				Check:    "enabled",
				Backend:  lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Server{
				Name:     "test-server-02.http." + testClusterName,
				IP:       "8.8.8.8",
				Port:     42037,
				Check:    "enabled",
				Backend:  lbaasv1.Backend{Identifier: httpBackendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			apiClient.FakeExisting(&lbaasv1.Server{
				Name:     "test-server-02.https." + testClusterName,
				IP:       "8.8.8.8",
				Port:     37042,
				Check:    "enabled",
				Backend:  lbaasv1.Backend{Identifier: httpsBackendIdentifier},
				HasState: gs.HasState{State: lbaasv1.Deployed},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			err := recon.retrieveState()
			Expect(err).NotTo(HaveOccurred())
		})

		It("retrieves everything", func() {
			Expect(recon.frontends).To(HaveLen(2))
			Expect(recon.binds).To(HaveLen(4))
			Expect(recon.backends).To(HaveLen(2))
			Expect(recon.servers).To(HaveLen(4))
		})

		It("accepts the existing resources as already correct", func() {
			toCreate, toDestroy, err := recon.ReconcileCheck()
			Expect(err).NotTo(HaveOccurred())
			Expect(toCreate).To(HaveLen(0))
			Expect(toDestroy).To(HaveLen(0))
		})

		Context("destroy and create a server in a single Reconcile call", func() {
			BeforeEach(func() {
				servers = []Server{
					{
						Name:    "test-server-01",
						Address: net.ParseIP("10.244.0.4"),
					},
					{
						Name:    "test-server-03",
						Address: net.ParseIP("8.8.4.4"),
					},
				}
			})

			It("sets the `cloud_provider_anexia_reconcile_resources_pending` back to 0 after successful reconciliation", func() {
				err := recon.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				err = testutil.GatherAndCompare(kubeRegistry, strings.NewReader(`
				# HELP cloud_provider_anexia_reconcile_resources_pending [ALPHA] Gauge of pending creation or deletion operations of resources
				# TYPE cloud_provider_anexia_reconcile_resources_pending gauge
				cloud_provider_anexia_reconcile_resources_pending{operation="create",service="lbaas"} 0
				cloud_provider_anexia_reconcile_resources_pending{operation="destroy",service="lbaas"} 0
				`), "cloud_provider_anexia_reconcile_resources_pending")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("deleting the service", func() {
			BeforeEach(func() {
				externalAddresses = make([]net.IP, 0)
				ports = make(map[string]Port)
				servers = make([]Server, 0)
			})

			It("detects all resources as having to be destroyed", func() {
				// -1: do not destroy the LoadBalancer itself
				destroyCount := len(apiClient.Existing()) - 1

				toCreate, toDestroy, err := recon.ReconcileCheck()
				Expect(err).NotTo(HaveOccurred())
				Expect(toCreate).To(HaveLen(0))
				Expect(toDestroy).To(HaveLen(destroyCount))
			})

			It("destroys all resources", func() {
				err := recon.Reconcile()
				Expect(err).NotTo(HaveOccurred())

				Expect(apiClient.Existing()).To(
					ConsistOf(
						Object(&lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier}, "Identifier"),
					),
				)
				Expect(apiClient.Existing()).To(HaveLen(1))
			})
		})
	})
})

func TestReconcilation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LoadBalancer reconciliation test suite")
}
