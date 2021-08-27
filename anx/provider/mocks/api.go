// Code generated by mockery v2.9.4. DO NOT EDIT.

package mocks

import (
	clouddns "github.com/anexia-it/go-anxcloud/pkg/clouddns"
	ipam "github.com/anexia-it/go-anxcloud/pkg/ipam"

	lbaas "github.com/anexia-it/go-anxcloud/pkg/lbaas"

	mock "github.com/stretchr/testify/mock"

	test "github.com/anexia-it/go-anxcloud/pkg/test"

	vlan "github.com/anexia-it/go-anxcloud/pkg/vlan"

	vsphere "github.com/anexia-it/go-anxcloud/pkg/vsphere"
)

// API is an autogenerated mock type for the API type
type API struct {
	mock.Mock
}

// CloudDNS provides a mock function with given fields:
func (_m *API) CloudDNS() clouddns.API {
	ret := _m.Called()

	var r0 clouddns.API
	if rf, ok := ret.Get(0).(func() clouddns.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(clouddns.API)
		}
	}

	return r0
}

// IPAM provides a mock function with given fields:
func (_m *API) IPAM() ipam.API {
	ret := _m.Called()

	var r0 ipam.API
	if rf, ok := ret.Get(0).(func() ipam.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(ipam.API)
		}
	}

	return r0
}

// LBaaS provides a mock function with given fields:
func (_m *API) LBaaS() lbaas.API {
	ret := _m.Called()

	var r0 lbaas.API
	if rf, ok := ret.Get(0).(func() lbaas.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(lbaas.API)
		}
	}

	return r0
}

// Test provides a mock function with given fields:
func (_m *API) Test() test.API {
	ret := _m.Called()

	var r0 test.API
	if rf, ok := ret.Get(0).(func() test.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(test.API)
		}
	}

	return r0
}

// VLAN provides a mock function with given fields:
func (_m *API) VLAN() vlan.API {
	ret := _m.Called()

	var r0 vlan.API
	if rf, ok := ret.Get(0).(func() vlan.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(vlan.API)
		}
	}

	return r0
}

// VSphere provides a mock function with given fields:
func (_m *API) VSphere() vsphere.API {
	ret := _m.Called()

	var r0 vsphere.API
	if rf, ok := ret.Get(0).(func() vsphere.API); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(vsphere.API)
		}
	}

	return r0
}
