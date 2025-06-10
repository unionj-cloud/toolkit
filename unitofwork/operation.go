package unitofwork

import (
	"fmt"
	"reflect"
	"time"

	"github.com/wubin1989/gorm"
)

// Operation 数据库操作接口
type Operation interface {
	// GetEntityType 获取操作的实体类型
	GetEntityType() reflect.Type

	// GetOperationType 获取操作类型
	GetOperationType() OperationType

	// Execute 执行操作
	Execute(db *gorm.DB) error

	// GetEntity 获取操作的实体（如果有）
	GetEntity() Entity

	// SameIdentity 检查是否是同一个实体的操作
	SameIdentity(other Operation) bool

	// CanMerge 检查是否可以与其他操作合并
	CanMerge(other Operation) bool

	// Merge 与其他操作合并
	Merge(other Operation) Operation
}

// OperationType 操作类型
type OperationType int

const (
	OperationTypeInsert OperationType = iota
	OperationTypeUpdate
	OperationTypeDelete
	OperationTypeBulkInsert
	OperationTypeBulkUpdate
	OperationTypeBulkDelete
)

// String 返回操作类型的字符串表示
func (t OperationType) String() string {
	switch t {
	case OperationTypeInsert:
		return "INSERT"
	case OperationTypeUpdate:
		return "UPDATE"
	case OperationTypeDelete:
		return "DELETE"
	case OperationTypeBulkInsert:
		return "BULK_INSERT"
	case OperationTypeBulkUpdate:
		return "BULK_UPDATE"
	case OperationTypeBulkDelete:
		return "BULK_DELETE"
	default:
		return "UNKNOWN"
	}
}

// InsertOperation 插入操作
type InsertOperation struct {
	entity Entity
	uow    *UnitOfWork
}

// NewInsertOperation 创建插入操作
func NewInsertOperation(entity Entity, uow *UnitOfWork) *InsertOperation {
	return &InsertOperation{
		entity: entity,
		uow:    uow,
	}
}

// GetEntityType 实现Operation接口
func (op *InsertOperation) GetEntityType() reflect.Type {
	return reflect.TypeOf(op.entity)
}

// GetOperationType 实现Operation接口
func (op *InsertOperation) GetOperationType() OperationType {
	return OperationTypeInsert
}

// Execute 实现Operation接口
func (op *InsertOperation) Execute(db *gorm.DB) error {
	// 验证实体
	if validatable, ok := op.entity.(Validatable); ok {
		if err := validatable.Validate(); err != nil {
			return fmt.Errorf("validation failed for entity %T: %w", op.entity, err)
		}
	}

	// 设置时间戳
	if timestamped, ok := op.entity.(HasTimestamps); ok && op.entity.IsNew() {
		now := time.Now()
		timestamped.SetCreatedAt(now)
		timestamped.SetUpdatedAt(now)
	}

	// 执行插入 - 直接使用原始数据库连接
	result := db.Create(op.entity)
	if result.Error != nil {
		return fmt.Errorf("failed to insert entity %T: %w", op.entity, result.Error)
	}

	return nil
}

// GetEntity 实现Operation接口
func (op *InsertOperation) GetEntity() Entity {
	return op.entity
}

// SameIdentity 实现Operation接口
func (op *InsertOperation) SameIdentity(other Operation) bool {
	if other.GetOperationType() != OperationTypeInsert {
		return false
	}

	otherEntity := other.GetEntity()
	if otherEntity == nil {
		return false
	}

	return op.entity.GetID() == otherEntity.GetID() &&
		reflect.TypeOf(op.entity) == reflect.TypeOf(otherEntity)
}

// CanMerge 实现Operation接口
func (op *InsertOperation) CanMerge(other Operation) bool {
	return other.GetOperationType() == OperationTypeInsert &&
		op.GetEntityType() == other.GetEntityType()
}

// Merge 实现Operation接口
func (op *InsertOperation) Merge(other Operation) Operation {
	if !op.CanMerge(other) {
		return op
	}

	entities := []Entity{op.entity, other.GetEntity()}
	return NewBulkInsertOperation(entities, op)
}

// UpdateOperation 更新操作
type UpdateOperation struct {
	entity  Entity
	changes map[string]FieldChange
}

// NewUpdateOperation 创建更新操作
func NewUpdateOperation(entity Entity, changes map[string]FieldChange) *UpdateOperation {
	return &UpdateOperation{
		entity:  entity,
		changes: changes,
	}
}

// GetEntityType 实现Operation接口
func (op *UpdateOperation) GetEntityType() reflect.Type {
	return reflect.TypeOf(op.entity)
}

// GetOperationType 实现Operation接口
func (op *UpdateOperation) GetOperationType() OperationType {
	return OperationTypeUpdate
}

