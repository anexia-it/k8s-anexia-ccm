// Code generated by mockery v2.9.4. DO NOT EDIT.

package mocks

import (
	context "context"

	bind "github.com/anexia-it/go-anxcloud/pkg/lbaas/bind"

	mock "github.com/stretchr/testify/mock"

	pagination "github.com/anexia-it/go-anxcloud/pkg/pagination"

	param "github.com/anexia-it/go-anxcloud/pkg/utils/param"
)

// Bind is an autogenerated mock type for the API type
type Bind struct {
	mock.Mock
}

// Create provides a mock function with given fields: ctx, definition
func (_m *Bind) Create(ctx context.Context, definition bind.Definition) (bind.Bind, error) {
	ret := _m.Called(ctx, definition)

	var r0 bind.Bind
	if rf, ok := ret.Get(0).(func(context.Context, bind.Definition) bind.Bind); ok {
		r0 = rf(ctx, definition)
	} else {
		r0 = ret.Get(0).(bind.Bind)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, bind.Definition) error); ok {
		r1 = rf(ctx, definition)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteByID provides a mock function with given fields: ctx, identifier
func (_m *Bind) DeleteByID(ctx context.Context, identifier string) error {
	ret := _m.Called(ctx, identifier)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, identifier)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: ctx, page, limit
func (_m *Bind) Get(ctx context.Context, page int, limit int) ([]bind.BindInfo, error) {
	ret := _m.Called(ctx, page, limit)

	var r0 []bind.BindInfo
	if rf, ok := ret.Get(0).(func(context.Context, int, int) []bind.BindInfo); ok {
		r0 = rf(ctx, page, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]bind.BindInfo)
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

// GetByID provides a mock function with given fields: ctx, identifier
func (_m *Bind) GetByID(ctx context.Context, identifier string) (bind.Bind, error) {
	ret := _m.Called(ctx, identifier)

	var r0 bind.Bind
	if rf, ok := ret.Get(0).(func(context.Context, string) bind.Bind); ok {
		r0 = rf(ctx, identifier)
	} else {
		r0 = ret.Get(0).(bind.Bind)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, identifier)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPage provides a mock function with given fields: ctx, page, limit, opts
func (_m *Bind) GetPage(ctx context.Context, page int, limit int, opts ...param.Parameter) (pagination.Page, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, page, limit)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 pagination.Page
	if rf, ok := ret.Get(0).(func(context.Context, int, int, ...param.Parameter) pagination.Page); ok {
		r0 = rf(ctx, page, limit, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pagination.Page)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int, int, ...param.Parameter) error); ok {
		r1 = rf(ctx, page, limit, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NextPage provides a mock function with given fields: ctx, page
func (_m *Bind) NextPage(ctx context.Context, page pagination.Page) (pagination.Page, error) {
	ret := _m.Called(ctx, page)

	var r0 pagination.Page
	if rf, ok := ret.Get(0).(func(context.Context, pagination.Page) pagination.Page); ok {
		r0 = rf(ctx, page)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pagination.Page)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, pagination.Page) error); ok {
		r1 = rf(ctx, page)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Update provides a mock function with given fields: ctx, identifier, definition
func (_m *Bind) Update(ctx context.Context, identifier string, definition bind.Definition) (bind.Bind, error) {
	ret := _m.Called(ctx, identifier, definition)

	var r0 bind.Bind
	if rf, ok := ret.Get(0).(func(context.Context, string, bind.Definition) bind.Bind); ok {
		r0 = rf(ctx, identifier, definition)
	} else {
		r0 = ret.Get(0).(bind.Bind)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, bind.Definition) error); ok {
		r1 = rf(ctx, identifier, definition)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
