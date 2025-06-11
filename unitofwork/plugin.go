package unitofwork

import (
	"context"
	"fmt"
	"reflect"

	"github.com/unionj-cloud/toolkit/zlogger"
	"github.com/wubin1989/gorm"
)

// Plugin 工作单元GORM插件
type Plugin struct {
	name           string
	config         *PluginConfig
	originalCreate func(*gorm.DB)
	originalUpdate func(*gorm.DB)
	originalDelete func(*gorm.DB)
}

// PluginConfig 插件配置
type PluginConfig struct {
	// 工作单元配置
	UnitOfWorkConfig *Config

	// 是否自动开启工作单元
	AutoManage bool

	// 上下文键名
	ContextKey string

	// 是否启用自动依赖注册
	AutoDependencyRegistration bool

	// 依赖关系映射
	DependencyMapping map[reflect.Type][]reflect.Type
}

// DefaultPluginConfig 默认插件配置
func DefaultPluginConfig() *PluginConfig {
	return &PluginConfig{
		UnitOfWorkConfig:           DefaultConfig(),
		AutoManage:                 true,
		ContextKey:                 "unitofwork",
		AutoDependencyRegistration: true,
		DependencyMapping:          make(map[reflect.Type][]reflect.Type),
	}
}

// NewPlugin 创建工作单元插件
func NewPlugin(options ...PluginOption) *Plugin {
	config := DefaultPluginConfig()

	for _, option := range options {
		option(config)
	}

	return &Plugin{
		name:   "unitofwork",
		config: config,
	}
}

// PluginOption 插件配置选项
type PluginOption func(*PluginConfig)

// WithPluginAutoManage 配置自动管理
func WithPluginAutoManage(enabled bool) PluginOption {
	return func(c *PluginConfig) {
		c.AutoManage = enabled
	}
}

// WithPluginContextKey 配置上下文键名
func WithPluginContextKey(key string) PluginOption {
	return func(c *PluginConfig) {
		c.ContextKey = key
	}
}

// WithPluginUnitOfWorkConfig 配置工作单元
func WithPluginUnitOfWorkConfig(config *Config) PluginOption {
	return func(c *PluginConfig) {
		c.UnitOfWorkConfig = config
	}
}

// WithPluginDependencyMapping 配置依赖关系映射
func WithPluginDependencyMapping(mapping map[reflect.Type][]reflect.Type) PluginOption {
	return func(c *PluginConfig) {
		c.DependencyMapping = mapping
	}
}

// Name 实现gorm.Plugin接口
func (p *Plugin) Name() string {
	return p.name
}

// Initialize 实现gorm.Plugin接口
func (p *Plugin) Initialize(db *gorm.DB) error {
	zlogger.Info().Str("plugin", p.name).Msg("Initializing UnitOfWork plugin")

	// 注册回调函数
	if err := p.registerCallbacks(db); err != nil {
		return fmt.Errorf("failed to register callbacks: %w", err)
	}

	zlogger.Info().Str("plugin", p.name).Msg("UnitOfWork plugin initialized successfully")
	return nil
}

// registerCallbacks 注册回调函数
func (p *Plugin) registerCallbacks(db *gorm.DB) error {
	// 保存原始回调函数
	p.originalCreate = db.Callback().Create().Get("gorm:create")
	p.originalUpdate = db.Callback().Update().Get("gorm:update")
	p.originalDelete = db.Callback().Delete().Get("gorm:delete")

	// Create callbacks - 替换 gorm:create 回调
	if err := db.Callback().Create().Replace("gorm:create", p.unitOfWorkCreate); err != nil {
		return err
	}

	if err := db.Callback().Create().After("gorm:create").Register("unitofwork:after_create", p.afterCreate); err != nil {
		return err
	}

	// Update callbacks - 替换 gorm:update 回调
	if err := db.Callback().Update().Replace("gorm:update", p.unitOfWorkUpdate); err != nil {
		return err
	}

	if err := db.Callback().Update().After("gorm:update").Register("unitofwork:after_update", p.afterUpdate); err != nil {
		return err
	}

	// Delete callbacks - 替换 gorm:delete 回调
	if err := db.Callback().Delete().Replace("gorm:delete", p.unitOfWorkDelete); err != nil {
		return err
	}

	if err := db.Callback().Delete().After("gorm:delete").Register("unitofwork:after_delete", p.afterDelete); err != nil {
		return err
	}

	// Query callbacks - 在查询后创建快照
	if err := db.Callback().Query().After("gorm:query").Register("unitofwork:after_query", p.afterQuery); err != nil {
		return err
	}

	return nil
}

