package unitofwork

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/wubin1989/gorm"

	"github.com/unionj-cloud/toolkit/zlogger"
)

// UnitOfWork 工作单元核心实现
type UnitOfWork struct {
	// 数据库连接
	db *gorm.DB

	// 状态管理
	newEntities     map[reflect.Type][]Entity
	dirtyEntities   map[reflect.Type][]Entity
	removedEntities map[reflect.Type][]Entity
	cleanEntities   map[reflect.Type][]Entity

	// 快照管理器
	snapshotManager *SnapshotManager

	// 依赖管理器
	dependencyManager *DependencyManager

	// 操作队列
	operations []Operation

	// 状态标志
	isCommitted  bool
	isRolledBack bool

	// 同步锁
	mu sync.RWMutex

	// 配置
	config *Config

	// 上下文
	ctx context.Context
}

// Config 工作单元配置
type Config struct {
	// 是否启用自动脏检查
	EnableDirtyCheck bool

	// 批量操作大小
	BatchSize int

	// 是否启用操作合并
	EnableOperationMerge bool

	// 最大实体数量（内存保护）
	MaxEntityCount int

	// 是否启用详细日志
	EnableDetailLog bool
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		EnableDirtyCheck:     true,
		BatchSize:            1000,
		EnableOperationMerge: true,
		MaxEntityCount:       10000,
		EnableDetailLog:      false,
	}
}

// NewUnitOfWork 创建工作单元
func NewUnitOfWork(db *gorm.DB, options ...ConfigOption) *UnitOfWork {
	config := DefaultConfig()
	for _, option := range options {
		option(config)
	}

	return &UnitOfWork{
		db:                db,
		newEntities:       make(map[reflect.Type][]Entity),
		dirtyEntities:     make(map[reflect.Type][]Entity),
		removedEntities:   make(map[reflect.Type][]Entity),
		cleanEntities:     make(map[reflect.Type][]Entity),
		snapshotManager:   NewSnapshotManager(),
		dependencyManager: DefaultDependencyManager(),
		operations:        make([]Operation, 0),
		config:            config,
		ctx:               context.Background(),
	}
}

// ConfigOption 配置选项函数
type ConfigOption func(*Config)

// WithDirtyCheck 配置脏检查
func WithDirtyCheck(enabled bool) ConfigOption {
	return func(c *Config) {
		c.EnableDirtyCheck = enabled
	}
}

// WithBatchSize 配置批量大小
func WithBatchSize(size int) ConfigOption {
	return func(c *Config) {
		c.BatchSize = size
	}
}

// WithOperationMerge 配置操作合并
func WithOperationMerge(enabled bool) ConfigOption {
	return func(c *Config) {
		c.EnableOperationMerge = enabled
	}
}

// WithMaxEntityCount 配置最大实体数量
func WithMaxEntityCount(count int) ConfigOption {
	return func(c *Config) {
		c.MaxEntityCount = count
	}
}

// WithDetailLog 配置详细日志
func WithDetailLog(enabled bool) ConfigOption {
	return func(c *Config) {
		c.EnableDetailLog = enabled
	}
}

// WithContext 设置上下文
func (uow *UnitOfWork) WithContext(ctx context.Context) *UnitOfWork {
	uow.ctx = ctx
	return uow
}

// RegisterNew 注册新实体
func (uow *UnitOfWork) RegisterNew(entity Entity) error {
	uow.mu.Lock()
	defer uow.mu.Unlock()

	if uow.isCommitted || uow.isRolledBack {
		return fmt.Errorf("unit of work is already finished")
	}

	if entity == nil {
		return fmt.Errorf("entity cannot be nil")
	}

	entityType := reflect.TypeOf(entity)

	// 检查是否已经在其他状态中
	if uow.containsEntity(uow.dirtyEntities, entity) {
		return fmt.Errorf("entity is already marked as dirty")
	}

	if uow.containsEntity(uow.removedEntities, entity) {
		return fmt.Errorf("entity is already marked for removal")
	}

	if uow.containsEntity(uow.newEntities, entity) {
		return nil
	}

	// 检查内存限制
	if err := uow.checkMemoryLimit(); err != nil {
		return err
	}

	// 添加到新实体列表
	uow.newEntities[entityType] = append(uow.newEntities[entityType], entity)

	// 添加操作
	operation := NewInsertOperation(entity, uow)
	uow.addOperation(operation)

	if uow.config.EnableDetailLog {
		zlogger.Info().
			Str("entity_type", entityType.String()).
			Interface("entity_id", entity.GetID()).
			Msg("Registered new entity")
	}

	return nil
}

