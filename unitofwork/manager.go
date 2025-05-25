package unitofwork

import (
	"context"
	"fmt"
	"sync"

	"github.com/wubin1989/gorm"

	"github.com/unionj-cloud/toolkit/caches"
	"github.com/unionj-cloud/toolkit/zlogger"
)

// Manager 工作单元管理器
type Manager struct {
	db           *gorm.DB
	config       *Config
	cachesPlugin *caches.Caches
	mu           sync.RWMutex
	currentUoW   *UnitOfWork
	contextKey   string
}

// NewManager 创建工作单元管理器
func NewManager(db *gorm.DB, options ...ConfigOption) *Manager {
	config := DefaultConfig()
	for _, option := range options {
		option(config)
	}

	manager := &Manager{
		db:         db,
		config:     config,
		contextKey: "unit_of_work",
	}

	return manager
}

// WithCaches 配置缓存插件
func (m *Manager) WithCaches(cachesPlugin *caches.Caches) *Manager {
	m.cachesPlugin = cachesPlugin
	return m
}

// ExecuteInUnitOfWork 在工作单元中执行操作
func (m *Manager) ExecuteInUnitOfWork(ctx context.Context, fn func(*UnitOfWork) error) error {
	uow := NewUnitOfWork(m.db, func(c *Config) {
		*c = *m.config
	}).WithContext(ctx)

	// 设置当前工作单元
	m.setCurrentUoW(uow)
	defer m.clearCurrentUoW()

	// 执行业务逻辑
	if err := fn(uow); err != nil {
		if rollbackErr := uow.Rollback(); rollbackErr != nil {
			zlogger.Error().Err(rollbackErr).Msg("Failed to rollback unit of work")
		}
		return fmt.Errorf("business logic failed: %w", err)
	}

	// 提交工作单元
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

// ExecuteInNewUnitOfWork 创建新的工作单元并执行
func (m *Manager) ExecuteInNewUnitOfWork(fn func(*UnitOfWork) error) error {
	return m.ExecuteInUnitOfWork(context.Background(), fn)
}

// GetCurrentUoW 获取当前工作单元
func (m *Manager) GetCurrentUoW() *UnitOfWork {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentUoW
}

// GetUoWFromContext 从上下文获取工作单元
func (m *Manager) GetUoWFromContext(ctx context.Context) (*UnitOfWork, bool) {
	uow, ok := ctx.Value(m.contextKey).(*UnitOfWork)
	return uow, ok
}

// setCurrentUoW 设置当前工作单元
func (m *Manager) setCurrentUoW(uow *UnitOfWork) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentUoW = uow
}

// clearCurrentUoW 清除当前工作单元
func (m *Manager) clearCurrentUoW() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentUoW = nil
}

// UnitOfWorkCallback 工作单元回调函数
type UnitOfWorkCallback func(*UnitOfWork) error

// UnitOfWorkFactory 工作单元工厂接口
type UnitOfWorkFactory interface {
	ExecuteInUnitOfWork(ctx context.Context, callback UnitOfWorkCallback) error
}

// DefaultUnitOfWorkFactory 默认工作单元工厂
type DefaultUnitOfWorkFactory struct {
	manager *Manager
}

// NewUnitOfWorkFactory 创建工作单元工厂
func NewUnitOfWorkFactory(db *gorm.DB, options ...ConfigOption) UnitOfWorkFactory {
	manager := NewManager(db, options...)
	return &DefaultUnitOfWorkFactory{
		manager: manager,
	}
}

// ExecuteInUnitOfWork 实现工厂接口
func (f *DefaultUnitOfWorkFactory) ExecuteInUnitOfWork(ctx context.Context, callback UnitOfWorkCallback) error {
	return f.manager.ExecuteInUnitOfWork(ctx, callback)
}