// processEntities 处理db.Statement.Dest中的实体，支持单个实体或实体切片
func (p *Plugin) processEntities(db *gorm.DB, processor func(Entity) error) error {
	if db.Statement.Dest == nil {
		return nil
	}

	// 初始化影响行数为0
	db.RowsAffected = 0

	destValue := reflect.ValueOf(db.Statement.Dest)
	if destValue.Kind() == reflect.Ptr {
		destValue = destValue.Elem()
	}

	switch destValue.Kind() {
	case reflect.Struct:
		// 单个实体
		if entity, ok := db.Statement.Dest.(Entity); ok {
			if err := processor(entity); err != nil {
				return err
			}
			// 成功处理单个实体，设置影响行数为1
			db.RowsAffected = 1
			return nil
		}
		return nil
	case reflect.Slice:
		// 实体切片
		var processedCount int
		for i := 0; i < destValue.Len(); i++ {
			item := destValue.Index(i)
			if item.Kind() == reflect.Ptr && !item.IsNil() {
				if entity, ok := item.Interface().(Entity); ok {
					if err := processor(entity); err != nil {
						return err
					}
					processedCount++
				}
			} else if item.CanInterface() {
				if entity, ok := item.Interface().(Entity); ok {
					if err := processor(entity); err != nil {
						return err
					}
					processedCount++
				}
			}
		}
		// 设置影响的行数
		db.RowsAffected = int64(processedCount)
		return nil
	default:
		// 尝试直接转换
		if entity, ok := db.Statement.Dest.(Entity); ok {
			if err := processor(entity); err != nil {
				return err
			}
			// 成功处理实体，设置影响行数为1
			db.RowsAffected = 1
			return nil
		}
		return nil
	}
}

// unitOfWorkCreate 工作单元创建回调，替换 gorm:create
func (p *Plugin) unitOfWorkCreate(db *gorm.DB) {
	if !p.config.AutoManage {
		// 如果未启用自动管理，则调用原始的 GORM Create 逻辑
		if p.originalCreate != nil {
			p.originalCreate(db)
		}
		return
	}

	// 首先尝试从上下文获取现有的工作单元
	uow := GetUnitOfWorkFromContext(db.Statement.Context, p.config.ContextKey)

	// 如果没有现有的工作单元，直接调用原始的 GORM Create 逻辑
	if uow == nil {
		if p.originalCreate != nil {
			p.originalCreate(db)
		}
		return
	}

	// 如果工作单元正在执行操作，直接调用原始回调避免死锁
	if p.isExecuting(uow) {
		if p.originalCreate != nil {
			p.originalCreate(db)
		}
		return
	}

	// 处理实体（支持单个实体或实体切片）
	err := p.processEntities(db, func(entity Entity) error {
		if err := uow.Create(entity); err != nil {
			return err
		}

		if p.config.UnitOfWorkConfig.EnableDetailLog {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Interface("entity_id", entity.GetID()).
				Msg("Entity registered for creation in unit of work")
		}
		return nil
	})

	if err != nil {
		zlogger.Error().Err(err).Msg("Failed to register entity for creation")
		db.AddError(err)
		return
	}

	// 如果没有处理任何实体，则调用原始的 GORM Create 逻辑
	if db.RowsAffected == 0 {
		if p.originalCreate != nil {
			p.originalCreate(db)
		}
	}
}

