// Code generated by MockGen. DO NOT EDIT.
// Source: go.anx.io/go-anxcloud/pkg/ipam/prefix (interfaces: API)

// Package legacyapimock is a generated GoMock package.
package legacyapimock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	prefix "go.anx.io/go-anxcloud/pkg/ipam/prefix"
)

// MockIPAMPrefixAPI is a mock of API interface.
type MockIPAMPrefixAPI struct {
	ctrl     *gomock.Controller
	recorder *MockIPAMPrefixAPIMockRecorder
}

// MockIPAMPrefixAPIMockRecorder is the mock recorder for MockIPAMPrefixAPI.
type MockIPAMPrefixAPIMockRecorder struct {
	mock *MockIPAMPrefixAPI
}

// NewMockIPAMPrefixAPI creates a new mock instance.
func NewMockIPAMPrefixAPI(ctrl *gomock.Controller) *MockIPAMPrefixAPI {
	mock := &MockIPAMPrefixAPI{ctrl: ctrl}
	mock.recorder = &MockIPAMPrefixAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIPAMPrefixAPI) EXPECT() *MockIPAMPrefixAPIMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockIPAMPrefixAPI) Create(arg0 context.Context, arg1 prefix.Create) (prefix.Summary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0, arg1)
	ret0, _ := ret[0].(prefix.Summary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockIPAMPrefixAPIMockRecorder) Create(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockIPAMPrefixAPI)(nil).Create), arg0, arg1)
}

// Delete mocks base method.
func (m *MockIPAMPrefixAPI) Delete(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockIPAMPrefixAPIMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockIPAMPrefixAPI)(nil).Delete), arg0, arg1)
}

// Get mocks base method.
func (m *MockIPAMPrefixAPI) Get(arg0 context.Context, arg1 string) (prefix.Info, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(prefix.Info)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockIPAMPrefixAPIMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockIPAMPrefixAPI)(nil).Get), arg0, arg1)
}

// List mocks base method.
func (m *MockIPAMPrefixAPI) List(arg0 context.Context, arg1, arg2 int) ([]prefix.Summary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0, arg1, arg2)
	ret0, _ := ret[0].([]prefix.Summary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockIPAMPrefixAPIMockRecorder) List(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockIPAMPrefixAPI)(nil).List), arg0, arg1, arg2)
}

// Update mocks base method.
func (m *MockIPAMPrefixAPI) Update(arg0 context.Context, arg1 string, arg2 prefix.Update) (prefix.Summary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0, arg1, arg2)
	ret0, _ := ret[0].(prefix.Summary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockIPAMPrefixAPIMockRecorder) Update(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockIPAMPrefixAPI)(nil).Update), arg0, arg1, arg2)
}
