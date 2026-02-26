package test

import (
	"fmt"

	dto "github.com/prometheus/client_model/go"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	gomockapi "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/gomockapi"
	legacyapimock "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/legacyapimock"
	mockvsphere "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere"
	mockvsphere_info "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/info"
	mockvsphere_search "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/search"
	mockvsphere_powercontrol "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/powercontrol"
	mockvsphere_vmlist "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/vmlist"

	"github.com/prometheus/client_golang/prometheus"
	clouddns "go.anx.io/go-anxcloud/pkg/clouddns"
	"go.anx.io/go-anxcloud/pkg/ipam"
	"go.anx.io/go-anxcloud/pkg/lbaas"
	anxcloudtest "go.anx.io/go-anxcloud/pkg/test"
	vlan "go.anx.io/go-anxcloud/pkg/vlan"
	vsphere "go.anx.io/go-anxcloud/pkg/vsphere"
	v1 "k8s.io/api/core/v1"
)

// NOTE: mock generation now uses MockGen (GoMock). The generated mocks are stored
// under subpackages like ./gomockapi and ./mocklbaas. If you need to regenerate
// them locally, run the appropriate mockgen commands. Examples:
//
//go:generate mockgen -package gomockapi -destination ./gomockapi/api.go go.anx.io/go-anxcloud/pkg API
//go:generate mockgen -package mocklbaas -destination ./mocklbaas/lbaas.go go.anx.io/go-anxcloud/pkg/lbaas API
//go:generate mockgen -package mocklbaas -destination ./mocklbaas/backend/backend.go go.anx.io/go-anxcloud/pkg/lbaas/backend API
//go:generate mockgen -package mocklbaas -destination ./mocklbaas/bind/bind.go go.anx.io/go-anxcloud/pkg/lbaas/bind API
//go:generate mockgen -package mocklbaas -destination ./mocklbaas/server/server.go go.anx.io/go-anxcloud/pkg/lbaas/server API
//go:generate mockgen -package mocklbaas -destination ./mocklbaas/frontend.go go.anx.io/go-anxcloud/pkg/lbaas/frontend API

type MockedProvider struct {
	// central GoMock API
	Apimock *gomockapi.MockAPI

	// GoMock-generated submocks for vSphere APIs
	VsphereMock      *mockvsphere.MockAPI
	PowerControlMock *mockvsphere_powercontrol.MockAPI
	SearchMock       *mockvsphere_search.MockAPI
	InfoMock         *mockvsphere_info.MockAPI
	VmListMock       *mockvsphere_vmlist.MockAPI

	// IPAM (legacy generated mocks)
	IPAMMock    *legacyapimock.MockIPAMAPI
	AddressMock *legacyapimock.MockIPAMAddressAPI
	PrefixMock  *legacyapimock.MockIPAMPrefixAPI

	// DNS / LBaaS / VLAN / Test (keep existing legacy mocks where necessary)
	CloudDNSMock interface{}
	LbaasMock    interface{}
	BackendMock  interface{}
	FrontendMock interface{}
	ServerMock   interface{}
	BindMock     interface{}
	// Vlan mock removed; use Apimock.VLAN() or mockvlan package if needed

	ProviderConfig *configuration.ProviderConfig
}

// NOTE: The concrete factory that wires GoMock mocks is provided in
// GetMockedAnxProviderWithController in utils_gomock.go. Tests should create a
// gomock.Controller and call that factory so they can control mock lifecycle.

func (m MockedProvider) IPAM() ipam.API {
	return m.Apimock.IPAM()
}

func (m MockedProvider) Test() anxcloudtest.API {
	return m.Apimock.Test()
}

func (m MockedProvider) VLAN() vlan.API {
	return m.Apimock.VLAN()
}

func (m MockedProvider) VSphere() vsphere.API {
	return m.Apimock.VSphere()
}

func (m MockedProvider) CloudDNS() clouddns.API {
	return m.Apimock.CloudDNS()
}

func (m MockedProvider) LBaaS() lbaas.API {
	return m.Apimock.LBaaS()
}

func (m MockedProvider) Config() *configuration.ProviderConfig {
	return m.ProviderConfig
}

func ProviderManagedNode(identifier string) v1.Node {
	return v1.Node{
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("%s%s", configuration.CloudProviderScheme, identifier),
		},
	}
}

func GetHistogramSum(collector prometheus.Collector) float64 {
	ch := make(chan prometheus.Metric, 1)
	collector.Collect(ch)

	m := dto.Metric{}
	_ = (<-ch).Write(&m) // read metric value from the channel

	return m.Histogram.GetSampleSum()
}
