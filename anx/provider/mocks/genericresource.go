package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	genericresource "go.anx.io/go-anxcloud/pkg/genericresource"
	pagination "go.anx.io/go-anxcloud/pkg/pagination"
	param "go.anx.io/go-anxcloud/pkg/utils/param"
)

// GenericResourceAPI is a hand-written mock for the generic
// go.anx.io/go-anxcloud/pkg/genericresource.API[R, D] interface, which
// mockery (v2.9.4, in use for the other mocks in this package) cannot
// generate mocks for since it does not support Go generics.
type GenericResourceAPI[R any, D any] struct {
	mock.Mock
}

func (_m *GenericResourceAPI[R, D]) Get(ctx context.Context, page int, limit int) ([]genericresource.Identity, error) {
	ret := _m.Called(ctx, page, limit)

	var r0 []genericresource.Identity
	if rf, ok := ret.Get(0).(func(context.Context, int, int) []genericresource.Identity); ok {
		r0 = rf(ctx, page, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]genericresource.Identity)
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

func (_m *GenericResourceAPI[R, D]) GetByID(ctx context.Context, identifier string) (R, error) {
	ret := _m.Called(ctx, identifier)

	var r0 R
	if rf, ok := ret.Get(0).(func(context.Context, string) R); ok {
		r0 = rf(ctx, identifier)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(R)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, identifier)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GenericResourceAPI[R, D]) Create(ctx context.Context, definition D) (R, error) {
	ret := _m.Called(ctx, definition)

	var r0 R
	if rf, ok := ret.Get(0).(func(context.Context, D) R); ok {
		r0 = rf(ctx, definition)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(R)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, D) error); ok {
		r1 = rf(ctx, definition)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GenericResourceAPI[R, D]) Update(ctx context.Context, identifier string, definition D) (R, error) {
	ret := _m.Called(ctx, identifier, definition)

	var r0 R
	if rf, ok := ret.Get(0).(func(context.Context, string, D) R); ok {
		r0 = rf(ctx, identifier, definition)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(R)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, D) error); ok {
		r1 = rf(ctx, identifier, definition)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func (_m *GenericResourceAPI[R, D]) DeleteByID(ctx context.Context, identifier string) error {
	ret := _m.Called(ctx, identifier)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, identifier)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *GenericResourceAPI[R, D]) GetPage(ctx context.Context, page int, limit int, opts ...param.Parameter) (pagination.Page, error) {
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

func (_m *GenericResourceAPI[R, D]) NextPage(ctx context.Context, page pagination.Page) (pagination.Page, error) {
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
