package reconciliation

import (
	"context"
	"fmt"
	"net"
	"testing"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"

	"k8s.io/apimachinery/pkg/util/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	testClusterName            = "testcluster"
	testLoadBalancerIdentifier = "testLoadBalancerEngineIdentifier"
)

var _ = Describe("reconcile", func() {
	var apiClient api.API
	var mock LBaaSMock

	var recon *reconciliation

	// these are configured in BeforeEach for different tests
	var svcUID string
	var externalAddresses []net.IP
	var ports map[string]Port
	var servers []Server

	BeforeEach(func() {
		apiClient, mock = lbaasMockAPI()

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

		r, err := New(ctx, apiClient, testClusterName, testLoadBalancerIdentifier, svcUID, externalAddresses, ports, servers)
		Expect(err).NotTo(HaveOccurred())

		recon = r.(*reconciliation)
	})

	Context("with existing resources but none matching our tag", func() {
		JustBeforeEach(func() {
			mock.FakeExisting(&lbaasv1.Frontend{Name: "foo"})
			mock.FakeExisting(&lbaasv1.Bind{Name: "foo"})
			mock.FakeExisting(&lbaasv1.Backend{Name: "foo"})
			mock.FakeExisting(&lbaasv1.Server{Name: "foo"})

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
				mock.FakeExisting(&lbaasv1.Frontend{
					Name: "foo",
					LoadBalancer: &lbaasv1.LoadBalancer{
						Identifier: testLoadBalancerIdentifier,
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			expectDestroyBinds = []string{
				mock.FakeExisting(&lbaasv1.Bind{
					Name: "foo",
					Frontend: lbaasv1.Frontend{
						Identifier: expectDestroyFrontends[0],
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			expectDestroyBackends = []string{
				mock.FakeExisting(&lbaasv1.Backend{
					Name: "foo",
					LoadBalancer: lbaasv1.LoadBalancer{
						Identifier: testLoadBalancerIdentifier,
					},
				}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID)),
			}

			expectDestroyServers = []string{
				mock.FakeExisting(&lbaasv1.Server{
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
							identifier, err := api.GetObjectIdentifier(e, true)
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

		JustBeforeEach(func() {
			httpBackendIdentifier = mock.FakeExisting(&lbaasv1.Backend{
				Name:         "http." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsBackendIdentifier = mock.FakeExisting(&lbaasv1.Backend{
				Name:         "https." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpFrontendIdentifier := mock.FakeExisting(&lbaasv1.Frontend{
				Name:           "http." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsFrontendIdentifier := mock.FakeExisting(&lbaasv1.Frontend{
				Name:           "https." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpsBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.http." + testClusterName,
				Address:  "8.8.8.8",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.https." + testClusterName,
				Address:  "8.8.8.8",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.http." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.https." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Server{
				Name:    "https.invalid-server." + testClusterName,
				IP:      "10.244.1.1",
				Port:    4223,
				Check:   "disabled",
				Backend: lbaasv1.Backend{Identifier: httpsBackendIdentifier},
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
			httpBackendIdentifier = mock.FakeExisting(&lbaasv1.Backend{
				Name:         "http." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsBackendIdentifier = mock.FakeExisting(&lbaasv1.Backend{
				Name:         "https." + testClusterName,
				Mode:         lbaasv1.TCP,
				HealthCheck:  `"adv_check": "tcp-check"`,
				LoadBalancer: lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpFrontendIdentifier := mock.FakeExisting(&lbaasv1.Frontend{
				Name:           "http." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			httpsFrontendIdentifier := mock.FakeExisting(&lbaasv1.Frontend{
				Name:           "https." + testClusterName,
				Mode:           lbaasv1.TCP,
				LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: testLoadBalancerIdentifier},
				DefaultBackend: &lbaasv1.Backend{Identifier: httpsBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.http." + testClusterName,
				Address:  "8.8.8.8",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v4.https." + testClusterName,
				Address:  "8.8.8.8",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.http." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     80,
				Frontend: lbaasv1.Frontend{Identifier: httpFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Bind{
				Name:     "v6.https." + testClusterName,
				Address:  "2001:4860:4860::8888",
				Port:     443,
				Frontend: lbaasv1.Frontend{Identifier: httpsFrontendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Server{
				Name:    "test-server-01.http." + testClusterName,
				IP:      "10.244.0.4",
				Port:    42037,
				Check:   "enabled",
				Backend: lbaasv1.Backend{Identifier: httpBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Server{
				Name:    "test-server-01.https." + testClusterName,
				IP:      "10.244.0.4",
				Port:    37042,
				Check:   "enabled",
				Backend: lbaasv1.Backend{Identifier: httpsBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Server{
				Name:    "test-server-02.http." + testClusterName,
				IP:      "8.8.8.8",
				Port:    42037,
				Check:   "enabled",
				Backend: lbaasv1.Backend{Identifier: httpBackendIdentifier},
			}, fmt.Sprintf("anxccm-svc-uid=%v", svcUID))

			mock.FakeExisting(&lbaasv1.Server{
				Name:    "test-server-02.https." + testClusterName,
				IP:      "8.8.8.8",
				Port:    37042,
				Check:   "enabled",
				Backend: lbaasv1.Backend{Identifier: httpsBackendIdentifier},
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

		Context("deleting the service", func() {
			BeforeEach(func() {
				externalAddresses = make([]net.IP, 0)
				ports = make(map[string]Port)
				servers = make([]Server, 0)
			})

			It("detects all resources as having to be destroyed", func() {
				destroyCount := len(mock.Backends()) + len(mock.Binds()) + len(mock.Frontends()) + len(mock.Servers())

				toCreate, toDestroy, err := recon.ReconcileCheck()
				Expect(err).NotTo(HaveOccurred())
				Expect(toCreate).To(HaveLen(0))
				Expect(toDestroy).To(HaveLen(destroyCount))
			})

			It("destroys all resources", func() {
				err := recon.Reconcile()
				Expect(err).NotTo(HaveOccurred())

				Expect(mock.Frontends()).To(HaveLen(0))
				Expect(mock.Binds()).To(HaveLen(0))
				Expect(mock.Backends()).To(HaveLen(0))
				Expect(mock.Servers()).To(HaveLen(0))
			})
		})
	})
})

func TestReconcilation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LoadBalancer reconciliation test suite")
}
