package reconciliation

import (
	"context"
	"fmt"
	"net"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.anx.io/go-anxcloud/pkg/api/mock"
	. "go.anx.io/go-anxcloud/pkg/api/mock/matcher"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("stateRetriever", func() {
	Context("with MultiReconciliation", func() {
		var (
			ctx                 context.Context
			a                   mock.API
			svcUID              string
			externalIPAddresses []net.IP
			ports               map[string]Port
			servers             []Server
			multirecon          MultiReconciliation
		)
		BeforeEach(func() {

			ctx = context.TODO()
			a = mock.NewMockAPI()
			svcUID = rand.String(32)
			externalIPAddresses = []net.IP{net.ParseIP("8.8.8.8")}
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

			lbIDs := []string{"foo", "bar", "baz"}

			multirecon = Multi(a, svcUID)

			for _, lbID := range lbIDs {
				a.FakeExisting(&lbaasv1.LoadBalancer{Identifier: lbID})
				recon, err := New(ctx, a, "test-suffix", lbID, svcUID, externalIPAddresses, ports, servers)
				Expect(err).ToNot(HaveOccurred())
				multirecon.Add(recon)
			}
		})

		It("can provide state for Reconcile", func() {
			err := multirecon.Reconcile(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			Expect(a.Existing()).To(
				SatisfyAll(
					ContainElementNTimes(Object(&lbaasv1.Backend{LoadBalancer: lbaasv1.LoadBalancer{Identifier: "foo"}}, "LoadBalancer.Identifier"), 2),
					ContainElementNTimes(Object(&lbaasv1.Backend{LoadBalancer: lbaasv1.LoadBalancer{Identifier: "bar"}}, "LoadBalancer.Identifier"), 2),
					ContainElementNTimes(Object(&lbaasv1.Backend{LoadBalancer: lbaasv1.LoadBalancer{Identifier: "baz"}}, "LoadBalancer.Identifier"), 2),

					ContainElementNTimes(Object(&lbaasv1.Frontend{LoadBalancer: &lbaasv1.LoadBalancer{Identifier: "foo"}}, "LoadBalancer.Identifier"), 2),
					ContainElementNTimes(Object(&lbaasv1.Frontend{LoadBalancer: &lbaasv1.LoadBalancer{Identifier: "bar"}}, "LoadBalancer.Identifier"), 2),
					ContainElementNTimes(Object(&lbaasv1.Frontend{LoadBalancer: &lbaasv1.LoadBalancer{Identifier: "baz"}}, "LoadBalancer.Identifier"), 2),
				),
			)
		})

		It("can provide state for Status", func() {
			status, err := multirecon.Status(context.TODO())
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(HaveLen(0))

			err = multirecon.Reconcile(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			status, err = multirecon.Status(context.TODO())
			Expect(err).ToNot(HaveOccurred())
			Expect(status).To(HaveLen(1))
		})

		It("can provide state for ReconcileCheck", func() {
			toCreate, toDestroy, err := multirecon.ReconcileCheck(context.TODO())
			Expect(err).ToNot(HaveOccurred())
			Expect(toCreate).To(HaveLen(6))
			Expect(toDestroy).To(HaveLen(0))
		})
	})

	Context("with fresh stateRetriever", func() {
		a := mock.NewMockAPI()
		ctx := context.TODO()

		It("supports Done signal in different iterations", func() {
			lbCount := 4
			retriever := newStateRetriever(ctx, a, "fake-service-id", lbCount)
			signalChan := make(chan int, 6)

			wg := sync.WaitGroup{}
			wg.Add(lbCount)

			for i := 0; i < lbCount; i++ {
				go func(i int) {
					lbID := fmt.Sprintf("lb-%d", i)
					defer func() {
						retriever.Done(lbID)
						wg.Done()
					}()
					for j := 0; j < i; j++ {
						_, err := retriever.FilteredState(ctx, lbID)
						if err != nil {
							return
						}

						signalChan <- j
					}
				}(i)
			}

			wg.Wait()

			close(signalChan)

			signals := make([]int, 0, 6)
			for s := range signalChan {
				signals = append(signals, s)
			}

			Expect(signals).To(Equal([]int{0, 0, 0, 1, 1, 2}))
		})
	})
})
