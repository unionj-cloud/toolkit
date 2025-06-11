package unitofwork

import (
	"context"
	"fmt"
	"github.com/wubin1989/gorm"

	"github.com/unionj-cloud/toolkit/zlogger"
)

// Manager 工作单元管理器
type Manager struct {
	db     *gorm.DB
	config *Config
}

// NewManager 创建工作单元管理器
func NewManager(db *gorm.DB, options ...ConfigOption) *Manager {
	config := DefaultConfig()
	for _, option := range options {
		option(config)
	}

	manager := &Manager{
		db:     db,
		config: config,
	}

	return manager
}

// ExecuteInUnitOfWork 在工作单元中执行操作
func (m *Manager) ExecuteInUnitOfWork(ctx context.Context, fn Callback) error {
	uow := NewUnitOfWork(m.db, func(c *Config) {
		*c = *m.config
	}).WithContext(ctx)

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
func (m *Manager) ExecuteInNewUnitOfWork(fn Callback) error {
	return m.ExecuteInUnitOfWork(context.Background(), fn)
}

// Callback 工作单元回调函数
type Callback func(*UnitOfWork) error