// unitOfWorkUpdate 工作单元更新回调，替换 gorm:update
func (p *Plugin) unitOfWorkUpdate(db *gorm.DB) {
	if !p.config.AutoManage {
		// 如果未启用自动管理，则调用原始的 GORM Update 逻辑
		if p.originalUpdate != nil {
			p.originalUpdate(db)
		}
		return
	}

	// 首先尝试从上下文获取现有的工作单元
	uow := GetUnitOfWorkFromContext(db.Statement.Context, p.config.ContextKey)

	// 如果没有现有的工作单元，直接调用原始的 GORM Update 逻辑
	if uow == nil {
		if p.originalUpdate != nil {
			p.originalUpdate(db)
		}
		return
	}

	// 如果工作单元正在执行操作，直接调用原始回调避免死锁
	if p.isExecuting(uow) {
		if p.originalUpdate != nil {
			p.originalUpdate(db)
		}
		return
	}

	// 处理实体（支持单个实体或实体切片）
	err := p.processEntities(db, func(entity Entity) error {
		if err := uow.Update(entity); err != nil {
			return err
		}

		if p.config.UnitOfWorkConfig.EnableDetailLog {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Msg("Entity registered for update in unit of work")
		}
		return nil
	})

	if err != nil {
		zlogger.Error().Err(err).Msg("Failed to register entity for update")
		db.AddError(err)
		return
	}

	// 如果没有处理任何实体，则调用原始的 GORM Update 逻辑
	if db.RowsAffected == 0 {
		if p.originalUpdate != nil {
			p.originalUpdate(db)
		}
	}
}

// unitOfWorkDelete 工作单元删除回调，替换 gorm:delete
func (p *Plugin) unitOfWorkDelete(db *gorm.DB) {
	if !p.config.AutoManage {
		// 如果未启用自动管理，则调用原始的 GORM Delete 逻辑
		if p.originalDelete != nil {
			p.originalDelete(db)
		}
		return
	}

	// 首先尝试从上下文获取现有的工作单元
	uow := GetUnitOfWorkFromContext(db.Statement.Context, p.config.ContextKey)

	// 如果没有现有的工作单元，直接调用原始的 GORM Delete 逻辑
	if uow == nil {
		if p.originalDelete != nil {
			p.originalDelete(db)
		}
		return
	}

	// 如果工作单元正在执行操作，直接调用原始回调避免死锁
	if p.isExecuting(uow) {
		if p.originalDelete != nil {
			p.originalDelete(db)
		}
		return
	}

	// 处理实体（支持单个实体或实体切片）
	err := p.processEntities(db, func(entity Entity) error {
		if err := uow.Delete(entity); err != nil {
			return err
		}

		if p.config.UnitOfWorkConfig.EnableDetailLog {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Msg("Entity registered for deletion in unit of work")
		}
		return nil
	})

	if err != nil {
		zlogger.Error().Err(err).Msg("Failed to register entity for deletion")
		db.AddError(err)
		return
	}

	// 如果没有处理任何实体，则调用原始的 GORM Delete 逻辑
	if db.RowsAffected == 0 {
		if p.originalDelete != nil {
			p.originalDelete(db)
		}
	}
}

// afterCreate 创建后回调
func (p *Plugin) afterCreate(db *gorm.DB) {
	if !p.config.AutoManage {
		return
	}

	// 处理实体（支持单个实体或实体切片）
	if p.config.UnitOfWorkConfig.EnableDetailLog {
		p.processEntities(db, func(entity Entity) error {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Interface("entity_id", entity.GetID()).
				Msg("Entity created in unit of work")
			return nil
		})
	}
}

// afterUpdate 更新后回调
func (p *Plugin) afterUpdate(db *gorm.DB) {
	if !p.config.AutoManage {
		return
	}

	// 处理实体（支持单个实体或实体切片）
	if p.config.UnitOfWorkConfig.EnableDetailLog {
		p.processEntities(db, func(entity Entity) error {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Interface("entity_id", entity.GetID()).
				Msg("Entity updated in unit of work")
			return nil
		})
	}
}

// afterDelete 删除后回调
func (p *Plugin) afterDelete(db *gorm.DB) {
	if !p.config.AutoManage {
		return
	}

	// 处理实体（支持单个实体或实体切片）
	if p.config.UnitOfWorkConfig.EnableDetailLog {
		p.processEntities(db, func(entity Entity) error {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Interface("entity_id", entity.GetID()).
				Msg("Entity deleted in unit of work")
			return nil
		})
	}
}

