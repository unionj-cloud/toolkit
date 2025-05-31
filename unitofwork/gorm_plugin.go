package unitofwork

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/unionj-cloud/toolkit/zlogger"
	"github.com/wubin1989/gorm"
)

// 上下文键
type contextKey int

const (
	uowContextKey contextKey = iota
	uowConfigContextKey
)

// AutoUnitOfWorkPlugin 自动化工作单元插件
type AutoUnitOfWorkPlugin struct {
	config       *AutoUowConfig
	globalConfig *Config
	mu           sync.RWMutex
}

// AutoUowConfig 自动化工作单元配置
type AutoUowConfig struct {
	// 是否启用插件
	Enabled bool

	// 是否自动检测实体接口
	AutoDetectEntity bool

	// 是否在查询时自动创建快照
	AutoSnapshot bool

	// 是否跳过只读操作
	SkipReadOnly bool

	// 自定义实体检测函数
	EntityDetector func(interface{}) Entity

	// 是否启用详细日志
	VerboseLog bool

	// 排除的表名（不参与工作单元管理）
	ExcludedTables []string

	// 自定义上下文键名
	ContextKey string
}

// DefaultAutoUowConfig 默认自动化工作单元配置
func DefaultAutoUowConfig() *AutoUowConfig {
	return &AutoUowConfig{
		Enabled:          true,
		AutoDetectEntity: true,
		AutoSnapshot:     true,
		SkipReadOnly:     true,
		VerboseLog:       false,
		ExcludedTables:   []string{},
		ContextKey:       "auto_uow",
	}
}

// NewAutoUnitOfWorkPlugin 创建自动化工作单元插件
func NewAutoUnitOfWorkPlugin(config *AutoUowConfig, globalConfig *Config) *AutoUnitOfWorkPlugin {
	if config == nil {
		config = DefaultAutoUowConfig()
	}
	if globalConfig == nil {
		globalConfig = DefaultConfig()
	}

	return &AutoUnitOfWorkPlugin{
		config:       config,
		globalConfig: globalConfig,
	}
}

// Name 实现 gorm.Plugin 接口
func (p *AutoUnitOfWorkPlugin) Name() string {
	return "gorm:auto_unitofwork"
}

// Initialize 实现 gorm.Plugin 接口，注册回调
func (p *AutoUnitOfWorkPlugin) Initialize(db *gorm.DB) error {
	if !p.config.Enabled {
		zlogger.Info().Msg("Auto UnitOfWork plugin is disabled")
		return nil
	}

	// 注册事务开始回调
	err := db.Callback().Begin().Before("gorm:begin_transaction").Register("uow:auto_begin", p.onTransactionBegin)
	if err != nil {
		return fmt.Errorf("failed to register begin callback: %w", err)
	}

	// 注册查询回调（用于自动快照）
	if p.config.AutoSnapshot {
		err = db.Callback().Query().After("gorm:query").Register("uow:auto_snapshot", p.onQueryAfter)
		if err != nil {
			return fmt.Errorf("failed to register query callback: %w", err)
		}
	}

	// 注册创建回调
	err = db.Callback().Create().Before("gorm:create").Register("uow:auto_create", p.onCreateBefore)
	if err != nil {
		return fmt.Errorf("failed to register create callback: %w", err)
	}

	// 注册更新回调
	err = db.Callback().Update().Before("gorm:update").Register("uow:auto_update", p.onUpdateBefore)
	if err != nil {
		return fmt.Errorf("failed to register update callback: %w", err)
	}

	// 注册删除回调
	err = db.Callback().Delete().Before("gorm:delete").Register("uow:auto_delete", p.onDeleteBefore)
	if err != nil {
		return fmt.Errorf("failed to register delete callback: %w", err)
	}

	// 注册事务提交回调
	err = db.Callback().Commit().After("gorm:commit_or_rollback_transaction").Register("uow:auto_commit", p.onTransactionCommit)
	if err != nil {
		return fmt.Errorf("failed to register commit callback: %w", err)
	}

	// 注册事务回滚回调
	err = db.Callback().Rollback().After("gorm:commit_or_rollback_transaction").Register("uow:auto_rollback", p.onTransactionRollback)
	if err != nil {
		return fmt.Errorf("failed to register rollback callback: %w", err)
	}

	zlogger.Info().Msg("Auto UnitOfWork plugin initialized successfully")
	return nil
}