// RegisterDirty 注册脏实体
func (uow *UnitOfWork) RegisterDirty(entity Entity) error {
	uow.mu.Lock()
	defer uow.mu.Unlock()

	if uow.isCommitted || uow.isRolledBack {
		return fmt.Errorf("unit of work is already finished")
	}

	if entity == nil {
		return fmt.Errorf("entity cannot be nil")
	}

	entityType := reflect.TypeOf(entity)

	// 如果是新实体，不需要标记为脏
	if uow.containsEntity(uow.newEntities, entity) {
		return nil
	}

	// 检查是否已经标记为删除
	if uow.containsEntity(uow.removedEntities, entity) {
		return fmt.Errorf("cannot mark removed entity as dirty")
	}

	// 检查是否已经标记为脏
	if uow.containsEntity(uow.dirtyEntities, entity) {
		return nil
	}

	// 脏检查
	var changes map[string]FieldChange
	if uow.config.EnableDirtyCheck {
		if !uow.snapshotManager.IsDirty(entity) {
			return nil // 实际上没有变更
		}
		changes = uow.snapshotManager.GetChangedFields(entity)
	}

	// 添加到脏实体列表
	uow.dirtyEntities[entityType] = append(uow.dirtyEntities[entityType], entity)

	// 添加操作
	operation := NewUpdateOperation(entity, changes)
	uow.addOperation(operation)

	if uow.config.EnableDetailLog {
		zlogger.Info().
			Str("entity_type", entityType.String()).
			Interface("entity_id", entity.GetID()).
			Int("changed_fields", len(changes)).
			Msg("Registered dirty entity")
	}

	return nil
}

// RegisterRemoved 注册删除实体
func (uow *UnitOfWork) RegisterRemoved(entity Entity) error {
	uow.mu.Lock()
	defer uow.mu.Unlock()

	if uow.isCommitted || uow.isRolledBack {
		return fmt.Errorf("unit of work is already finished")
	}

	if entity == nil {
		return fmt.Errorf("entity cannot be nil")
	}

	entityType := reflect.TypeOf(entity)

	// 如果是新实体，直接从新实体列表移除
	if uow.removeFromEntityList(uow.newEntities, entity) {
		uow.removeOperationByEntity(entity)
		if uow.config.EnableDetailLog {
			zlogger.Info().
				Str("entity_type", entityType.String()).
				Interface("entity_id", entity.GetID()).
				Msg("Removed new entity from registration")
		}
		return nil
	}

	// 从脏实体列表移除
	uow.removeFromEntityList(uow.dirtyEntities, entity)
	uow.removeOperationByEntity(entity)

	// 添加到删除列表
	uow.removedEntities[entityType] = append(uow.removedEntities[entityType], entity)

	// 添加操作
	operation := NewDeleteOperation(entity)
	uow.addOperation(operation)

	if uow.config.EnableDetailLog {
		zlogger.Info().
			Str("entity_type", entityType.String()).
			Interface("entity_id", entity.GetID()).
			Msg("Registered entity for removal")
	}

	return nil
}

// RegisterClean 注册干净实体（用于脏检查）
func (uow *UnitOfWork) RegisterClean(entity Entity) error {
	if !uow.config.EnableDirtyCheck {
		return nil
	}

	uow.mu.Lock()
	defer uow.mu.Unlock()

	if uow.isCommitted || uow.isRolledBack {
		return fmt.Errorf("unit of work is already finished")
	}

	if uow.containsEntity(uow.newEntities, entity) {
		return nil
	}

	// 检查是否已经标记为删除
	if uow.containsEntity(uow.removedEntities, entity) {
		return nil
	}

	// 检查是否已经标记为脏
	if uow.containsEntity(uow.dirtyEntities, entity) {
		return nil
	}

	if uow.containsEntity(uow.cleanEntities, entity) {
		return nil
	}

	uow.snapshotManager.TakeSnapshot(entity)

	entityType := reflect.TypeOf(entity)

	uow.cleanEntities[entityType] = append(uow.cleanEntities[entityType], entity)

	return nil
}