// registerDependencies 注册依赖关系
func (p *Plugin) registerDependencies(uow *UnitOfWork) {
	depManager := uow.GetDependencyManager()

	for dependent, dependencies := range p.config.DependencyMapping {
		for _, dependency := range dependencies {
			depManager.RegisterDependency(dependent, dependency)
		}
	}
}

// isExecuting 检查工作单元是否正在执行操作
func (p *Plugin) isExecuting(uow *UnitOfWork) bool {
	uow.mu.RLock()
	defer uow.mu.RUnlock()
	return uow.isExecuting
}

// ContextUnitOfWork 上下文工作单元键
type ContextUnitOfWork string

// GetUnitOfWorkFromContext 从上下文获取工作单元
func GetUnitOfWorkFromContext(ctx context.Context, key string) *UnitOfWork {
	if ctx == nil {
		return nil
	}

	if uow, ok := ctx.Value(ContextUnitOfWork(key)).(*UnitOfWork); ok {
		return uow
	}

	return nil
}

// SetUnitOfWorkToContext 将工作单元设置到上下文
func SetUnitOfWorkToContext(ctx context.Context, uow *UnitOfWork, key string) context.Context {
	return context.WithValue(ctx, ContextUnitOfWork(key), uow)
}

// WithUnitOfWork 在上下文中使用工作单元
func WithUnitOfWork(ctx context.Context, db *gorm.DB, fn func(*gorm.DB, *UnitOfWork) error, options ...ConfigOption) error {
	config := DefaultConfig()
	for _, option := range options {
		option(config)
	}

	// 在事务中执行
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 使用事务连接创建工作单元
		uow := NewUnitOfWork(tx, func(c *Config) {
			*c = *config
		})

		// 将工作单元添加到上下文
		txCtx := SetUnitOfWorkToContext(tx.Statement.Context, uow, "unitofwork")
		tx.Statement.Context = txCtx

		// 注册验证回调
		validationCallback := func(db *gorm.DB) {
			if entity, ok := db.Statement.Dest.(Validatable); ok {
				if err := entity.Validate(); err != nil {
					db.AddError(fmt.Errorf("validation failed for entity %T: %w", entity, err))
				}
			}
		}

		// 注册到所有相关的回调点
		tx.Callback().Create().Before("gorm:create").Register("unitofwork:validate_create", validationCallback)
		tx.Callback().Update().Before("gorm:update").Register("unitofwork:validate_update", validationCallback)

		// 执行业务逻辑
		err := func() error {
			defer func() {
				// 清理回调
				tx.Callback().Create().Remove("unitofwork:validate_create")
				tx.Callback().Update().Remove("unitofwork:validate_update")
			}()

			return fn(tx, uow)
		}()

		if err != nil {
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				zlogger.Error().Err(rollbackErr).Msg("Failed to rollback unit of work")
			}
			return fmt.Errorf("business logic failed: %w", err)
		}

		// 提交工作单元
		if err := uow.Commit(); err != nil {
			return fmt.Errorf("failed to commit unit of work: %w", err)
		}

		return nil
	})
}

// afterQuery 查询后回调，为查询到的实体创建快照
func (p *Plugin) afterQuery(db *gorm.DB) {
	if !p.config.AutoManage {
		return
	}

	// 尝试从上下文获取工作单元
	uow := GetUnitOfWorkFromContext(db.Statement.Context, p.config.ContextKey)
	if uow == nil {
		return
	}

	// 处理实体（支持单个实体或实体切片）
	p.processEntities(db, func(entity Entity) error {
		uow.TakeSnapshot(entity)
		if p.config.UnitOfWorkConfig.EnableDetailLog {
			zlogger.Debug().
				Str("entity_type", reflect.TypeOf(entity).String()).
				Interface("entity_id", entity.GetID()).
				Msg("Created snapshot for queried entity")
		}
		return nil
	})
}
