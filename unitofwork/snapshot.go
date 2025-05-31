package unitofwork

import (
	"reflect"
	"time"

	"github.com/unionj-cloud/toolkit/copier"
)

// EntitySnapshot 实体状态快照，用于脏检查
type EntitySnapshot struct {
	entityType   reflect.Type
	entityID     interface{}
	fieldValues  map[string]interface{}
	snapshotTime time.Time
}

// NewEntitySnapshot 创建实体状态快照
func NewEntitySnapshot(entity Entity) *EntitySnapshot {
	fieldValues := extractFieldValues(entity)

	return &EntitySnapshot{
		entityType:   reflect.TypeOf(entity),
		entityID:     entity.GetID(),
		fieldValues:  fieldValues,
		snapshotTime: time.Now(),
	}
}

// IsDirty 检查实体是否发生变更
func (s *EntitySnapshot) IsDirty(entity Entity) bool {
	if s.entityType != reflect.TypeOf(entity) {
		return true // 类型不匹配，认为是脏的
	}

	if s.entityID != entity.GetID() {
		return true // ID不匹配，认为是脏的
	}

	currentValues := extractFieldValues(entity)

	// 比较字段值
	for fieldName, originalValue := range s.fieldValues {
		currentValue, exists := currentValues[fieldName]
		if !exists {
			return true // 字段不存在，认为是脏的
		}

		if !deepEqual(originalValue, currentValue) {
			return true // 字段值不同，认为是脏的
		}
	}

	// 检查是否有新增字段
	for fieldName := range currentValues {
		if _, exists := s.fieldValues[fieldName]; !exists {
			return true // 有新增字段，认为是脏的
		}
	}

	return false
}

// GetChangedFields 获取发生变更的字段
func (s *EntitySnapshot) GetChangedFields(entity Entity) map[string]FieldChange {
	changes := make(map[string]FieldChange)

	if s.entityType != reflect.TypeOf(entity) {
		return changes
	}

	currentValues := extractFieldValues(entity)

	// 检查修改和删除的字段
	for fieldName, originalValue := range s.fieldValues {
		currentValue, exists := currentValues[fieldName]
		if !exists {
			changes[fieldName] = FieldChange{
				FieldName: fieldName,
				OldValue:  originalValue,
				NewValue:  nil,
				Type:      FieldChangeTypeDeleted,
			}
		} else if !deepEqual(originalValue, currentValue) {
			changes[fieldName] = FieldChange{
				FieldName: fieldName,
				OldValue:  originalValue,
				NewValue:  currentValue,
				Type:      FieldChangeTypeModified,
			}
		}
	}

	// 检查新增字段
	for fieldName, currentValue := range currentValues {
		if _, exists := s.fieldValues[fieldName]; !exists {
			changes[fieldName] = FieldChange{
				FieldName: fieldName,
				OldValue:  nil,
				NewValue:  currentValue,
				Type:      FieldChangeTypeAdded,
			}
		}
	}

	return changes
}

// FieldChange 字段变更信息
type FieldChange struct {
	FieldName string
	OldValue  interface{}
	NewValue  interface{}
	Type      FieldChangeType
}

// FieldChangeType 字段变更类型
type FieldChangeType int

const (
	FieldChangeTypeAdded FieldChangeType = iota
	FieldChangeTypeModified
	FieldChangeTypeDeleted
)

// String 返回字段变更类型的字符串表示
func (t FieldChangeType) String() string {
	switch t {
	case FieldChangeTypeAdded:
		return "ADDED"
	case FieldChangeTypeModified:
		return "MODIFIED"
	case FieldChangeTypeDeleted:
		return "DELETED"
	default:
		return "UNKNOWN"
	}
}

// extractFieldValues 提取实体的所有字段值
func extractFieldValues(entity Entity) map[string]interface{} {
	fieldValues := make(map[string]interface{})

	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	if !entityValue.IsValid() || entityValue.Kind() != reflect.Struct {
		return fieldValues
	}

	entityType := entityValue.Type()

	// 遍历所有字段
	for i := 0; i < entityValue.NumField(); i++ {
		field := entityValue.Field(i)
		fieldType := entityType.Field(i)

		// 跳过非导出字段
		if !field.CanInterface() {
			continue
		}

		// 跳过静态字段
		if fieldType.Tag.Get("unitofwork") == "ignore" {
			continue
		}

		fieldName := fieldType.Name
		fieldValue := field.Interface()

		// 深拷贝复杂类型
		if needDeepCopy(field.Type()) {
			var copiedValue interface{}
			if err := copier.DeepCopy(fieldValue, &copiedValue); err == nil {
				fieldValues[fieldName] = copiedValue
			} else {
				fieldValues[fieldName] = fieldValue
			}
		} else {
			fieldValues[fieldName] = fieldValue
		}
	}

	return fieldValues
}

// needDeepCopy 判断是否需要深拷贝
func needDeepCopy(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Ptr:
		return true
	case reflect.Struct:
		// 时间类型不需要深拷贝
		if t == reflect.TypeOf(time.Time{}) {
			return false
		}
		return true
	default:
		return false
	}
}

// deepEqual 深度比较两个值是否相等
func deepEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)

	if va.Type() != vb.Type() {
		return false
	}

	return reflect.DeepEqual(a, b)
}

// SnapshotManager 快照管理器
type SnapshotManager struct {
	snapshots map[string]*EntitySnapshot
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]*EntitySnapshot),
	}
}

// TakeSnapshot 创建实体快照
func (sm *SnapshotManager) TakeSnapshot(entity Entity) {
	key := sm.buildKey(entity)
	sm.snapshots[key] = NewEntitySnapshot(entity)
}

// IsDirty 检查实体是否脏
func (sm *SnapshotManager) IsDirty(entity Entity) bool {
	key := sm.buildKey(entity)
	snapshot, exists := sm.snapshots[key]
	if !exists {
		return false // 没有快照，认为不是脏的
	}

	return snapshot.IsDirty(entity)
}

// GetChangedFields 获取实体变更字段
func (sm *SnapshotManager) GetChangedFields(entity Entity) map[string]FieldChange {
	key := sm.buildKey(entity)
	snapshot, exists := sm.snapshots[key]
	if !exists {
		return make(map[string]FieldChange)
	}

	return snapshot.GetChangedFields(entity)
}

// RemoveSnapshot 移除实体快照
func (sm *SnapshotManager) RemoveSnapshot(entity Entity) {
	key := sm.buildKey(entity)
	delete(sm.snapshots, key)
}

// Clear 清空所有快照
func (sm *SnapshotManager) Clear() {
	sm.snapshots = make(map[string]*EntitySnapshot)
}

// buildKey 构建实体唯一键
func (sm *SnapshotManager) buildKey(entity Entity) string {
	entityType := reflect.TypeOf(entity).String()
	entityID := entity.GetID()
	return entityType + "#" + toString(entityID)
}

// toString 将任意类型转换为字符串
func toString(v interface{}) string {
	if v == nil {
		return "nil"
	}

	return reflect.ValueOf(v).String()
}
