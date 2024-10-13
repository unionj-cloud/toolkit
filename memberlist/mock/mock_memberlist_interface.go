// Code generated by MockGen. DO NOT EDIT.
// Source: ./memberlist_interface.go

// Package mock is a generated GoMock package.
package mock

import (
	net "net"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	memberlist "github.com/unionj-cloud/toolkit/memberlist"
)

// MockIMemberlist is a mock of IMemberlist interface.
type MockIMemberlist struct {
	ctrl     *gomock.Controller
	recorder *MockIMemberlistMockRecorder
}

// MockIMemberlistMockRecorder is the mock recorder for MockIMemberlist.
type MockIMemberlistMockRecorder struct {
	mock *MockIMemberlist
}

// NewMockIMemberlist creates a new mock instance.
func NewMockIMemberlist(ctrl *gomock.Controller) *MockIMemberlist {
	mock := &MockIMemberlist{ctrl: ctrl}
	mock.recorder = &MockIMemberlistMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIMemberlist) EXPECT() *MockIMemberlistMockRecorder {
	return m.recorder
}

// AdvertiseAddr mocks base method.
func (m *MockIMemberlist) AdvertiseAddr() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AdvertiseAddr")
	ret0, _ := ret[0].(string)
	return ret0
}

// AdvertiseAddr indicates an expected call of AdvertiseAddr.
func (mr *MockIMemberlistMockRecorder) AdvertiseAddr() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AdvertiseAddr", reflect.TypeOf((*MockIMemberlist)(nil).AdvertiseAddr))
}

// Config mocks base method.
func (m *MockIMemberlist) Config() *memberlist.Config {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Config")
	ret0, _ := ret[0].(*memberlist.Config)
	return ret0
}

// Config indicates an expected call of Config.
func (mr *MockIMemberlistMockRecorder) Config() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Config", reflect.TypeOf((*MockIMemberlist)(nil).Config))
}

// GetHealthScore mocks base method.
func (m *MockIMemberlist) GetHealthScore() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetHealthScore")
	ret0, _ := ret[0].(int)
	return ret0
}

// GetHealthScore indicates an expected call of GetHealthScore.
func (mr *MockIMemberlistMockRecorder) GetHealthScore() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetHealthScore", reflect.TypeOf((*MockIMemberlist)(nil).GetHealthScore))
}

// Join mocks base method.
func (m *MockIMemberlist) Join(existing []string) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join", existing)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Join indicates an expected call of Join.
func (mr *MockIMemberlistMockRecorder) Join(existing interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockIMemberlist)(nil).Join), existing)
}

// Leave mocks base method.
func (m *MockIMemberlist) Leave(timeout time.Duration) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Leave", timeout)
	ret0, _ := ret[0].(error)
	return ret0
}

// Leave indicates an expected call of Leave.
func (mr *MockIMemberlistMockRecorder) Leave(timeout interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Leave", reflect.TypeOf((*MockIMemberlist)(nil).Leave), timeout)
}

// LocalNode mocks base method.
func (m *MockIMemberlist) LocalNode() *memberlist.Node {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalNode")
	ret0, _ := ret[0].(*memberlist.Node)
	return ret0
}

// LocalNode indicates an expected call of LocalNode.
func (mr *MockIMemberlistMockRecorder) LocalNode() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalNode", reflect.TypeOf((*MockIMemberlist)(nil).LocalNode))
}

// Members mocks base method.
func (m *MockIMemberlist) Members() []*memberlist.Node {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Members")
	ret0, _ := ret[0].([]*memberlist.Node)
	return ret0
}

// Members indicates an expected call of Members.
func (mr *MockIMemberlistMockRecorder) Members() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Members", reflect.TypeOf((*MockIMemberlist)(nil).Members))
}

// NumMembers mocks base method.
func (m *MockIMemberlist) NumMembers() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NumMembers")
	ret0, _ := ret[0].(int)
	return ret0
}

// NumMembers indicates an expected call of NumMembers.
func (mr *MockIMemberlistMockRecorder) NumMembers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NumMembers", reflect.TypeOf((*MockIMemberlist)(nil).NumMembers))
}

// Ping mocks base method.
func (m *MockIMemberlist) Ping(node string, addr net.Addr) (time.Duration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping", node, addr)
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Ping indicates an expected call of Ping.
func (mr *MockIMemberlistMockRecorder) Ping(node, addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*MockIMemberlist)(nil).Ping), node, addr)
}

// ProtocolVersion mocks base method.
func (m *MockIMemberlist) ProtocolVersion() uint8 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProtocolVersion")
	ret0, _ := ret[0].(uint8)
	return ret0
}

// ProtocolVersion indicates an expected call of ProtocolVersion.
func (mr *MockIMemberlistMockRecorder) ProtocolVersion() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProtocolVersion", reflect.TypeOf((*MockIMemberlist)(nil).ProtocolVersion))
}

// SendBestEffort mocks base method.
func (m *MockIMemberlist) SendBestEffort(to *memberlist.Node, msg []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendBestEffort", to, msg)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendBestEffort indicates an expected call of SendBestEffort.
func (mr *MockIMemberlistMockRecorder) SendBestEffort(to, msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendBestEffort", reflect.TypeOf((*MockIMemberlist)(nil).SendBestEffort), to, msg)
}

// SendReliable mocks base method.
func (m *MockIMemberlist) SendReliable(to *memberlist.Node, msg []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendReliable", to, msg)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendReliable indicates an expected call of SendReliable.
func (mr *MockIMemberlistMockRecorder) SendReliable(to, msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendReliable", reflect.TypeOf((*MockIMemberlist)(nil).SendReliable), to, msg)
}

// SendToAddress mocks base method.
func (m *MockIMemberlist) SendToAddress(a memberlist.Address, msg []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendToAddress", a, msg)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendToAddress indicates an expected call of SendToAddress.
func (mr *MockIMemberlistMockRecorder) SendToAddress(a, msg interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendToAddress", reflect.TypeOf((*MockIMemberlist)(nil).SendToAddress), a, msg)
}

// Shutdown mocks base method.
func (m *MockIMemberlist) Shutdown() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Shutdown")
	ret0, _ := ret[0].(error)
	return ret0
}

// Shutdown indicates an expected call of Shutdown.
func (mr *MockIMemberlistMockRecorder) Shutdown() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Shutdown", reflect.TypeOf((*MockIMemberlist)(nil).Shutdown))
}

// UpdateNode mocks base method.
func (m *MockIMemberlist) UpdateNode(timeout time.Duration) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateNode", timeout)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateNode indicates an expected call of UpdateNode.
func (mr *MockIMemberlistMockRecorder) UpdateNode(timeout interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateNode", reflect.TypeOf((*MockIMemberlist)(nil).UpdateNode), timeout)
}