// Commit 提交所有变更
func (uow *UnitOfWork) Commit() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()

	if uow.isCommitted {
		return fmt.Errorf("unit of work is already committed")
	}

	if uow.isRolledBack {
		return fmt.Errorf("unit of work is already rolled back")
	}

	startTime := time.Now()
	totalEntities := uow.getTotalEntityCount()

	zlogger.Info().
		Int("new_entities", uow.getEntityCountByType(uow.newEntities)).
		Int("dirty_entities", uow.getEntityCountByType(uow.dirtyEntities)).
		Int("removed_entities", uow.getEntityCountByType(uow.removedEntities)).
		Int("total_operations", len(uow.operations)).
		Msg("Starting unit of work commit")

	// 在事务中执行所有操作
	err := uow.db.WithContext(uow.ctx).Transaction(func(tx *gorm.DB) error {
		return uow.executeOperations(tx)
	})

	if err != nil {
		zlogger.Error().Err(err).Msg("Unit of work commit failed")
		return fmt.Errorf("unit of work commit failed: %w", err)
	}

	uow.isCommitted = true
	duration := time.Since(startTime)

	zlogger.Info().
		Int("total_entities", totalEntities).
		Dur("duration", duration).
		Msg("Unit of work committed successfully")

	// 清理资源
	uow.clear()

	return nil
}

// Rollback 回滚所有变更
func (uow *UnitOfWork) Rollback() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()

	if uow.isCommitted {
		return fmt.Errorf("unit of work is already committed")
	}

	if uow.isRolledBack {
		return fmt.Errorf("unit of work is already rolled back")
	}

	uow.isRolledBack = true

	zlogger.Info().Msg("Unit of work rolled back")

	// 清理资源
	uow.clear()

	return nil
}

// executeOperations 执行所有操作
func (uow *UnitOfWork) executeOperations(tx *gorm.DB) error {
	// 自动脏检查
	if uow.config.EnableDirtyCheck {
		if err := uow.detectDirtyEntities(); err != nil {
			return err
		}
	}

	// 操作优化
	optimizedOps := uow.optimizeOperations()

	// 按依赖顺序排序操作
	sortedOps, err := uow.sortOperationsByDependency(optimizedOps)
	if err != nil {
		return err
	}

	// 执行操作
	for i, operation := range sortedOps {
		if uow.config.EnableDetailLog {
			zlogger.Debug().
				Int("operation_index", i).
				Str("operation_type", operation.GetOperationType().String()).
				Str("entity_type", operation.GetEntityType().String()).
				Msg("Executing operation")
		}

		if err := operation.Execute(uow.ctx, tx); err != nil {
			return fmt.Errorf("operation %d failed: %w", i, err)
		}
	}

	return nil
}

// detectDirtyEntities 自动检测脏实体
func (uow *UnitOfWork) detectDirtyEntities() error {
	if !uow.config.EnableDirtyCheck {
		return nil // 脏检查未启用，直接返回
	}

	if uow.config.EnableDetailLog {
		zlogger.Debug().Msg("Starting automatic dirty entity detection")
	}

	// 获取快照管理器中的所有快照
	snapshots := uow.snapshotManager.snapshots
	detectedCount := 0

	// 遍历所有快照，检查对应的实体是否发生了变更
	for key, snapshot := range snapshots {
		// 尝试从已知的实体集合中找到对应的实体
		entity := uow.findEntityBySnapshot(snapshot)
		if entity == nil {
			// 实体不在当前工作单元的管理范围内，跳过
			continue
		}

		// 检查实体是否已经被标记为脏或删除
		if uow.isEntityAlreadyTracked(entity) {
			continue
		}

		// 使用快照检查实体是否发生变更
		if snapshot.IsDirty(entity) {
			// 获取变更的字段
			changes := snapshot.GetChangedFields(entity)

			// 注册为脏实体
			err := uow.registerDirtyEntityInternal(entity, changes)
			if err != nil {
				zlogger.Error().
					Err(err).
					Str("entity_type", reflect.TypeOf(entity).String()).
					Interface("entity_id", entity.GetID()).
					Msg("Failed to register automatically detected dirty entity")
				continue
			}

			detectedCount++

			if uow.config.EnableDetailLog {
				zlogger.Debug().
					Str("entity_type", reflect.TypeOf(entity).String()).
					Interface("entity_id", entity.GetID()).
					Int("changed_fields", len(changes)).
					Str("snapshot_key", key).
					Msg("Automatically detected dirty entity")
			}
		}
	}

	if uow.config.EnableDetailLog {
		zlogger.Debug().
			Int("detected_count", detectedCount).
			Int("total_snapshots", len(snapshots)).
			Msg("Automatic dirty entity detection completed")
	}

	return nil
}

