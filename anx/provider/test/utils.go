package test

import (
	"fmt"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/configuration"
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/mocks"
	"go.anx.io/go-anxcloud/pkg/clouddns"
	"go.anx.io/go-anxcloud/pkg/ipam"
	"go.anx.io/go-anxcloud/pkg/lbaas"
	"go.anx.io/go-anxcloud/pkg/test"
	"go.anx.io/go-anxcloud/pkg/vlan"
	"go.anx.io/go-anxcloud/pkg/vsphere"
	v1 "k8s.io/api/core/v1"
)

//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg --name API --structname API --filename api.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/vsphere/powercontrol --name API --structname PowerControl --filename powercontrol.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/clouddns --name API --structname CloudDNS --filename clouddns.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/vsphere --name API --structname VSphere --filename vsphere.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/vsphere/search --name API --structname Search --filename search.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/vsphere/info --name API --structname Info --filename info.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/vsphere/vmlist --name API --structname VMList --filename vmlist.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/lbaas/ --name API --structname LBaaS --filename lbaas.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/lbaas/frontend --name API --structname Frontend --filename frontend.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/lbaas/backend --name API --structname Backend --filename backend.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/lbaas/bind --name API --structname Bind --filename bind.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/lbaas/server --name API --structname Server --filename server.go --output ../mocks
//go:generate mockery --srcpkg go.anx.io/go-anxcloud/pkg/lbaas/loadbalancer --name API --structname LoadBalancer --filename loadbalancer.go --output ../mocks

type MockedProvider struct {
	ApiMock          *mocks.API
	VsphereMock      *mocks.VSphere
	PowerControlMock *mocks.PowerControl
	SearchMock       *mocks.Search
	InfoMock         *mocks.Info
	VmListMock       *mocks.VMList
	CloudDNSMock     *mocks.CloudDNS
	LbaasMock        *mocks.LBaaS
	BackendMock      *mocks.Backend
	FrontendMock     *mocks.Frontend
	ServerMock       *mocks.Server
	BindMock         *mocks.Bind

	ProviderConfig *configuration.ProviderConfig
}

func GetMockedAnxProvider() MockedProvider {
	apiMock := &mocks.API{}
	vsphereMock := &mocks.VSphere{}
	powerControlMock := &mocks.PowerControl{}
	searchMock := &mocks.Search{}
	infoMock := &mocks.Info{}
	vmListMock := &mocks.VMList{}
	cloudDNSMock := &mocks.CloudDNS{}
	lbaasMock := &mocks.LBaaS{}
	lbBackendMock := &mocks.Backend{}
	lbFrontendMock := &mocks.Frontend{}
	lbServerMock := &mocks.Server{}
	lbBindMock := &mocks.Bind{}

	// setup lbaas mock
	lbaasMock.On("Frontend").Return(lbFrontendMock)
	lbaasMock.On("Backend").Return(lbBackendMock)
	lbaasMock.On("Frontend").Return(lbFrontendMock)
	lbaasMock.On("Server").Return(lbServerMock)
	lbaasMock.On("Bind").Return(lbBindMock)

	// setup vsphere mock
	vsphereMock.On("PowerControl").Return(powerControlMock)
	vsphereMock.On("Search").Return(searchMock)
	vsphereMock.On("Info").Return(infoMock)
	vsphereMock.On("VMList").Return(vmListMock)

	apiMock.On("VSphere").Return(vsphereMock)
	apiMock.On("CloudDNS").Return(cloudDNSMock)

	return MockedProvider{
		ApiMock:          apiMock,
		VsphereMock:      vsphereMock,
		PowerControlMock: powerControlMock,
		SearchMock:       searchMock,
		InfoMock:         infoMock,
		VmListMock:       vmListMock,
		LbaasMock:        lbaasMock,
		BackendMock:      lbBackendMock,
		FrontendMock:     lbFrontendMock,
		ServerMock:       lbServerMock,
		BindMock:         lbBindMock,
		ProviderConfig: &configuration.ProviderConfig{
			Token:                  "<TOKEN>",
			CustomerID:             "<CUSTOMER_ID>",
			LoadBalancerIdentifier: "<IDENTIFIER>",
		},
	}
}

func (m MockedProvider) IPAM() ipam.API {
	panic("implement me")
}

func (m MockedProvider) Test() test.API {
	panic("implement me")
}

func (m MockedProvider) VLAN() vlan.API {
	panic("implement me")
}

func (m MockedProvider) VSphere() vsphere.API {
	return m.VsphereMock
}

func (m MockedProvider) CloudDNS() clouddns.API {
	return m.CloudDNSMock
}

func (m MockedProvider) LBaaS() lbaas.API {
	return m.LbaasMock
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
