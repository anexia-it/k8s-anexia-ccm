package reconciliation

import (
	"go.anx.io/go-anxcloud/pkg/api/types"
	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testRecon struct {
	status    map[string][]uint16
	toCreate  []types.Object
	toDestroy []types.Object
}

func (tr testRecon) Status() (map[string][]uint16, error) {
	return tr.status, nil
}

func (tr testRecon) Reconcile() error {
	return nil
}

func (tr testRecon) ReconcileCheck() ([]types.Object, []types.Object, error) {
	toCreate, toDestroy := tr.toCreate, tr.toDestroy

	if toCreate == nil {
		toCreate = []types.Object{}
	}

	if toDestroy == nil {
		toDestroy = []types.Object{}
	}

	return toCreate, toDestroy, nil
}

var _ = Describe("Multi", func() {
	var recon Reconciliation

	Context("Status aggregation", func() {
		BeforeEach(func() {
			recon = Multi(
				testRecon{
					status: map[string][]uint16{
						"8.8.8.8": {80, 443, 53},
						"8.8.4.4": {80, 443, 53},
					},
				},
				testRecon{
					status: map[string][]uint16{
						"8.8.4.4": {80, 443, 53},
					},
				},
			)
		})
	})

	Context("ReconcileCheck aggregation", func() {
		Context("toCreate", func() {
			BeforeEach(func() {
				recon = Multi(
					testRecon{
						toCreate: []types.Object{
							&lbaasv1.Frontend{Name: "test-01"},
							&lbaasv1.Frontend{Name: "test-02"},
						},
					},
					testRecon{
						toCreate: []types.Object{
							&lbaasv1.Frontend{Name: "test-01"},
						},
					},
				)
			})

			It("aggregates correctly", func() {
				toCreate, toDestroy, err := recon.ReconcileCheck()
				Expect(err).NotTo(HaveOccurred())
				Expect(toCreate).To(HaveLen(3))
				Expect(toDestroy).To(HaveLen(0))
			})
		})

		Context("toDestroy", func() {
			BeforeEach(func() {
				recon = Multi(
					testRecon{
						toDestroy: []types.Object{
							&lbaasv1.Frontend{Name: "test-01"},
							&lbaasv1.Frontend{Name: "test-02"},
						},
					},
					testRecon{
						toDestroy: []types.Object{
							&lbaasv1.Frontend{Name: "test-01"},
						},
					},
				)
			})

			It("aggregates correctly", func() {
				toCreate, toDestroy, err := recon.ReconcileCheck()
				Expect(err).NotTo(HaveOccurred())
				Expect(toCreate).To(HaveLen(0))
				Expect(toDestroy).To(HaveLen(3))
			})
		})
	})
})

var _ = DescribeTable("mergeReconStatus",
	func(status []map[string][]uint16, expected map[string][]uint16) {
		merged := mergeReconStatus(status)

		for addr, ports := range expected {
			Expect(merged).To(HaveKey(addr))
			Expect(merged[addr]).To(ContainElements(ports))
		}
	},
	Entry(
		"merges two complete ones correctly",
		[]map[string][]uint16{
			{
				"8.8.8.8": {80, 443, 53},
				"8.8.4.4": {80, 443},
			},
			{
				"8.8.8.8": {80, 443, 53},
				"8.8.4.4": {80, 443},
			},
		},
		map[string][]uint16{
			"8.8.4.4": {80, 443},
			"8.8.8.8": {80, 443, 53},
		},
	),
	Entry(
		"merges two incomplete ones correctly",
		[]map[string][]uint16{
			{
				"8.8.8.8": {80, 443, 53},
				"8.8.4.4": {80, 443},
			},
			{
				"8.8.8.8": {80, 443},
				"8.8.4.4": {80, 443},
			},
		},
		map[string][]uint16{
			"8.8.4.4": {80, 443},
			"8.8.8.8": {80, 443},
		},
	),
	Entry(
		"merges completely unrelated ones correctly",
		[]map[string][]uint16{
			{
				"8.8.8.8": {80, 443, 53},
				"8.8.4.4": {80, 443},
			},
			{
				"10.244.0.1": {80, 443},
				"10.244.0.2": {80, 443},
			},
		},
		map[string][]uint16{},
	),
)