// findEntityBySnapshot 根据快照查找对应的实体
func (uow *UnitOfWork) findEntityBySnapshot(snapshot *EntitySnapshot) Entity {
	// 在新实体中查找
	if entities, exists := uow.newEntities[snapshot.entityType]; exists {
		for _, entity := range entities {
			if entity.GetID() == snapshot.entityID {
				return entity
			}
		}
	}

	// 在脏实体中查找
	if entities, exists := uow.dirtyEntities[snapshot.entityType]; exists {
		for _, entity := range entities {
			if entity.GetID() == snapshot.entityID {
				return entity
			}
		}
	}

	// 在删除实体中查找
	if entities, exists := uow.removedEntities[snapshot.entityType]; exists {
		for _, entity := range entities {
			if entity.GetID() == snapshot.entityID {
				return entity
			}
		}
	}

	if entities, exists := uow.cleanEntities[snapshot.entityType]; exists {
		for _, entity := range entities {
			if entity.GetID() == snapshot.entityID {
				return entity
			}
		}
	}
	return nil
}

// isEntityAlreadyTracked 检查实体是否已经被跟踪
func (uow *UnitOfWork) isEntityAlreadyTracked(entity Entity) bool {
	return uow.containsEntity(uow.newEntities, entity) ||
		uow.containsEntity(uow.dirtyEntities, entity) ||
		uow.containsEntity(uow.removedEntities, entity)
}

// registerDirtyEntityInternal 内部注册脏实体方法（不加锁）
func (uow *UnitOfWork) registerDirtyEntityInternal(entity Entity, changes map[string]FieldChange) error {
	if uow.isCommitted || uow.isRolledBack {
		return fmt.Errorf("unit of work is already finished")
	}

	if entity == nil {
		return fmt.Errorf("entity cannot be nil")
	}

	entityType := reflect.TypeOf(entity)

	// 如果是新实体，不需要标记为脏
	if uow.containsEntity(uow.newEntities, entity) {
		return nil
	}

	// 检查是否已经标记为删除
	if uow.containsEntity(uow.removedEntities, entity) {
		return fmt.Errorf("cannot mark removed entity as dirty")
	}

	// 检查是否已经标记为脏
	if uow.containsEntity(uow.dirtyEntities, entity) {
		return nil
	}

	// 添加到脏实体列表
	uow.dirtyEntities[entityType] = append(uow.dirtyEntities[entityType], entity)

	// 添加操作
	operation := NewUpdateOperation(entity, changes)
	uow.addOperation(operation)

	return nil
}

// optimizeOperations 优化操作序列
func (uow *UnitOfWork) optimizeOperations() []Operation {
	if !uow.config.EnableOperationMerge {
		return uow.operations
	}

	// 移除无效操作（插入后立即删除的实体）
	operations := uow.removeInvalidOperations()

	// 合并同类型操作
	operations = uow.mergeOperations(operations)

	return operations
}

