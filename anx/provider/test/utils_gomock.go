package test

import (
	"github.com/golang/mock/gomock"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/configuration"
	gomockapi "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/gomockapi"
	mockvsphere "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere"
	mockvsphere_info "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/info"
	mockvsphere_search "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/search"
	mockvsphere_powercontrol "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/powercontrol"
	mockvsphere_vmlist "github.com/anexia-it/k8s-anexia-ccm/anx/provider/test/mockvsphere/vmlist"
)

// GetMockedAnxProviderWithController creates a MockedProvider wired with a GoMock
// apimock.MockAPI (returned in Apimock) and also populates the existing testify
// sub-mocks so tests can incrementally migrate from testify to GoMock.
func GetMockedAnxProviderWithController(ctrl *gomock.Controller) MockedProvider {
	apiMock := gomockapi.NewMockAPI(ctrl)

	// create GoMock-generated vSphere submocks
	vsphereMock := mockvsphere.NewMockAPI(ctrl)
	powerControlMock := mockvsphere_powercontrol.NewMockAPI(ctrl)
	searchMock := mockvsphere_search.NewMockAPI(ctrl)
	infoMock := mockvsphere_info.NewMockAPI(ctrl)
	vmListMock := mockvsphere_vmlist.NewMockAPI(ctrl)

	// wire vSphere submocks to the vsphere mock
	vsphereMock.EXPECT().PowerControl().AnyTimes().Return(powerControlMock)
	vsphereMock.EXPECT().Search().AnyTimes().Return(searchMock)
	vsphereMock.EXPECT().Info().AnyTimes().Return(infoMock)
	vsphereMock.EXPECT().VMList().AnyTimes().Return(vmListMock)

	// Tell the GoMock apimock to return our gomock sub-mocks where appropriate.
	apiMock.EXPECT().VSphere().AnyTimes().Return(vsphereMock)

	return MockedProvider{
		Apimock:          apiMock,
		VsphereMock:      vsphereMock,
		PowerControlMock: powerControlMock,
		SearchMock:       searchMock,
		InfoMock:         infoMock,
		VmListMock:       vmListMock,
		ProviderConfig: &configuration.ProviderConfig{
			Token:                  "<TOKEN>",
			CustomerID:             "<CUSTOMER_ID>",
			LoadBalancerIdentifier: "<IDENTIFIER>",
		},
	}
}
