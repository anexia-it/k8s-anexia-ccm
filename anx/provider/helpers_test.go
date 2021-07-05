package provider

import (
	"github.com/anexia-it/anxcloud-cloud-controller-manager/anx/provider/mocks"
	"github.com/anexia-it/go-anxcloud/pkg/ipam"
	"github.com/anexia-it/go-anxcloud/pkg/test"
	"github.com/anexia-it/go-anxcloud/pkg/vlan"
	"github.com/anexia-it/go-anxcloud/pkg/vsphere"
)

//go:generate mockery --srcpkg github.com/anexia-it/go-anxcloud/pkg --name API --structname API --filename api.go
//go:generate mockery --srcpkg github.com/anexia-it/go-anxcloud/pkg/vsphere/powercontrol --name API --structname PowerControl --filename powercontrol.go
//go:generate mockery --srcpkg github.com/anexia-it/go-anxcloud/pkg/vsphere --name API --structname VSphere --filename vsphere.go
//go:generate mockery --srcpkg github.com/anexia-it/go-anxcloud/pkg/vsphere/search --name API --structname Search --filename search.go
//go:generate mockery --srcpkg github.com/anexia-it/go-anxcloud/pkg/vsphere/infoMock --name API --structname Info --filename infoMock.go

type mockedProvider struct {
	apiMock          *mocks.API
	vsphereMock      *mocks.VSphere
	powerControlMock *mocks.PowerControl
	searchMock       *mocks.Search
	infoMock         *mocks.Info
}

func getMockedAnxProvider() mockedProvider {
	apiMock := &mocks.API{}
	vsphereMock := &mocks.VSphere{}
	powerControlMock := &mocks.PowerControl{}
	searchMock := &mocks.Search{}
	infoMock := &mocks.Info{}

	vsphereMock.On("PowerControl").Return(powerControlMock)
	vsphereMock.On("Search").Return(searchMock)
	vsphereMock.On("Info").Return(infoMock)
	apiMock.On("VSphere").Return(vsphereMock)

	return mockedProvider{
		apiMock:          apiMock,
		vsphereMock:      vsphereMock,
		powerControlMock: powerControlMock,
		searchMock:       searchMock,
		infoMock:         infoMock,
	}
}

func (m mockedProvider) IPAM() ipam.API {
	panic("implement me")
}

func (m mockedProvider) Test() test.API {
	panic("implement me")
}

func (m mockedProvider) VLAN() vlan.API {
	panic("implement me")
}

func (m mockedProvider) VSphere() vsphere.API {
	return m.vsphereMock
}