// removeInvalidOperations 移除无效操作
func (uow *UnitOfWork) removeInvalidOperations() []Operation {
	validOps := make([]Operation, 0, len(uow.operations))
	canceledOps := make(map[string]bool)

	// 找出可以抵消的操作对
	for i, op1 := range uow.operations {
		if canceledOps[fmt.Sprintf("%d", i)] {
			continue
		}

		for j, op2 := range uow.operations {
			if i >= j || canceledOps[fmt.Sprintf("%d", j)] {
				continue
			}

			// 检查是否是同一个实体的插入和删除操作
			if op1.GetOperationType() == OperationTypeInsert &&
				op2.GetOperationType() == OperationTypeDelete &&
				op1.SameIdentity(op2) {
				canceledOps[fmt.Sprintf("%d", i)] = true
				canceledOps[fmt.Sprintf("%d", j)] = true

				if uow.config.EnableDetailLog {
					zlogger.Debug().
						Str("entity_type", op1.GetEntityType().String()).
						Interface("entity_id", op1.GetEntity().GetID()).
						Msg("Canceled insert-delete operation pair")
				}
				break
			}
		}
	}

	// 保留有效操作
	for i, op := range uow.operations {
		if !canceledOps[fmt.Sprintf("%d", i)] {
			validOps = append(validOps, op)
		}
	}

	return validOps
}

// mergeOperations 合并操作
func (uow *UnitOfWork) mergeOperations(operations []Operation) []Operation {
	if len(operations) <= 1 {
		return operations
	}

	merged := make([]Operation, 0, len(operations))
	processed := make(map[int]bool)

	for i, op1 := range operations {
		if processed[i] {
			continue
		}

		currentOp := op1
		processed[i] = true

		// 尝试与后续操作合并
		for j := i + 1; j < len(operations); j++ {
			if processed[j] {
				continue
			}

			op2 := operations[j]
			if currentOp.CanMerge(op2) {
				currentOp = currentOp.Merge(op2)
				processed[j] = true

				if uow.config.EnableDetailLog {
					zlogger.Debug().
						Str("operation_type", currentOp.GetOperationType().String()).
						Str("entity_type", currentOp.GetEntityType().String()).
						Msg("Merged operations")
				}
			}
		}

		merged = append(merged, currentOp)
	}

	return merged
}

// sortOperationsByDependency 按依赖关系排序操作
func (uow *UnitOfWork) sortOperationsByDependency(operations []Operation) ([]Operation, error) {
	if len(operations) <= 1 {
		return operations, nil
	}

	// 按操作类型分组
	insertOps := make([]Operation, 0)
	updateOps := make([]Operation, 0)
	deleteOps := make([]Operation, 0)

	for _, op := range operations {
		switch op.GetOperationType() {
		case OperationTypeInsert, OperationTypeBulkInsert:
			insertOps = append(insertOps, op)
		case OperationTypeUpdate, OperationTypeBulkUpdate:
			updateOps = append(updateOps, op)
		case OperationTypeDelete, OperationTypeBulkDelete:
			deleteOps = append(deleteOps, op)
		}
	}

	// 对每类操作内部排序
	sortedInserts, err := uow.sortOperationsByEntityDependency(insertOps, false)
	if err != nil {
		return nil, err
	}

	// 更新操作可以并行，无需特殊排序
	sortedUpdates := updateOps

	sortedDeletes, err := uow.sortOperationsByEntityDependency(deleteOps, true)
	if err != nil {
		return nil, err
	}

	// 合并所有排序后的操作
	result := make([]Operation, 0, len(operations))
	result = append(result, sortedInserts...)
	result = append(result, sortedUpdates...)
	result = append(result, sortedDeletes...)

	return result, nil
}

// sortOperationsByEntityDependency 按实体依赖关系排序操作
func (uow *UnitOfWork) sortOperationsByEntityDependency(operations []Operation, reverse bool) ([]Operation, error) {
	if len(operations) <= 1 {
		return operations, nil
	}

	// 提取实体
	entities := make([]Entity, 0, len(operations))
	opMap := make(map[string]Operation)

	for _, op := range operations {
		entity := op.GetEntity()
		if entity != nil {
			entities = append(entities, entity)
			key := fmt.Sprintf("%s#%v", reflect.TypeOf(entity).String(), entity.GetID())
			opMap[key] = op
		}
	}

	// 按依赖关系排序实体
	var sortedEntities []Entity
	var err error

	if reverse {
		sortedEntities, err = uow.dependencyManager.GetDeletionOrder(entities)
	} else {
		sortedEntities, err = uow.dependencyManager.GetInsertionOrder(entities)
	}

	if err != nil {
		return nil, err
	}

	// 重新组织操作
	sortedOps := make([]Operation, 0, len(operations))
	for _, entity := range sortedEntities {
		key := fmt.Sprintf("%s#%v", reflect.TypeOf(entity).String(), entity.GetID())
		if op, exists := opMap[key]; exists {
			sortedOps = append(sortedOps, op)
		}
	}

	return sortedOps, nil
}