// Execute 实现Operation接口
func (op *UpdateOperation) Execute(db *gorm.DB) error {
	// 验证实体
	if validatable, ok := op.entity.(Validatable); ok {
		if err := validatable.Validate(); err != nil {
			return fmt.Errorf("validation failed for entity %T: %w", op.entity, err)
		}
	}

	// 设置更新时间戳
	if timestamped, ok := op.entity.(HasTimestamps); ok {
		timestamped.SetUpdatedAt(time.Now())
	}

	// 乐观锁处理
	if revisioned, ok := op.entity.(HasRevision); ok {
		originalRevision := revisioned.GetRevision()
		revisioned.SetRevision(revisioned.GetRevisionNext())

		result := db.Where("id = ? AND revision = ?", op.entity.GetID(), originalRevision).Updates(op.entity)

		if result.Error != nil {
			return fmt.Errorf("failed to update entity %T: %w", op.entity, result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("optimistic lock failed for entity %T with id %v", op.entity, op.entity.GetID())
		}
	} else {
		result := db.Save(op.entity)
		if result.Error != nil {
			return fmt.Errorf("failed to update entity %T: %w", op.entity, result.Error)
		}
	}

	return nil
}

// GetEntity 实现Operation接口
func (op *UpdateOperation) GetEntity() Entity {
	return op.entity
}

// SameIdentity 实现Operation接口
func (op *UpdateOperation) SameIdentity(other Operation) bool {
	if other.GetOperationType() != OperationTypeUpdate {
		return false
	}

	otherEntity := other.GetEntity()
	if otherEntity == nil {
		return false
	}

	return op.entity.GetID() == otherEntity.GetID() &&
		reflect.TypeOf(op.entity) == reflect.TypeOf(otherEntity)
}

// CanMerge 实现Operation接口
func (op *UpdateOperation) CanMerge(other Operation) bool {
	return other.GetOperationType() == OperationTypeUpdate &&
		op.GetEntityType() == other.GetEntityType()
}

// Merge 实现Operation接口
func (op *UpdateOperation) Merge(other Operation) Operation {
	if !op.CanMerge(other) {
		return op
	}

	entities := []Entity{op.entity, other.GetEntity()}
	return NewBulkUpdateOperation(entities)
}

// GetChanges 获取变更信息
func (op *UpdateOperation) GetChanges() map[string]FieldChange {
	return op.changes
}

// DeleteOperation 删除操作
type DeleteOperation struct {
	entity Entity
}

// NewDeleteOperation 创建删除操作
func NewDeleteOperation(entity Entity) *DeleteOperation {
	return &DeleteOperation{
		entity: entity,
	}
}

// GetEntityType 实现Operation接口
func (op *DeleteOperation) GetEntityType() reflect.Type {
	return reflect.TypeOf(op.entity)
}

// GetOperationType 实现Operation接口
func (op *DeleteOperation) GetOperationType() OperationType {
	return OperationTypeDelete
}

// Execute 实现Operation接口
func (op *DeleteOperation) Execute(db *gorm.DB) error {
	// 软删除处理
	if softDeletable, ok := op.entity.(SoftDelete); ok {
		now := time.Now()
		softDeletable.SetDeletedAt(gorm.DeletedAt{
			Time:  now,
			Valid: true,
		})

		// 设置更新时间戳
		if timestamped, ok := op.entity.(HasTimestamps); ok {
			timestamped.SetUpdatedAt(now)
		}

		result := db.Save(op.entity)
		if result.Error != nil {
			return fmt.Errorf("failed to soft delete entity %T: %w", op.entity, result.Error)
		}
	} else {
		// 硬删除
		result := db.Unscoped().Delete(op.entity)
		if result.Error != nil {
			return fmt.Errorf("failed to delete entity %T: %w", op.entity, result.Error)
		}
	}

	return nil
}

// GetEntity 实现Operation接口
func (op *DeleteOperation) GetEntity() Entity {
	return op.entity
}

// SameIdentity 实现Operation接口
func (op *DeleteOperation) SameIdentity(other Operation) bool {
	if other.GetOperationType() != OperationTypeDelete {
		return false
	}

	otherEntity := other.GetEntity()
	if otherEntity == nil {
		return false
	}

	return op.entity.GetID() == otherEntity.GetID() &&
		reflect.TypeOf(op.entity) == reflect.TypeOf(otherEntity)
}

// CanMerge 实现Operation接口
func (op *DeleteOperation) CanMerge(other Operation) bool {
	return other.GetOperationType() == OperationTypeDelete &&
		op.GetEntityType() == other.GetEntityType()
}

// Merge 实现Operation接口
func (op *DeleteOperation) Merge(other Operation) Operation {
	if !op.CanMerge(other) {
		return op
	}

	entities := []Entity{op.entity, other.GetEntity()}
	return NewBulkDeleteOperation(entities)
}

// BulkInsertOperation 批量插入操作
type BulkInsertOperation struct {
	entities        []Entity
	entityType      reflect.Type
	insertOperation *InsertOperation
}

// NewBulkInsertOperation 创建批量插入操作
func NewBulkInsertOperation(entities []Entity, insertOperation *InsertOperation) *BulkInsertOperation {
	var entityType reflect.Type
	if len(entities) > 0 {
		entityType = reflect.TypeOf(entities[0])
	}

	return &BulkInsertOperation{
		entities:        entities,
		entityType:      entityType,
		insertOperation: insertOperation,
	}
}

// GetEntityType 实现Operation接口
func (op *BulkInsertOperation) GetEntityType() reflect.Type {
	return op.entityType
}

// GetOperationType 实现Operation接口
func (op *BulkInsertOperation) GetOperationType() OperationType {
	return OperationTypeBulkInsert
}

func ConvertToTypedSlice(input []Entity, elemType reflect.Type) interface{} {
	// 创建新的切片类型：[]T
	sliceType := reflect.SliceOf(elemType)
	result := reflect.MakeSlice(sliceType, 0, len(input))

	for _, item := range input {
		v := reflect.ValueOf(item)

		// 类型检查：防止非法转换
		if !v.Type().AssignableTo(elemType) {
			panic(fmt.Sprintf("元素类型 %v 不能转换为 %v", v.Type(), elemType))
		}

		result = reflect.Append(result, v)
	}

	return result.Interface() // 返回 interface{}，你可以断言回来
}

// Execute 实现Operation接口
func (op *BulkInsertOperation) Execute(db *gorm.DB) error {
	if len(op.entities) == 0 {
		return nil
	}

	// 验证所有实体
	for _, entity := range op.entities {
		if validatable, ok := entity.(Validatable); ok {
			if err := validatable.Validate(); err != nil {
				return fmt.Errorf("validation failed for entity %T: %w", entity, err)
			}
		}

		// 设置时间戳
		if timestamped, ok := entity.(HasTimestamps); ok && entity.IsNew() {
			now := time.Now()
			timestamped.SetCreatedAt(now)
			timestamped.SetUpdatedAt(now)
		}
	}

	// 批量插入
	typedSlice := ConvertToTypedSlice(op.entities, op.GetEntityType())
	result := db.CreateInBatches(typedSlice, op.insertOperation.uow.config.BatchSize)
	if result.Error != nil {
		return fmt.Errorf("failed to bulk insert entities %T: %w", op.entityType, result.Error)
	}

	return nil
}

// GetEntity 实现Operation接口
func (op *BulkInsertOperation) GetEntity() Entity {
	if len(op.entities) > 0 {
		return op.entities[0]
	}
	return nil
}

// SameIdentity 实现Operation接口
func (op *BulkInsertOperation) SameIdentity(other Operation) bool {
	return false // 批量操作不参与身份比较
}

// CanMerge 实现Operation接口
func (op *BulkInsertOperation) CanMerge(other Operation) bool {
	return (other.GetOperationType() == OperationTypeInsert ||
		other.GetOperationType() == OperationTypeBulkInsert) &&
		op.GetEntityType() == other.GetEntityType()
}

// Merge 实现Operation接口
func (op *BulkInsertOperation) Merge(other Operation) Operation {
	if !op.CanMerge(other) {
		return op
	}

	var allEntities []Entity
	allEntities = append(allEntities, op.entities...)

	if other.GetOperationType() == OperationTypeInsert {
		allEntities = append(allEntities, other.GetEntity())
	} else if bulkOther, ok := other.(*BulkInsertOperation); ok {
		allEntities = append(allEntities, bulkOther.entities...)
	}

	return NewBulkInsertOperation(allEntities, op.insertOperation)
}

// GetEntities 获取所有实体
func (op *BulkInsertOperation) GetEntities() []Entity {
	return op.entities
}

// BulkUpdateOperation 批量更新操作
type BulkUpdateOperation struct {
	entities   []Entity
	entityType reflect.Type
}

// NewBulkUpdateOperation 创建批量更新操作
func NewBulkUpdateOperation(entities []Entity) *BulkUpdateOperation {
	var entityType reflect.Type
	if len(entities) > 0 {
		entityType = reflect.TypeOf(entities[0])
	}

	return &BulkUpdateOperation{
		entities:   entities,
		entityType: entityType,
	}
}

// GetEntityType 实现Operation接口
func (op *BulkUpdateOperation) GetEntityType() reflect.Type {
	return op.entityType
}

// GetOperationType 实现Operation接口
func (op *BulkUpdateOperation) GetOperationType() OperationType {
	return OperationTypeBulkUpdate
}

// Execute 实现Operation接口
func (op *BulkUpdateOperation) Execute(db *gorm.DB) error {
	if len(op.entities) == 0 {
		return nil
	}

	// 逐个更新（保持乐观锁特性）
	for _, entity := range op.entities {
		updateOp := NewUpdateOperation(entity, nil)
		if err := updateOp.Execute(db); err != nil {
			return err
		}
	}

	return nil
}

// GetEntity 实现Operation接口
func (op *BulkUpdateOperation) GetEntity() Entity {
	if len(op.entities) > 0 {
		return op.entities[0]
	}
	return nil
}

// SameIdentity 实现Operation接口
func (op *BulkUpdateOperation) SameIdentity(other Operation) bool {
	return false // 批量操作不参与身份比较
}

// CanMerge 实现Operation接口
func (op *BulkUpdateOperation) CanMerge(other Operation) bool {
	return (other.GetOperationType() == OperationTypeUpdate ||
		other.GetOperationType() == OperationTypeBulkUpdate) &&
		op.GetEntityType() == other.GetEntityType()
}

// Merge 实现Operation接口
func (op *BulkUpdateOperation) Merge(other Operation) Operation {
	if !op.CanMerge(other) {
		return op
	}

	var allEntities []Entity
	allEntities = append(allEntities, op.entities...)

	if other.GetOperationType() == OperationTypeUpdate {
		allEntities = append(allEntities, other.GetEntity())
	} else if bulkOther, ok := other.(*BulkUpdateOperation); ok {
		allEntities = append(allEntities, bulkOther.entities...)
	}

	return NewBulkUpdateOperation(allEntities)
}

// GetEntities 获取所有实体
func (op *BulkUpdateOperation) GetEntities() []Entity {
	return op.entities
}

// BulkDeleteOperation 批量删除操作
type BulkDeleteOperation struct {
	entities   []Entity
	entityType reflect.Type
}

// NewBulkDeleteOperation 创建批量删除操作
func NewBulkDeleteOperation(entities []Entity) *BulkDeleteOperation {
	var entityType reflect.Type
	if len(entities) > 0 {
		entityType = reflect.TypeOf(entities[0])
	}

	return &BulkDeleteOperation{
		entities:   entities,
		entityType: entityType,
	}
}

// GetEntityType 实现Operation接口
func (op *BulkDeleteOperation) GetEntityType() reflect.Type {
	return op.entityType
}

// GetOperationType 实现Operation接口
func (op *BulkDeleteOperation) GetOperationType() OperationType {
	return OperationTypeBulkDelete
}

// Execute 实现Operation接口
func (op *BulkDeleteOperation) Execute(db *gorm.DB) error {
	if len(op.entities) == 0 {
		return nil
	}

	// 检查是否支持软删除
	var hasSoftDelete bool
	if len(op.entities) > 0 {
		_, hasSoftDelete = op.entities[0].(SoftDelete)
	}

	if hasSoftDelete {
		// 软删除：逐个处理
		for _, entity := range op.entities {
			deleteOp := NewDeleteOperation(entity)
			if err := deleteOp.Execute(db); err != nil {
				return err
			}
		}
	} else {
		// 硬删除：可以批量处理
		result := db.Delete(op.entities)
		if result.Error != nil {
			return fmt.Errorf("failed to bulk delete entities %T: %w", op.entityType, result.Error)
		}
	}

	return nil
}

// GetEntity 实现Operation接口
func (op *BulkDeleteOperation) GetEntity() Entity {
	if len(op.entities) > 0 {
		return op.entities[0]
	}
	return nil
}

// SameIdentity 实现Operation接口
func (op *BulkDeleteOperation) SameIdentity(other Operation) bool {
	return false // 批量操作不参与身份比较
}

// CanMerge 实现Operation接口
func (op *BulkDeleteOperation) CanMerge(other Operation) bool {
	return (other.GetOperationType() == OperationTypeDelete ||
		other.GetOperationType() == OperationTypeBulkDelete) &&
		op.GetEntityType() == other.GetEntityType()
}

// Merge 实现Operation接口
func (op *BulkDeleteOperation) Merge(other Operation) Operation {
	if !op.CanMerge(other) {
		return op
	}

	var allEntities []Entity
	allEntities = append(allEntities, op.entities...)

	if other.GetOperationType() == OperationTypeDelete {
		allEntities = append(allEntities, other.GetEntity())
	} else if bulkOther, ok := other.(*BulkDeleteOperation); ok {
		allEntities = append(allEntities, bulkOther.entities...)
	}

	return NewBulkDeleteOperation(allEntities)
}

// GetEntities 获取所有实体
func (op *BulkDeleteOperation) GetEntities() []Entity {
	return op.entities
}