// onTransactionBegin 事务开始时的回调
func (p *AutoUnitOfWorkPlugin) onTransactionBegin(db *gorm.DB) {
	if db.Error != nil {
		return
	}

	// 检查是否已经有工作单元
	if p.getUowFromContext(db.Statement.Context) != nil {
		return
	}

	// 创建新的工作单元
	uow := NewUnitOfWork(db, func(c *Config) {
		*c = *p.globalConfig
	}).WithContext(db.Statement.Context)

	// 绑定到上下文
	db.Statement.Context = p.setUowToContext(db.Statement.Context, uow)

	if p.config.VerboseLog {
		zlogger.Debug().Msg("Auto UnitOfWork: Created new unit of work for transaction")
	}
}

// onQueryAfter 查询后的回调（用于自动快照）
func (p *AutoUnitOfWorkPlugin) onQueryAfter(db *gorm.DB) {
	if db.Error != nil || !p.config.AutoSnapshot {
		return
	}

	uow := p.getUowFromContext(db.Statement.Context)
	if uow == nil {
		return
	}

	// 为查询结果创建快照
	if db.Statement.Dest != nil {
		entities := p.extractEntitiesFromDest(db.Statement.Dest)
		for _, entity := range entities {
			if entity != nil && !entity.IsNew() {
				uow.RegisterClean(entity)
				if p.config.VerboseLog {
					zlogger.Debug().
						Str("entity_type", reflect.TypeOf(entity).String()).
						Interface("entity_id", entity.GetID()).
						Msg("Auto UnitOfWork: Created snapshot for queried entity")
				}
			}
		}
	}
}

// onCreateBefore 创建前的回调
func (p *AutoUnitOfWorkPlugin) onCreateBefore(db *gorm.DB) {
	if db.Error != nil {
		return
	}

	uow := p.getUowFromContext(db.Statement.Context)
	if uow == nil {
		return // 不在事务中，直接执行
	}

	if p.shouldSkipTable(db.Statement.Table) {
		return
	}

	// 提取实体并注册为新实体
	entities := p.extractEntitiesFromDest(db.Statement.Dest)
	for _, entity := range entities {
		if entity != nil {
			err := uow.RegisterNew(entity)
			if err != nil {
				db.AddError(fmt.Errorf("failed to register new entity: %w", err))
				return
			}

			if p.config.VerboseLog {
				zlogger.Debug().
					Str("entity_type", reflect.TypeOf(entity).String()).
					Interface("entity_id", entity.GetID()).
					Msg("Auto UnitOfWork: Registered new entity")
			}
		}
	}

	// 阻止 GORM 立即执行，等待工作单元统一提交
	db.SkipDefaultTransaction = true
}

// onUpdateBefore 更新前的回调
func (p *AutoUnitOfWorkPlugin) onUpdateBefore(db *gorm.DB) {
	if db.Error != nil {
		return
	}

	uow := p.getUowFromContext(db.Statement.Context)
	if uow == nil {
		return // 不在事务中，直接执行
	}

	if p.shouldSkipTable(db.Statement.Table) {
		return
	}

	// 提取实体并注册为脏实体
	entities := p.extractEntitiesFromDest(db.Statement.Dest)
	for _, entity := range entities {
		if entity != nil {
			err := uow.RegisterDirty(entity)
			if err != nil {
				db.AddError(fmt.Errorf("failed to register dirty entity: %w", err))
				return
			}

			if p.config.VerboseLog {
				zlogger.Debug().
					Str("entity_type", reflect.TypeOf(entity).String()).
					Interface("entity_id", entity.GetID()).
					Msg("Auto UnitOfWork: Registered dirty entity")
			}
		}
	}

	// 阻止 GORM 立即执行，等待工作单元统一提交
	db.SkipDefaultTransaction = true
}