// 辅助方法

func (uow *UnitOfWork) containsEntity(entityMap map[reflect.Type][]Entity, entity Entity) bool {
	entityType := reflect.TypeOf(entity)
	entities, exists := entityMap[entityType]
	if !exists {
		return false
	}

	for _, e := range entities {
		if e.GetID() == entity.GetID() {
			return true
		}
	}

	return false
}

func (uow *UnitOfWork) removeFromEntityList(entityMap map[reflect.Type][]Entity, entity Entity) bool {
	entityType := reflect.TypeOf(entity)
	entities, exists := entityMap[entityType]
	if !exists {
		return false
	}

	for i, e := range entities {
		if e.GetID() == entity.GetID() {
			// 移除实体
			entityMap[entityType] = append(entities[:i], entities[i+1:]...)
			return true
		}
	}

	return false
}

func (uow *UnitOfWork) removeOperationByEntity(entity Entity) {
	newOps := make([]Operation, 0, len(uow.operations))
	for _, op := range uow.operations {
		if !op.SameIdentity(NewInsertOperation(entity, uow)) &&
			!op.SameIdentity(NewUpdateOperation(entity, nil)) &&
			!op.SameIdentity(NewDeleteOperation(entity)) {
			newOps = append(newOps, op)
		}
	}
	uow.operations = newOps
}

func (uow *UnitOfWork) addOperation(operation Operation) {
	uow.operations = append(uow.operations, operation)
}

func (uow *UnitOfWork) checkMemoryLimit() error {
	totalCount := uow.getTotalEntityCount()
	if totalCount >= uow.config.MaxEntityCount {
		return fmt.Errorf("entity count limit exceeded: %d >= %d", totalCount, uow.config.MaxEntityCount)
	}
	return nil
}

func (uow *UnitOfWork) getTotalEntityCount() int {
	return uow.getEntityCountByType(uow.newEntities) +
		uow.getEntityCountByType(uow.dirtyEntities) +
		uow.getEntityCountByType(uow.removedEntities)
}

func (uow *UnitOfWork) getEntityCountByType(entityMap map[reflect.Type][]Entity) int {
	count := 0
	for _, entities := range entityMap {
		count += len(entities)
	}
	return count
}

func (uow *UnitOfWork) clear() {
	uow.newEntities = make(map[reflect.Type][]Entity)
	uow.dirtyEntities = make(map[reflect.Type][]Entity)
	uow.removedEntities = make(map[reflect.Type][]Entity)
	uow.cleanEntities = make(map[reflect.Type][]Entity)
	uow.operations = make([]Operation, 0)
	uow.snapshotManager.Clear()
}

// IsCommitted 检查是否已提交
func (uow *UnitOfWork) IsCommitted() bool {
	uow.mu.RLock()
	defer uow.mu.RUnlock()
	return uow.isCommitted
}

// IsRolledBack 检查是否已回滚
func (uow *UnitOfWork) IsRolledBack() bool {
	uow.mu.RLock()
	defer uow.mu.RUnlock()
	return uow.isRolledBack
}

// GetDependencyManager 获取依赖管理器
func (uow *UnitOfWork) GetDependencyManager() *DependencyManager {
	return uow.dependencyManager
}

// GetStats 获取统计信息
func (uow *UnitOfWork) GetStats() map[string]interface{} {
	uow.mu.RLock()
	defer uow.mu.RUnlock()

	return map[string]interface{}{
		"new_entities":     uow.getEntityCountByType(uow.newEntities),
		"dirty_entities":   uow.getEntityCountByType(uow.dirtyEntities),
		"removed_entities": uow.getEntityCountByType(uow.removedEntities),
		"total_operations": len(uow.operations),
		"is_committed":     uow.isCommitted,
		"is_rolled_back":   uow.isRolledBack,
	}
}
