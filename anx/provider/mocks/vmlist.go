// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	vmlist "github.com/anexia-it/go-anxcloud/pkg/vsphere/vmlist"
	mock "github.com/stretchr/testify/mock"
)

// VMList is an autogenerated mock type for the API type
type VMList struct {
	mock.Mock
}

// Get provides a mock function with given fields: ctx, page, limit
func (_m *VMList) Get(ctx context.Context, page int, limit int) ([]vmlist.VM, error) {
	ret := _m.Called(ctx, page, limit)

	var r0 []vmlist.VM
	if rf, ok := ret.Get(0).(func(context.Context, int, int) []vmlist.VM); ok {
		r0 = rf(ctx, page, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]vmlist.VM)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int, int) error); ok {
		r1 = rf(ctx, page, limit)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