// onDeleteBefore 删除前的回调
func (p *AutoUnitOfWorkPlugin) onDeleteBefore(db *gorm.DB) {
	if db.Error != nil {
		return
	}

	uow := p.getUowFromContext(db.Statement.Context)
	if uow == nil {
		return // 不在事务中，直接执行
	}

	if p.shouldSkipTable(db.Statement.Table) {
		return
	}

	// 提取实体并注册为删除实体
	entities := p.extractEntitiesFromDest(db.Statement.Dest)
	for _, entity := range entities {
		if entity != nil {
			err := uow.RegisterRemoved(entity)
			if err != nil {
				db.AddError(fmt.Errorf("failed to register removed entity: %w", err))
				return
			}

			if p.config.VerboseLog {
				zlogger.Debug().
					Str("entity_type", reflect.TypeOf(entity).String()).
					Interface("entity_id", entity.GetID()).
					Msg("Auto UnitOfWork: Registered removed entity")
			}
		}
	}

	// 阻止 GORM 立即执行，等待工作单元统一提交
	db.SkipDefaultTransaction = true
}

// onTransactionCommit 事务提交时的回调
func (p *AutoUnitOfWorkPlugin) onTransactionCommit(db *gorm.DB) {
	uow := p.getUowFromContext(db.Statement.Context)
	if uow == nil {
		return
	}

	if db.Error != nil {
		// 如果已经有错误，执行回滚
		p.rollbackUow(uow, db)
		return
	}

	// 提交工作单元
	err := uow.Commit()
	if err != nil {
		db.AddError(fmt.Errorf("unit of work commit failed: %w", err))
		p.rollbackUow(uow, db)
		return
	}

	if p.config.VerboseLog {
		stats := uow.GetStats()
		zlogger.Info().
			Interface("stats", stats).
			Msg("Auto UnitOfWork: Successfully committed unit of work")
	}
}

// onTransactionRollback 事务回滚时的回调
func (p *AutoUnitOfWorkPlugin) onTransactionRollback(db *gorm.DB) {
	uow := p.getUowFromContext(db.Statement.Context)
	if uow == nil {
		return
	}

	p.rollbackUow(uow, db)
}

// rollbackUow 回滚工作单元
func (p *AutoUnitOfWorkPlugin) rollbackUow(uow *UnitOfWork, db *gorm.DB) {
	err := uow.Rollback()
	if err != nil {
		zlogger.Error().Err(err).Msg("Auto UnitOfWork: Failed to rollback unit of work")
	} else if p.config.VerboseLog {
		zlogger.Info().Msg("Auto UnitOfWork: Successfully rolled back unit of work")
	}
}

// extractEntitiesFromDest 从目标对象中提取实体
func (p *AutoUnitOfWorkPlugin) extractEntitiesFromDest(dest interface{}) []Entity {
	if dest == nil {
		return nil
	}

	var entities []Entity

	destValue := reflect.ValueOf(dest)
	if destValue.Kind() == reflect.Ptr {
		destValue = destValue.Elem()
	}

	switch destValue.Kind() {
	case reflect.Slice:
		for i := 0; i < destValue.Len(); i++ {
			item := destValue.Index(i)
			if entity := p.convertToEntity(item.Interface()); entity != nil {
				entities = append(entities, entity)
			}
		}
	case reflect.Struct:
		if entity := p.convertToEntity(dest); entity != nil {
			entities = append(entities, entity)
		}
	default:
		if entity := p.convertToEntity(dest); entity != nil {
			entities = append(entities, entity)
		}
	}

	return entities
}

// convertToEntity 尝试将对象转换为实体
func (p *AutoUnitOfWorkPlugin) convertToEntity(obj interface{}) Entity {
	if obj == nil {
		return nil
	}

	// 直接类型断言
	if entity, ok := obj.(Entity); ok {
		return entity
	}

	// 如果是指针，尝试解引用
	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		if entity, ok := reflect.ValueOf(obj).Elem().Interface().(Entity); ok {
			return entity
		}
	}

	// 使用自定义检测器
	if p.config.EntityDetector != nil {
		return p.config.EntityDetector(obj)
	}

	// 自动检测：检查是否实现了必要的方法
	if p.config.AutoDetectEntity {
		return p.autoDetectEntity(obj)
	}

	return nil
}

// autoDetectEntity 自动检测实体接口
func (p *AutoUnitOfWorkPlugin) autoDetectEntity(obj interface{}) Entity {
	if obj == nil {
		return nil
	}

	objType := reflect.TypeOf(obj)
	objValue := reflect.ValueOf(obj)

	// 检查是否有必要的方法
	requiredMethods := []string{"GetID", "SetID", "GetTableName", "IsNew"}
	for _, methodName := range requiredMethods {
		if _, found := objType.MethodByName(methodName); !found {
			return nil
		}
	}

	// 创建一个包装器实现 Entity 接口
	wrapper := &entityWrapper{
		obj:   obj,
		value: objValue,
		typ:   objType,
	}

	return wrapper
}

// entityWrapper 实体包装器，用于自动检测的实体
type entityWrapper struct {
	obj   interface{}
	value reflect.Value
	typ   reflect.Type
}

func (w *entityWrapper) GetID() uint {
	method := w.value.MethodByName("GetID")
	if !method.IsValid() {
		return 0
	}
	results := method.Call(nil)
	if len(results) > 0 {
		return results[0].Interface().(uint)
	}
	return 0
}

func (w *entityWrapper) SetID(id uint) {
	method := w.value.MethodByName("SetID")
	if !method.IsValid() {
		return
	}
	method.Call([]reflect.Value{reflect.ValueOf(id)})
}

func (w *entityWrapper) GetTableName() string {
	method := w.value.MethodByName("GetTableName")
	if !method.IsValid() {
		return ""
	}
	results := method.Call(nil)
	if len(results) > 0 {
		if tableName, ok := results[0].Interface().(string); ok {
			return tableName
		}
	}
	return ""
}

func (w *entityWrapper) IsNew() bool {
	method := w.value.MethodByName("IsNew")
	if !method.IsValid() {
		return true
	}
	results := method.Call(nil)
	if len(results) > 0 {
		if isNew, ok := results[0].Interface().(bool); ok {
			return isNew
		}
	}
	return true
}

// shouldSkipTable 检查是否应该跳过指定表
func (p *AutoUnitOfWorkPlugin) shouldSkipTable(tableName string) bool {
	for _, excluded := range p.config.ExcludedTables {
		if strings.EqualFold(tableName, excluded) {
			return true
		}
	}
	return false
}

// getUowFromContext 从上下文获取工作单元
func (p *AutoUnitOfWorkPlugin) getUowFromContext(ctx context.Context) *UnitOfWork {
	if ctx == nil {
		return nil
	}

	uow, _ := ctx.Value(uowContextKey).(*UnitOfWork)
	return uow
}

// setUowToContext 将工作单元设置到上下文
func (p *AutoUnitOfWorkPlugin) setUowToContext(ctx context.Context, uow *UnitOfWork) context.Context {
	return context.WithValue(ctx, uowContextKey, uow)
}

// GetCurrentUnitOfWork 获取当前上下文中的工作单元（公共 API）
func GetCurrentUnitOfWork(ctx context.Context) *UnitOfWork {
	if ctx == nil {
		return nil
	}
	uow, _ := ctx.Value(uowContextKey).(*UnitOfWork)
	return uow
}

// WithManualUnitOfWork 手动设置工作单元到上下文（用于特殊场景）
func WithManualUnitOfWork(ctx context.Context, uow *UnitOfWork) context.Context {
	return context.WithValue(ctx, uowContextKey, uow)
}
